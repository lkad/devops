package logstream

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestFakeStreamer_EmitsCannedLines is the spec scenario
// "Subscribe to pod log stream" for the unit-test seam. The
// fake just replays Lines on a buffered channel; the test
// drains it and asserts ordering.
func TestFakeStreamer_EmitsCannedLines(t *testing.T) {
	stamp := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	f := &FakeStreamer{
		Lines: []LogLine{
			{Timestamp: stamp, Line: "first", Stream: StreamStdout},
			{Timestamp: stamp.Add(time.Second), Line: "second", Stream: StreamStderr},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := f.Stream(ctx, StreamRequest{
		ClusterID: "c1", Namespace: "default", Pod: "nginx", Container: "nginx", Follow: true,
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var got []LogLine
	for line := range ch {
		got = append(got, line)
	}
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(got), got)
	}
	if got[0].Line != "first" || got[1].Line != "second" {
		t.Errorf("order broken: %+v", got)
	}
	if got[1].Stream != StreamStderr {
		t.Errorf("stream kind lost: %+v", got[1])
	}
}

// TestFakeStreamer_RespectsContextCancel: the channel MUST close
// when ctx is cancelled, even if more lines are buffered. The
// streamer spec says "the streaming endpoints MUST honour the
// request `ctx` for cancellation".
func TestFakeStreamer_RespectsContextCancel(t *testing.T) {
	f := &FakeStreamer{
		Lines: []LogLine{
			{Line: "a", Stream: StreamStdout},
			{Line: "b", Stream: StreamStdout},
			{Line: "c", Stream: StreamStdout},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := f.Stream(ctx, StreamRequest{ClusterID: "c1", Pod: "p", Namespace: "ns"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	cancel()
	deadline := time.After(2 * time.Second)
	for range ch {
		select {
		case <-deadline:
			t.Fatal("channel did not close after cancel")
		default:
		}
	}
}

// TestFakeStreamer_StreamError: when StreamErr is set, Stream
// returns it synchronously. Matches the production code path
// where a misconfigured cluster yields a 5xx before we open
// the channel.
func TestFakeStreamer_StreamError(t *testing.T) {
	want := errors.New("backend on fire")
	f := &FakeStreamer{StreamErr: want}
	ch, err := f.Stream(context.Background(), StreamRequest{ClusterID: "c1", Pod: "p", Namespace: "ns"})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if ch != nil {
		t.Errorf("expected nil channel on error, got %v", ch)
	}
}

// TestKubeStreamer_DelegatesToLogClient: the KubeStreamer
// builds on top of the k8s cluster service (which we
// represent as LogClient here) — it MUST forward the request
// shape and the context, and return the channel the
// LogClient provides.
func TestKubeStreamer_DelegatesToLogClient(t *testing.T) {
	req := StreamRequest{
		ClusterID: "c1", Namespace: "ns", Pod: "p", Container: "ctr",
		Since: time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
		Follow: true,
	}
	want := []LogLine{{Line: "hello", Stream: StreamStdout}}
	lc := &FakeLogClient{
		StreamChan: func(_ context.Context, r StreamRequest) <-chan LogLine {
			if r.ClusterID != req.ClusterID || r.Container != req.Container {
				t.Errorf("request not forwarded: %+v", r)
			}
			ch := make(chan LogLine, 1)
			ch <- want[0]
			close(ch)
			return ch
		},
	}
	s := NewKubeStreamer(lc)
	gotCh, err := s.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var got []LogLine
	for l := range gotCh {
		got = append(got, l)
	}
	if len(got) != 1 || got[0].Line != "hello" {
		t.Errorf("lines = %+v, want %+v", got, want)
	}
}

// TestKubeStreamer_NilLogClient: defensive — the streamer MUST
// fail fast with a clear error if the LogClient seam is nil.
func TestKubeStreamer_NilLogClient(t *testing.T) {
	s := &KubeStreamer{}
	_, err := s.Stream(context.Background(), StreamRequest{ClusterID: "c1"})
	if err == nil {
		t.Fatal("expected error for nil log client")
	}
}

// TestKubeStreamer_ContextCancelPropagates: the channel MUST
// close when the caller's ctx is cancelled, even mid-stream.
// The KubeStreamer wraps the LogClient's channel with a
// select so cancellation is responsive.
func TestKubeStreamer_ContextCancelPropagates(t *testing.T) {
	stuck := make(chan LogLine) // never written, never closed
	lc := &FakeLogClient{
		StreamChan: func(_ context.Context, _ StreamRequest) <-chan LogLine {
			return stuck
		},
	}
	s := NewKubeStreamer(lc)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.Stream(ctx, StreamRequest{ClusterID: "c1"})
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
