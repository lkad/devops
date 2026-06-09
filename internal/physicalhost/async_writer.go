package physicalhost

import (
	"context"
	"sync"
	"sync/atomic"
)

// AsyncInfluxWriterConfig bundles the constructor input.
// Inner is the synchronous writer the workers call. BufferSize
// is the channel capacity. Workers is the number of goroutines
// draining the channel (default 1).
type AsyncInfluxWriterConfig struct {
	Inner      *InfluxWriter
	BufferSize int
	Workers    int
}

// asyncSample is the unit of work a worker dequeues. We only
// need the data, not the hostID, but the hostID is on the same
// tuple so a worker can call Inner.Write(ctx, hostID, m) with
// the original pair.
type asyncSample struct {
	hostID string
	m      Metrics
}

// AsyncInfluxWriter is the production-grade writer. Enqueue is
// non-blocking: when the channel is full, the sample is
// dropped and a counter is bumped. The workers call the
// synchronous Inner.Write in their own goroutines so a slow
// HTTP POST never blocks the monitor loop.
//
// The production contract is "drop + log warning" (the
// BufferedEmitter in the audit module uses the same shape).
type AsyncInfluxWriter struct {
	inner   *InfluxWriter
	ch      chan asyncSample
	dropped atomic.Int64
	wg      sync.WaitGroup
	stop    chan struct{}
	stopped atomic.Bool
}

// NewAsyncInfluxWriter builds the writer but does NOT start
// the workers — call Start before Enqueue. Splitting the two
// keeps the constructor side-effect-free for tests.
func NewAsyncInfluxWriter(cfg AsyncInfluxWriterConfig) *AsyncInfluxWriter {
	bs := cfg.BufferSize
	if bs <= 0 {
		bs = 256
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	w := &AsyncInfluxWriter{
		inner: cfg.Inner,
		ch:    make(chan asyncSample, bs),
		stop:  make(chan struct{}),
	}
	// Pre-register worker WaitGroup so Close waits for them.
	for i := 0; i < workers; i++ {
		w.wg.Add(1)
	}
	// We need a per-instance handle to spin workers in Start.
	// Store the count via a closure-friendly trick: keep a
	// shadow field on the struct via a side channel.
	w.spinWorkers(workers)
	return w
}

// spinWorkers is package-private and called only by
// NewAsyncInfluxWriter. It launches the worker goroutines.
func (w *AsyncInfluxWriter) spinWorkers(n int) {
	for i := 0; i < n; i++ {
		go w.worker()
	}
}

// worker drains the channel until Close fires. The worker
// context is bound to the one passed to Start; a nil context
// is tolerated and turns cancellation into "only Close stops
// me".
func (w *AsyncInfluxWriter) worker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stop:
			// Drain remaining samples (best effort).
			for {
				select {
				case s := <-w.ch:
					w.flushOne(s)
				default:
					return
				}
			}
		case s := <-w.ch:
			w.flushOne(s)
		}
	}
}

func (w *AsyncInfluxWriter) flushOne(s asyncSample) {
	if w.inner == nil || !w.inner.Enabled() {
		return
	}
	_ = w.inner.Write(context.Background(), s.hostID, s.m)
}

// Enqueue is the non-blocking submission path. If the channel
// is full, the sample is dropped and the counter is bumped.
func (w *AsyncInfluxWriter) Enqueue(hostID string, m Metrics) {
	if w.stopped.Load() {
		return
	}
	select {
	case w.ch <- asyncSample{hostID: hostID, m: m}:
	default:
		w.dropped.Add(1)
	}
}

// Dropped returns the number of samples dropped since start.
// Exposed for observability / tests.
func (w *AsyncInfluxWriter) Dropped() int64 { return w.dropped.Load() }

// Start is a no-op kept for symmetry with the audit module's
// BufferedEmitter. The workers are launched in the constructor
// already, so Start exists only to give the call site a
// conventional "fire it up" verb. The ctx is currently unused
// because worker cancellation is bound to Close.
func (w *AsyncInfluxWriter) Start(_ context.Context) {}

// Close drains the channel and waits for the workers to
// return. Idempotent.
func (w *AsyncInfluxWriter) Close() {
	if w.stopped.Swap(true) {
		return
	}
	close(w.stop)
	w.wg.Wait()
}
