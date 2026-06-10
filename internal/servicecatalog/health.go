package servicecatalog

import (
	"sort"
	"time"
)

// HealthStatus is the derived health value the rollup
// returns. The values are stable strings so the frontend
// can match on them.
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"  // last run succeeded OR is recent + running
	HealthDegraded HealthStatus = "degraded" // last run failed OR is stuck running
	HealthUnknown  HealthStatus = "unknown" // no runs at all
)

// stuckRunningThreshold is the const for "a running run
// that started this long ago is no longer a healthy
// in-progress deploy — it's stuck". Future iterations can
// swap in real K8s / probe data behind the same wire
// shape; nothing the frontend renders needs to change.
const stuckRunningThreshold = 10 * time.Minute

// HealthRun is a small projection of a pipeline run used
// by the rollup. It carries just enough fields for the
// operator to see "what deploy last touched this" without
// hauling the full PipelineRun row.
type HealthRun struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
}

// HealthResult is the wire shape of GET /services/:id/health.
type HealthResult struct {
	ServiceID   string      `json:"service_id"`
	Status      HealthStatus `json:"status"`
	DerivedFrom string      `json:"derived_from"` // which signal produced the status; always "last_pipeline_run" in P0
	Reason      string      `json:"reason,omitempty"` // populated for unknown / degraded with the trigger
	LastRun     *HealthRun  `json:"last_run,omitempty"`
	RecentRuns  []HealthRun `json:"recent_runs"`
}

// RunSource is the seam the Health struct calls to read
// pipeline runs. Production wires this to the
// pipeline.Repository; tests substitute an in-memory fake.
// Defined as an interface so the service-catalog package
// does not import the pipeline package (which would be a
// cycle — pipeline already imports nothing from us).
type RunSource interface {
	// LastNRunsForService returns up to n most recent runs
	// for the service, newest first.
	LastNRunsForService(serviceID string, n int) ([]runRow, error)
}

// runRow is the package-local projection we receive from
// the RunSource. The real implementation maps the
// pipeline.Run row onto this shape.
type runRow struct {
	ID         string
	Status     string
	StartedAt  time.Time
	DurationMs int64
}

// Health is the derived-health service. It owns the
// rule that turns the last N runs into a single
// HealthStatus. The rule is intentionally simple in P0 —
// a real K8s/probe data source will replace it in P1.
type Health struct {
	src    RunSource
	stale  time.Duration // injected for tests; default = stuckRunningThreshold
}

// NewHealth builds a Health with the given RunSource.
func NewHealth(src RunSource) *Health {
	return &Health{src: src, stale: stuckRunningThreshold}
}

// Rollup computes the health for a single service. The
// returned struct is safe to JSON-encode directly.
func (h *Health) Rollup(serviceID string) (*HealthResult, error) {
	rows, err := h.src.LastNRunsForService(serviceID, 5)
	if err != nil {
		return nil, err
	}
	// Newest first (caller's contract).
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].StartedAt.After(rows[j].StartedAt)
	})

	out := &HealthResult{
		ServiceID:   serviceID,
		DerivedFrom: "last_pipeline_run",
		RecentRuns:  make([]HealthRun, 0, len(rows)),
	}
	for _, r := range rows {
		out.RecentRuns = append(out.RecentRuns, HealthRun{
			ID:         r.ID,
			Status:     r.Status,
			StartedAt:  r.StartedAt,
			DurationMs: r.DurationMs,
		})
	}
	if len(rows) == 0 {
		out.Status = HealthUnknown
		out.Reason = "no_pipeline_runs"
		return out, nil
	}
	latest := rows[0]
	latestHR := HealthRun{
		ID:         latest.ID,
		Status:     latest.Status,
		StartedAt:  latest.StartedAt,
		DurationMs: latest.DurationMs,
	}
	out.LastRun = &latestHR

	switch latest.Status {
	case "succeeded":
		out.Status = HealthHealthy
	case "failed", "cancelled":
		out.Status = HealthDegraded
		out.Reason = "last_run_" + latest.Status
	case "running", "pending":
		// Only "stuck" if older than the threshold. A
		// run that just started is still healthy in
		// progress.
		if time.Since(latest.StartedAt) > h.stale {
			out.Status = HealthDegraded
			out.Reason = "running_too_long"
		} else {
			out.Status = HealthHealthy
		}
	default:
		// Unknown status string — treat as degraded so
		// the operator notices the gap.
		out.Status = HealthDegraded
		out.Reason = "unknown_status"
	}
	return out, nil
}

// stubRunSource is the in-memory RunSource used by tests.
// Production code never sees this — it is a test-only
// helper.
type stubRunSource struct {
	svcID string
	runs  []runRow
}

func (s *stubRunSource) LastNRunsForService(serviceID string, n int) ([]runRow, error) {
	if serviceID != s.svcID {
		return nil, nil
	}
	rows := make([]runRow, len(s.runs))
	copy(rows, s.runs)
	// Newest first
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].StartedAt.After(rows[j].StartedAt)
	})
	if n > 0 && len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}
