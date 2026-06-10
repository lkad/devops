package k8s

import (
	"context"
	"errors"
	"testing"
)

// TestFakeClient_Ping_OK verifies the Fake's default Ping
// succeeds. Tests that need a failing client can override the
// PingErr / etc. fields.
func TestFakeClient_Ping_OK(t *testing.T) {
	c := &FakeClient{}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

// TestFakeClient_Ping_Error pins the failure path used by the
// Service to surface connectivity errors.
func TestFakeClient_Ping_Error(t *testing.T) {
	want := errors.New("dial tcp: connection refused")
	c := &FakeClient{PingErr: want}
	if err := c.Ping(context.Background()); !errors.Is(err, want) {
		t.Errorf("ping err = %v, want %v", err, want)
	}
}

// TestFakeClient_ListPods returns the canned pod list.
func TestFakeClient_ListPods(t *testing.T) {
	c := &FakeClient{
		Pods: []Pod{
			{Name: "p1", Namespace: "default", Phase: "Running"},
			{Name: "p2", Namespace: "default", Phase: "Running"},
		},
	}
	pods, err := c.ListPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("pods len = %d, want 2", len(pods))
	}
}

// TestFakeClient_ListDeployments returns the canned deployment
// list.
func TestFakeClient_ListDeployments(t *testing.T) {
	c := &FakeClient{
		Deployments: []Deployment{
			{Name: "web", Namespace: "default", Ready: "3/3"},
		},
	}
	deps, err := c.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("deps len = %d, want 1", len(deps))
	}
}

// TestFakeClient_ListServices returns the canned service list.
func TestFakeClient_ListServices(t *testing.T) {
	c := &FakeClient{
		Services: []ServiceEntry{
			{Name: "svc", Namespace: "default", Type: "ClusterIP"},
		},
	}
	svcs, err := c.ListServices(context.Background(), "default")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(svcs) != 1 {
		t.Errorf("svcs len = %d, want 1", len(svcs))
	}
}

// TestFakeClient_ImplementsClient pins the seam contract — the
// Fake MUST satisfy the Client interface, so the service can
// accept it via the same constructor as the real client.
func TestFakeClient_ImplementsClient(t *testing.T) {
	var _ Client = (*FakeClient)(nil)
	var _ Client = (*KubeClient)(nil)
}

// TestNewKubeClient_NotNil ensures the constructor returns
// a non-nil pointer. We pass nil as the iface — methods
// would error, but the constructor itself is nil-safe so the
// registry can still cache the result.
func TestNewKubeClient_NotNil(t *testing.T) {
	c := NewKubeClient(nil)
	if c == nil {
		t.Fatal("NewKubeClient returned nil")
	}
}

// TestNewKubeClientFromKubeconfig_InvalidContent pins the
// registry's sticky-error path: a kubeconfig that fails to
// parse surfaces an error the registry will cache.
func TestNewKubeClientFromKubeconfig_InvalidContent(t *testing.T) {
	_, err := NewKubeClientFromKubeconfig("not a kubeconfig")
	if err == nil {
		t.Fatal("expected error for malformed kubeconfig, got nil")
	}
}
