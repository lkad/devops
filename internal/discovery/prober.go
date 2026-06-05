package discovery

import (
	"context"
	"errors"
)

// ProbeResult is the per-host output of a probe. OpenPorts is
// the set of TCP ports that responded to a SYN; SNMPSysDescr is
// the sysDescr MIB value when an SNMP query succeeded. Both
// fields are optional — the prober may not be able to gather
// them (firewalled host, SNMP disabled) — and the service
// layer treats empty values as "unknown".
type ProbeResult struct {
	// OpenPorts is the list of TCP ports that accepted a
	// connection during the probe. nil means "no probe
	// attempted"; an explicit empty slice means "probe ran
	// and found no open ports".
	OpenPorts []int
	// SNMPSysDescr is the sysDescr.0 MIB value. Empty means
	// "no SNMP response".
	SNMPSysDescr string
	// Reachable reports whether the host responded to at
	// least one probe (ICMP, TCP, or SNMP). A false value
	// means the host is "down" from the discovery point of
	// view and should not be promoted.
	Reachable bool
}

// Prober performs active probing against a single host. The
// real implementation will use ICMP echo + TCP SYN + UDP SNMP
// GET; the Fake is the test seam.
//
// The interface is intentionally narrow (one host at a time)
// so the service can fan out probes in parallel. A bulk variant
// would couple the prober to the orchestration policy.
type Prober interface {
	Probe(ctx context.Context, host Host) ProbeResult
}

// ErrProbe is the typed sentinel returned by Prober
// implementations when a probe fails for a non-cancellation
// reason. The service layer wraps it into a 500 INTERNAL_ERROR
// for handler rendering.
var ErrProbe = errors.New("discovery: probe failed")

// ProberWithError is the wider test seam. Real Prober
// implementations may want to surface errors; the test fake
// (and any future production Prober that uses net.DialTimeout)
// returns (ProbeResult, error). The narrower Prober interface
// is what the service holds; the wider one is for tests and
// for any future batch prober that needs partial-failure
// semantics.
type ProberWithError interface {
	ProbeWithError(ctx context.Context, host Host) (ProbeResult, error)
}
