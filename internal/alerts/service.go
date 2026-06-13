package alerts

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrUnauthenticated is the sentinel returned when a service
// method is invoked without a caller on the context. The
// handler maps it to a 401 UNAUTHORIZED APIError.
var ErrUnauthenticated = errors.New("alerts: unauthenticated")

// ErrForbidden is the sentinel returned when the caller's
// tenant membership does not allow the requested operation.
// The handler maps it to a 403 FORBIDDEN APIError.
var ErrForbidden = errors.New("alerts: forbidden")

// ServiceConfig bundles the dependencies of Service. Kept as a
// struct (not positional args) so new dependencies (clock, rate
// limiter) can be added without changing call sites.
type ServiceConfig struct {
	Repo        *Repository
	Dispatcher  Dispatcher
	Suppression SuppressionChecker
	Logger      *slog.Logger
	// Audit is the cross-module audit service. Optional
	// (nil means "no audit emission"); production wires
	// a real service so v0.2.0.0 P0 #3 audit-trail
	// coverage holds.
	Audit *audit.Service
	// Now is the clock the service uses to stamp FiredAt /
	// ResolvedAt / AcknowledgedAt. Defaults to time.Now when
	// nil so unit tests can substitute a fixed clock.
	Now func() time.Time
}

// Service is the business-logic layer for the alert subsystem.
// It owns validation, the alert state machine, the suppression
// decision, and the channel-dispatch orchestration. The
// repository is GORM-only; the dispatcher is the seam for
// notification delivery; the suppression checker is the seam for
// maintenance-mode awareness; the audit service is the seam for
// cross-module RecordAction calls.
type Service struct {
	repo        *Repository
	dispatcher  Dispatcher
	suppression SuppressionChecker
	log         *slog.Logger
	audit       *audit.Service
	now         func() time.Time
}

// NewService builds a Service. The dispatcher and suppression
// checker are required; a nil dispatcher is treated as the
// LogDispatcher (defensive default so dev callers never crash on
// a missing wiring).
func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	disp := cfg.Dispatcher
	if disp == nil {
		disp = NewLogDispatcher(log)
	}
	supp := cfg.Suppression
	if supp == nil {
		supp = NewFakeSuppressionChecker()
	}
	return &Service{
		repo:        cfg.Repo,
		dispatcher:  disp,
		suppression: supp,
		log:         log,
		audit:       cfg.Audit,
		now:         now,
	}
}

// CreateAlertInput is the input DTO for CreateAlert. The handler
// decodes the wire JSON into this struct; the service then
// validates and maps it onto an Alert. ChannelIDs is the
// optional list of channels the alert should be dispatched to
// in addition to the default "log" channel.
type CreateAlertInput struct {
	Name       string
	Severity   Severity
	SourceType SourceType
	SourceID   string
	Labels     JSONMap
	ChannelIDs []string
	// Message is a human-readable description carried on the
	// alert for the audit trail. Optional.
	Message string
}

// FireInput is the input DTO for Fire. It carries the channel
// list and the source identification so the service can apply
// the suppression decision and route to the dispatcher.
type FireInput struct {
	Name       string
	Severity   Severity
	SourceType SourceType
	SourceID   string
	Labels     JSONMap
	ChannelIDs []string
	Message    string
}

// CreateChannelInput is the input DTO for CreateChannel.
type CreateChannelInput struct {
	Type    ChannelType
	Config  JSONMap
	Enabled bool
}

// UpdateChannelInput is the partial-update DTO for channels. A
// nil pointer means "do not change"; a non-nil pointer to a
// zero value means "set to zero".
type UpdateChannelInput struct {
	Type    *ChannelType
	Config  JSONMap
	Enabled *bool
}

// Stats is the aggregate view returned by Stats. The shape
// matches the spec's "Get alert stats" scenario.
type Stats struct {
	Total           int64            `json:"total"`
	BySeverity      map[Severity]int `json:"by_severity"`
	BySourceType    map[SourceType]int `json:"by_source_type"`
	SuppressedCount int64            `json:"suppressed_count"`
	SuppressedByHost map[string]int64 `json:"suppressed_by_host"`
}

