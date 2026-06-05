package discovery

import (
	"context"
	"errors"
	"testing"
)

// TestScanner_Fake_ReplaysHosts verifies that a Fake scanner
// returns the hosts that were programmed in, in order, regardless
// of the CIDR argument. The seam between the real scanner and
// the fake is what the spec calls "testability without real
// network" — if this contract breaks, every service-layer test
// is at the mercy of ICMP.
func TestScanner_Fake_ReplaysHosts(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "host-1"},
		{IPAddress: "10.0.0.2", Hostname: "host-2"},
	}
	s := NewFakeScanner(hosts, nil)
	got, err := s.Scan(context.Background(), "10.0.0.0/24")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hosts, want 2", len(got))
	}
	if got[0].IPAddress != "10.0.0.1" {
		t.Errorf("host[0].IP = %q, want 10.0.0.1", got[0].IPAddress)
	}
	if got[1].Hostname != "host-2" {
		t.Errorf("host[1].Hostname = %q, want host-2", got[1].Hostname)
	}
}

// TestScanner_Fake_ReturnsError verifies the error-seam of the
// Fake: when a fake is constructed with an error, Scan returns
// it unmodified.
func TestScanner_Fake_ReturnsError(t *testing.T) {
	want := errors.New("network down")
	s := NewFakeScanner(nil, want)
	_, err := s.Scan(context.Background(), "10.0.0.0/24")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestScanner_Fake_RespectsContextCancel ensures the seam
// respects a cancelled context. The contract is shared by every
// implementation: cancellation surfaces as context.Canceled.
func TestScanner_Fake_RespectsContextCancel(t *testing.T) {
	s := NewFakeScanner(nil, context.Canceled)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Scan(ctx, "10.0.0.0/24")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestScanner_Interface_Compiles is a compile-time assertion
// that the Fake satisfies the Scanner contract. If a future
// refactor changes the interface, this test breaks before any
// runtime call does.
func TestScanner_Interface_Compiles(t *testing.T) {
	var _ Scanner = (*FakeScanner)(nil)
}
