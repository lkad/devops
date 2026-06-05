package physicalhost

import (
	"context"
	"errors"
	"testing"
	"time"
)

// monitorFixture wires a MonitorService with a fake prober, a
// fresh repo, and a configurable consecutive-fail threshold. The
// monitor owns the state-transition logic and the LastCheckAt /
// NextCheckAt bookkeeping.
type monitorFixture struct {
	repo    *Repository
	monitor *MonitorService
	prober  *Fake
}

func newMonitorFixture(t *testing.T, threshold int) *monitorFixture {
	t.Helper()
	repo := NewRepository(openDB(t))
	prober := NewFake()
	mon := NewMonitorService(MonitorConfig{
		Repo:                repo,
		Prober:              prober,
		ConsecutiveFailures: threshold,
		CheckInterval:       time.Minute,
	})
	maint := NewMaintenanceService(MaintenanceConfig{
		Repo:    repo,
		Auditor: &fakeAuditor{},
	})
	mon.SetMaintenance(maint)
	return &monitorFixture{repo: repo, monitor: mon, prober: prober}
}

func (f *monitorFixture) createOnline(t *testing.T) *PhysicalHost {
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

// TestMonitor_Check_OnlineStaysOnline pins the happy path: a
// successful ping does not change state but updates LastCheckAt /
// NextCheckAt.
func TestMonitor_Check_OnlineStaysOnline(t *testing.T) {
	f := newMonitorFixture(t, 3)
	p := f.createOnline(t)
	before := time.Now().UTC()
	if err := f.monitor.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	got, _ := f.repo.Get(p.ID)
	if got.State != StateOnline {
		t.Errorf("state = %q, want online", got.State)
	}
	if got.LastCheckAt == nil || got.LastCheckAt.Before(before) {
		t.Errorf("LastCheckAt = %v, want >= %v", got.LastCheckAt, before)
	}
	if got.NextCheckAt == nil || !got.NextCheckAt.After(*got.LastCheckAt) {
		t.Errorf("NextCheckAt = %v, want > LastCheckAt", got.NextCheckAt)
	}
}

// TestMonitor_Check_OnlineToMonitoringIssue transitions online to
// monitoring_issue on the first failed probe. The spec
// "monitoring_issue" state means "something looks wrong, but
// we're not yet sure the host is gone".
func TestMonitor_Check_OnlineToMonitoringIssue(t *testing.T) {
	f := newMonitorFixture(t, 3)
	p := f.createOnline(t)
	f.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})

	if err := f.monitor.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	got, _ := f.repo.Get(p.ID)
	if got.State != StateMonitoringIssue {
		t.Errorf("state = %q, want monitoring_issue", got.State)
	}
}

// TestMonitor_Check_MonitoringIssueToOffline_AfterThreshold covers
// the "consecutive failures exceed threshold" branch from the
// spec — the host only flips to offline after N consecutive
// failures, not on the first one.
func TestMonitor_Check_MonitoringIssueToOffline_AfterThreshold(t *testing.T) {
	f := newMonitorFixture(t, 3)
	p := f.createOnline(t)
	f.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})

	// Probe 1: online -> monitoring_issue
	_ = f.monitor.Check(context.Background(), p.ID)
	// Probe 2: still monitoring_issue
	_ = f.monitor.Check(context.Background(), p.ID)
	// Probe 3: hits the threshold -> offline
	_ = f.monitor.Check(context.Background(), p.ID)

	got, _ := f.repo.Get(p.ID)
	if got.State != StateOffline {
		t.Errorf("state = %q, want offline", got.State)
	}
}

// TestMonitor_Check_OfflineRecoversToOnline covers a transient
// outage: the host is offline, a successful probe flips it back
// to online.
func TestMonitor_Check_OfflineRecoversToOnline(t *testing.T) {
	f := newMonitorFixture(t, 1)
	p := f.createOnline(t)
	f.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})

	// First failure with threshold=1 -> offline.
	_ = f.monitor.Check(context.Background(), p.ID)
	// Heal the host: clear the scripted failure.
	f.prober.ScriptPing(p.IPAddress, PingResult{Reachable: true})
	_ = f.monitor.Check(context.Background(), p.ID)

	got, _ := f.repo.Get(p.ID)
	if got.State != StateOnline {
		t.Errorf("state = %q, want online", got.State)
	}
}

// TestMonitor_Check_MaintenanceBlocksAutoTransition is the
// "auto state change blocked during maintenance" scenario from
// the spec. A failing probe must NOT change the state out of
// maintenance; the failure is recorded in LastCheckAt so the
// post-maintenance review can see it.
func TestMonitor_Check_MaintenanceBlocksAutoTransition(t *testing.T) {
	f := newMonitorFixture(t, 1)
	p := f.createOnline(t)
	// Put the host in maintenance first.
	if err := f.monitor.EnterMaintenanceForTest(p.ID, "patching", "alice"); err != nil {
		t.Fatalf("enter maintenance: %v", err)
	}
	// Now a failing probe must NOT flip the state to offline.
	f.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})
	_ = f.monitor.Check(context.Background(), p.ID)
	got, _ := f.repo.Get(p.ID)
	if got.State != StateMaintenance {
		t.Errorf("state = %q, want maintenance", got.State)
	}
	if !got.InMaintenance() {
		t.Errorf("InMaintenance() = false, want true")
	}
	if got.LastCheckAt == nil {
		t.Error("LastCheckAt should still be updated during maintenance")
	}
}

// TestMonitor_EnterMaintenance_InvalidStateForOnlineReentry pins
// the "orthogonal flag" contract: a host already in maintenance
// cannot enter maintenance again. The state machine must reject
// this with CodeInvalidState (422).
func TestMonitor_EnterMaintenance_RejectsAlreadyMaintained(t *testing.T) {
	f := newMonitorFixture(t, 1)
	p := f.createOnline(t)
	if err := f.monitor.EnterMaintenanceForTest(p.ID, "first", "alice"); err != nil {
		t.Fatalf("first enter: %v", err)
	}
	err := f.monitor.EnterMaintenanceForTest(p.ID, "second", "alice")
	if err == nil {
		t.Fatal("expected error on re-enter")
	}
}

// TestMonitor_ExitMaintenance_RejectsIfNotInMaintenance covers the
// inverse: trying to exit maintenance when not in maintenance
// must return a typed error.
func TestMonitor_ExitMaintenance_RejectsIfNotInMaintenance(t *testing.T) {
	f := newMonitorFixture(t, 1)
	p := f.createOnline(t)
	err := f.monitor.ExitMaintenanceForTest(p.ID, "alice")
	if err == nil {
		t.Fatal("expected error on exit when not in maintenance")
	}
}

// TestMonitor_Check_HostMissingReturnsNotFound — a probe against
// a deleted host must surface a NotFound error, not a panic.
func TestMonitor_Check_HostMissingReturnsNotFound(t *testing.T) {
	f := newMonitorFixture(t, 3)
	err := f.monitor.Check(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
