package logstream

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Streamer is the contract the service uses to obtain a
// channel of log lines. Implementations live in this file
// (KubeStreamer, FakeStreamer) and are seam-able so the
// service is unit-testable without a real K8s cluster.
type Streamer interface {
	// Stream returns a channel of LogLine. The channel is
	// closed when the source ends OR when ctx is cancelled,
	// whichever happens first. A non-nil error is returned
	// only for synchronous failures (e.g. invalid request,
	// backend unavailable); once the channel is returned, the
	// consumer treats its closure as end-of-stream.
	Stream(ctx context.Context, req StreamRequest) (<-chan LogLine, error)
}

// FakeStreamer is the test streamer. It replays Lines on a
// buffered channel, honouring ctx cancellation and
// StreamErr.
type FakeStreamer struct {
	// Lines is the canned backlog. Stream emits them in order
	// on a buffered channel and closes it.
	Lines []LogLine
	// StreamErr short-circuits the call (channel is nil).
	StreamErr error
	// StreamFn, if set, replaces the canned replay. Tests
	// use it to inject a custom channel (e.g. one that
	// blocks forever to verify cancellation).
	StreamFn func(ctx context.Context, req StreamRequest) <-chan LogLine
	// Calls records the requests the fake received.
	mu    sync.Mutex
	Calls []StreamRequest
}

// Stream implements Streamer.
func (f *FakeStreamer) Stream(ctx context.Context, req StreamRequest) (<-chan LogLine, error) {
	if f.StreamErr != nil {
		f.mu.Lock()
		f.Calls = append(f.Calls, req)
		f.mu.Unlock()
		return nil, f.StreamErr
	}
	if f.StreamFn != nil {
		f.mu.Lock()
		f.Calls = append(f.Calls, req)
		f.mu.Unlock()
		return f.streamWithCancel(ctx, f.StreamFn(ctx, req)), nil
	}
	f.mu.Lock()
	f.Calls = append(f.Calls, req)
	f.mu.Unlock()
	src := make(chan LogLine, len(f.Lines))
	for _, l := range f.Lines {
		l := l
		src <- l
	}
	close(src)
	return f.streamWithCancel(ctx, src), nil
}

// CallsCopy returns a copy of the recorded requests. Tests
// use this to assert on forwarding without racing against
// the handler's writer goroutine.
func (f *FakeStreamer) CallsCopy() []StreamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]StreamRequest, len(f.Calls))
	copy(out, f.Calls)
	return out
}

// streamWithCancel wraps src in a goroutine that closes the
// returned channel when src closes OR ctx is cancelled. This
// is the cancellation path the spec demands for the
// streaming endpoints.
func (f *FakeStreamer) streamWithCancel(ctx context.Context, src <-chan LogLine) <-chan LogLine {
	out := make(chan LogLine)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-src:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- line:
				}
			}
		}
	}()
	return out
}

// KubeStreamer wires a Streamer to a LogClient. The streamer
// owns no state of its own — it just plumbs the request shape
// and the cancellation signal.
type KubeStreamer struct {
	lc LogClient
}

// NewKubeStreamer builds a KubeStreamer from a LogClient. A
// nil LogClient is tolerated (Stream returns an explicit
// error) so partial wiring is fail-loud.
func NewKubeStreamer(lc LogClient) *KubeStreamer {
	return &KubeStreamer{lc: lc}
}

// Stream implements Streamer.
func (k *KubeStreamer) Stream(ctx context.Context, req StreamRequest) (<-chan LogLine, error) {
	if k.lc == nil {
		return nil, errors.New("k8s/logstream: KubeStreamer has no LogClient")
	}
	src, err := k.lc.StreamLogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("k8s/logstream: StreamLogs: %w", err)
	}
	out := make(chan LogLine)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-src:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- line:
				}
			}
		}
	}()
	return out, nil
}

// BackpressureSink is the backpressure policy. The handler
// hands lines to a BackpressureSink; when the consumer falls
// behind by more than MaxLines, the oldest buffered line is
// dropped and the `dropped` counter is incremented.
//
// The counter is exposed via Snapshot() so the handler can
// stamp it on the next outgoing frame. We use an int64
// instead of an int so the wire format can hold a 32-bit
// counter without overflow.
type BackpressureSink struct {
	MaxLines int
	dropped  int64
	mu       sync.Mutex
	// buffered is a FIFO of pending lines, capped at
	// MaxLines. When full, the head is dropped on Push.
	buffered []LogLine
}

// NewBackpressureSink constructs a sink. MaxLines <= 0 falls
// back to DefaultMaxBackpressureLines.
func NewBackpressureSink(max int) *BackpressureSink {
	if max <= 0 {
		max = DefaultMaxBackpressureLines
	}
	return &BackpressureSink{MaxLines: max}
}

// Push appends a line. If the buffer is full, the oldest
// line is dropped and the counter is incremented. The caller
// does not need to know whether a drop occurred — the next
// Snapshot() will reveal it.
func (b *BackpressureSink) Push(line LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffered) >= b.MaxLines {
		// Drop oldest.
		b.buffered = b.buffered[1:]
		b.dropped++
	}
	b.buffered = append(b.buffered, line)
}

// Pop removes and returns the oldest line. The boolean is
// false when the buffer is empty.
func (b *BackpressureSink) Pop() (LogLine, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffered) == 0 {
		return LogLine{}, false
	}
	line := b.buffered[0]
	b.buffered = b.buffered[1:]
	return line, true
}

// Snapshot returns the current dropped count.
func (b *BackpressureSink) Snapshot() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}

// Reset clears the counter and the buffer. Used between
// stream sessions in tests.
func (b *BackpressureSink) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dropped = 0
	b.buffered = b.buffered[:0]
}

// nowFn is exposed for tests that want to freeze time. The
// production code path uses time.Now via the indirection
// declared in logclient.go; we re-export the same variable
// here so the test helper can swap it for both packages at
// once. It is set in an init() so the linker does not strip
// it; the production code never reads it.
var _ = func() {}
