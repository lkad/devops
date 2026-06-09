// Package prober is the seam between the physical-host monitor
// and any TCP/SSH-based reachability probe. The contract types
// (Host, PingResult, Prober) live here so the parent
// physicalhost package can import them without creating an
// import cycle (the parent calls collectors that wrap a
// Prober, and the prober implementations want the same Host
// type).
package prober

import "context"

// Host is the input to the Prober. It is a value type (no
// pointers) so the prober cannot mutate the persisted record
// during a probe. Fields are limited to what the prober needs;
// everything else (state, maintenance columns, etc.) lives on
// the PhysicalHost row and is not exposed here.
type Host struct {
	IPAddress string
	SSHPort   int
	SSHUser   string
}

// PingResult is the structured outcome of a reachability probe.
// Reachable=true means the host answered the TCP/ICMP probe;
// LatencyMs is the round-trip time in milliseconds. The Err
// field is populated on connection refused / timeout so the
// monitor can branch on error vs. clean negative.
type PingResult struct {
	Reachable bool
	LatencyMs int64
	Err       error
}

// Prober is the seam the monitor talks to. The interface is
// deliberately small so the Fake implementation is trivial and
// the real SSH/TCP client (a future phase) does not have to be
// mocked at a method-by-method level.
//
// All methods take a context so the monitor can apply a per-host
// timeout without exposing it through the interface.
type Prober interface {
	// Ping returns whether the host is reachable. Implementations
	// are expected to honour ctx.Deadline().
	Ping(ctx context.Context, host Host) (PingResult, error)

	// SSHExec runs a single command on the host and returns its
	// stdout. Implementations are expected to honour ctx.Deadline().
	SSHExec(ctx context.Context, host Host, cmd string) ([]byte, error)
}
