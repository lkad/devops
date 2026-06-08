// Boundary / edge-case tests for the alerts service.
//
// Focuses on the four things that tend to bite ops in production:
//   - empty / whitespace fields slipping through validation
//   - overlong names that might break UI tables
//   - severity / state combinations that the spec didn't pin
package alerts

import (
	"context"
	"strings"
	"testing"
)

// ─── channel edge cases ────────────────────────────────────────
//
// Channel has no Name field (channels are typed by ChannelType).
// The interesting edge cases are empty config and unknown types.

func TestBoundary_Channel_LogTypeWithEmptyConfigIsAccepted(t *testing.T) {
	// The "log" channel type legitimately needs no config — it
	// just records to slog. Rejecting empty config would force the
	// caller to fabricate a key.
	svc, _, _, _ := newServiceWithDB(t)
	c, err := svc.CreateChannel(CreateChannelInput{
		Type:    ChannelTypeLog,
		Enabled: true,
	})
	if err != nil {
		t.Errorf("log channel with no config should be accepted, got %v", err)
	}
	if c == nil {
		t.Fatalf("returned channel is nil")
	}
}

func TestBoundary_Channel_UnknownTypeIsRejected(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	_, err := svc.CreateChannel(CreateChannelInput{
		Type:    "pagerfax", // not a real channel type
		Enabled: true,
	})
	if err == nil {
		t.Errorf("unknown channel type must be rejected")
	}
}

// ─── alert edge cases ──────────────────────────────────────────

func TestBoundary_Alert_EmptyNameIsRejected(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	_, err := svc.Fire(context.Background(), FireInput{
		Name:       "",
		Severity:   SeverityWarning,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "d1",
	})
	if err == nil {
		t.Errorf("empty alert name must be rejected")
	}
}

func TestBoundary_Alert_UnknownSeverityIsRejected(t *testing.T) {
	// A misbehaving alert producer might send severity "critical!!"
	// (with extra punctuation). The service must reject because
	// the dispatcher and the front-end severity badge both branch
	// on the enum value; silently normalising would hide the bug.
	svc, _, _, _ := newServiceWithDB(t)
	_, err := svc.Fire(context.Background(), FireInput{
		Name:       "weird-sev",
		Severity:   "critical!!",
		SourceType: SourceTypePhysicalHost,
		SourceID:   "d1",
	})
	if err == nil {
		t.Errorf("unknown severity must be rejected")
	}
}

func TestBoundary_Alert_EmptySourceIDIsRejected(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	_, err := svc.Fire(context.Background(), FireInput{
		Name:       "orphan",
		Severity:   SeverityInfo,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "",
	})
	if err == nil {
		t.Errorf("empty source_id must be rejected")
	}
}

func TestBoundary_Alert_OverlongNameAccepted(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	long := strings.Repeat("a", 1000)
	a, err := svc.Fire(context.Background(), FireInput{
		Name:       long,
		Severity:   SeverityInfo,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "d1",
	})
	if err != nil {
		t.Errorf("1KB alert name should be accepted, got %v", err)
	}
	if a == nil || a.Name != long {
		t.Errorf("name was not round-tripped verbatim")
	}
}

