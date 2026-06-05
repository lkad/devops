package alerts

import (
	"context"
	"testing"
)

// TestFakeSuppressionChecker_Basics verifies the test double's
// setters / getters behave as expected: no hosts are suppressed
// by default, and the caller can flip a host into and out of
// maintenance explicitly.
func TestFakeSuppressionChecker_Basics(t *testing.T) {
	f := NewFakeSuppressionChecker()
	ctx := context.Background()
	if f.IsInMaintenance(ctx, "h1") {
		t.Error("h1 should not be in maintenance by default")
	}
	f.SetMaintenance("h1", true)
	if !f.IsInMaintenance(ctx, "h1") {
		t.Error("h1 should be in maintenance after SetMaintenance(true)")
	}
	f.SetMaintenance("h1", false)
	if f.IsInMaintenance(ctx, "h1") {
		t.Error("h1 should be out of maintenance after SetMaintenance(false)")
	}
}

// TestSuppressionChecker_InterfaceConformance is a compile-time
// guard: both the real and fake implementations must satisfy the
// SuppressionChecker interface. The test fails at compile time
// if a method is missing.
func TestSuppressionChecker_InterfaceConformance(t *testing.T) {
	var _ SuppressionChecker = NewFakeSuppressionChecker()
	var _ SuppressionChecker = NewDefaultSuppressionChecker(nil)
}
