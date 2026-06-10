package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// int32Ptr returns a pointer to v. Inline helper because the
// fixture builders below need it in multiple places.
func int32Ptr(v int32) *int32 { return &v }

// TestKubeClient_Ping_OK pins the happy path: a server-version
// query succeeds via the fake discovery client.
func TestKubeClient_Ping_OK(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClient(cs)
	if err := k.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

// TestKubeClient_Ping_NilIface pins the nil-safety branch
// the registry depends on: ClientFor returns the wrapped
// client even when iface is nil, but calling Ping must
// surface a clear error rather than panicking.
func TestKubeClient_Ping_NilIface(t *testing.T) {
	k := NewKubeClient(nil)
	err := k.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for nil iface, got nil")
	}
	if !strings.Contains(err.Error(), "not wired") {
		t.Errorf("err = %v, want substring 'not wired'", err)
	}
}

// TestKubeClient_Ping_CtxCanceled verifies that a pre-cancelled
// context short-circuits before the discovery call. The
// production timeout still relies on rest.Config.Timeout for
// the in-flight case; this test just pins the cheap fast path.
func TestKubeClient_Ping_CtxCanceled(t *testing.T) {
	cs := fake.NewSimpleClientset()
	k := NewKubeClient(cs)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := k.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestKubeClient_ListPods_RealConversion pins the wire-shape
// translation: a core/v1 Pod becomes a wire Pod with the right
// Ready string (counted from container statuses) and NodeName.
func TestKubeClient_ListPods_RealConversion(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "default"},
			Spec:       corev1.PodSpec{NodeName: "node-a"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app", Ready: true},
					{Name: "sidecar", Ready: false},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "other"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
	)
	k := NewKubeClient(cs)

	pods, err := k.ListPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("pods len = %d, want 1 (namespace filter)", len(pods))
	}
	got := pods[0]
	if got.Name != "web-0" || got.Phase != "Running" || got.NodeName != "node-a" {
		t.Errorf("pod = %+v, want web-0/Running/node-a", got)
	}
	if got.Ready != "1/2" {
		t.Errorf("ready = %q, want %q", got.Ready, "1/2")
	}
}

// TestKubeClient_ListPods_AllNamespaces pins the empty-namespace
// behaviour: client-go uses "" to mean "all namespaces".
func TestKubeClient_ListPods_AllNamespaces(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns-1"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns-2"}},
	)
	k := NewKubeClient(cs)
	pods, err := k.ListPods(context.Background(), "")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("pods len = %d, want 2 (all-ns)", len(pods))
	}
}

// TestKubeClient_ListDeployments_ReadyAndReplicas pins the
// deployment translation: Replicas defaults to 1 when Spec
// is nil, and the Ready string is "available/desired".
func TestKubeClient_ListDeployments_ReadyAndReplicas(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "tiny", Namespace: "default"},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 1},
		},
	)
	k := NewKubeClient(cs)
	deps, err := k.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("deps len = %d, want 2", len(deps))
	}
	byName := map[string]Deployment{}
	for _, d := range deps {
		byName[d.Name] = d
	}
	if got := byName["web"]; got.Ready != "2/3" || got.Replicas != 3 || got.Available != 2 {
		t.Errorf("web = %+v, want ready=2/3 replicas=3 available=2", got)
	}
	if got := byName["tiny"]; got.Ready != "1/1" || got.Replicas != 1 {
		t.Errorf("tiny = %+v, want ready=1/1 replicas=1 (nil Spec.Replicas defaults to 1)", got)
	}
}

// TestKubeClient_ListServices_TypeAndClusterIP pins the
// service translation: Type and ClusterIP are surfaced as
// the wire shape.
func TestKubeClient_ListServices_TypeAndClusterIP(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.42"},
		},
	)
	k := NewKubeClient(cs)
	svcs, err := k.ListServices(context.Background(), "default")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("svcs len = %d, want 1", len(svcs))
	}
	got := svcs[0]
	if got.Name != "api" || got.Type != "ClusterIP" || got.ClusterIP != "10.0.0.42" {
		t.Errorf("svc = %+v, want api/ClusterIP/10.0.0.42", got)
	}
}

// TestKubeClient_ListPods_PropagatesApiError verifies that an
// apiserver error surfaces as a wrapped error so the caller
// (the servicecatalog walker) can decide to skip vs fail.
func TestKubeClient_ListPods_PropagatesApiError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("apiserver: 503 unavailable")
	})
	k := NewKubeClient(cs)
	_, err := k.ListPods(context.Background(), "default")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want substring '503'", err)
	}
}
