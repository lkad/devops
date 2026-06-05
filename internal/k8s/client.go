package k8s

import (
	"context"
	"fmt"
)

// Pod is the wire shape for a Kubernetes pod. Fields are
// limited to what the current read-only flows need; richer
// fields (e.g. nodeName) can be added without breaking the
// JSON contract.
type Pod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
	NodeName  string `json:"node_name,omitempty"`
	Ready     string `json:"ready,omitempty"` // "1/1"
}

// Deployment is the wire shape for a Kubernetes deployment.
type Deployment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     string `json:"ready"` // "3/3"
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
}

// ServiceEntry is the wire shape for a Kubernetes service.
// Named with the "Entry" suffix so it does not collide with
// the Service *type* (the package's business-logic struct).
type ServiceEntry struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`     // ClusterIP, NodePort, LoadBalancer
	ClusterIP string `json:"cluster_ip"`
}

// Client is the seam between the k8s subsystem and the
// Kubernetes API. The interface is intentionally tiny — only
// the read paths used by /api/v1/k8s/clusters/:id/{pods,
// deployments, services} and the connectivity probe. A real
// implementation lives in KubeClient; tests use FakeClient.
type Client interface {
	// Ping verifies the API server is reachable and the
	// supplied credentials are valid. It returns nil on
	// success, an error on failure.
	Ping(ctx context.Context) error

	// ListPods returns the pods in the given namespace.
	// An empty namespace means "all namespaces".
	ListPods(ctx context.Context, namespace string) ([]Pod, error)

	// ListDeployments returns the deployments in the given
	// namespace. An empty namespace means "all namespaces".
	ListDeployments(ctx context.Context, namespace string) ([]Deployment, error)

	// ListServices returns the services in the given
	// namespace. An empty namespace means "all namespaces".
	ListServices(ctx context.Context, namespace string) ([]ServiceEntry, error)
}

// FakeClient is an in-memory Client for unit tests. Each
// method returns the canned field value, or the canned error
// if the corresponding Err* field is set.
type FakeClient struct {
	Pods        []Pod
	Deployments []Deployment
	Services    []ServiceEntry

	// Per-method error overrides; if set, the method returns
	// this error instead of the canned data.
	PingErr            error
	ListPodsErr        error
	ListDeploymentsErr error
	ListServicesErr    error
}

// Ping implements Client.
func (f *FakeClient) Ping(ctx context.Context) error {
	return f.PingErr
}

// ListPods implements Client.
func (f *FakeClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	if f.ListPodsErr != nil {
		return nil, f.ListPodsErr
	}
	out := []Pod{}
	for _, p := range f.Pods {
		if namespace == "" || p.Namespace == namespace {
			out = append(out, p)
		}
	}
	return out, nil
}

// ListDeployments implements Client.
func (f *FakeClient) ListDeployments(ctx context.Context, namespace string) ([]Deployment, error) {
	if f.ListDeploymentsErr != nil {
		return nil, f.ListDeploymentsErr
	}
	out := []Deployment{}
	for _, d := range f.Deployments {
		if namespace == "" || d.Namespace == namespace {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListServices implements Client.
func (f *FakeClient) ListServices(ctx context.Context, namespace string) ([]ServiceEntry, error) {
	if f.ListServicesErr != nil {
		return nil, f.ListServicesErr
	}
	out := []ServiceEntry{}
	for _, s := range f.Services {
		if namespace == "" || s.Namespace == namespace {
			out = append(out, s)
		}
	}
	return out, nil
}

// KubeClient is the production Client implementation. It wraps
// a real Kubernetes client-go interface; the seam lives at the
// Client boundary, so the production wiring can swap in a
// real kubernetes.Interface when the cluster-management spec's
// exec/log streaming lands in Phase 6.
type KubeClient struct {
	// iface is the real client-go interface. We don't import
	// it in this file directly to keep the build fast for
	// tests that don't use it; NewKubeClient is the only
	// caller and can wire it from main.
	iface    any
	inCluster bool
	// kubeconfigPath is the on-disk path of a kubeconfig;
	// empty when running in-cluster.
	kubeconfigPath string
}

// NewKubeClient builds a KubeClient. iface should be a
// kubernetes.Interface from k8s.io/client-go/kubernetes; we
// accept any to keep this file's imports minimal. A nil iface
// is tolerated (the test suite uses it to verify the
// constructor's nil-safety).
func NewKubeClient(iface any, kubeconfigPath string) *KubeClient {
	return &KubeClient{
		iface:           iface,
		inCluster:       kubeconfigPath == "",
		kubeconfigPath:  kubeconfigPath,
	}
}

// Ping implements Client. The real implementation will call
// Discovery().ServerVersion() against the supplied iface.
// For now it returns an error so callers don't accidentally
// trust an un-wired client.
func (k *KubeClient) Ping(ctx context.Context) error {
	if k.iface == nil {
		return fmt.Errorf("k8s: KubeClient is not wired to a real client-go interface")
	}
	// Real implementation will go here in Phase 6.
	return fmt.Errorf("k8s: KubeClient.Ping not yet implemented")
}

// ListPods implements Client. The real implementation will use
// the iface's CoreV1().Pods(namespace).List(ctx) call.
func (k *KubeClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	return nil, fmt.Errorf("k8s: KubeClient.ListPods not yet implemented")
}

// ListDeployments implements Client. The real implementation
// will use the iface's AppsV1().Deployments(namespace).List(ctx)
// call.
func (k *KubeClient) ListDeployments(ctx context.Context, namespace string) ([]Deployment, error) {
	return nil, fmt.Errorf("k8s: KubeClient.ListDeployments not yet implemented")
}

// ListServices implements Client. The real implementation will
// use the iface's CoreV1().Services(namespace).List(ctx) call.
func (k *KubeClient) ListServices(ctx context.Context, namespace string) ([]ServiceEntry, error) {
	return nil, fmt.Errorf("k8s: KubeClient.ListServices not yet implemented")
}

// Compile-time interface checks.
var (
	_ Client = (*FakeClient)(nil)
	_ Client = (*KubeClient)(nil)
)
