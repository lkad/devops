// Boundary / edge-case tests for the metrics service.
//
// What a sane time-series backend should accept:
//   - Negative values (e.g. temperature in °C, network packet loss
//     as a signed percentage).
//   - Future-dated samples (clock skew between producer and server).
//   - Very large absolute values (counters > 2^32).
// What it should reject:
//   - NaN / Inf (would corrupt aggregates and SVG sparklines).
package metrics

import (
	"math"
	"testing"
	"time"
)

// ─── value edge cases ─────────────────────────────────────────

func TestBoundary_Ingest_AcceptsNegativeValue(t *testing.T) {
	// -273.15 is absolute zero in °C; valid for temperature metrics.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "temp_celsius",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      -273.15,
		Timestamp:  time.Now().UTC(),
		Labels:     map[string]any{"sensor": "cpu0"},
	})
	if err != nil {
		t.Errorf("negative value should be accepted, got %v", err)
	}
}

func TestBoundary_Ingest_AcceptsFutureTimestamp(t *testing.T) {
	// Clock skew between hosts can put timestamps a few seconds in
	// the future. Rejecting this would block legitimate ingest from
	// misconfigured NTP clients.
	f := newServiceFixture(t)
	future := time.Now().Add(5 * time.Minute)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu_percent",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      42.0,
		Timestamp:  future,
	})
	if err != nil {
		t.Errorf("future timestamp should be accepted, got %v", err)
	}
}

func TestBoundary_Ingest_AcceptsVeryLargeValue(t *testing.T) {
	// Network interface counters routinely exceed 2^32 (4 billion).
	// The service stores them as float64, so up to ~1.8e308 is fine.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "if_in_octets",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      1.7e10, // 17 billion, realistic for a busy NIC
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Errorf("large value should be accepted, got %v", err)
	}
}

func TestBoundary_Ingest_AcceptsVerySmallValue(t *testing.T) {
	// Subnormal / tiny values are fine for float64.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "latency_us",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      1e-6,
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Errorf("subnormal value should be accepted, got %v", err)
	}
}

func TestBoundary_Ingest_AcceptsAncientTimestamp(t *testing.T) {
	// Unix epoch is a fine default for "very old" data; storage
	// layer will serialise as datetime, which SQLite handles.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "boot_time",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      0,
		Timestamp:  time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Errorf("ancient timestamp should be accepted, got %v", err)
	}
}

func TestBoundary_Ingest_RejectsNaN(t *testing.T) {
	// NaN would break the sparkline render (SVG polyline with NaN
	// is invalid) and aggregate math (NaN poisons sums). Must be
	// rejected at the service boundary.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu_percent",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      math.NaN(),
		Timestamp:  time.Now().UTC(),
	})
	if err == nil {
		t.Errorf("NaN must be rejected; aggregator safety")
	}
}

func TestBoundary_Ingest_RejectsPositiveInf(t *testing.T) {
	// +Inf is a common bug from divide-by-zero. Aggregates and
	// sparklines cannot render it. Must be rejected.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu_percent",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      math.Inf(1),
		Timestamp:  time.Now().UTC(),
	})
	if err == nil {
		t.Errorf("+Inf must be rejected")
	}
}

func TestBoundary_Ingest_RejectsNegativeInf(t *testing.T) {
	// -Inf likewise.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu_percent",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      math.Inf(-1),
		Timestamp:  time.Now().UTC(),
	})
	if err == nil {
		t.Errorf("-Inf must be rejected")
	}
}

// ─── empty / whitespace / overlong name ──────────────────────

func TestBoundary_Ingest_RejectsWhitespaceOnlyName(t *testing.T) {
	// The existing TestService_Ingest_RejectsEmptyName covers "".
	// Whitespace-only is a sibling edge case the trim-then-check
	// path should catch.
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "   ",
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      1,
		Timestamp:  time.Now().UTC(),
	})
	if err == nil {
		t.Errorf("whitespace-only name must be rejected")
	}
}

func TestBoundary_Ingest_AcceptsOverlongName(t *testing.T) {
	// We do not currently cap metric name length; SQLite TEXT has
	// no length limit. Pin the behaviour — if a cap is added later,
	// this test will start failing and that is the right time to
	// decide on the limit.
	f := newServiceFixture(t)
	longName := "metric_" + string(make([]byte, 1000))
	for i := range longName {
		if longName[i] == 0 {
			longName = longName[:i] + "x" + longName[i+1:]
		}
	}
	_, err := f.svc.Ingest(IngestInput{
		Name:       longName,
		TargetType: "physical_host",
		TargetID:   "d1",
		Value:      1,
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Errorf("1KB metric name should be accepted, got %v", err)
	}
}
