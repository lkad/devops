package physicalhost

import (
	"context"
	"log/slog"
	"time"
)

// MonitorLoopConfig bundles the constructor input. Tick is
// the fixed interval between checks; Jitter is a random
// per-tick offset to avoid a thundering-herd when many
// instances of the binary come up at once. The default Tick
// is 1 minute; the default Jitter is 5s (the spec says "small
// randomised skew").
type MonitorLoopConfig struct {
	Tick   time.Duration
	Jitter time.Duration
	Logger *slog.Logger
}

// MonitorLoop is the production periodic Check loop. It is
// split from MonitorService so a unit test can drive it with
// a short Tick (5ms in the test suite, 1 minute in prod).
// Run blocks until ctx is cancelled.
type MonitorLoop struct {
	mon    *MonitorService
	cfg    MonitorLoopConfig
	logger *slog.Logger
}

// NewMonitorLoop builds the loop with sane defaults. A nil
// logger falls back to slog.Default(); the production main.go
// passes a structured logger so loop errors land in the same
// JSON pipeline as the rest of the app.
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
	return &MonitorLoop{mon: mon, cfg: MonitorLoopConfig{Tick: tick, Jitter: jitter, Logger: logger}, logger: logger}
}

// Run blocks until ctx is cancelled. When hostIDs is nil the
// loop walks every row in physical_hosts; when non-nil the
// caller is asserting a single-tenant loop (e.g. a CLI tool
// or a per-tenant runbook). Errors from one Check are logged
// and skipped — the loop is best-effort, never fatal.
func (l *MonitorLoop) Run(ctx context.Context, hostIDs []string) {
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
// stays a one-page function.
func (l *MonitorLoop) tick(ctx context.Context, hostIDs []string) {
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
		// Per-host context derived from the loop's parent so
		// a per-host timeout (5s) doesn't leak past the loop's
		// lifetime. The monitor.Check path is the same one the
		// POST /probe handler exercises, so any fix in one
		// path is automatically picked up by the other.
		hCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := l.mon.Check(hCtx, id); err != nil {
			l.logger.Warn("monitor loop: check failed", "host_id", id, "err", err)
		}
		cancel()
	}
}
