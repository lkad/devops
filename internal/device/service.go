package device

import (
	"fmt"
	"strings"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

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
	repo *Repository
}

// NewService builds a Service. The repository is the only
// dependency; everything else (clocks, label normalizers, etc.)
// can be injected later by extending the constructor.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create validates the input and persists a new device. The
// returned Device is the freshly-stored row, including its
// generated ID and timestamps.
func (s *Service) Create(in CreateDeviceInput) (*Device, error) {
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
// stored value wholesale.
func (s *Service) Update(id string, in UpdateDeviceInput) (*Device, error) {
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
	return d, nil
}

// Delete soft-deletes a device. A 404 APIError is returned when
// the row does not exist (either never created or already
// deleted).
func (s *Service) Delete(id string) error {
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
	return nil
}

// ApplyAction dispatches an action by name. The current action
// set is intentionally small (the spec's "Execute device action"
// scenario); a richer state machine lives in the per-type
// services and is not in scope here.
func (s *Service) ApplyAction(id, action string) (*Device, error) {
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
	return d, nil
}
