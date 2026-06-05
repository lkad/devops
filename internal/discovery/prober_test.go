package discovery

import (
	"context"
	"errors"
	"testing"
)

// TestProber_Fake_ReplaysResult verifies the Fake's basic seam:
// programmed ProbeResults are returned unchanged for the
// matching IP. IPs not in the map yield the zero ProbeResult.
func TestProber_Fake_ReplaysResult(t *testing.T) {
	results := map[string]ProbeResult{
		"10.0.0.1": {
			OpenPorts:    []int{22, 80},
			SNMPSysDescr: "Linux router 1.0",
		},
		"10.0.0.2": {
			OpenPorts: []int{161},
		},
	}
	p := NewFakeProber(results, nil)
	got := p.Probe(context.Background(), Host{IPAddress: "10.0.0.1"})
	if len(got.OpenPorts) != 2 || got.OpenPorts[0] != 22 {
		t.Errorf("OpenPorts = %v, want [22 80]", got.OpenPorts)
	}
	if got.SNMPSysDescr != "Linux router 1.0" {
		t.Errorf("SNMPSysDescr = %q", got.SNMPSysDescr)
	}
	zero := p.Probe(context.Background(), Host{IPAddress: "10.0.0.99"})
	if len(zero.OpenPorts) != 0 {
		t.Errorf("zero result has OpenPorts = %v, want []", zero.OpenPorts)
	}
	if zero.SNMPSysDescr != "" {
		t.Errorf("zero result has SNMPSysDescr = %q, want empty", zero.SNMPSysDescr)
	}
}

// TestProber_Fake_ReturnsErrorOnCancel verifies the fake
// returns context.Canceled when the context is cancelled,
// regardless of the programmed results. This mirrors the
// production behaviour: cancellation always wins.
func TestProber_Fake_ReturnsErrorOnCancel(t *testing.T) {
	p := NewFakeProber(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.ProbeWithError(ctx, Host{IPAddress: "10.0.0.1"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestProber_Fake_ReturnsProgrammedError verifies the fake
// surfaces the constructor error when the context is healthy.
// The seam is what lets tests assert on the failure path
// without setting up a real network.
func TestProber_Fake_ReturnsProgrammedError(t *testing.T) {
	want := errors.New("snmp timeout")
	p := NewFakeProber(nil, want)
	_, err := p.ProbeWithError(context.Background(), Host{IPAddress: "10.0.0.1"})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestProber_Interface_Compiles is a compile-time check that
// the Fake satisfies the Prober interface. If the interface
// changes, this test breaks before the runtime tests do.
func TestProber_Interface_Compiles(t *testing.T) {
	var _ Prober = (*FakeProber)(nil)
}
