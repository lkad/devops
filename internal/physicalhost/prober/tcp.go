// Package prober provides production Prober implementations for the
// physical-host monitor. The Fake implementation (in prober.go) is
// the test seam; the real one lives here and is selected by main.go
// for any deployment where the host row carries a routable IP.
package prober

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCP returns a Prober that opens a TCP connection to host:port and
// measures the round-trip latency. It is intentionally minimal —
// no SSH handshake, no banner parsing — so the same code path works
// for any TCP-speaking daemon (sshd, telnet, mosh, etc.).
//
// Failures are signalled through the PingResult.Err field so the
// monitor can distinguish "host refused" from "host answered
// slowly" without changing the interface.
func TCP(timeout time.Duration) Prober {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &tcpProber{timeout: timeout}
}

type tcpProber struct {
	timeout time.Duration
}

func (p *tcpProber) Ping(ctx context.Context, host Host) (PingResult, error) {
	start := time.Now()
	dialer := &net.Dialer{Timeout: p.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host.IPAddress, host.SSHPort))
	if err != nil {
		return PingResult{Reachable: false, LatencyMs: 0, Err: err}, nil
	}
	latency := time.Since(start).Milliseconds()
	_ = conn.Close()
	return PingResult{Reachable: true, LatencyMs: latency}, nil
}

func (p *tcpProber) SSHExec(ctx context.Context, host Host, cmd string) ([]byte, error) {
	// We do not implement SSH client here — the spec's Tier 3
	// contract defers a real client to a follow-up. For now the
	// non-ping half of the interface returns "not implemented".
	return nil, fmt.Errorf("prober.TCP: SSHExec not implemented (ssh client is a Tier 3 follow-up)")
}
