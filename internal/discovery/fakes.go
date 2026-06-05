package discovery

import (
	"context"
)

// FakeScanner is the in-memory replacement for the real network
// scanner. Tests construct it with a fixed set of hosts and an
// optional error; production code never sees it.
//
// If err is non-nil, every Scan call returns err. If err is
// context.Canceled, the fake honours a cancelled context but
// still returns the error verbatim (mirroring the real
// implementation's behaviour).
type FakeScanner struct {
	hosts []Host
	err   error
}

// NewFakeScanner builds a FakeScanner. The hosts slice is
// returned as-is; callers should not mutate it after
// construction.
func NewFakeScanner(hosts []Host, err error) *FakeScanner {
	return &FakeScanner{hosts: hosts, err: err}
}

// Scan implements Scanner. It returns the programmed hosts and
// the programmed error (in that order: error takes precedence
// so tests can assert on the failure path).
func (f *FakeScanner) Scan(ctx context.Context, cidr string) ([]Host, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	// Defensive copy so the caller's later mutations do not
	// poison the fake's state.
	out := make([]Host, len(f.hosts))
	copy(out, f.hosts)
	return out, nil
}

// FakeProber is the in-memory replacement for the real active
// prober. Tests program it with a per-IP map of ProbeResult
// values; IPs not in the map produce the zero ProbeResult
// (unreachable, no open ports, no sysDescr).
type FakeProber struct {
	results map[string]ProbeResult
	err     error
}

// NewFakeProber builds a FakeProber. The results map is
// shallow-copied during construction; the caller is free to
// mutate the original without affecting the fake.
func NewFakeProber(results map[string]ProbeResult, err error) *FakeProber {
	cp := make(map[string]ProbeResult, len(results))
	for k, v := range results {
		cp[k] = v
	}
	return &FakeProber{results: cp, err: err}
}

// Probe implements Prober. It does not surface the
// constructor-level error — the service layer's polling
// loop is built around the narrower Prober interface, where
// every probe either returns a result or the context is
// cancelled. Use ProbeWithError when the test needs to
// assert on the failure path.
func (f *FakeProber) Probe(ctx context.Context, host Host) ProbeResult {
	if err := ctx.Err(); err != nil {
		return ProbeResult{}
	}
	if f.err != nil {
		return ProbeResult{}
	}
	if r, ok := f.results[host.IPAddress]; ok {
		return r
	}
	return ProbeResult{}
}

// ProbeWithError implements ProberWithError. Tests that want to
// assert on errors (cancellation or a programmed failure) use
// this method; the service layer never calls it directly.
func (f *FakeProber) ProbeWithError(ctx context.Context, host Host) (ProbeResult, error) {
	if err := ctx.Err(); err != nil {
		return ProbeResult{}, err
	}
	if f.err != nil {
		return ProbeResult{}, f.err
	}
	if r, ok := f.results[host.IPAddress]; ok {
		return r, nil
	}
	return ProbeResult{}, nil
}
