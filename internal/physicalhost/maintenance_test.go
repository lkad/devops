package physicalhost

import (
	"context"
	"testing"
	"time"
)

// fakeAuditor is the in-test implementation of the AuditEmitter
// interface. Phase 7 wires the real audit-logging service here;
// until then, tests assert on the captured events.
type fakeAuditor struct {
	events []AuditEvent
}

func (f *fakeAuditor) EmitMaintenanceEnter(ctx context.Context, evt AuditEvent) {
	evt.Action = "maintenance_enter"
	f.events = append(f.events, evt)
}

func (f *fakeAuditor) EmitMaintenanceExit(ctx context.Context, evt AuditEvent) {
	evt.Action = "maintenance_exit"
	f.events = append(f.events, evt)
}

func (f *fakeAuditor) Events() []AuditEvent { return f.events }

// maintenanceFixture wires a MaintenanceService with a real
// repository and a fake auditor. The MaintenanceService is the
// single place that knows about audit emission.
type maintenanceFixture struct {
	repo    *Repository
	auditor *fakeAuditor
	svc     *MaintenanceService
}

func newMaintenanceFixture(t *testing.T) *maintenanceFixture {
	t.Helper()
	repo := NewRepository(openDB(t))
	auditor := &fakeAuditor{}
	svc := NewMaintenanceService(MaintenanceConfig{
		Repo:   repo,
		Auditor: auditor,
	})
	return &maintenanceFixture{repo: repo, auditor: auditor, svc: svc}
}

func (f *maintenanceFixture) createOnline(t *testing.T) *PhysicalHost {
	t.Helper()
	p := &PhysicalHost{
		DeviceID:  "dev-" + t.Name(),
		IPAddress: "10.0.0.1",
		SSHUser:   "root",
		SSHPort:   22,
		State:     StateOnline,
	}
	if err := f.repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	return p
}

// TestMaintenance_Enter_TransitionsAndAudits covers the
// "Audit entry on enter" + "Enter maintenance" scenarios: the
// state flips, the maintenance columns are filled, and a
// maintenance_enter audit event is emitted.
func TestMaintenance_Enter_TransitionsAndAudits(t *testing.T) {
	f := newMaintenanceFixture(t)
	p := f.createOnline(t)
	host, evt, err := f.svc.EnterMaintenance(context.Background(), p.ID, "kernel upgrade", "alice")
	if err != nil {
		t.Fatalf("enter: %v", err)
	}
	if host.State != StateMaintenance {
		t.Errorf("state = %q, want maintenance", host.State)
	}
	if host.MaintenanceReason != "kernel upgrade" {
		t.Errorf("reason = %q, want kernel upgrade", host.MaintenanceReason)
	}
	if host.MaintenanceSetBy != "alice" {
		t.Errorf("set_by = %q, want alice", host.MaintenanceSetBy)
	}
	if host.MaintenanceStartedAt == nil {
		t.Error("maintenance_started_at should be set")
	}
	if evt.Action != "maintenance_enter" {
		t.Errorf("event action = %q, want maintenance_enter", evt.Action)
	}
	if evt.HostID != p.ID {
		t.Errorf("event host_id = %q, want %q", evt.HostID, p.ID)
	}
	if evt.UserID != "alice" {
		t.Errorf("event user_id = %q, want alice", evt.UserID)
	}
	if evt.Reason != "kernel upgrade" {
		t.Errorf("event reason = %q, want kernel upgrade", evt.Reason)
	}
	if len(f.auditor.Events()) != 1 {
		t.Errorf("auditor captured %d events, want 1", len(f.auditor.Events()))
	}
}

// TestMaintenance_Enter_AlreadyMaintained returns CodeInvalidState
// (HTTP 422). The spec treats a second enter as an invalid
// transition; the test pins both the error type and the wire code.
func TestMaintenance_Enter_AlreadyMaintained(t *testing.T) {
	f := newMaintenanceFixture(t)
	p := f.createOnline(t)
	if _, _, err := f.svc.EnterMaintenance(context.Background(), p.ID, "first", "alice"); err != nil {
		t.Fatalf("first enter: %v", err)
	}
	_, _, err := f.svc.EnterMaintenance(context.Background(), p.ID, "second", "alice")
	if err == nil {
		t.Fatal("expected invalid-state error")
	}
	if !IsInvalidState(err) {
		t.Errorf("expected IsInvalidState, got %v", err)
	}
}

// TestMaintenance_Exit_TransitionsAndAudits covers "Exit
// maintenance" + "Audit entry on exit". The state returns to
// online, the maintenance columns are cleared, and a
// maintenance_exit event is emitted with the duration.
func TestMaintenance_Exit_TransitionsAndAudits(t *testing.T) {
	f := newMaintenanceFixture(t)
	p := f.createOnline(t)
	if _, _, err := f.svc.EnterMaintenance(context.Background(), p.ID, "kernel upgrade", "alice"); err != nil {
		t.Fatalf("enter: %v", err)
	}
	// Sleep a moment so the duration is non-zero.
	time.Sleep(5 * time.Millisecond)
	host, evt, err := f.svc.ExitMaintenance(context.Background(), p.ID, "alice")
	if err != nil {
		t.Fatalf("exit: %v", err)
	}
	if host.State != StateOnline {
		t.Errorf("state = %q, want online", host.State)
	}
	if host.MaintenanceStartedAt != nil {
		t.Errorf("maintenance_started_at = %v, want nil", host.MaintenanceStartedAt)
	}
	if evt.Action != "maintenance_exit" {
		t.Errorf("event action = %q, want maintenance_exit", evt.Action)
	}
	if evt.UserID != "alice" {
		t.Errorf("event user_id = %q, want alice", evt.UserID)
	}
	if evt.Duration < 0 {
		t.Errorf("event duration = %v, want >= 0", evt.Duration)
	}
	if len(f.auditor.Events()) != 2 {
		t.Errorf("auditor captured %d events, want 2 (enter + exit)", len(f.auditor.Events()))
	}
}

// TestMaintenance_Exit_NotInMaintenance is the inverse of the
// "already maintained" case: exiting maintenance when the host
// was never in maintenance must fail with CodeInvalidState.
func TestMaintenance_Exit_NotInMaintenance(t *testing.T) {
	f := newMaintenanceFixture(t)
	p := f.createOnline(t)
	_, _, err := f.svc.ExitMaintenance(context.Background(), p.ID, "alice")
	if err == nil {
		t.Fatal("expected invalid-state error")
	}
	if !IsInvalidState(err) {
		t.Errorf("expected IsInvalidState, got %v", err)
	}
	if len(f.auditor.Events()) != 0 {
		t.Errorf("auditor should not have received any event, got %d", len(f.auditor.Events()))
	}
}

// TestMaintenance_Enter_HostNotFound covers the missing-row case.
func TestMaintenance_Enter_HostNotFound(t *testing.T) {
	f := newMaintenanceFixture(t)
	_, _, err := f.svc.EnterMaintenance(context.Background(), "missing-id", "r", "alice")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}
