package alerts

import (
	"context"
	"testing"
)

// fakePHProbe implements PhysicalHostMaintenanceProbe for
// the suppression integration test. The spec mandates the
// production checker consults the physicalhost service, so
// we need a tiny double that returns a configurable answer.
type fakePHProbe struct {
	hosts map[string]bool
}

func (f *fakePHProbe) IsInMaintenance(_ context.Context, hostID string) bool {
	return f.hosts[hostID]
}

// TestDefaultSuppressionChecker_NilProbeIsFalse pins the
// nil-safety contract: a deployment that forgets to wire
// the probe never suppresses anything. This is the dev
// fallback so a partial setup doesn't silently drop alerts.
func TestDefaultSuppressionChecker_NilProbeIsFalse(t *testing.T) {
	d := NewDefaultSuppressionChecker(nil)
	if d.IsInMaintenance(context.Background(), "h-1") {
		t.Error("nil probe should never suppress")
	}
}

// TestDefaultSuppressionChecker_DelegatesToProbe covers the
// real path: when the probe says the host is in
// maintenance, the suppression checker says so too.
func TestDefaultSuppressionChecker_DelegatesToProbe(t *testing.T) {
	p := &fakePHProbe{hosts: map[string]bool{
		"h-1": true,
		"h-2": false,
	}}
	d := NewDefaultSuppressionChecker(p)
	if !d.IsInMaintenance(context.Background(), "h-1") {
		t.Errorf("h-1 should be suppressed")
	}
	if d.IsInMaintenance(context.Background(), "h-2") {
		t.Errorf("h-2 should NOT be suppressed")
	}
	if d.IsInMaintenance(context.Background(), "h-unknown") {
		t.Errorf("unknown host should NOT be suppressed")
	}
}
