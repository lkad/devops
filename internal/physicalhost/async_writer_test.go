package physicalhost

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// TestAsyncInfluxWriter_HappyPath enqueues one sample and
// confirms the worker POSTs it within the deadline.
func TestAsyncInfluxWriter_HappyPath(t *testing.T) {
	rec := &recordingClient{counter: &atomic.Int32{}}
	inner := NewInfluxWriter(InfluxWriterConfig{URL: "http://influx:8086"})
	inner.SetClient(rec)
	w := NewAsyncInfluxWriter(AsyncInfluxWriterConfig{
		Inner:      inner,
		BufferSize: 16,
		Workers:    1,
	})
	w.Start(context.Background())
	defer w.Close()

	w.Enqueue("h-1", Metrics{CPU: prober.CPUMetrics{Cores: 4, UsagePercent: 50.0}})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rec.counter.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := rec.counter.Load(); got != 1 {
		t.Errorf("recording client saw %d POSTs, want 1", got)
	}
}

// TestAsyncInfluxWriter_OverflowDrops: when the channel is
// full, Enqueue is non-blocking and drops the sample.
func TestAsyncInfluxWriter_OverflowDrops(t *testing.T) {
	rec := &recordingClient{
		counter:   &atomic.Int32{},
		mu:        &sync.Mutex{},
		blocking:  make(chan struct{}),
	}
	inner := NewInfluxWriter(InfluxWriterConfig{URL: "http://x"})
	inner.SetClient(rec)
	w := NewAsyncInfluxWriter(AsyncInfluxWriterConfig{
		Inner:      inner,
		BufferSize: 1,
		Workers:    1,
	})
	w.Start(context.Background())
	defer func() {
		close(rec.blocking)
		w.Close()
	}()

	for i := 0; i < 200; i++ {
		w.Enqueue("h-1", Metrics{CPU: prober.CPUMetrics{Cores: i % 8, UsagePercent: float64(i)}})
	}
	if w.Dropped() == 0 {
		t.Errorf("expected some drops, got 0")
	}
}

// TestAsyncInfluxWriter_DisabledInnerIsNoop: a zero-URL
// underlying writer turns Enqueue into a no-op.
func TestAsyncInfluxWriter_DisabledInnerIsNoop(t *testing.T) {
	rec := &recordingClient{counter: &atomic.Int32{}}
	inner := NewInfluxWriter(InfluxWriterConfig{}) // disabled
	inner.SetClient(rec)
	w := NewAsyncInfluxWriter(AsyncInfluxWriterConfig{
		Inner:      inner,
		BufferSize: 16,
		Workers:    1,
	})
	w.Start(context.Background())
	defer w.Close()

	for i := 0; i < 10; i++ {
		w.Enqueue("h-1", Metrics{CPU: prober.CPUMetrics{Cores: 1}})
	}
	time.Sleep(50 * time.Millisecond)
	if rec.counter.Load() != 0 {
		t.Errorf("disabled inner should not POST, got %d", rec.counter.Load())
	}
	if w.Dropped() != 0 {
		t.Errorf("disabled inner should not count drops, got %d", w.Dropped())
	}
}
