package logstream

import (
	"context"
	"errors"
	"sync"
	"time"
)

// LogClient is the seam between the streamer and the K8s
// apiserver. The production implementation (wired from main.go
// when the cluster subsystem lands) calls
// CoreV1().Pods(ns).GetLogs(name, opts).Stream(ctx); the
// test suite uses FakeLogClient.
//
// The interface deliberately does NOT expose the apiserver
// shape — the streamer asks for "stream me these logs" or
// "give me the historical tail", and the implementation
// decides how to honour that. The KubeStreamer is the only
// direct caller.
type LogClient interface {
	// StreamLogs opens a long-lived channel of log lines. The
	// channel is closed when ctx is cancelled or the underlying
	// apiserver stream ends. An error is returned only for
	// synchronous failures (e.g. malformed request); once the
	// channel is returned, late failures are signalled by
	// closing the channel and the consumer must treat that as
	// end-of-stream.
	StreamLogs(ctx context.Context, req StreamRequest) (<-chan LogLine, error)

	// GetLogs returns a bounded historical tail. Honours
	// req.TailLines (clamped to a backend-defined max) and
	// req.Since (clamped to MaxSinceWindow). Used by the
	// one-shot /logs endpoint.
	GetLogs(ctx context.Context, req StreamRequest) ([]LogLine, error)
}

// FakeLogClient is the test seam. Each method is driven by an
// optional function field; if the field is nil, the canned
// data is returned. Tests that need fine-grained control (e.g.
// "the channel blocks until cancel") set the function
// explicitly.
type FakeLogClient struct {
	// StreamLines, if set, is replayed on a buffered channel
	// from StreamLogs. The channel is closed once all lines
	// are delivered.
	StreamLines []LogLine

	// StreamChan, if set, is called instead of StreamLines.
	// Tests use this to return a custom channel (e.g. one
	// that blocks forever to test cancellation).
	StreamChan func(ctx context.Context, req StreamRequest) <-chan LogLine

	// StreamErr, if set, is returned synchronously from
	// StreamLogs (the channel is nil).
	StreamErr error

	// GetLogsLines, if set, is returned from GetLogs.
	GetLogsLines []LogLine
	// GetLogsErr, if set, is returned synchronously from
	// GetLogs.
	GetLogsErr error

	// Calls records the requests the fake received. Tests
	// assert on this to verify forwarding.
	Calls []StreamRequest
	mu   sync.Mutex
}

// record appends a request to Calls. The mutex makes the
// slice safe under the -race detector when several goroutines
// share a FakeLogClient.
func (f *FakeLogClient) record(r StreamRequest) {
	f.mu.Lock()
	f.Calls = append(f.Calls, r)
	f.mu.Unlock()
}

// CallsCopy returns a copy of the recorded requests.
func (f *FakeLogClient) CallsCopy() []StreamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]StreamRequest, len(f.Calls))
	copy(out, f.Calls)
	return out
}

// StreamLogs implements LogClient.
func (f *FakeLogClient) StreamLogs(ctx context.Context, req StreamRequest) (<-chan LogLine, error) {
	f.record(req)
	if f.StreamErr != nil {
		return nil, f.StreamErr
	}
	if f.StreamChan != nil {
		return f.StreamChan(ctx, req), nil
	}
	ch := make(chan LogLine, len(f.StreamLines))
	for _, l := range f.StreamLines {
		l := l
		ch <- l
	}
	close(ch)
	return ch, nil
}

// GetLogs implements LogClient.
func (f *FakeLogClient) GetLogs(ctx context.Context, req StreamRequest) ([]LogLine, error) {
	f.record(req)
	if f.GetLogsErr != nil {
		return nil, f.GetLogsErr
	}
	out := make([]LogLine, len(f.GetLogsLines))
	copy(out, f.GetLogsLines)
	return out, nil
}

// KubeLogClient is the production LogClient. It talks to the
// K8s apiserver via the CoreV1 REST client; we accept a
// function-shaped seam so the test suite can swap in a fake
// without importing k8s.io/client-go.
//
// In a future phase `coreV1GetLogs` will call
// `cli.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{
// 	Container: req.Container, Follow: req.Follow,
// 	SinceTime: ..., TailLines: ..., Timestamps: true}).Stream(ctx)`.
//
// Until then, KubeLogClient returns an explicit "not wired"
// error so callers don't accidentally trust a half-built
// implementation.
type KubeLogClient struct {
	// coreV1GetLogs is the apiserver seam. nil → the client is
	// "not wired" and all calls return an error.
	coreV1GetLogs func(ctx context.Context, req StreamRequest) (<-chan LogLine, error)
	// coreV1GetHistorical is the bounded-tail seam.
	coreV1GetHistorical func(ctx context.Context, req StreamRequest) ([]LogLine, error)
}

// NewKubeLogClient builds a KubeLogClient from the supplied
// function seams. Passing nil for either seam is permitted
// and the corresponding method will return a "not wired"
// error — useful during the phase where only the test path
// is exercised.
func NewKubeLogClient(
	stream func(ctx context.Context, req StreamRequest) (<-chan LogLine, error),
	historical func(ctx context.Context, req StreamRequest) ([]LogLine, error),
) *KubeLogClient {
	return &KubeLogClient{
		coreV1GetLogs:      stream,
		coreV1GetHistorical: historical,
	}
}

// errNotWired is the canonical "this production seam is not
// implemented yet" error.
var errNotWired = errors.New("k8s/logstream: KubeLogClient is not wired to a real apiserver client")

// StreamLogs implements LogClient.
func (k *KubeLogClient) StreamLogs(ctx context.Context, req StreamRequest) (<-chan LogLine, error) {
	if k.coreV1GetLogs == nil {
		return nil, errNotWired
	}
	return k.coreV1GetLogs(ctx, req)
}

// GetLogs implements LogClient.
func (k *KubeLogClient) GetLogs(ctx context.Context, req StreamRequest) ([]LogLine, error) {
	if k.coreV1GetHistorical == nil {
		return nil, errNotWired
	}
	return k.coreV1GetHistorical(ctx, req)
}

// Helper for callers that want a "now()" clock. The tests use
// time.Now directly; this indirection exists so production
// code can swap a fake clock without rewiring callers.
var nowFn = time.Now
