package logstream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// fakePublisher captures events the service fans out to
// "realtime" subscribers. The service treats it as the
// internal/ws/realtime seam — read-only from our perspective,
// but a swap-in fake for tests.
type fakePublisher struct {
	mu     sync.Mutex
	events []realtimeEvent
}

type realtimeEvent struct {
	Channel string
	Payload any
}

func (p *fakePublisher) Publish(channel string, payload any) {
	p.mu.Lock()
	p.events = append(p.events, realtimeEvent{Channel: channel, Payload: payload})
	p.mu.Unlock()
}

func (p *fakePublisher) Events() []realtimeEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]realtimeEvent, len(p.events))
	copy(out, p.events)
	return out
}

// TestService_RejectsSinceBeyond30Days is the spec scenario
// "Time range cap". The service MUST reject a `since` older
// than MaxSinceWindow with a *contracts.APIError whose code
// is CodeTimeRangeExceeded (HTTP 422) — re-using
// internal/logs.ErrTimeRangeExceeded. The streamer is NEVER
// called in this path.
func TestService_RejectsSinceBeyond30Days(t *testing.T) {
	s := NewService(&FakeStreamer{}, nil, &fakePublisher{})
	req := StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p",
		Since: time.Now().Add(-31 * 24 * time.Hour),
	}
	_, err := s.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *contracts.APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("Code = %v, want %v", apiErr.Code, contracts.CodeTimeRangeExceeded)
	}
	if apiErr.Code.HTTPStatus() != 422 {
		t.Errorf("HTTPStatus = %d, want 422", apiErr.Code.HTTPStatus())
	}
}

// TestService_AcceptsSinceWithinWindow: a `since` inside the
// 30-day window MUST be accepted and forwarded to the
// streamer. The streamer's emitted lines flow through the
// returned channel.
func TestService_AcceptsSinceWithinWindow(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{
			{Line: "hello", Stream: StreamStdout},
		},
	}
	s := NewService(st, nil, &fakePublisher{})
	req := StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p",
		Since: time.Now().Add(-2 * time.Hour),
	}
	ch, err := s.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var got []LogLine
	for l := range ch {
		got = append(got, l)
	}
	if len(got) != 1 || got[0].Line != "hello" {
		t.Errorf("lines = %+v, want [hello]", got)
	}
}

