package physicalhost

import (
	"context"
	"errors"
	"testing"
)

// TestFakeProber_PingSucceeds verifies the default Fake behaviour
// when no scripted result has been queued. The default is "ping
// succeeds" so most tests do not need to script anything.
func TestFakeProber_PingSucceeds(t *testing.T) {
	p := NewFake()
	res, err := p.Ping(context.Background(), Host{IPAddress: "10.0.0.1"})
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !res.Reachable {
		t.Errorf("reachable = false, want true")
	}
	if res.LatencyMs < 0 {
		t.Errorf("latency = %d, want >= 0", res.LatencyMs)
	}
}

// TestFakeProber_PingScripted overrides the default behaviour so a
// test can simulate a connection refused.
func TestFakeProber_PingScripted(t *testing.T) {
	p := NewFake()
	p.ScriptPing("10.0.0.99", PingResult{Reachable: false, Err: errors.New("refused")})
	res, err := p.Ping(context.Background(), Host{IPAddress: "10.0.0.99"})
	if err == nil {
		t.Fatal("expected scripted error")
	}
	if res.Reachable {
		t.Errorf("reachable = true, want false")
	}
}

// TestFakeProber_SSHExecSucceeds covers the SSH command path. The
// default Fake returns an empty stdout and no error.
func TestFakeProber_SSHExecSucceeds(t *testing.T) {
	p := NewFake()
	out, err := p.SSHExec(context.Background(), Host{IPAddress: "10.0.0.1"}, "uptime")
	if err != nil {
		t.Fatalf("ssh exec: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("stdout = %q, want empty", string(out))
	}
}

// TestFakeProber_SSHExecScripted overrides the SSH result for one
// (host, command) pair. Used by tests that exercise a probe that
// pulls a script and parses it.
func TestFakeProber_SSHExecScripted(t *testing.T) {
	p := NewFake()
	p.ScriptSSHExec("10.0.0.1", "uptime", []byte("up 3 days"), nil)
	out, err := p.SSHExec(context.Background(), Host{IPAddress: "10.0.0.1"}, "uptime")
	if err != nil {
		t.Fatalf("ssh exec: %v", err)
	}
	if string(out) != "up 3 days" {
		t.Errorf("stdout = %q, want up 3 days", string(out))
	}
}

// TestFakeProber_CallCounts lets assertions verify that a probe
// path actually invoked the Prober (e.g. that the monitor loop
// did the SSH call, not just relied on cached state).
func TestFakeProber_CallCounts(t *testing.T) {
	p := NewFake()
	host := Host{IPAddress: "10.0.0.1"}
	_, _ = p.Ping(context.Background(), host)
	_, _ = p.Ping(context.Background(), host)
	_, _ = p.SSHExec(context.Background(), host, "uptime")
	if got := p.PingCount(host.IPAddress); got != 2 {
		t.Errorf("ping count = %d, want 2", got)
	}
	if got := p.SSHExecCount(host.IPAddress, "uptime"); got != 1 {
		t.Errorf("ssh-exec count = %d, want 1", got)
	}
}
