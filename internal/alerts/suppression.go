package alerts

import (
	"context"
	"sync"
)

// SuppressionChecker reports whether a given target is currently
// in maintenance mode. The interface lives in the alerts package
// so the rest of the alerts code never has to import
// physicalhost; main.go wires the real implementation (which
// delegates to physicalhost.Service) into the service at boot.
//
// Implementations:
//
//   - DefaultSuppressionChecker: wraps a physicalhost.Service
//                                (interface typed in this file so
//                                the alerts package stays free of
//                                the physicalhost import).
//   - FakeSuppressionChecker:    in-memory map keyed by host id,
//                                used in tests.
type SuppressionChecker interface {
	IsInMaintenance(ctx context.Context, hostID string) bool
}

// PhysicalHostMaintenanceProbe is the subset of
// physicalhost.Service the DefaultSuppressionChecker depends on.
// Declared here so the alerts package does not import
// physicalhost directly; main.go passes the real
// physicalhost.Service value to NewDefaultSuppressionChecker.
type PhysicalHostMaintenanceProbe interface {
	IsInMaintenance(ctx context.Context, hostID string) bool
}

// DefaultSuppressionChecker is the production implementation.
// It delegates to a PhysicalHostMaintenanceProbe (the
// physicalhost.Service in main.go) so a host in maintenance
// suppresses outbound notifications.
type DefaultSuppressionChecker struct {
	probe PhysicalHostMaintenanceProbe
}

// NewDefaultSuppressionChecker builds a DefaultSuppressionChecker.
// A nil probe is tolerated and treated as "no maintenance", so
// the constructor never panics in dev / test setups.
func NewDefaultSuppressionChecker(p PhysicalHostMaintenanceProbe) *DefaultSuppressionChecker {
	return &DefaultSuppressionChecker{probe: p}
}

// IsInMaintenance returns true if the probe says so. A nil
// probe is a clean "false" so unit tests of downstream code do
// not have to construct a fake unless they care.
func (d *DefaultSuppressionChecker) IsInMaintenance(ctx context.Context, hostID string) bool {
	if d.probe == nil {
		return false
	}
	return d.probe.IsInMaintenance(ctx, hostID)
}

// FakeSuppressionChecker is an in-memory SuppressionChecker for
// tests. Hosts can be flipped into and out of maintenance with
// SetMaintenance; queries are guarded by a mutex so the
// race-detector is happy.
type FakeSuppressionChecker struct {
	mu    sync.Mutex
	hosts map[string]bool
}

// NewFakeSuppressionChecker returns an empty FakeSuppressionChecker.
func NewFakeSuppressionChecker() *FakeSuppressionChecker {
	return &FakeSuppressionChecker{hosts: map[string]bool{}}
}

// SetMaintenance toggles the maintenance flag for the given
// host. The setter is on the test double rather than the
// interface to keep the production seam free of write methods.
func (f *FakeSuppressionChecker) SetMaintenance(hostID string, inMaintenance bool) {
	f.mu.Lock()
	f.hosts[hostID] = inMaintenance
	f.mu.Unlock()
}

// IsInMaintenance returns the stored flag for hostID, or false
// if the host has never been set.
func (f *FakeSuppressionChecker) IsInMaintenance(_ context.Context, hostID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hosts[hostID]
}
