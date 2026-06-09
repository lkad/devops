package physicalhost

import (
	"context"

	"github.com/devops-toolkit/backend/internal/audit"
)

// AuditEmitterAdapter bridges the physicalhost package's
// internal AuditEvent shape to the cross-module audit.Service.
// MaintenanceService.EnterMaintenance / ExitMaintenance call
// the AuditEmitter interface, but the production code wants
// to persist rows through audit.Service so the /api/audit
// surface is the single source of truth.
//
// The adapter is a value receiver so a ServiceConfig can hold
// it directly without an extra factory call.
type AuditEmitterAdapter struct {
	svc *audit.Service
}

// NewAuditEmitterAdapter builds the adapter. A nil service
// turns Emit* into no-ops so test wiring is forgiving.
func NewAuditEmitterAdapter(svc *audit.Service) *AuditEmitterAdapter {
	return &AuditEmitterAdapter{svc: svc}
}

// EmitMaintenanceEnter translates the physicalhost event into
// the audit.RecordAction DTO and forwards it. The reason and
// expected duration (if any) are recorded in Metadata.
func (a *AuditEmitterAdapter) EmitMaintenanceEnter(ctx context.Context, evt AuditEvent) {
	if a == nil || a.svc == nil {
		return
	}
	a.svc.RecordAction(ctx, audit.RecordActionInput{
		Action:       audit.ActionMaintenanceEnter,
		ResourceType: audit.ResourcePhysicalHost,
		ResourceID:   evt.HostID,
		ActorID:      evt.UserID,
		ActorName:    evt.UserID,
		Metadata:     audit.JSONMap{"reason": evt.Reason, "at": evt.At},
	})
}

// EmitMaintenanceExit mirrors EmitMaintenanceEnter for the
// exit action. The maintenance duration is recorded so the
// audit row carries "how long was the host in maintenance".
func (a *AuditEmitterAdapter) EmitMaintenanceExit(ctx context.Context, evt AuditEvent) {
	if a == nil || a.svc == nil {
		return
	}
	a.svc.RecordAction(ctx, audit.RecordActionInput{
		Action:       audit.ActionMaintenanceExit,
		ResourceType: audit.ResourcePhysicalHost,
		ResourceID:   evt.HostID,
		ActorID:      evt.UserID,
		ActorName:    evt.UserID,
		Metadata:     audit.JSONMap{"duration_seconds": int64(evt.Duration.Seconds()), "at": evt.At},
	})
}

// Compile-time check that *AuditEmitterAdapter satisfies
// AuditEmitter. The two-method signature is small enough that
// the compiler will fail loudly if either method drifts.
var _ AuditEmitter = (*AuditEmitterAdapter)(nil)
