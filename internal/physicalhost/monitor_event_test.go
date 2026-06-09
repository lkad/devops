package physicalhost

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/ws/realtime"
)

// TestMonitor_EmitsDeviceEventOnStateChange is the contract
// for the realtime hook: when the monitor transitions a host
// from online to monitoring_issue, it must publish a
// device_event on the realtime hub. The frontend subscribes to
// this channel and re-fetches the host row when it fires.
func TestMonitor_EmitsDeviceEventOnStateChange(t *testing.T) {
	pub := realtime.NewNoopPublisher()
	mon := buildMonitorWithPublisher(t, pub, 3)
	p := createOnlineHost(t, mon)

	// Script a failed ping so the next Check transitions to
	// monitoring_issue (a state change).
	mon.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})

	if err := mon.monitor.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}

	events := pub.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	ev := events[0]
	if ev.Channel != "physical_host.state_change" {
		t.Errorf("channel = %q, want physical_host.state_change", ev.Channel)
	}
	if ev.Type != "physical_host.state_change" {
		t.Errorf("type = %q, want physical_host.state_change", ev.Type)
	}
	data := ev.Payload
	if prev, _ := data["previous_state"].(string); prev != "online" {
		t.Errorf("previous_state = %q, want online", prev)
	}
	if next, _ := data["new_state"].(string); next != "monitoring_issue" {
		t.Errorf("new_state = %q, want monitoring_issue", next)
	}
	if id, _ := data["host_id"].(string); id != p.ID {
		t.Errorf("host_id = %q, want %q", id, p.ID)
	}
}

// TestMonitor_NoEventWhenStateStable: a successful ping does
// NOT fire a state change (the state stays online).
func TestMonitor_NoEventWhenStateStable(t *testing.T) {
	pub := realtime.NewNoopPublisher()
	mon := buildMonitorWithPublisher(t, pub, 3)
	p := createOnlineHost(t, mon)

	if err := mon.monitor.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	if got := len(pub.Events()); got != 0 {
		t.Errorf("expected 0 events on stable state, got %d", got)
	}
}

// TestMonitor_NoEventDuringMaintenance: while in maintenance,
// a failed ping should not transition state and not fire an
// event.
func TestMonitor_NoEventDuringMaintenance(t *testing.T) {
	pub := realtime.NewNoopPublisher()
	mon := buildMonitorWithPublisher(t, pub, 3)
	p := createOnlineHost(t, mon)
	// Enter maintenance.
	if err := mon.monitor.EnterMaintenanceForTest(p.ID, "kernel upgrade", "alice"); err != nil {
		t.Fatalf("enter maintenance: %v", err)
	}
	// Script a failure; Check should still not emit because
	// the spec short-circuits state changes in maintenance.
	mon.prober.ScriptPing(p.IPAddress, PingResult{Reachable: false, Err: errors.New("refused")})
	if err := mon.monitor.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	if got := len(pub.Events()); got != 0 {
		t.Errorf("expected 0 events during maintenance, got %d", got)
	}
}

// monitorWithPublisher bundles a monitor with a publisher for
// the device_event tests.
type monitorWithPublisher struct {
	repo    *Repository
	monitor *MonitorService
	prober  *Fake
}

func buildMonitorWithPublisher(t *testing.T, pub realtime.Publisher, threshold int) *monitorWithPublisher {
	t.Helper()
	repo := NewRepository(openDB(t))
	prober := NewFake()
	mon := NewMonitorService(MonitorConfig{
		Repo: repo, Prober: prober,
		ConsecutiveFailures: threshold, CheckInterval: time.Minute,
	})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	// New dependency: the realtime publisher.
	mon.SetPublisher(pub)
	return &monitorWithPublisher{repo: repo, monitor: mon, prober: prober}
}

func createOnlineHost(t *testing.T, m *monitorWithPublisher) *PhysicalHost {
	t.Helper()
	p := &PhysicalHost{
		DeviceID: "dev-" + t.Name(), IPAddress: "10.0.0.1",
		SSHUser: "root", SSHPort: 22, State: StateOnline,
	}
	if err := m.repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	return p
}
