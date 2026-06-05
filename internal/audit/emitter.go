package audit

import (
	"context"
	"log/slog"
	"sync"
)

// Emitter is the seam every other module's service layer talks
// to when it wants to record an audit event. The contract is
// non-blocking: callers must not stall the user request on a slow
// write to the audit log.
//
// Implementations:
//
//   - DBEmitter:        persists the event via the repository.
//                       Synchronous; used in tests and as the
//                       "real" inner of the production stack.
//   - NoopEmitter:      discards the event. For unit tests of
//                       downstream modules that do not care
//                       about the audit row.
//   - BufferedEmitter:  wraps another emitter with a
//                       non-blocking channel + worker pool. The
//                       default production wiring: a slow DB
//                       write never blocks the API.
type Emitter interface {
	Emit(ctx context.Context, evt AuditEvent)
}

// NoopEmitter accepts the event and discards it. Use in tests
// that exercise the calling module's logic but not the audit
// persistence path.
type NoopEmitter struct{}

// Emit satisfies Emitter.
func (NoopEmitter) Emit(_ context.Context, _ AuditEvent) {}

// DBEmitter persists the event through the supplied repository.
// The emit is synchronous: the row is visible in the database
// when Emit returns. This is the right shape for unit tests of
// the audit module itself; in production wrap it in a
// BufferedEmitter so the API never blocks on a slow write.
type DBEmitter struct {
	repo *Repository
}

// NewDBEmitter returns a DBEmitter backed by the supplied
// repository. A nil repository is tolerated (Emit becomes a
// no-op) so dev callers never crash on a missing wiring.
func NewDBEmitter(repo *Repository) *DBEmitter {
	return &DBEmitter{repo: repo}
}

// Emit inserts the event row. A nil repository is silently
// dropped so the dev wiring is forgiving.
func (d *DBEmitter) Emit(_ context.Context, evt AuditEvent) {
	if d == nil || d.repo == nil {
		return
	}
	// Make a stack-local copy so the caller's pointer is not
	// mutated by the BeforeCreate hook (which assigns the ID).
	row := evt
	_ = d.repo.Create(&row)
}

// BufferedEmitterConfig is the constructor input for the
// buffered emitter. Defaults are applied so a zero-value config
// still works (BufferSize=256, Workers=1).
type BufferedEmitterConfig struct {
	// BufferSize is the channel capacity. Events are dropped
	// (with a warning) once the channel is full. The default
	// is generous enough to absorb a 1s burst at 1k events/s.
	BufferSize int
	// Workers is the number of goroutines that drain the
	// channel. One is enough for the audit workload; raise
	// this if the inner emitter blocks for >1ms per call.
	Workers int
	// Logger is the slog logger used for the drop warning.
	// Nil falls back to slog.Default().
	Logger *slog.Logger
	// OnEmitted / OnDropped are optional observability hooks.
	// The default behaviour is a single slog.Warn per drop.
	OnEmitted func()
	OnDropped func()
}

// BufferedEmitter wraps another Emitter with a buffered
// channel + worker pool. Emit is non-blocking: when the channel
// is full, the event is dropped and a warning is logged. The
// spec's "if buffer full, drop + log a warning" requirement is
// implemented here.
//
// Close drains the channel and waits for the workers to
// finish. It is safe to call multiple times; the second and
// subsequent calls are no-ops.
type BufferedEmitter struct {
	inner Emitter
	cfg   BufferedEmitterConfig
	ch    chan AuditEvent
	wg    sync.WaitGroup
	close sync.Once
	done  chan struct{}
}

// NewBufferedEmitter starts the worker pool and returns the
// emitter. A zero-value config yields sensible defaults
// (BufferSize=256, Workers=1).
func NewBufferedEmitter(inner Emitter, cfg BufferedEmitterConfig) *BufferedEmitter {
	if inner == nil {
		inner = NoopEmitter{}
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	b := &BufferedEmitter{
		inner: inner,
		cfg:   cfg,
		ch:    make(chan AuditEvent, cfg.BufferSize),
		done:  make(chan struct{}),
	}
	for i := 0; i < cfg.Workers; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	return b
}

// Emit hands the event to the worker pool. Non-blocking: when
// the channel is full, the event is dropped and (if configured)
// the OnDropped hook is called. A nil receiver is a no-op so
// callers can pass a nil emitter through a config struct.
func (b *BufferedEmitter) Emit(_ context.Context, evt AuditEvent) {
	if b == nil {
		return
	}
	select {
	case b.ch <- evt:
		if b.cfg.OnEmitted != nil {
			b.cfg.OnEmitted()
		}
	default:
		// Buffer full: drop with a warning. The spec calls
		// this out explicitly so we never silently lose
		// events.
		b.cfg.Logger.Warn("audit emitter buffer full; dropping event",
			"action", evt.Action,
			"resource_type", evt.ResourceType,
			"resource_id", evt.ResourceID,
		)
		if b.cfg.OnDropped != nil {
			b.cfg.OnDropped()
		}
	}
}

// Close signals the workers to stop and waits for them to drain
// in-flight events. Safe to call multiple times. After Close the
// emitter MUST NOT be reused.
func (b *BufferedEmitter) Close() {
	if b == nil {
		return
	}
	b.close.Do(func() {
		close(b.done)
		close(b.ch)
	})
	b.wg.Wait()
}

// worker drains the channel until it is closed.
func (b *BufferedEmitter) worker() {
	defer b.wg.Done()
	for evt := range b.ch {
		b.inner.Emit(context.Background(), evt)
		// Exit early if Close has been called and the channel
		// is closed; the range will return on the next
		// iteration.
		select {
		case <-b.done:
			// Drain any remaining buffered events.
			for {
				select {
				case e, ok := <-b.ch:
					if !ok {
						return
					}
					b.inner.Emit(context.Background(), e)
				default:
					return
				}
			}
		default:
		}
	}
}
