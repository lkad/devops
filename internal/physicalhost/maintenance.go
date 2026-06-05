package physicalhost

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// AuditEvent is the wire shape the audit-logging service receives
// for a maintenance transition. Defined locally so the
// physicalhost package has no compile-time dependency on the
// (future) audit package; Phase 7 wires the real AuditLogger in
// place of the in-test fake.
type AuditEvent struct {
	Action   string        // "maintenance_enter" | "maintenance_exit"
	HostID   string
	UserID   string
	Reason   string
	Duration time.Duration // populated on exit, zero on enter
	At       time.Time
}

// AuditEmitter is the seam the MaintenanceService talks to. The
// real implementation will translate AuditEvent into the
// project-wide audit-log row format; tests substitute a fake
// that captures the events for assertion.
type AuditEmitter interface {
	EmitMaintenanceEnter(ctx context.Context, evt AuditEvent)
	EmitMaintenanceExit(ctx context.Context, evt AuditEvent)
}

// MaintenanceConfig bundles the dependencies of
// MaintenanceService. Kept as a struct (not positional args) so
// new dependencies can be added without changing call sites.
type MaintenanceConfig struct {
	Repo    *Repository
	Auditor AuditEmitter
}

// MaintenanceService owns the "enter maintenance" and "exit
// maintenance" actions plus the audit-emission side effect.
// Splitting it from MonitorService keeps the audit logic in one
// place: a host can only enter / exit maintenance via this
// service, so the audit row is guaranteed to be emitted.
//
// The service does not call the Prober — health checks continue
// to run during maintenance (per the spec), and the monitor
// short-circuits state changes while MaintenanceStartedAt is
// non-nil.
type MaintenanceService struct {
	repo    *Repository
	auditor AuditEmitter
	now     func() time.Time
}

// NewMaintenanceService builds a MaintenanceService.
func NewMaintenanceService(cfg MaintenanceConfig) *MaintenanceService {
	return &MaintenanceService{
		repo:    cfg.Repo,
		auditor: cfg.Auditor,
		now:     time.Now,
	}
}

// EnterMaintenance flips the host into the maintenance state.
// The audit event is emitted AFTER the DB write succeeds so a
// failed write does not produce a phantom audit row.
func (s *MaintenanceService) EnterMaintenance(ctx context.Context, hostID, reason, userID string) (*PhysicalHost, AuditEvent, error) {
	p, err := s.repo.Get(hostID)
	if err != nil {
		if IsNotFound(err) {
			return nil, AuditEvent{}, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("physical host %q not found", hostID),
			}
		}
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load physical host",
			Cause:   err,
		}
	}
	if p.InMaintenance() {
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: "host is already in maintenance",
		}
	}

	now := s.now().UTC()
	p.State = StateMaintenance
	p.MaintenanceStartedAt = &now
	p.MaintenanceReason = reason
	p.MaintenanceSetBy = userID
	if err := s.repo.Update(p); err != nil {
		if IsNotFound(err) {
			return nil, AuditEvent{}, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("physical host %q not found", hostID),
			}
		}
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update physical host",
			Cause:   err,
		}
	}

	evt := AuditEvent{
		Action: "maintenance_enter",
		HostID: p.ID,
		UserID: userID,
		Reason: reason,
		At:     now,
	}
	if s.auditor != nil {
		s.auditor.EmitMaintenanceEnter(ctx, evt)
	}
	return p, evt, nil
}

// ExitMaintenance flips the host out of maintenance, returning
// it to the operational state machine. The exit event carries
// the maintenance duration so the audit row records how long
// the host was in maintenance.
func (s *MaintenanceService) ExitMaintenance(ctx context.Context, hostID, userID string) (*PhysicalHost, AuditEvent, error) {
	p, err := s.repo.Get(hostID)
	if err != nil {
		if IsNotFound(err) {
			return nil, AuditEvent{}, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("physical host %q not found", hostID),
			}
		}
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load physical host",
			Cause:   err,
		}
	}
	if !p.InMaintenance() {
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: "host is not in maintenance",
		}
	}

	now := s.now().UTC()
	var duration time.Duration
	if p.MaintenanceStartedAt != nil {
		duration = now.Sub(*p.MaintenanceStartedAt)
	}
	p.State = StateOnline
	p.MaintenanceStartedAt = nil
	p.MaintenanceReason = ""
	p.MaintenanceSetBy = ""
	if err := s.repo.Update(p); err != nil {
		if IsNotFound(err) {
			return nil, AuditEvent{}, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("physical host %q not found", hostID),
			}
		}
		return nil, AuditEvent{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update physical host",
			Cause:   err,
		}
	}

	evt := AuditEvent{
		Action:   "maintenance_exit",
		HostID:   p.ID,
		UserID:   userID,
		Duration: duration,
		At:       now,
	}
	if s.auditor != nil {
		s.auditor.EmitMaintenanceExit(ctx, evt)
	}
	return p, evt, nil
}

// IsInvalidState reports whether err is (or wraps) a
// CodeInvalidState APIError. Helper so the test files do not
// have to import the contracts package for the assertion.
func IsInvalidState(err error) bool {
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == contracts.CodeInvalidState
	}
	return false
}
