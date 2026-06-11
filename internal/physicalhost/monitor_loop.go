package physicalhost

import (
	"context"
	"log/slog"
	"time"
)

// MonitorLoopMetrics is the contract the monitor loop needs
// from observability. Defining a tiny interface here lets
// the loop be unit-tested without spinning up a real
// Prometheus registry; production wires the real
// *observability.Metrics (which satisfies this interface)
// and tests can pass nil to run a no-op loop.
//
// (We can't reuse the package's existing MetricsSink —
// that's the AsyncInfluxWriter hook on MonitorService.)
type MonitorLoopMetrics interface {
	IncMonitorLoopIteration()
	SetMonitorLoopLastTick(t time.Time)
	IncMonitorLoopError(hostID string)
}

// MonitorLoopConfig bundles the constructor input. Tick is
// the fixed interval between checks; Jitter is a random
// per-tick offset to avoid a thundering-herd when many
// instances of the binary come up at once. The default Tick
// is 1 minute; the default Jitter is 5s (the spec says "small
// randomised skew"). Metrics is optional — nil means a
// no-op sink so test code does not have to build a registry.
type MonitorLoopConfig struct {
	Tick    time.Duration
	Jitter  time.Duration
	Logger  *slog.Logger
	Metrics MonitorLoopMetrics
}

// MonitorLoop is the production periodic Check loop. It is
// split from MonitorService so a unit test can drive it with
// a short Tick (5ms in the test suite, 1 minute in prod).
// Run blocks until ctx is cancelled.
type MonitorLoop struct {
	mon     *MonitorService
	cfg     MonitorLoopConfig
	logger  *slog.Logger
	metrics MonitorLoopMetrics
}

// noopLoopMetrics is the no-op default for the Metrics field.
// A unit test that does not care about Prometheus can leave
// cfg.Metrics == nil; the constructor swaps it for the noop
// so the loop's hot path never needs a nil check.
type noopLoopMetrics struct{}

func (noopLoopMetrics) IncMonitorLoopIteration()            {}
func (noopLoopMetrics) SetMonitorLoopLastTick(_ time.Time) {}
func (noopLoopMetrics) IncMonitorLoopError(_ string)        {}

// NewMonitorLoop builds the loop with sane defaults. A nil
// logger falls back to slog.Default(); the production main.go
// passes a structured logger so loop errors land in the same
// JSON pipeline as the rest of the app. A nil Metrics falls
// back to a no-op sink so the loop is testable without a
// Prometheus registry.
func NewMonitorLoop(mon *MonitorService, cfg MonitorLoopConfig) *MonitorLoop {
	tick := cfg.Tick
	if tick <= 0 {
		tick = time.Minute
	}
	jitter := cfg.Jitter
	if jitter < 0 {
		jitter = 0
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	metrics := cfg.Metrics
	if metrics == nil {
		metrics = noopLoopMetrics{}
	}
	return &MonitorLoop{
		mon:     mon,
		cfg:     MonitorLoopConfig{Tick: tick, Jitter: jitter, Logger: logger, Metrics: metrics},
		logger:  logger,
		metrics: metrics,
	}
}

// SetMetrics replaces the metrics sink. Useful for tests
// that construct the loop with the noop default and then
// wire a real registry mid-flight.
func (l *MonitorLoop) SetMetrics(m MonitorLoopMetrics) {
	if m == nil {
		m = noopLoopMetrics{}
	}
	l.metrics = m
}

// Start blocks until ctx is cancelled. When hostIDs is nil
// the loop walks every row in physical_hosts; when non-nil
// the caller is asserting a single-tenant loop (e.g. a CLI
// tool or a per-tenant runbook). Errors from one Check are
// logged and skipped — the loop is best-effort, never fatal.
//
// Start is the alias of Run kept for symmetry with
// AsyncInfluxWriter.Start and audit.BufferedEmitter.Start.
// Both names exist so call sites can pick the verb that
// reads best ("go loop.Start(ctx, nil)" vs
// "go loop.Run(ctx, nil)").
func (l *MonitorLoop) Start(ctx context.Context, hostIDs []string) {
	l.Run(ctx, hostIDs)
}

// Run blocks until ctx is cancelled. When hostIDs is nil the
// loop walks every row in physical_hosts; when non-nil the
// caller is asserting a single-tenant loop (e.g. a CLI tool
// or a per-tenant runbook). Errors from one Check are logged
// and skipped — the loop is best-effort, never fatal.
//
// Run honours ctx.Done() between per-host checks (so a 200-host
// fleet's shutdown takes milliseconds instead of 1000s) AND
// inside the per-host context (so a TCP dial already in flight
// can be cancelled).
func (l *MonitorLoop) Run(ctx context.Context, hostIDs []string) {
	// Honour ctx.Done() before we even spin the ticker so a
	// caller that cancels immediately does not pay the cost
	// of one in-flight tick.
	if err := ctx.Err(); err != nil {
		return
	}
	ticker := time.NewTicker(l.cfg.Tick)
	defer ticker.Stop()

	// Run one check immediately so a fresh boot doesn't wait
	// a full Tick interval before the first probe.
	l.tick(ctx, hostIDs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.tick(ctx, hostIDs)
		}
	}
}

// tick does one pass over the host set. Pulled out so Run
// stays a one-page function. The metric updates wrap the
// tick's "I tried" semantics: iterations is bumped at the
// START of the tick (so a wedged host list still counts as
// an iteration), errors is bumped per failed check.
func (l *MonitorLoop) tick(ctx context.Context, hostIDs []string) {
	l.metrics.IncMonitorLoopIteration()
	l.metrics.SetMonitorLoopLastTick(time.Now())

	ids := hostIDs
	if ids == nil {
		// Pull the current host list. The repository returns
		// every non-soft-deleted row; a future iteration can
		// add a "due" filter (next_check_at <= now) so an
		// instance with thousands of hosts doesn't probe them
		// all on every tick.
		rows, _, err := l.mon.repo.List(ListFilter{Limit: 10000})
		if err != nil {
			l.logger.Error("monitor loop: list hosts failed", "err", err)
			return
		}
		ids = make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
	}
	for _, id := range ids {
		// Bail before we even start the next host if the
		// parent ctx is done — this is what makes a 200-host
		// fleet's shutdown take milliseconds instead of 1000s.
		if err := ctx.Err(); err != nil {
			return
		}
		// Per-host context derived from the loop's parent so
		// a per-host timeout (5s) doesn't leak past the loop's
		// lifetime. The monitor.Check path is the same one the
		// POST /probe handler exercises, so any fix in one
		// path is automatically picked up by the other.
		hCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := l.mon.Check(hCtx, id); err != nil {
			l.logger.Warn("monitor loop: check failed", "host_id", id, "err", err)
			l.metrics.IncMonitorLoopError(id)
		}
		cancel()
	}
}
