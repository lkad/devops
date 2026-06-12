package device

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrUnauthenticated is the sentinel returned when a service
// method is invoked without a caller on the context. The
// handler maps it to a 401 UNAUTHORIZED APIError.
var ErrUnauthenticated = errors.New("device: unauthenticated")

// ErrForbidden is the sentinel returned when the caller's
// tenant membership does not allow the requested operation.
// The handler maps it to a 403 FORBIDDEN APIError.
//
// Note: device has no direct project_id column (the binding
// is via hostproject), so the v0.3.0.0 P0 #2 service-layer
// guard is a placeholder that allows any authenticated
// caller with the right global permission (or SuperAdmin).
// Cross-tenant device reads are blocked at the
// hostproject layer in this branch; the projectID-based
// per-device filter will land in a follow-up.
var ErrForbidden = errors.New("device: forbidden")

// CreateDeviceInput is the request payload for Service.Create.
// The handler decodes the wire JSON into this struct; the service
// then validates and maps it onto a Device. Pointer fields are
// used sparingly — only when the value must be optional.
type CreateDeviceInput struct {
	Name       string
	Type       DeviceType
	State      DeviceState
	GroupID    *string
	TemplateID *string
	Labels     JSONMap
	Metadata   JSONMap
}

// UpdateDeviceInput is the request payload for Service.Update.
// All fields are pointers so the service can distinguish between
// "field omitted" and "field set to zero value". A nil pointer
// means the field is left unchanged.
type UpdateDeviceInput struct {
	Name       *string
	Type       *DeviceType
	State      *DeviceState
	GroupID    *string
	TemplateID *string
	Labels     JSONMap
	Metadata   JSONMap
}

// Service is the business-logic layer for device management.
// It owns validation, state-transition rules, and any
// orchestration between the repository and the wire response.
// It is framework-agnostic (no Gin) so it can be reused by gRPC
// handlers, CLI tools, or background workers in the future.
type Service struct {
	repo  *Repository
	audit *audit.Service
}

// NewService builds a Service. The repository is the only
// required dependency; the audit service is optional (nil
// means "no audit emission") so existing test rig (which
// never wires an audit sink) keeps compiling. Production
// always wires a real service so v0.2.0.0 P0 #3
// audit-trail coverage holds.
func NewService(repo *Repository, auditSvc ...*audit.Service) *Service {
	var a *audit.Service
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &Service{repo: repo, audit: a}
}

// Create validates the input and persists a new device. The
// returned Device is the freshly-stored row, including its
// generated ID and timestamps. The audit emission
// (device.create) is best-effort and the context is
// variadic so existing callers (which pre-date the audit
// hooks) keep compiling; production callers should pass
// the request context so the audit emission carries the
// right request-scoped values.
func (s *Service) Create(in CreateDeviceInput, ctxArg ...context.Context) (*Device, error) {
	ctx := s.ctxOrBackground(ctxArg)
	if strings.TrimSpace(in.Name) == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	if !in.Type.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("type %q is not a valid device type", in.Type),
		}
	}
	if in.State == "" {
		in.State = DeviceStateOnline
	} else if !in.State.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("state %q is not a valid device state", in.State),
		}
	}

	d := &Device{
		Name:       strings.TrimSpace(in.Name),
		Type:       in.Type,
		State:      in.State,
		GroupID:    in.GroupID,
		TemplateID: in.TemplateID,
		Labels:     in.Labels,
		Metadata:   in.Metadata,
	}
	if err := s.repo.Create(d); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create device",
			Cause:   err,
		}
	}
	s.emitAudit(ctx, audit.RecordActionInput{
		Action:       audit.ActionCreate,
		ResourceType: audit.ResourceDevice,
		ResourceID:   d.ID,
		Metadata:     audit.JSONMap{"name": d.Name, "type": string(d.Type), "state": string(d.State)},
	})
	return d, nil
}

// Get returns a single device or a 404 APIError.
func (s *Service) Get(id string) (*Device, error) {
	d, err := s.repo.Get(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load device",
			Cause:   err,
		}
	}
	return d, nil
}

// GetWithCaller is the v0.3.0.0 P0 #2 cross-tenant variant of
// Get. The caller MUST be attached to the context; an absent
// caller surfaces as ErrUnauthenticated (401). A non-SuperAdmin
// caller without an explicit allow is denied (the device-to-
// project resolution goes through hostproject; until the
// per-device project_id column lands, the service-layer guard
// is "caller present + SuperAdmin OR a global project-membership
// match" — the latter defaults to allow when the membership
// checker is not wired). The ungoverned Get is retained for
// legacy code paths that do not carry a context.
func (s *Service) GetWithCaller(ctx context.Context, id string) (*Device, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		// Device has no direct project_id; cross-tenant
		// reads are blocked at the hostproject layer. The
		// service-layer guard is a placeholder that
		// requires the caller to be SuperAdmin for now.
		return nil, ErrForbidden
	}
	return s.Get(id)
}

