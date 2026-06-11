package physicalhost

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// blockingProber blocks until ctx is cancelled. Used to assert
// the per-host context is honoured (audit item 1: an in-flight
// TCP dial must abort on SIGTERM, not leak for 5s).
type blockingProber struct {
	entered atomic.Int32
}

func (b *blockingProber) Ping(ctx context.Context, _ Host) (PingResult, error) {
	b.entered.Add(1)
	<-ctx.Done()
	return PingResult{Reachable: false, Err: ctx.Err()}, ctx.Err()
}
func (b *blockingProber) SSHExec(_ context.Context, _ Host, _ string) ([]byte, error) {
	return nil, nil
}

// TestMonitorLoop_ContextCancelReturnsFast asserts the loop
// returns within 100ms of ctx cancel (audit item 1's "200-host
// fleet's shutdown takes ms not 1000s" guarantee).
func TestMonitorLoop_ContextCancelReturnsFast(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 1 * time.Second, Jitter: 0})

	// Many hosts so we'd notice a per-host sequential loop
	// holding shutdown for 1000s.
	ids := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i+1)
		p := &PhysicalHost{DeviceID: "loop-fast-" + ip, IPAddress: ip, SSHUser: "root", SSHPort: 22, State: StateOnline}
		if err := repo.Create(p); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		ids = append(ids, p.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loop.Run(ctx, ids)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond) // let the loop enter tick()
	cancel()
	select {
	case <-done:
		// good — returned within budget
	case <-time.After(100 * time.Millisecond):
		t.Fatal("loop did not return within 100ms of context cancel (audit item 1 regression)")
	}
}

// TestMonitorLoop_PerHostContextCancelled asserts the per-host
// 5s ctx is cancelled when the parent ctx is cancelled mid-check.
// The blocking prober parks on <-ctx.Done(); if the per-host
// ctx is not cancelled the test would hang for 5s.
func TestMonitorLoop_PerHostContextCancelled(t *testing.T) {
	repo := NewRepository(openDB(t))
	bp := &blockingProber{}
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: bp, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 1 * time.Second, Jitter: 0})

	p := &PhysicalHost{DeviceID: "loop-block", IPAddress: "10.0.0.42", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loop.Run(ctx, []string{p.ID})
		close(done)
	}()
	// Wait for the prober to enter; if the per-host ctx is
	// never cancelled, this select blocks until the test
	// times out.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && bp.entered.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if bp.entered.Load() == 0 {
		cancel()
		t.Fatal("blocking prober never entered Ping")
	}
	cancel()
	select {
	case <-done:
		// good — the per-host ctx was cancelled, prober
		// returned ctx.Err(), and the loop unwound.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("loop did not return; per-host ctx was not honoured")
	}
}

// recordingLoopMetrics counts the 3 monitor-loop metric calls
// in memory so a unit test can assert increments without
// standing up a real Prometheus registry.
type recordingLoopMetrics struct {
	iterations atomic.Int32
	lastTick   atomic.Int32
	errors     sync.Map // host_id -> *atomic.Int32
}

func (r *recordingLoopMetrics) IncMonitorLoopIteration() { r.iterations.Add(1) }
func (r *recordingLoopMetrics) SetMonitorLoopLastTick(_ time.Time) {
	r.lastTick.Add(1)
}
func (r *recordingLoopMetrics) IncMonitorLoopError(hostID string) {
	v, _ := r.errors.LoadOrStore(hostID, &atomic.Int32{})
	v.(*atomic.Int32).Add(1)
}
func (r *recordingLoopMetrics) errorCount(hostID string) int32 {
	v, ok := r.errors.Load(hostID)
	if !ok {
		return 0
	}
	return v.(*atomic.Int32).Load()
}

// TestMonitorLoop_IncrementsMetrics asserts the 3 new
// Prometheus metrics are bumped on every tick + per-host error.
// We use a missing host ID to force MonitorService.Check to
// return ErrNotFound (the loop's "check failed" path); this
// exercises the error counter without depending on the prober's
// error semantics.
func TestMonitorLoop_IncrementsMetrics(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	rec := &recordingLoopMetrics{}
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 5 * time.Millisecond, Jitter: 0, Metrics: rec})

	// Two host IDs that don't exist in the DB. MonitorService.Check
	// returns ErrNotFound, which the loop treats as a failure
	// and increments the error counter.
	missing := []string{"loop-metric-missing-1", "loop-metric-missing-2"}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	loop.Run(ctx, missing)

	if got := rec.iterations.Load(); got < 1 {
		t.Errorf("iterations = %d, want >= 1", got)
	}
	if got := rec.lastTick.Load(); got < 1 {
		t.Errorf("lastTick calls = %d, want >= 1", got)
	}
	for _, id := range missing {
		if got := rec.errorCount(id); got < 1 {
			t.Errorf("error count for %s = %d, want >= 1", id, got)
		}
	}
}

// TestMonitorLoop_NilMetricsIsNoop confirms the no-op default
// kicks in when the constructor receives cfg.Metrics == nil —
// a unit test that doesn't care about Prometheus shouldn't
// have to wire a registry.
func TestMonitorLoop_NilMetricsIsNoop(t *testing.T) {
	repo := NewRepository(openDB(t))
	pr := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: pr, ConsecutiveFailures: 3, CheckInterval: 10 * time.Millisecond})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	loop := NewMonitorLoop(mon, MonitorLoopConfig{Tick: 5 * time.Millisecond, Jitter: 0 /* Metrics: nil */})

	p := &PhysicalHost{DeviceID: "loop-noop", IPAddress: "10.0.0.99", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	loop.Run(ctx, []string{p.ID}) // must not panic
}
