package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
// a real client-go kubernetes.Interface and translates the
// typed API objects into the package's wire shapes.
//
// Two constructors:
//
//   - NewKubeClient(iface) wires a pre-built
//     kubernetes.Interface directly. Tests use this with
//     kubernetes/fake.NewSimpleClientset so they do not need
//     a real apiserver.
//   - NewKubeClientFromKubeconfig(content) parses an in-memory
//     kubeconfig string (the format returned by k8s.Service.
//     DecryptKubeconfig) and builds the iface via client-go's
//     standard clientcmd loader.
type KubeClient struct {
	iface kubernetes.Interface
}

// NewKubeClient wraps a pre-built kubernetes.Interface. Used by
// tests with a fake clientset; production code uses
// NewKubeClientFromKubeconfig.
func NewKubeClient(iface kubernetes.Interface) *KubeClient {
	return &KubeClient{iface: iface}
}

// NewKubeClientFromKubeconfig parses an in-memory kubeconfig
// string and builds a KubeClient backed by client-go. The
// kubeconfig is the YAML returned by k8s.Service.DecryptKubeconfig,
// so the registry never touches disk.
//
// Returns an error on parse failure or REST config build
// failure; the caller (ClientRegistry) is expected to cache
// the error stickily and never retry.
func NewKubeClientFromKubeconfig(content string) (*KubeClient, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return &KubeClient{iface: clientset}, nil
}

// Ping implements Client by calling Discovery().ServerVersion()
// — the canonical client-go connectivity probe (one round
// trip, no namespace required, fails fast on credential
// errors). We honour ctx.Err() around the call because
// Discovery() itself does not accept a context.
func (k *KubeClient) Ping(ctx context.Context) error {
	if k.iface == nil {
		return fmt.Errorf("k8s: KubeClient is not wired to a client-go interface")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := k.iface.Discovery().ServerVersion(); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return ctx.Err()
}

// ListPods implements Client by calling CoreV1().Pods(ns).List
// and translating the result. An empty namespace lists across
// all namespaces (client-go uses "" as the all-namespaces
// sentinel).
func (k *KubeClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	if k.iface == nil {
		return nil, fmt.Errorf("k8s: KubeClient is not wired to a client-go interface")
	}
	list, err := k.iface.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	out := make([]Pod, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, podToWire(&list.Items[i]))
	}
	return out, nil
}

// ListDeployments implements Client by calling
// AppsV1().Deployments(ns).List and translating the result.
func (k *KubeClient) ListDeployments(ctx context.Context, namespace string) ([]Deployment, error) {
	if k.iface == nil {
		return nil, fmt.Errorf("k8s: KubeClient is not wired to a client-go interface")
	}
	list, err := k.iface.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	out := make([]Deployment, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, deploymentToWire(&list.Items[i]))
	}
	return out, nil
}

// ListServices implements Client by calling
// CoreV1().Services(ns).List and translating the result.
func (k *KubeClient) ListServices(ctx context.Context, namespace string) ([]ServiceEntry, error) {
	if k.iface == nil {
		return nil, fmt.Errorf("k8s: KubeClient is not wired to a client-go interface")
	}
	list, err := k.iface.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	out := make([]ServiceEntry, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, serviceToWire(&list.Items[i]))
	}
	return out, nil
}

// podToWire translates a core/v1 Pod into the package's Pod
// wire shape. The Ready string is "ready/total" container
// counts, matching kubectl get pods.
func podToWire(p *corev1.Pod) Pod {
	ready := 0
	total := len(p.Status.ContainerStatuses)
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return Pod{
		Name:      p.Name,
		Namespace: p.Namespace,
		Phase:     string(p.Status.Phase),
		NodeName:  p.Spec.NodeName,
		Ready:     fmt.Sprintf("%d/%d", ready, total),
	}
}

// deploymentToWire translates an apps/v1 Deployment into the
// package's Deployment wire shape. Replicas defaults to 1 when
// unset (matching kubectl behaviour) so the "available/desired"
// ratio is never 0/0 for a real deployment.
func deploymentToWire(d *appsv1.Deployment) Deployment {
	var desired int32 = 1
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	available := d.Status.AvailableReplicas
	return Deployment{
		Name:      d.Name,
		Namespace: d.Namespace,
		Ready:     fmt.Sprintf("%d/%d", available, desired),
		Replicas:  desired,
		Available: available,
	}
}

// serviceToWire translates a core/v1 Service into the package's
// ServiceEntry wire shape.
func serviceToWire(s *corev1.Service) ServiceEntry {
	return ServiceEntry{
		Name:      s.Name,
		Namespace: s.Namespace,
		Type:      string(s.Spec.Type),
		ClusterIP: s.Spec.ClusterIP,
	}
}

// Compile-time interface checks.
var (
	_ Client = (*FakeClient)(nil)
	_ Client = (*KubeClient)(nil)
)