// CreateAlert validates the input and persists the alert. The
// returned Alert is the freshly-stored row. The audit emission
// (alert.create) records the alert for the audit log; a
// failure to record does not roll back the mutation.
func (s *Service) CreateAlert(ctx context.Context, in CreateAlertInput) (*Alert, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	if !in.Severity.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "severity is not one of info / warning / critical",
		}
	}
	if !in.SourceType.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "source_type is not one of physical_host / k8s / pipeline / custom",
		}
	}
	if strings.TrimSpace(in.SourceID) == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "source_id is required",
		}
	}
	a := &Alert{
		Name:       strings.TrimSpace(in.Name),
		Severity:   in.Severity,
		State:      StateFiring,
		SourceType: in.SourceType,
		SourceID:   in.SourceID,
		Labels:     in.Labels,
		FiredAt:    s.now(),
	}
	if err := s.repo.CreateAlert(a); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create alert",
			Cause:   err,
		}
	}
	// Dispatch to the supplied channels unless suppressed.
	s.dispatchChannels(a, in.ChannelIDs)
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionCreate,
			ResourceType: audit.ResourceAlert,
			ResourceID:   a.ID,
			Metadata:     audit.JSONMap{"name": a.Name, "severity": string(a.Severity), "source_type": string(a.SourceType), "source_id": a.SourceID},
		})
	}
	return a, nil
}

// GetAlert returns a single alert or a 404 APIError.
func (s *Service) GetAlert(id string) (*Alert, error) {
	a, err := s.repo.GetAlert(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "alert " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load alert",
			Cause:   err,
		}
	}
	return a, nil
}

// GetAlertWithCaller is the v0.3.0.0 P0 #2 cross-tenant
// variant of GetAlert. The caller MUST be attached to the
// context; an absent caller surfaces as ErrUnauthenticated
// (401). A non-SuperAdmin caller is denied (alerts are
// global today; per-source project binding is the
// follow-up work).
func (s *Service) GetAlertWithCaller(ctx context.Context, id string) (*Alert, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		return nil, ErrForbidden
	}
	return s.GetAlert(id)
}

// ListWithCaller is the v0.3.0.0 P0 #2 cross-tenant variant
// of List. The caller MUST be attached to the context. A
// non-SuperAdmin caller is denied.
func (s *Service) ListWithCaller(ctx context.Context, f AlertFilter) ([]Alert, int64, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, 0, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		return nil, 0, ErrForbidden
	}
	return s.List(f)
}

// List returns a page of alerts plus the unfiltered total.
func (s *Service) List(f AlertFilter) ([]Alert, int64, error) {
	rows, total, err := s.repo.ListAlerts(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list alerts",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Acknowledge flips the alert into the acknowledged state and
// stamps the user id / timestamp. The userID argument is
// supplied by the handler (from the X-User-Id header in tests,
// from the JWT context in production). The audit emission
// (alert.acknowledge) records the ack for the audit log.
func (s *Service) Acknowledge(ctx context.Context, id, userID string) (*Alert, error) {
	if _, err := s.GetAlert(id); err != nil {
		return nil, err
	}
	now := s.now()
	patch := map[string]any{
		"state":           StateAcknowledged,
		"acknowledged_by": userID,
		"acknowledged_at": now,
	}
	if err := s.repo.UpdateAlert(id, patch); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "alert " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to acknowledge alert",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionAcknowledge,
			ResourceType: audit.ResourceAlert,
			ResourceID:   id,
			ActorID:      userID,
		})
	}
	return s.GetAlert(id)
}

// Resolve flips the alert into the resolved state and stamps
// the timestamp. The audit emission (alert.resolve) records
// the resolution for the audit log.
func (s *Service) Resolve(ctx context.Context, id string) (*Alert, error) {
	if _, err := s.GetAlert(id); err != nil {
		return nil, err
	}
	now := s.now()
	patch := map[string]any{
		"state":       StateResolved,
		"resolved_at": now,
	}
	if err := s.repo.UpdateAlert(id, patch); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "alert " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to resolve alert",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionResolve,
			ResourceType: audit.ResourceAlert,
			ResourceID:   id,
		})
	}
	return s.GetAlert(id)
}