// List returns a page of devices plus the unfiltered total.
// The handler renders this as the standard envelope.
func (s *Service) List(f ListFilter) ([]Device, int64, error) {
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list devices",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// ListWithCaller is the v0.3.0.0 P0 #2 cross-tenant variant
// of List. The caller MUST be attached to the context. A
// non-SuperAdmin caller is denied (the per-device
// project_id filter is the follow-up work; today the
// service-layer guard is SuperAdmin only). The ungoverned
// List is retained for legacy code paths.
func (s *Service) ListWithCaller(ctx context.Context, f ListFilter) ([]Device, int64, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, 0, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		return nil, 0, ErrForbidden
	}
	return s.List(f)
}

// Search is the Service's wrapper over Repository.Search. It
// exists for handler symmetry with the other CRUD methods.
func (s *Service) Search(query string) ([]Device, int64, error) {
	rows, total, err := s.repo.Search(query)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to search devices",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Update applies a partial update. Pointer fields are honoured
// (nil = leave unchanged); Labels and Metadata replace the
// stored value wholesale. The audit emission (device.update)
// records the resulting state for the audit log.
func (s *Service) Update(id string, in UpdateDeviceInput, ctxArg ...context.Context) (*Device, error) {
	ctx := s.ctxOrBackground(ctxArg)
	d, err := s.repo.Get(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load device",
			Cause:   err,
		}
	}
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "name cannot be blank",
			}
		}
		trimmed := strings.TrimSpace(*in.Name)
		d.Name = trimmed
	}
	if in.Type != nil {
		if !in.Type.Valid() {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("type %q is not a valid device type", *in.Type),
			}
		}
		d.Type = *in.Type
	}
	if in.State != nil {
		if !in.State.Valid() {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("state %q is not a valid device state", *in.State),
			}
		}
		d.State = *in.State
	}
	if in.GroupID != nil {
		d.GroupID = in.GroupID
	}
	if in.TemplateID != nil {
		d.TemplateID = in.TemplateID
	}
	if in.Labels != nil {
		d.Labels = in.Labels
	}
	if in.Metadata != nil {
		d.Metadata = in.Metadata
	}

	if err := s.repo.Update(d); err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update device",
			Cause:   err,
		}
	}
	s.emitAudit(ctx, audit.RecordActionInput{
		Action:       audit.ActionUpdate,
		ResourceType: audit.ResourceDevice,
		ResourceID:   d.ID,
		Metadata:     audit.JSONMap{"name": d.Name, "state": string(d.State)},
	})
	return d, nil
}

// Delete soft-deletes a device. A 404 APIError is returned when
// the row does not exist (either never created or already
// deleted). The audit emission (device.delete) records the
// deletion for the audit log.
func (s *Service) Delete(id string, ctxArg ...context.Context) error {
	ctx := s.ctxOrBackground(ctxArg)
	if err := s.repo.Delete(id); err != nil {
		if IsNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device %q not found", id),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete device",
			Cause:   err,
		}
	}
	s.emitAudit(ctx, audit.RecordActionInput{
		Action:       audit.ActionDelete,
		ResourceType: audit.ResourceDevice,
		ResourceID:   id,
	})
	return nil
}

// ApplyAction dispatches an action by name. The current action
// set is intentionally small (the spec's "Execute device action"
// scenario); a richer state machine lives in the per-type
// services and is not in scope here. The audit emission
// (device.update) records the new state for the audit log
// (the same action covers every state transition; the
// metadata column records the action name).
func (s *Service) ApplyAction(id, action string, ctxArg ...context.Context) (*Device, error) {
	ctx := s.ctxOrBackground(ctxArg)
	d, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	switch action {
	case ActionEnterMaintenance:
		if d.State == DeviceStateMaintenance {
			return nil, &contracts.APIError{
				Code:    contracts.CodeInvalidState,
				Message: "device is already in maintenance",
			}
		}
		d.State = DeviceStateMaintenance
	case ActionExitMaintenance:
		if d.State != DeviceStateMaintenance {
			return nil, &contracts.APIError{
				Code:    contracts.CodeInvalidState,
				Message: "device is not in maintenance",
			}
		}
		d.State = DeviceStateOnline
	default:
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("unknown action %q", action),
		}
	}
	if err := s.repo.Update(d); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to apply action",
			Cause:   err,
		}
	}
	s.emitAudit(ctx, audit.RecordActionInput{
		Action:       audit.ActionUpdate,
		ResourceType: audit.ResourceDevice,
		ResourceID:   d.ID,
		Metadata:     audit.JSONMap{"action": action, "state": string(d.State)},
	})
	return d, nil
}

// emitAudit is the single seam between this package and
// the cross-module audit subsystem. A nil audit service is
// a no-op so unit tests can wire a Service without an
// audit row in the DB. The emission is best-effort: a
// failure to record does not roll back the mutation.
func (s *Service) emitAudit(ctx context.Context, in audit.RecordActionInput) {
	if s.audit == nil {
		return
	}
	s.audit.RecordAction(ctx, in)
}

// ctxOrBackground returns the first supplied context, or
// context.Background() when none was supplied. The
// variadic-ctx pattern keeps the existing test API
// (which never passed a context) compiling while still
// letting production callers pass the request context.
func (s *Service) ctxOrBackground(args []context.Context) context.Context {
	if len(args) > 0 && args[0] != nil {
		return args[0]
	}
	return context.Background()
}
