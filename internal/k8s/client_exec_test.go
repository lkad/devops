package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

// TestFakeClient_ExecInPod_RejectsEmptyCommand pins the
// sentinel-error branch: an empty argv returns
// ErrInvalidExecRequest without any internal state change.
func TestFakeClient_ExecInPod_RejectsEmptyCommand(t *testing.T) {
	c := &FakeClient{PodExecResult: &PodExecResult{ExitCode: 0}}
	_, err := c.ExecInPod(context.Background(), "default", "pod", "app", nil, 0)
	if !errors.Is(err, ErrInvalidExecRequest) {
		t.Errorf("err = %v, want ErrInvalidExecRequest", err)
	}
}

// TestFakeClient_ExecInPod_RejectsEmptyContainer pins the
// other required-input branch: an empty container is
// rejected because multi-container pods would otherwise
// produce an ambiguous "exec into which container" request.
func TestFakeClient_ExecInPod_RejectsEmptyContainer(t *testing.T) {
	c := &FakeClient{}
	_, err := c.ExecInPod(context.Background(), "default", "pod", "", []string{"ls"}, 0)
	if !errors.Is(err, ErrInvalidExecRequest) {
		t.Errorf("err = %v, want ErrInvalidExecRequest", err)
	}
}

// TestFakeClient_ExecInPod_ReturnsCannedResult pins the
// unit-test path: when a test sets f.PodExecResult, the
// fake returns it verbatim. This is how the route layer is
// tested without an apiserver round trip.
func TestFakeClient_ExecInPod_ReturnsCannedResult(t *testing.T) {
	canned := &PodExecResult{
		Stdout:   []string{"line1", "line2"},
		Stderr:   []string{},
		ExitCode: 0,
	}
	c := &FakeClient{PodExecResult: canned}
	got, err := c.ExecInPod(context.Background(), "default", "pod", "app", []string{"ls"}, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ExitCode != 0 || len(got.Stdout) != 2 || got.Stdout[0] != "line1" {
		t.Errorf("got = %+v, want +canned", got)
	}
}

// TestFakeClient_ExecInPod_PropagatesErr pins the
// error-override path: the test-fake honours f.ExecInPodErr
// the same way the other methods honour their Err* fields.
func TestFakeClient_ExecInPod_PropagatesErr(t *testing.T) {
	want := errors.New("exec: pod not found")
	c := &FakeClient{ExecInPodErr: want}
	_, err := c.ExecInPod(context.Background(), "default", "pod", "app", []string{"ls"}, 0)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestKubeClient_ExecInPod_RequiresExecConfig pins the
// production-side precondition: KubeClient built with a
// non-nil iface but no execConfig (i.e. via NewKubeClient
// rather than NewKubeClientWithExecConfig) returns a
// typed error when ExecInPod is called. The route handler
// in main.go must use NewKubeClientFromKubeconfig (which
// wires execConfig) for exec to be available.
func TestKubeClient_ExecInPod_RequiresExecConfig(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClient(cs) // no execConfig
	_, err := k.ExecInPod(context.Background(), "default", "pod", "app", []string{"ls"}, 0)
	if err == nil {
		t.Fatal("expected error for KubeClient without execConfig, got nil")
	}
	if !strings.Contains(err.Error(), "no exec-capable client") {
		t.Errorf("err = %v, want substring 'no exec-capable client'", err)
	}
}

// TestKubeClient_ExecInPod_NilIface pins the upstream
// precondition: KubeClient without an iface (the
// constructor-time nil case) surfaces a clear error rather
// than panicking on the iface call inside ExecInPod.
//
// In practice the test exercises the "KubeClient built via
// the read-only NewKubeClient" path — the constructor does
// not wire execClient, so the "no exec-capable client"
// error fires first. The iface-nil check is a
// defence-in-depth for the hypothetical "iface nil but
// execConfig set" case which cannot occur via the public
// constructors.
func TestKubeClient_ExecInPod_NilIface(t *testing.T) {
	k := NewKubeClient(nil)
	_, err := k.ExecInPod(context.Background(), "default", "pod", "app", []string{"ls"}, 0)
	if err == nil {
		t.Fatal("expected error for nil iface, got nil")
	}
	if !strings.Contains(err.Error(), "no exec-capable client") {
		t.Errorf("err = %v, want substring 'no exec-capable client'", err)
	}
}

// TestKubeClient_ExecInPod_RejectsEmptyCommand pins the
// production-side input validation. The command must be
// non-empty argv; an empty array would round-trip to the
// apiserver and surface as a generic 400 from kubelet.
func TestKubeClient_ExecInPod_RejectsEmptyCommand(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClientWithExecConfig(cs, nil)
	_, err := k.ExecInPod(context.Background(), "default", "pod", "app", nil, 0)
	if !errors.Is(err, ErrInvalidExecRequest) {
		t.Errorf("err = %v, want ErrInvalidExecRequest", err)
	}
}

// TestKubeClient_ExecInPod_RejectsEmptyContainer pins the
// other required input. Container must be explicit so
// multi-container pods do not produce an ambiguous exec.
func TestKubeClient_ExecInPod_RejectsEmptyContainer(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClientWithExecConfig(cs, nil)
	_, err := k.ExecInPod(context.Background(), "default", "pod", "", []string{"ls"}, 0)
	if !errors.Is(err, ErrInvalidExecRequest) {
		t.Errorf("err = %v, want ErrInvalidExecRequest", err)
	}
}

// TestKubeClient_ExecInPod_RejectsEmptyNamespace pins the
// remaining required input. Without a namespace the SPDY
// URL would be ambiguous (cluster-wide) and almost always
// a caller bug.
func TestKubeClient_ExecInPod_RejectsEmptyNamespace(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClientWithExecConfig(cs, nil)
	_, err := k.ExecInPod(context.Background(), "", "pod", "app", []string{"ls"}, 0)
	if !errors.Is(err, ErrInvalidExecRequest) {
		t.Errorf("err = %v, want ErrInvalidExecRequest", err)
	}
}

// TestSplitLogLines_EmptyAndCRLF pins the line-splitting
// helper. The exec path collects stdout/stderr via
// bytes.Buffer and the helper splits them on newline; CR
// line endings are stripped (Windows containers emit
// CRLF) and empty input yields an empty slice, not nil.
func TestSplitLogLines_EmptyAndCRLF(t *testing.T) {
	if got := splitLogLines(nil); got == nil {
		t.Errorf("empty input returned nil, want empty slice")
	}
	got := splitLogLines([]byte("line1\r\nline2\r\nline3\r\n"))
	want := []string{"line1", "line2", "line3"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestSplitLogLines_PreservesEmptyMiddleLines pins the
// exec-specific behaviour: empty middle lines are
// PRESERVED (unlike log lines, which are stripped) because
// the operator needs to see "command printed a blank
// line" as a literal line. Only the trailing newline is
// implicit.
func TestSplitLogLines_PreservesEmptyMiddleLines(t *testing.T) {
	got := splitLogLines([]byte("a\n\nb\n"))
	want := []string{"a", "", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (got %+v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}
