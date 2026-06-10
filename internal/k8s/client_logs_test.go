package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// TestParseLogLines_SingleLine pins the line-splitting
// contract: one line, no leading timestamp, becomes one
// LogEntry with zero Timestamp and the whole line in
// LogEntry.Line. This is the common shape for raw file log
// backends that don't prefix timestamps.
func TestParseLogLines_SingleLine(t *testing.T) {
	out := parseLogLines("pod-a", "app", []byte("hello world"))
	if len(out) != 1 {
		t.Fatalf("entries = %d, want 1", len(out))
	}
	if out[0].Pod != "pod-a" || out[0].Container != "app" {
		t.Errorf("labels = %+v, want pod=pod-a, container=app", out[0])
	}
	if !out[0].Timestamp.IsZero() {
		t.Errorf("timestamp = %v, want zero", out[0].Timestamp)
	}
	if out[0].Line != "hello world" {
		t.Errorf("line = %q, want %q", out[0].Line, "hello world")
	}
}

// TestParseLogLines_TimestampPrefix pins the apiserver
// shape: "2026-06-10T12:00:00Z message body" splits into
// Timestamp=2026-06-10T12:00:00Z + Line="message body".
func TestParseLogLines_TimestampPrefix(t *testing.T) {
	out := parseLogLines("pod-a", "app", []byte("2026-06-10T12:00:00Z message body"))
	if len(out) != 1 {
		t.Fatalf("entries = %d, want 1", len(out))
	}
	got := out[0].Timestamp
	want := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("timestamp = %v, want %v", got, want)
	}
	if out[0].Line != "message body" {
		t.Errorf("line = %q, want %q", out[0].Line, "message body")
	}
}

// TestParseLogLines_TimestampNanos pins the nano-precision
// variant. Apiserver can emit nanosecond precision under
// high-throughput conditions.
func TestParseLogLines_TimestampNanos(t *testing.T) {
	out := parseLogLines("pod-a", "app", []byte("2026-06-10T12:00:00.123456789Z nano body"))
	if len(out) != 1 {
		t.Fatalf("entries = %d, want 1", len(out))
	}
	if out[0].Line != "nano body" {
		t.Errorf("line = %q, want %q", out[0].Line, "nano body")
	}
	if out[0].Timestamp.Nanosecond() != 123456789 {
		t.Errorf("nanos = %d, want 123456789", out[0].Timestamp.Nanosecond())
	}
}

// TestParseLogLines_MultipleLinesAndBlankSkip pins the
// multi-line + blank-line skip behaviour. Three real lines
// + one blank line = three LogEntries.
func TestParseLogLines_MultipleLinesAndBlankSkip(t *testing.T) {
	body := []byte("line1\n\nline2\nline3\n")
	out := parseLogLines("pod-a", "app", body)
	if len(out) != 3 {
		t.Fatalf("entries = %d, want 3 (blank line skipped)", len(out))
	}
	if out[0].Line != "line1" || out[1].Line != "line2" || out[2].Line != "line3" {
		t.Errorf("lines = [%q %q %q]", out[0].Line, out[1].Line, out[2].Line)
	}
}

// TestParseLogLines_EmptyBodyReturnsEmpty pins the no-data
// branch: an empty body yields zero LogEntries, not nil
// (the caller is expected to iterate and a nil slice would
// cause a different code path than an empty slice).
func TestParseLogLines_EmptyBodyReturnsEmpty(t *testing.T) {
	out := parseLogLines("pod-a", "app", nil)
	if out == nil {
		t.Fatal("out = nil, want empty slice")
	}
	if len(out) != 0 {
		t.Errorf("entries = %d, want 0", len(out))
	}
}

// TestSplitLeadingRFC3339_NotATimestamp pins the defensive
// branch: a line that LOOKS like it could have a date (4th
// char is "-") but isn't actually an RFC3339 timestamp is
// returned unchanged with a zero time.
func TestSplitLeadingRFC3339_NotATimestamp(t *testing.T) {
	ts, rest := splitLeadingRFC3339("2026-06-10 not a time")
	if !ts.IsZero() || rest != "2026-06-10 not a time" {
		t.Errorf("got ts=%v rest=%q, want zero + unchanged", ts, rest)
	}
}

// TestSplitLeadingRFC3339_TooShort pins the early-out: a
// line shorter than 20 chars cannot contain an RFC3339
// timestamp (the shortest is "2006-01-02T15:04:05Z" at 20).
func TestSplitLeadingRFC3339_TooShort(t *testing.T) {
	ts, rest := splitLeadingRFC3339("short")
	if !ts.IsZero() || rest != "short" {
		t.Errorf("got ts=%v rest=%q, want zero + unchanged", ts, rest)
	}
}

// TestFakeClient_GetLogsBySelector_RequiresNamespace pins
// the sentinel-error branch: an empty namespace returns
// ErrInvalidLogQuery so the caller knows the call is
// malformed rather than silently returning an empty slice.
func TestFakeClient_GetLogsBySelector_RequiresNamespace(t *testing.T) {
	c := &FakeClient{LogEntries: []LogEntry{{Pod: "p1", Line: "x"}}}
	_, err := c.GetLogsBySelector(context.Background(), "", "app=x", LogQuery{})
	if !errors.Is(err, ErrInvalidLogQuery) {
		t.Errorf("err = %v, want ErrInvalidLogQuery", err)
	}
}

