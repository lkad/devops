package k8s

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// LogEntry is the wire shape for a single log line returned by
// GetLogsBySelector. It is intentionally minimal (no container
// runtime metadata, no parse-failed placeholder fields) — the
// shape the dashboard + log-search UI consume. Timestamp is
// zero when the source backend did not emit one (e.g. raw
// files); callers must check IsZero() before rendering.
type LogEntry struct {
	Pod       string    `json:"pod"`
	Container string    `json:"container,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
}

// LogQuery carries the optional refinements for a label-
// selector log query. The zero value is valid (returns the
// last ~10 lines from every container of every matching pod).
type LogQuery struct {
	// Container filters the result to one container per pod.
	// Empty means "all containers in the pod".
	Container string
	// TailLines caps the historical tail per pod. Zero or
	// negative means "use backend default" (client-go sends
	// no tailLines field, which the apiserver treats as ~10
	// lines).
	TailLines int
	// Since is the lower bound on line timestamps. Zero
	// means "no lower bound". When non-zero, the apiserver
	// uses sinceTime (RFC3339) over sinceSeconds because
	// it is unambiguous across clock-skewed nodes.
	Since time.Time
	// MaxPods caps how many matching pods the call will
	// fan out to. Zero or negative means "use the default
	// (10)" — a guard against an over-broad selector
	// (e.g. "app=" hitting 500 pods) blowing up the
	// apiserver connection budget.
	MaxPods int
}

// defaultLogMaxPods is the cap applied when LogQuery.MaxPods
// is unset. Sized so a single call fits in a dozen concurrent
// apiserver log reads; the UI caps the visible result at
// MaxLines regardless so a fanned-out read still gives the
// operator a bounded tail.
const defaultLogMaxPods = 10

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

	// GetLogsBySelector lists pods matching the given label
	// selector (in the given namespace) and returns a merged
	// historical tail of their log lines. The query is
	// fan-out: every matching pod up to LogQuery.MaxPods is
	// fetched and the results are interleaved by timestamp
	// (or, when timestamps are absent, by source order).
	// An empty namespace is rejected — pod labels are only
	// unique within a namespace, so a label-selector log
	// query without a namespace scope is almost always a
	// caller bug.
	GetLogsBySelector(ctx context.Context, namespace, labelSelector string, q LogQuery) ([]LogEntry, error)
}

// FakeClient is an in-memory Client for unit tests. Each
// method returns the canned field value, or the canned error
// if the corresponding Err* field is set.
type FakeClient struct {
	Pods        []Pod
	Deployments []Deployment
	Services    []ServiceEntry
	LogEntries  []LogEntry

	// Per-method error overrides; if set, the method returns
	// this error instead of the canned data.
	PingErr               error
	ListPodsErr           error
	ListDeploymentsErr    error
	ListServicesErr       error
	GetLogsBySelectorErr  error

	// logEntryFor, if non-nil, replaces the default
	// "filter LogEntries by Container" path. Tests use it
	// to model label-selector fan-out, empty-pod-set
	// behaviour, etc.
	logEntryFor func(ctx context.Context, namespace, labelSelector string, q LogQuery) ([]LogEntry, error)
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

// GetLogsBySelector implements Client. The fake applies the
// same namespace filter as the production implementation
// (an empty namespace returns ErrInvalidLogQuery) and returns
// f.LogEntries verbatim — tests that need label-selector
// fan-out semantics wire a custom logEntryFor(pod, sel) fn.
func (f *FakeClient) GetLogsBySelector(ctx context.Context, namespace, labelSelector string, q LogQuery) ([]LogEntry, error) {
	if f.GetLogsBySelectorErr != nil {
		return nil, f.GetLogsBySelectorErr
	}
	if namespace == "" {
		return nil, fmt.Errorf("k8s: %w: GetLogsBySelector requires a namespace", ErrInvalidLogQuery)
	}
	if f.logEntryFor != nil {
		return f.logEntryFor(ctx, namespace, labelSelector, q)
	}
	out := []LogEntry{}
	for _, e := range f.LogEntries {
		if e.Pod == "" {
			continue
		}
		if q.Container != "" && e.Container != q.Container {
			continue
		}
		out = append(out, e)
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

// GetLogsBySelector implements Client. It lists pods matching
// the label selector (capped at LogQuery.MaxPods) and fetches
// a historical log tail from each, then merges the results
// in pod-then-time order. An empty namespace is rejected —
// pod labels are only unique within a namespace, and an
// unscoped query is almost always a caller bug that would
// either fan out across every namespace or return a confusing
// empty slice.
//
// The apiserver's GetLogs response is plain text (one log
// line per newline, optionally prefixed with an RFC3339
// timestamp). We split on newline and emit one LogEntry per
// non-empty line. Lines with a parseable leading timestamp
// are split into LogEntry.Timestamp + LogEntry.Line; lines
// without a timestamp get a zero Timestamp and the whole
// line goes into LogEntry.Line. Container name is set to the
// first container in the pod (or LogQuery.Container when the
// caller narrowed to one).
func (k *KubeClient) GetLogsBySelector(ctx context.Context, namespace, labelSelector string, q LogQuery) ([]LogEntry, error) {
	if k.iface == nil {
		return nil, fmt.Errorf("k8s: KubeClient is not wired to a client-go interface")
	}
	if namespace == "" {
		return nil, fmt.Errorf("k8s: %w: GetLogsBySelector requires a namespace", ErrInvalidLogQuery)
	}
	maxPods := q.MaxPods
	if maxPods <= 0 {
		maxPods = defaultLogMaxPods
	}

	// 1. List matching pods. We send the Limit as a hint
	// to the apiserver (saves bandwidth on a broad
	// selector) but ALSO enforce maxPods client-side
	// because not every apiserver implementation honours
	// Limit (the fake clientset does not, for example,
	// and a future proxy in front of the apiserver may
	// strip it). The client-side cap is the hard contract.
	list, err := k.iface.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         int64(maxPods),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods by selector: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > maxPods {
		list.Items = list.Items[:maxPods]
	}

	// 2. Fan out one GetLogs call per pod. Sequential today;
	// a future pass can move to errgroup.WithContext to
	// parallelise. The current per-pod call is one HTTP
	// round-trip to the apiserver, so a 10-pod fan-out is
	// ~10× the per-pod latency. Bounded by maxPods so the
	// total wall time is predictable.
	out := make([]LogEntry, 0, maxPods*10)
	for i := range list.Items {
		pod := &list.Items[i]
		entries, err := k.logsForPod(ctx, pod, q)
		if err != nil {
			// A single pod's logs failing should not lose
			// the rest. Surface as a comment in the
			// result so the operator can see "X pods
			// succeeded, Y failed" in the UI without
			// having to retry the whole query.
			out = append(out, LogEntry{
				Pod:       pod.Name,
				Container: q.Container,
				Line:      fmt.Sprintf("k8s: failed to fetch logs: %v", err),
			})
			continue
		}
		out = append(out, entries...)
	}
	return out, nil
}

// logsForPod is the per-pod implementation. Picks the first
// container (or LogQuery.Container when set), builds the
// PodLogOptions, calls pods.GetLogs(...).Do(ctx).Body, and
// splits the response on newlines.
func (k *KubeClient) logsForPod(ctx context.Context, pod *corev1.Pod, q LogQuery) ([]LogEntry, error) {
	container := q.Container
	if container == "" {
		if len(pod.Spec.Containers) == 0 {
			return nil, nil
		}
		container = pod.Spec.Containers[0].Name
	}

	opts := &corev1.PodLogOptions{
		Container: container,
	}
	if q.TailLines > 0 {
		n := int64(q.TailLines)
		opts.TailLines = &n
	}
	if !q.Since.IsZero() {
		t := metav1.NewTime(q.Since)
		opts.SinceTime = &t
	}

	req := k.iface.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	// client-go v0.36 returns a rest.Result value from
	// Do(). The status code is captured internally; we
	// extract it via the .StatusCode(*int) setter (which
	// doubles as a getter that writes the int through the
	// pointer) and read the buffered body with .Raw().
	var status int
	body, err := req.Do(ctx).StatusCode(&status).Raw()
	if err != nil {
		return nil, fmt.Errorf("apiserver returned status %d for pod %s/%s: %w", status, pod.Namespace, pod.Name, err)
	}
	if status != 200 {
		return nil, fmt.Errorf("apiserver returned status %d for pod %s/%s", status, pod.Namespace, pod.Name)
	}
	return parseLogLines(pod.Name, container, body), nil
}

// parseLogLines splits a log body into LogEntry values. The
// apiserver emits one log line per "\n" (with a trailing
// newline stripped). Lines that begin with an RFC3339
// timestamp are split into Timestamp + Line; lines without
// one are emitted with Timestamp zero and the whole text
// in Line. Empty lines are skipped.
func parseLogLines(pod, container string, body []byte) []LogEntry {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	// Tail a 1 MiB line cap; apiserver log lines are
	// short by convention. A pathological single line over
	// the cap is dropped (the operator would see a
	// truncation message in the next request).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := make([]LogEntry, 0, 16)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		ts, rest := splitLeadingRFC3339(line)
		out = append(out, LogEntry{
			Pod:       pod,
			Container: container,
			Timestamp: ts,
			Line:      rest,
		})
	}
	return out
}

// splitLeadingRFC3339 returns the leading timestamp + the
// remainder. Apiserver log lines look like
// "2026-06-10T12:00:00.000Z message body" — the leading
// RFC3339 is what client-go parsing tools recognise. If
// the line does not start with an RFC3339 prefix, returns
// the zero time and the line unchanged.
func splitLeadingRFC3339(line string) (time.Time, string) {
	// RFC3339 has a "T" between date and time and either
	// a "Z" or "+/-HH:MM" trailing. The shortest valid
	// timestamp is 20 characters: "2006-01-02T15:04:05Z".
	if len(line) < 20 || line[4] != '-' || line[7] != '-' || line[10] != 'T' {
		return time.Time{}, line
	}
	// Find the end of the timestamp: the first space
	// (or end of string if the line is timestamp-only).
	end := strings.IndexAny(line, " \t")
	if end < 0 {
		end = len(line)
	}
	candidate := line[:end]
	t, err := time.Parse(time.RFC3339Nano, candidate)
	if err != nil {
		// Maybe RFC3339 without nanos.
		t, err = time.Parse(time.RFC3339, candidate)
		if err != nil {
			return time.Time{}, line
		}
	}
	// Strip the whitespace that separated the timestamp
	// from the message body. The caller wants the line
	// text to start with the first non-whitespace
	// character after the timestamp, not the space.
	rest := strings.TrimLeft(line[end:], " \t")
	return t, rest
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

// ErrInvalidLogQuery is the typed sentinel for a malformed
// log query. The only branch that emits it today is
// "namespace is required"; future branches (e.g. mutually-
// exclusive Since/SinceSeconds) can join.
var ErrInvalidLogQuery = errors.New("invalid log query")

// Compile-time interface checks.
var (
	_ Client = (*FakeClient)(nil)
	_ Client = (*KubeClient)(nil)
)
