package physicalhost

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// TestMonitorLoop_TickRunsCheck confirms the loop calls
// MonitorService.Check at the configured interval.
func TestMonitorLoop_TickRunsCheck(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	// Custom ticker so we control the pace in the test.
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 5 * time.Millisecond, Jitter: 0})

	p := &PhysicalHost{DeviceID: "loop-1", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	loop.Run(ctx, []string{p.ID})

	// The Fake prober counts every Ping; >= 1 means the loop
	// ticked at least once during the 200ms window.
	if got := pr.PingCount(p.IPAddress); got < 1 {
		t.Errorf("ping count = %d, want >= 1", got)
	}
}

// TestMonitorLoop_AllHosts covers the "loop over all hosts"
// path the spec calls for. We add two hosts and assert both
// get probed.
func TestMonitorLoop_AllHosts(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 5 * time.Millisecond, Jitter: 0})

	for _, ip := range []string{"10.0.0.1", "10.0.0.2"} {
		p := &PhysicalHost{DeviceID: "loop-" + ip, IPAddress: ip, SSHUser: "root", SSHPort: 22, State: StateOnline}
		if err := repo.Create(p); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	loop.Run(ctx, nil) // nil = all hosts

	if pr.PingCount("10.0.0.1") < 1 || pr.PingCount("10.0.0.2") < 1 {
		t.Errorf("expected both hosts probed; 10.0.0.1=%d 10.0.0.2=%d",
			pr.PingCount("10.0.0.1"), pr.PingCount("10.0.0.2"))
	}
}

// TestMonitorLoop_RespectsContextCancel ensures Run returns
// promptly when the context is cancelled. Otherwise a bug
// would leak goroutines forever.
func TestMonitorLoop_RespectsContextCancel(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 1 * time.Second, Jitter: 0})

	p := &PhysicalHost{DeviceID: "loop-cancel", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loop.Run(ctx, []string{p.ID})
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("loop did not return within 500ms of context cancel")
	}
}

// TestMonitorLoop_FailureLoggedNotFatal: one Check error must
// not stop the loop. The spec is "best effort; skip on error".
func TestMonitorLoop_FailureLoggedNotFatal(t *testing.T) {
	// The fake prober's "scripted" map approach can't error
	// by design; we substitute a real Prober that returns
	// an error every call. The loop must keep going.
	repo := NewRepository(openDB(t))
	errPr := &errProber{err: errors.New("ssh: nope")}
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: errPr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 5 * time.Millisecond, Jitter: 0})

	p := &PhysicalHost{DeviceID: "loop-err", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	loop.Run(ctx, []string{p.ID}) // should not panic / block
	if errPr.calls.Load() < 1 {
		t.Errorf("prober called %d times, want >= 1", errPr.calls.Load())
	}
	// Sink to silence unused-prober import lint.
	_ = prober.Host{}
	_ = atomic.Int64{}
}

type errProber struct {
	err   error
	calls atomic.Int32
}

func (e *errProber) Ping(_ context.Context, _ Host) (PingResult, error) {
	e.calls.Add(1)
	return PingResult{}, e.err
}
func (e *errProber) SSHExec(_ context.Context, _ Host, _ string) ([]byte, error) {
	return nil, e.err
}