// TestService_AcceptsZeroSince: a zero `since` MUST be
// treated as "no lower bound". This is the default state for
// callers that just want to tail the live stream.
func TestService_AcceptsZeroSince(t *testing.T) {
	st := &FakeStreamer{}
	s := NewService(st, nil, &fakePublisher{})
	_, err := s.Stream(context.Background(), StreamRequest{ClusterID: "c1", Namespace: "ns", Pod: "p"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
}

// TestService_FansOutToSubscribers: the service MUST publish
// each line to the realtime publisher so other consumers
// (e.g. a connected browser tab) can see the same stream.
// The channel returned from Stream() is for the WS/SSE
// caller; the publisher is the broadcast bus.
func TestService_FansOutToSubscribers(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{
			{Line: "a", Stream: StreamStdout},
			{Line: "b", Stream: StreamStderr},
		},
	}
	pub := &fakePublisher{}
	s := NewService(st, nil, pub)
	ch, err := s.Stream(context.Background(), StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p", Follow: true,
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	// Drain so the streamer goroutine completes.
	for range ch {
	}
	events := pub.Events()
	if len(events) != 2 {
		t.Fatalf("published events = %d, want 2: %+v", len(events), events)
	}
	for _, e := range events {
		if e.Channel != "k8s.pod.log" {
			t.Errorf("Channel = %q, want k8s.pod.log", e.Channel)
		}
	}
}

// TestService_GetHistorical: the one-shot Get must use the
// LogClient seam and return its slice.
func TestService_GetHistorical(t *testing.T) {
	lc := &FakeLogClient{
		GetLogsLines: []LogLine{
			{Line: "old-1", Stream: StreamStdout},
			{Line: "old-2", Stream: StreamStdout},
		},
	}
	s := NewService(&FakeStreamer{}, lc, &fakePublisher{})
	lines, err := s.GetHistorical(context.Background(), StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p", TailLines: 10,
	})
	if err != nil {
		t.Fatalf("GetHistorical: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("lines = %+v, want 2", lines)
	}
}

// TestService_GetHistorical_RejectsSinceBeyond30Days: the
// 30-day cap applies to the one-shot path too.
func TestService_GetHistorical_RejectsSinceBeyond30Days(t *testing.T) {
	s := NewService(&FakeStreamer{}, &FakeLogClient{}, &fakePublisher{})
	_, err := s.GetHistorical(context.Background(), StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p",
		Since: time.Now().Add(-60 * 24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected error for since > 30 days")
	}
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("Code = %v, want %v", err, contracts.CodeTimeRangeExceeded)
	}
}

// TestService_GetHistorical_NilLogClient: the service must
// return a clear error when the LogClient seam is nil rather
// than nil-panicking.
func TestService_GetHistorical_NilLogClient(t *testing.T) {
	s := NewService(&FakeStreamer{}, nil, &fakePublisher{})
	_, err := s.GetHistorical(context.Background(), StreamRequest{ClusterID: "c1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestService_StreamErrorPropagates: a synchronous error
// from the streamer MUST be wrapped in *contracts.APIError
// with CodeInternal so the handler can render the standard
// envelope.
func TestService_StreamErrorPropagates(t *testing.T) {
	want := errors.New("boom")
	st := &FakeStreamer{StreamErr: want}
	s := NewService(st, &FakeLogClient{}, &fakePublisher{})
	_, err := s.Stream(context.Background(), StreamRequest{ClusterID: "c1", Namespace: "ns", Pod: "p"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want wrap of %v", err, want)
	}
}

// TestService_ContextCancelStopsStream: the consumer MUST be
// able to cancel the stream by cancelling its ctx; the
// service's wrapper goroutine observes ctx.Done() and the
// returned channel closes.
func TestService_ContextCancelStopsStream(t *testing.T) {
	stuck := make(chan LogLine)
	st := &FakeStreamer{
		StreamFn: func(_ context.Context, _ StreamRequest) <-chan LogLine {
			return stuck
		},
	}
	s := NewService(st, &FakeLogClient{}, &fakePublisher{})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.Stream(ctx, StreamRequest{ClusterID: "c1", Namespace: "ns", Pod: "p", Follow: true})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after cancel")
	}
}

// TestService_DefaultsAreApplied: when the request omits
// optional fields (TailLines, Container, Follow), the
// service fills them in before forwarding to the streamer.
// The fake records what it received — that's what we
// assert on.
func TestService_DefaultsAreApplied(t *testing.T) {
	st := &FakeStreamer{}
	s := NewService(st, &FakeLogClient{}, &fakePublisher{})
	_, err := s.Stream(context.Background(), StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p",
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	calls := st.CallsCopy()
	if len(calls) != 1 {
		t.Fatalf("streamer got %d calls, want 1", len(calls))
	}
	if calls[0].Follow {
		t.Error("Follow default should be false")
	}
	if calls[0].TailLines != 0 {
		t.Errorf("TailLines default = %d, want 0 (caller-set)", calls[0].TailLines)
	}
}

// TestService_ConcurrentSubscribersDontDeadlock: two
// subscribers reading from the same service should both
// receive lines without deadlocking. This catches the
// classic "I closed the publisher channel" bug.
func TestService_ConcurrentSubscribersDontDeadlock(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{
			{Line: "a"}, {Line: "b"}, {Line: "c"},
		},
	}
	s := NewService(st, &FakeLogClient{}, &fakePublisher{})
	var wg sync.WaitGroup
	var read1, read2 int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			ch, err := s.Stream(context.Background(), StreamRequest{
				ClusterID: "c1", Namespace: "ns", Pod: "p",
			})
			if err != nil {
				t.Errorf("subscriber %d: %v", idx, err)
				return
			}
			for range ch {
				if idx == 0 {
					atomic.AddInt32(&read1, 1)
				} else {
					atomic.AddInt32(&read2, 1)
				}
			}
		}()
	}
	wg.Wait()
	if read1 == 0 || read2 == 0 {
		t.Errorf("subscriber reads: %d / %d", read1, read2)
	}
}
