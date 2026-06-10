package servicecatalog

import (
	"context"
	"errors"
	"testing"
)

// fakeGetter is a map-keyed K8sClientGetter the unit
// tests use to drive MultiClusterK8sSource.
type fakeGetter struct {
	clients map[string]K8sClient
}

func (f *fakeGetter) ClientFor(clusterID string) (K8sClient, error) {
	c, ok := f.clients[clusterID]
	if !ok {
		return nil, ErrK8sNoClient
	}
	return c, nil
}

// fakeClient returns a hard-coded set of deployments
// per namespace. The cluster_id is filled by the
// walker, not by this fake.
type fakeClient struct {
	deps []K8sDeployment
}

func (f *fakeClient) ListDeployments(_ context.Context, _ string) ([]K8sDeployment, error) {
	return f.deps, nil
}

// TestMultiCluster_AggregatesAcrossHealthyAndDegraded
// pins the "any unhealthy pod tips the service" rule
// at the multi-cluster level: cluster 1 is healthy,
// cluster 2 has a crashlooping deployment, the
// aggregated list contains both, and the health
// rollup (verified separately) marks the service
// degraded.
func TestMultiCluster_AggregatesAcrossHealthyAndDegraded(t *testing.T) {
	getter := &fakeGetter{clients: map[string]K8sClient{
		"cl-1": &fakeClient{deps: []K8sDeployment{
			{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 3, Available: 3},
		}},
		"cl-2": &fakeClient{deps: []K8sDeployment{
			{ClusterID: "cl-2", Namespace: "default", Name: "svc-x", Replicas: 5, Available: 2},
		}},
	}}
	src := NewMultiClusterK8sSource(getter, []string{"cl-1", "cl-2"}, "default")
	out, err := src.ListDeploymentsForService(context.Background(), "svc-x")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d deployments, want 2", len(out))
	}
	if out[0].ClusterID != "cl-1" || out[1].ClusterID != "cl-2" {
		t.Errorf("cluster order = %s,%s, want cl-1,cl-2",
			out[0].ClusterID, out[1].ClusterID)
	}
	if out[0].Ready != "3/3" {
		t.Errorf("cl-1 ready = %q, want 3/3", out[0].Ready)
	}
	if out[1].Ready != "2/5" {
		t.Errorf("cl-2 ready = %q, want 2/5", out[1].Ready)
	}
}

// TestMultiCluster_SkipsClusterWithoutClient pins the
// dev path: a cluster ID with no client configured
// returns ErrK8sNoClient and the walker silently
// skips it. The aggregate list is the deployments
// from clusters that DO have a client.
func TestMultiCluster_SkipsClusterWithoutClient(t *testing.T) {
	getter := &fakeGetter{clients: map[string]K8sClient{
		"cl-1": &fakeClient{deps: []K8sDeployment{
			{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 1, Available: 1},
		}},
		// "cl-2" absent: this is the dev case.
	}}
	src := NewMultiClusterK8sSource(getter, []string{"cl-1", "cl-2"}, "default")
	out, err := src.ListDeploymentsForService(context.Background(), "svc-x")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("got %d, want 1 (cl-2 should be skipped)", len(out))
	}
	if out[0].ClusterID != "cl-1" {
		t.Errorf("cluster = %s, want cl-1", out[0].ClusterID)
	}
}

// TestMultiCluster_FiltersByServiceName pins the
// "deployment name == service name" convention: a
// deployment named "other-svc" is not returned for
// service "svc-x".
func TestMultiCluster_FiltersByServiceName(t *testing.T) {
	getter := &fakeGetter{clients: map[string]K8sClient{
		"cl-1": &fakeClient{deps: []K8sDeployment{
			{ClusterID: "cl-1", Namespace: "default", Name: "svc-x", Replicas: 1, Available: 1},
			{ClusterID: "cl-1", Namespace: "default", Name: "other-svc", Replicas: 2, Available: 2},
		}},
	}}
	src := NewMultiClusterK8sSource(getter, []string{"cl-1"}, "default")
	out, err := src.ListDeploymentsForService(context.Background(), "svc-x")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("got %d, want 1 (other-svc should be filtered out)", len(out))
	}
	if out[0].Name != "svc-x" {
		t.Errorf("name = %s, want svc-x", out[0].Name)
	}
}

// TestMultiCluster_ClientErrorSurfaces pins the "never
// degraded on a query failure" rule at the walker
// level: a client.ListDeployments error is returned
// to the caller (which marks the service
// k8s_query_failed -> unknown).
func TestMultiCluster_ClientErrorSurfaces(t *testing.T) {
	errClient := &errClient{err: errors.New("connection refused")}
	getter := &fakeGetter{clients: map[string]K8sClient{"cl-1": errClient}}
	src := NewMultiClusterK8sSource(getter, []string{"cl-1"}, "default")
	_, err := src.ListDeploymentsForService(context.Background(), "svc-x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errClient.err) {
		t.Errorf("err = %v, want %v", err, errClient.err)
	}
}

type errClient struct{ err error }

func (e *errClient) ListDeployments(_ context.Context, _ string) ([]K8sDeployment, error) {
	return nil, e.err
}
