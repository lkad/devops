package servicecatalog

import (
	"context"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/k8s"
)

// k8sHealthSource is the seam the Health struct calls
// to read live K8s deployment data. Production wires
// this to a k8s-Cluster -> k8s.Client -> ListDeployments
// walker; tests substitute an in-memory fake.
type k8sHealthSource interface {
	// ListDeploymentsForService returns every Deployment
	// across all registered clusters whose name matches
	// the Service's name. The cluster_id is part of the
	// projection so the response can render "which
	// cluster is unhealthy".
	ListDeploymentsForService(ctx context.Context, serviceName string) ([]K8sDeploymentHealth, error)
}

// stubK8sSource is the in-memory fake the tests
// substitute for the real k8s client.
type stubK8sSource struct {
	deployments []K8sDeploymentHealth
	err         error
}

func (s *stubK8sSource) ListDeploymentsForService(_ context.Context, _ string) ([]K8sDeploymentHealth, error) {
	return s.deployments, s.err
}

// TestHealthRollup_K8sAllReady_Healthy pins the spec
// rule: every matched deployment fully ready, status is
// healthy and derived_from is k8s_pod_health.
func TestHealthRollup_K8sAllReady_Healthy(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", nil)).
		WithK8s(&stubK8sSource{deployments: []K8sDeploymentHealth{
			{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 3, Available: 3},
		}})
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthHealthy {
		t.Errorf("status = %q, want %q", out.Status, HealthHealthy)
	}
	if out.DerivedFrom != "k8s_pod_health" {
		t.Errorf("derived_from = %q, want k8s_pod_health", out.DerivedFrom)
	}
	if !out.K8s.AllReady {
		t.Errorf("k8s.all_ready = false, want true")
	}
	if len(out.K8s.Deployments) != 1 {
		t.Errorf("k8s.deployments len = %d, want 1", len(out.K8s.Deployments))
	}
}

// TestHealthRollup_K8sPartialReady_Degraded pins the
// "any pod below target tips the service to degraded"
// rule.
func TestHealthRollup_K8sPartialReady_Degraded(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "succeeded", StartedAt: time.Now().Add(-1 * time.Minute), DurationMs: 1000},
	})).WithK8s(&stubK8sSource{deployments: []K8sDeploymentHealth{
		{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 3, Available: 2},
	}})
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthDegraded {
		t.Errorf("status = %q, want %q", out.Status, HealthDegraded)
	}
	if out.Reason != "insufficient_replicas" {
		t.Errorf("reason = %q, want insufficient_replicas", out.Reason)
	}
	// The K8s signal wins even though the last run
	// was succeeded — verify last_run is still
	// reported so the operator can correlate.
	if out.LastRun == nil {
		t.Errorf("last_run should still be reported")
	}
}

// TestHealthRollup_K8sScaledToZero_Degraded pins the
// "scaled_to_zero" branch.
func TestHealthRollup_K8sScaledToZero_Degraded(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", nil)).WithK8s(&stubK8sSource{
		deployments: []K8sDeploymentHealth{
			{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 0, Available: 0},
		},
	})
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthDegraded {
		t.Errorf("status = %q, want degraded", out.Status)
	}
	if out.Reason != "scaled_to_zero" {
		t.Errorf("reason = %q, want scaled_to_zero", out.Reason)
	}
}

// TestHealthRollup_K8sQueryFailed_Unknown pins the
// "query failure must never be reported as degraded"
// rule. A network blip on the K8s client must not
// page the on-call.
func TestHealthRollup_K8sQueryFailed_Unknown(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", nil)).WithK8s(&stubK8sSource{
		err: context.DeadlineExceeded,
	})
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthUnknown {
		t.Errorf("status = %q, want unknown (query failure must not page)", out.Status)
	}
	if out.Reason != "k8s_query_failed" {
		t.Errorf("reason = %q, want k8s_query_failed", out.Reason)
	}
}

// TestHealthRollup_K8sNoMatch_FallsBackToPipeline pins
// the "no matching deployment" branch: the K8s source
// returns nothing (service is not deployed to K8s, or
// the cluster list is empty) — fall back to the P0
// pipeline-derived signal.
func TestHealthRollup_K8sNoMatch_FallsBackToPipeline(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "succeeded", StartedAt: time.Now().Add(-1 * time.Minute), DurationMs: 1000},
	})).WithK8s(&stubK8sSource{}) // empty
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthHealthy {
		t.Errorf("status = %q, want healthy (pipeline succeeded, no k8s data)", out.Status)
	}
	if out.DerivedFrom != "last_pipeline_run" {
		t.Errorf("derived_from = %q, want last_pipeline_run (fallback)", out.DerivedFrom)
	}
}

// TestHealthRollup_K8sNotWired_BehaviourUnchanged pins
// the backward-compat rule: when the k8s source is nil
// (legacy wiring, partial test setup) the response
// shape is exactly the P0 shape.
func TestHealthRollup_K8sNotWired_BehaviourUnchanged(t *testing.T) {
	h := NewHealth(seedRuns("svc-x", []fakeRun{
		{ID: "r1", Status: "failed", StartedAt: time.Now().Add(-1 * time.Minute), DurationMs: 500},
	}))
	// no WithK8s
	out, err := h.Rollup("svc-x")
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if out.Status != HealthDegraded {
		t.Errorf("status = %q, want degraded (pipeline failed)", out.Status)
	}
	if out.DerivedFrom != "last_pipeline_run" {
		t.Errorf("derived_from = %q, want last_pipeline_run (k8s not wired)", out.DerivedFrom)
	}
}

// k8sDeprecationAliasFor is a small helper that lets
// the test build a k8s.Deployment value (for the
// production adapter) without depending on the
// kubernetes/client-go package. The actual converter
// lives in main.go and is exercised by the integration
// tests when a real cluster is present.
//
// The signature is intentionally package-private; the
// test file is in `servicecatalog` and the production
// adapter is a `main.go` closure that imports
// k8s.Deployment directly.
var _ = k8s.Deployment{}
