package servicecatalog

import (
	"testing"
	"time"
)

// stubPipelineRuns lets the health-rollup tests seed
// runs without importing the pipeline package (which would
// be a circular import — servicecatalog depends on pipeline,
// not the other way around). The real run-source is the
// pipeline.Repository; for the unit test we drive the
// rollup logic against an in-package fake.
//
// We exercise the rollup via a small seam: HealthQuerier
// is the interface the rollup calls, and the test
// substitutes an in-memory implementation.
type fakeRun struct {
	ID         string
	Status     string
	StartedAt  time.Time
	DurationMs int64
}

func seedRuns(svcID string, runs []fakeRun) *stubRunSource {
	s := &stubRunSource{svcID: svcID}
	for _, r := range runs {
		s.runs = append(s.runs, runRow{
			ID:         r.ID,
			Status:     r.Status,
			StartedAt:  r.StartedAt,
			DurationMs: r.DurationMs,
		})
	}
	return s
}

// TestHealthRollup_NoRuns_ReturnsUnknown pins the "no
// pipeline runs yet" case from the spec.
func TestHealthRollup_NoRuns_ReturnsUnknown(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", nil))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthUnknown {
		t.Errorf("status = %q, want %q", out.Status, HealthUnknown)
	}
	if out.Reason != "no_pipeline_runs" {
		t.Errorf("reason = %q, want %q", out.Reason, "no_pipeline_runs")
	}
	if out.LastRun != nil {
		t.Errorf("last_run = %+v, want nil", out.LastRun)
	}
}

// TestHealthRollup_LastSucceeded_ReturnsHealthy pins the
// happy path: a service whose most recent run is "succeeded"
// is healthy.
func TestHealthRollup_LastSucceeded_ReturnsHealthy(t *testing.T) {
	now := time.Now()
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "succeeded", StartedAt: now.Add(-1 * time.Minute), DurationMs: 1000},
	}))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthHealthy {
		t.Errorf("status = %q, want %q", out.Status, HealthHealthy)
	}
	if out.DerivedFrom != "last_pipeline_run" {
		t.Errorf("derived_from = %q, want %q", out.DerivedFrom, "last_pipeline_run")
	}
	if out.LastRun == nil || out.LastRun.ID != "r1" {
		t.Errorf("last_run = %+v, want r1", out.LastRun)
	}
}

// TestHealthRollup_LastFailed_ReturnsDegraded pins the
// failure path.
func TestHealthRollup_LastFailed_ReturnsDegraded(t *testing.T) {
	now := time.Now()
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "failed", StartedAt: now.Add(-5 * time.Minute), DurationMs: 500},
	}))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthDegraded {
		t.Errorf("status = %q, want %q", out.Status, HealthDegraded)
	}
}

// TestHealthRollup_StuckRunning_ReturnsDegraded pins the
// "running for > 10 minutes" path. The threshold is a
// const for now.
func TestHealthRollup_StuckRunning_ReturnsDegraded(t *testing.T) {
	now := time.Now()
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "running", StartedAt: now.Add(-15 * time.Minute), DurationMs: 0},
	}))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthDegraded {
		t.Errorf("status = %q, want %q (stuck running > 10m)", out.Status, HealthDegraded)
	}
}

// TestHealthRollup_RecentRunning_ReturnsHealthy pins that
// a run currently in progress (started < 10m ago) is
// not yet "stuck" and is reported as healthy.
func TestHealthRollup_RecentRunning_ReturnsHealthy(t *testing.T) {
	now := time.Now()
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "running", StartedAt: now.Add(-2 * time.Minute), DurationMs: 0},
	}))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthHealthy {
		t.Errorf("status = %q, want %q (recently started running)", out.Status, HealthHealthy)
	}
}

// TestHealthRollup_RecentRunsLimitedTo5 pins the spec's
// "last 5 runs" requirement.
func TestHealthRollup_RecentRunsLimitedTo5(t *testing.T) {
	now := time.Now()
	runs := make([]fakeRun, 8)
	for i := range runs {
		runs[i] = fakeRun{ID: "r" + string(rune('0'+i)), Status: "succeeded", StartedAt: now.Add(-time.Duration(i) * time.Minute), DurationMs: 100}
	}
	h := NewHealth(seedRuns("svc-x", runs))
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if len(out.RecentRuns) != 5 {
		t.Errorf("recent_runs len = %d, want 5", len(out.RecentRuns))
	}
	// Newest first: r0 should be at index 0.
	if out.RecentRuns[0].ID != "r0" {
		t.Errorf("recent_runs[0] = %q, want r0", out.RecentRuns[0].ID)
	}
}