// MarkSuppressed stamps the alert as suppressed with the
// supplied reason. Used by the service when maintenance mode
// causes a channel to be skipped, so the audit trail records
// the decision.
func (s *Service) MarkSuppressed(id, reason string) (*Alert, error) {
	patch := map[string]any{
		"suppressed":        true,
		"suppression_reason": reason,
	}
	if err := s.repo.UpdateAlert(id, patch); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "alert " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to mark alert as suppressed",
			Cause:   err,
		}
	}
	return s.GetAlert(id)
}

// Delete soft-deletes an alert. The audit emission
// (alert.delete) records the deletion for the audit log.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.DeleteAlert(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "alert " + id + " not found",
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete alert",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionDelete,
			ResourceType: audit.ResourceAlert,
			ResourceID:   id,
		})
	}
	return nil
}

// Fire is the public entry point for "trigger an alert" and is
// what the spec's "Trigger Alert API" / "Within rate limit"
// scenarios call. It validates, persists, and dispatches the
// alert in one step. Suppression is applied to external
// channels when the source is a physical host in maintenance.
func (s *Service) Fire(ctx context.Context, in FireInput) (*Alert, error) {
	a, err := s.CreateAlert(ctx, CreateAlertInput{
		Name:       in.Name,
		Severity:   in.Severity,
		SourceType: in.SourceType,
		SourceID:   in.SourceID,
		Labels:     in.Labels,
		ChannelIDs: in.ChannelIDs,
		Message:    in.Message,
	})
	if err != nil {
		return nil, err
	}
	// dispatchChannels was already called inside CreateAlert;
	// the function name is kept for symmetry with the spec.
	_ = a
	return a, nil
}

// dispatchChannels applies the suppression decision and
// forwards the alert to the dispatcher. It is split out of
// CreateAlert so tests can exercise it directly.
//
// Two dispatch paths:
//
//  1. Audit log: every fired alert is dispatched to the
//     configured dispatcher with a synthetic "log" channel.
//     This is the spec's "log channel" requirement: the audit
//     trail survives maintenance suppression and the alert
//     history query is fulfilled.
//  2. Configured channels: each id in channelIDs is resolved
//     and dispatched, subject to the suppression decision.
func (s *Service) dispatchChannels(a *Alert, channelIDs []string) {
	// Suppression applies only to physical-host sources.
	suppress := false
	reason := ""
	if a.SourceType == SourceTypePhysicalHost && s.suppression != nil {
		if s.suppression.IsInMaintenance(context.Background(), a.SourceID) {
			suppress = true
			reason = "host_in_maintenance"
		}
	}
	if suppress {
		// Stamp the suppression flag on the alert.
		a.Suppressed = true
		a.SuppressionReason = reason
		_ = s.repo.UpdateAlert(a.ID, map[string]any{
			"suppressed":         true,
			"suppression_reason": reason,
		})
	}
	// 1. Audit log dispatch: always send the alert to the
	// dispatcher via a synthetic log channel so the audit trail
	// is complete (per the spec's "log channel still receives"
	// scenario). The synthetic channel is intentionally
	// minimal — it carries the channel type only — so tests can
	// distinguish it from configured channels by its empty ID.
	logCh := Channel{Type: ChannelTypeLog, Config: JSONMap{}, Enabled: true}
	if err := s.dispatcher.Dispatch(context.Background(), *a, logCh); err != nil {
		s.log.Warn("audit log dispatch failed",
			"alert_id", a.ID, "err", err)
	}
	// 2. Configured channels.
	for _, id := range channelIDs {
		ch, err := s.repo.GetChannel(id)
		if err != nil {
			s.log.Warn("dispatch target channel not found",
				"alert_id", a.ID, "channel_id", id)
			continue
		}
		if !ch.Enabled {
			continue
		}
		// Suppress external channels when the source host is in
		// maintenance; log channels still receive the alert.
		if suppress && ch.Type.IsExternal() {
			continue
		}
		if err := s.dispatcher.Dispatch(context.Background(), *a, *ch); err != nil {
			s.log.Warn("dispatch failed",
				"alert_id", a.ID, "channel_id", ch.ID, "err", err)
		}
	}
}