// TestFakeClient_GetLogsBySelector_ContainerFilter pins the
// optional Container field on LogQuery: only entries with
// the matching container pass the filter.
func TestFakeClient_GetLogsBySelector_ContainerFilter(t *testing.T) {
	c := &FakeClient{LogEntries: []LogEntry{
		{Pod: "p1", Container: "app", Line: "a"},
		{Pod: "p1", Container: "sidecar", Line: "s"},
		{Pod: "p1", Container: "app", Line: "a2"},
	}}
	out, err := c.GetLogsBySelector(context.Background(), "default", "app=x", LogQuery{Container: "app"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("entries = %d, want 2 (only app container)", len(out))
	}
	for _, e := range out {
		if e.Container != "app" {
			t.Errorf("entry %+v has non-app container", e)
		}
	}
}

// TestKubeClient_GetLogsBySelector_FanOutAndMerge pins the
// production path: two pods match the label selector, each
// returns a canned log body, and the result has both
// entries with the right pod + container labels.
//
// The fake clientset's GetLogs expects the reactor to
// return a *runtime.Unknown wrapping the canned bytes.
// Without that exact shape, the fake's default path kicks
// in and produces a generic "fake logs" body. We use the
// runtime.Unknown shape to ship deterministic content.
func TestKubeClient_GetLogsBySelector_FanOutAndMerge(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "p1",
				Namespace: "default",
				Labels:    map[string]string{"app": "x"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "p2",
				Namespace: "default",
				Labels:    map[string]string{"app": "x"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}},
		},
	)
	cs.PrependReactor("get", "pods/log", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &runtime.Unknown{Raw: []byte("2026-06-10T12:00:00Z hello from main\n")}, nil
	})
	k := NewKubeClient(cs)
	out, err := k.GetLogsBySelector(context.Background(), "default", "app=x", LogQuery{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("entries = %d, want 2 (one per matching pod)", len(out))
	}
	pods := map[string]bool{}
	for _, e := range out {
		pods[e.Pod] = true
		if e.Container != "main" {
			t.Errorf("entry %+v has container=%q, want main", e, e.Container)
		}
		if e.Line != "hello from main" {
			t.Errorf("entry %+v has line=%q, want %q", e, e.Line, "hello from main")
		}
		if e.Timestamp.IsZero() {
			t.Errorf("entry %+v has zero timestamp", e)
		}
	}
	if !pods["p1"] || !pods["p2"] {
		t.Errorf("entries missing p1 or p2: %+v", out)
	}
}

// TestKubeClient_GetLogsBySelector_NilIface pins the
// nil-safety branch the registry depends on: a KubeClient
// without an iface must surface a clear error rather than
// panicking when the caller fans out a log query.
func TestKubeClient_GetLogsBySelector_NilIface(t *testing.T) {
	k := NewKubeClient(nil)
	_, err := k.GetLogsBySelector(context.Background(), "default", "app=x", LogQuery{})
	if err == nil {
		t.Fatal("expected error for nil iface, got nil")
	}
	if !strings.Contains(err.Error(), "not wired") {
		t.Errorf("err = %v, want substring 'not wired'", err)
	}
}

// TestKubeClient_GetLogsBySelector_RejectsEmptyNamespace
// pins the production-side validation: an empty namespace
// returns ErrInvalidLogQuery from the production client
// too, not just the fake.
func TestKubeClient_GetLogsBySelector_RejectsEmptyNamespace(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClient(cs)
	_, err := k.GetLogsBySelector(context.Background(), "", "app=x", LogQuery{})
	if !errors.Is(err, ErrInvalidLogQuery) {
		t.Errorf("err = %v, want ErrInvalidLogQuery", err)
	}
}

// TestKubeClient_GetLogsBySelector_AppliesLimitToList pins
// the safety cap: even with 20 matching pods, only the
// first maxPods (=LogQuery.MaxPods) are fanned out. The
// fake clientset honours ListOptions.Limit, so we can
// assert on the call via the Reactor count.
func TestKubeClient_GetLogsBySelector_AppliesLimitToList(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default", Labels: map[string]string{"app": "x"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "default", Labels: map[string]string{"app": "x"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p3", Namespace: "default", Labels: map[string]string{"app": "x"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}}},
	)
	var listCalls int
	cs.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		listCalls++
		limit := action.(clienttesting.ListAction).GetListRestrictions().Fields
		_ = limit
		return false, nil, nil
	})
	cs.PrependReactor("get", "pods/log", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	k := NewKubeClient(cs)
	out, err := k.GetLogsBySelector(context.Background(), "default", "app=x", LogQuery{MaxPods: 2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// MaxPods=2 caps the list to 2, so the result should
	// have 2 entries (one per pod), not 3.
	if len(out) != 2 {
		t.Errorf("entries = %d, want 2 (MaxPods=2)", len(out))
	}
	if listCalls != 1 {
		t.Errorf("list calls = %d, want 1", listCalls)
	}
}
