package servicecatalog

import (
	"context"
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
	DerivedFrom string      `json:"derived_from"` // which signal produced the status
	Reason      string      `json:"reason,omitempty"` // populated for unknown / degraded with the trigger
	K8s         *K8sBlock   `json:"k8s,omitempty"` // omitted when no k8s data is available
	LastRun     *HealthRun  `json:"last_run,omitempty"`
	RecentRuns  []HealthRun `json:"recent_runs"`
}

// K8sBlock is the per-deployment projection the
// health rollup returns when K8s data is available. The
// field is omitted (not present at all) when the K8s
// source returns no deployments or fails — the
// frontend can distinguish "I don't know" from "I
// checked and it's fine".
type K8sBlock struct {
	Deployments []K8sDeploymentHealth `json:"deployments"`
	AllReady    bool                  `json:"all_ready"`
}

// K8sDeploymentHealth is the per-deployment entry in
// the K8sBlock. The wire shape matches the k8s
// package's Deployment closely (ready as "N/M",
// replicas + available as int32) plus a cluster_id
// so the response tells the operator which cluster
// the unhealthy pods live in.
type K8sDeploymentHealth struct {
	ClusterID string `json:"cluster_id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
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

// K8sSource is the seam the Health struct calls to
// read live K8s deployment data. Production wires this
// to a closure that walks every registered cluster's
// k8s.Client.ListDeployments; tests substitute an
// in-memory fake.
type K8sSource interface {
	ListDeploymentsForService(ctx context.Context, serviceName string) ([]K8sDeploymentHealth, error)
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
// rule that turns the last N runs (and, when wired, the
// K8s deployment data) into a single HealthStatus.
type Health struct {
	src    RunSource
	k8s    K8sSource // optional; nil means P0 behaviour
	stale  time.Duration
}

// NewHealth builds a Health with the given RunSource.
func NewHealth(src RunSource) *Health {
	return &Health{src: src, stale: stuckRunningThreshold}
}

// WithK8s injects the K8s source. Returns the receiver
// for fluent chaining, called once at startup after
// NewHealth, before serving traffic.
func (h *Health) WithK8s(k K8sSource) *Health { h.k8s = k; return h }

// Rollup computes the health for a single service. The
// returned struct is safe to JSON-encode directly.
//
// Order of evaluation:
//   1. K8s source (when wired): wins if it returns
//      degraded. Unknown / empty falls through to the
//      pipeline signal.
//   2. Pipeline source: the P0 derived signal. Always
//      populated in `last_run` / `recent_runs` so the
//      operator can correlate regardless of which signal
//      drove the status.
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
	if len(rows) > 0 {
		latest := rows[0]
		out.LastRun = &HealthRun{
			ID:         latest.ID,
			Status:     latest.Status,
			StartedAt:  latest.StartedAt,
			DurationMs: latest.DurationMs,
		}
	}

	// K8s signal first. Wins when it returns degraded
	// (the spec rule: crashlooping pods always page,
	// even if the last deploy succeeded). Unknown /
	// empty falls through to the pipeline signal.
	if h.k8s != nil {
		deps, err := h.k8s.ListDeploymentsForService(context.Background(), serviceID)
		if err != nil {
			// Query failure: NEVER degraded. Mark
			// unknown and let the operator see the
			// k8s_query_failed reason.
			out.Status = HealthUnknown
			out.Reason = "k8s_query_failed"
			out.DerivedFrom = "k8s_pod_health"
			return out, nil
		}
		if len(deps) > 0 {
			out.K8s = &K8sBlock{Deployments: deps}
			allReady := true
			for _, d := range deps {
				if d.Replicas == 0 {
					out.Status = HealthDegraded
					out.Reason = "scaled_to_zero"
					allReady = false
					break
				}
				if d.Available < d.Replicas {
					out.Status = HealthDegraded
					out.Reason = "insufficient_replicas"
					allReady = false
					break
				}
			}
			out.K8s.AllReady = allReady
			if out.Status == "" {
				out.Status = HealthHealthy
			}
			out.DerivedFrom = "k8s_pod_health"
			return out, nil
		}
		// K8s wired but no match: fall through to
		// the pipeline signal. derived_from stays
		// "last_pipeline_run".
	}

	// No K8s data (or no match): use the pipeline
	// signal. Existing P0 logic, unchanged.
	if len(rows) == 0 {
		out.Status = HealthUnknown
		out.Reason = "no_pipeline_runs"
		return out, nil
	}
	latest := rows[0]
	switch latest.Status {
	case "succeeded":
		out.Status = HealthHealthy
	case "failed", "cancelled":
		out.Status = HealthDegraded
		out.Reason = "last_run_" + latest.Status
	case "running", "pending":
		if time.Since(latest.StartedAt) > h.stale {
			out.Status = HealthDegraded
			out.Reason = "running_too_long"
		} else {
			out.Status = HealthHealthy
		}
	default:
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
