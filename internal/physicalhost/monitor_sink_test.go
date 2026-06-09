package physicalhost

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// fakeSink counts Enqueue calls.
type fakeSink struct{ n atomic.Int32 }

func (f *fakeSink) Enqueue(_ string, _ Metrics) { f.n.Add(1) }

// fakeSrc returns a fixed snapshot (or error) on every Collect.
type fakeSrc struct {
	out Metrics
	err error
}

func (f *fakeSrc) Collect(_ context.Context, _ Host) (Metrics, error) { return f.out, f.err }

// TestMonitor_PushesFreshSnapshot verifies the monitor feeds
// the metrics sink after every successful Check.
func TestMonitor_PushesFreshSnapshot(t *testing.T) {
	repo := NewRepository(openDB(t))
	fakePr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: fakePr, ConsecutiveFailures: 3, CheckInterval: time.Minute})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	sink := &fakeSink{}
	src := &fakeSrc{out: Metrics{DataStatus: DataStatusFresh, CPU: prober.CPUMetrics{Cores: 8}}}
	mon.SetMetricsSink(sink)
	mon.SetCollector(src)

	p := &PhysicalHost{DeviceID: "d-sink-1", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := mon.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	if sink.n.Load() != 1 {
		t.Errorf("sink calls = %d, want 1", sink.n.Load())
	}
}

// TestMonitor_SkipsSinkOnTotalFailure: a Collect error must
// not enqueue an "unavailable" snapshot to the sink.
func TestMonitor_SkipsSinkOnTotalFailure(t *testing.T) {
	repo := NewRepository(openDB(t))
	fakePr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: fakePr, ConsecutiveFailures: 3, CheckInterval: time.Minute})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	sink := &fakeSink{}
	src := &fakeSrc{err: errors.New("ssh down")}
	mon.SetMetricsSink(sink)
	mon.SetCollector(src)

	p := &PhysicalHost{DeviceID: "d-sink-2", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := mon.Check(context.Background(), p.ID); err != nil {
		t.Fatalf("check: %v", err)
	}
	if sink.n.Load() != 0 {
		t.Errorf("sink calls = %d, want 0 (total failure should skip)", sink.n.Load())
	}
}