// Stats computes the aggregate view returned by the GET
// /alerts/stats endpoint. The implementation issues a single
// SELECT and buckets the rows in Go so the database does not
// have to know about the spec's stats shape.
func (s *Service) Stats() (Stats, error) {
	rows, _, err := s.repo.ListAlerts(AlertFilter{})
	if err != nil {
		return Stats{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load alerts for stats",
			Cause:   err,
		}
	}
	out := Stats{
		BySeverity:        map[Severity]int{},
		BySourceType:      map[SourceType]int{},
		SuppressedByHost:  map[string]int64{},
	}
	out.Total = int64(len(rows))
	for _, a := range rows {
		out.BySeverity[a.Severity]++
		out.BySourceType[a.SourceType]++
		if a.Suppressed {
			out.SuppressedCount++
			out.SuppressedByHost[a.SourceID]++
		}
	}
	return out, nil
}

// CreateChannel validates the input and persists the channel.
// The audit emission (alert.create_channel / alert.create
// with metadata kind=channel) records the new channel for
// the audit log.
func (s *Service) CreateChannel(ctx context.Context, in CreateChannelInput) (*Channel, error) {
	if !in.Type.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "type is not a recognised channel type",
		}
	}
	c := &Channel{
		Type:    in.Type,
		Config:  in.Config,
		Enabled: in.Enabled,
	}
	if err := s.repo.CreateChannel(c); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create channel",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionCreate,
			ResourceType: audit.ResourceAlert,
			ResourceID:   c.ID,
			Metadata:     audit.JSONMap{"kind": "channel", "type": string(c.Type)},
		})
	}
	return c, nil
}

// GetChannel returns a single channel.
func (s *Service) GetChannel(id string) (*Channel, error) {
	c, err := s.repo.GetChannel(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "channel " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load channel",
			Cause:   err,
		}
	}
	return c, nil
}

// ListChannels returns every channel.
func (s *Service) ListChannels() ([]Channel, error) {
	rows, err := s.repo.ListChannels()
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list channels",
			Cause:   err,
		}
	}
	return rows, nil
}

// UpdateChannel applies a partial update. The audit emission
// (alert.update_channel) records the change for the audit
// log.
func (s *Service) UpdateChannel(ctx context.Context, id string, in UpdateChannelInput) error {
	if _, err := s.GetChannel(id); err != nil {
		return err
	}
	patch := map[string]any{}
	if in.Type != nil {
		if !in.Type.Valid() {
			return &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "type is not a recognised channel type",
			}
		}
		patch["type"] = *in.Type
	}
	if in.Config != nil {
		patch["config"] = in.Config
	}
	if in.Enabled != nil {
		patch["enabled"] = *in.Enabled
	}
	if len(patch) == 0 {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "update patch is empty",
		}
	}
	if err := s.repo.UpdateChannel(id, patch); err != nil {
		if errors.Is(err, ErrNotFound) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "channel " + id + " not found",
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update channel",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionUpdate,
			ResourceType: audit.ResourceAlert,
			ResourceID:   id,
			Metadata:     audit.JSONMap{"kind": "channel"},
		})
	}
	return nil
}

// DeleteChannel soft-deletes a channel. The audit emission
// (alert.delete_channel) records the deletion for the audit
// log.
func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	if err := s.repo.DeleteChannel(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "channel " + id + " not found",
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete channel",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionDelete,
			ResourceType: audit.ResourceAlert,
			ResourceID:   id,
			Metadata:     audit.JSONMap{"kind": "channel"},
		})
	}
	return nil
}
