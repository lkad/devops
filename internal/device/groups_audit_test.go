package device

import (
	"context"
	"sync"
	"testing"

	"github.com/devops-toolkit/backend/internal/audit"
)

// recordingGroupEmitter captures every AuditEvent for assertion.
// It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingGroupEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingGroupEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingGroupEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newGroupAuditFixture wires a fresh in-memory DB, a device
// group service, and a recording audit service. The audit
// service is the one used by GroupService.audit; the emitter
// captures every emit for assertion.
func newGroupAuditFixture(t *testing.T) (*GroupService, *recordingGroupEmitter) {
	t.Helper()
	db := openDeviceDB(t)
	groupRepo := NewGroupRepository(db)
	emitter := &recordingGroupEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})
	svc := NewGroupService(groupRepo, auditSvc)
	return svc, emitter
}

// TestGroupService_Create_EmitsAudit pins the service-layer
// audit emission for device-group creation. The audit emit
// uses ResourceDevice with a {group: true} metadata marker so
// the audit list view can filter device-group events apart
// from device-instance events (the resource_type is shared
// because groups are stored under the device model).
func TestGroupService_Create_EmitsAudit(t *testing.T) {
	svc, emitter := newGroupAuditFixture(t)
	g, err := svc.Create(GroupCreateInput{Name: "edge", Description: "edge routers"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceDevice {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceDevice)
	}
	if events[0].ResourceID != g.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, g.ID)
	}
}

// TestGroupService_Delete_EmitsAudit pins the delete path.
func TestGroupService_Delete_EmitsAudit(t *testing.T) {
	svc, emitter := newGroupAuditFixture(t)
	g, _ := svc.Create(GroupCreateInput{Name: "edge"})
	if err := svc.Delete(g.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionDelete {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionDelete)
	}
	if last.ResourceID != g.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, g.ID)
	}
}
