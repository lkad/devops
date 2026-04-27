package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterManager struct {
	provider      string
	kubeconfigDir string
	k3dPath       string
	kindPath      string
	kubectlPath   string
}

type Cluster struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"` // "k3d", "kind", "standard" (production)
	Agents     int       `json:"agents"`
	APIPort   int       `json:"api_port"`
	Kubeconfig string    `json:"kubeconfig"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type Node struct {
	Name      string            `json:"name"`
	Ready     bool              `json:"ready"`
	Role      string            `json:"role"`
	CPU       string            `json:"cpu"`
	Memory    string            `json:"memory"`
	Age       string            `json:"age"`
	Taints    []string          `json:"taints,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Condition string            `json:"condition"`
}

type Pod struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Ready     string   `json:"ready"`
	Status    string   `json:"status"`
	Restarts  int      `json:"restarts"`
	CPU        string   `json:"cpu"`
	Memory     string   `json:"memory"`
	Age        string   `json:"age"`
	NodeName   string   `json:"node_name"`
	IP         string   `json:"ip"`
}

type Workload struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Kind        string `json:"kind"`
	Replicas     int32  `json:"replicas"`
	ReadyReplicas int32 `json:"ready_replicas"`
	Available    int32  `json:"available"`
	Age          string `json:"age"`
}

type MaintenanceOp struct {
	Cluster   string `json:"cluster"`
	Node      string `json:"node"`
	Operation string `json:"operation"` // drain, cordon, uncordon, restart-pod
	Target    string `json:"target"`   // node name or namespace/pod-name
	Force     bool   `json:"force"`
}

func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		provider:      "k3d",
		kubeconfigDir: filepath.Join(os.Getenv("HOME"), ".kube"),
		k3dPath:       "k3d",
		kindPath:      "kind",
		kubectlPath:   "kubectl",
	}
}

func (m *ClusterManager) getKubeconfig(clusterName string) string {
	return filepath.Join(m.kubeconfigDir, fmt.Sprintf("config-%s", clusterName))
}

func (m *ClusterManager) ListClusters() ([]*Cluster, error) {
	cmd := exec.Command(m.k3dPath, "cluster", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var clusters []*Cluster
	lines := parseK3dList(string(output))
	for _, line := range lines {
		clusters = append(clusters, &Cluster{
			Name:   line["name"],
			Type:   "k3d", // k3d is used for testing/development
			Status: "running",
		})
	}
	return clusters, nil
}

func parseK3dList(output string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		// Skip header line and empty lines
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		// Parse k3d cluster list output
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			result = append(result, map[string]string{
				"name":    fields[0],
				"servers": fields[1],
				"agents":  fields[2],
				"lb":      fields[3],
			})
		}
	}
	return result
}

func (m *ClusterManager) CreateCluster(name string, agents int, apiPort int) (*Cluster, error) {
	args := []string{
		"cluster", "create", name,
		"--agents", fmt.Sprintf("%d", agents),
		"-p", fmt.Sprintf("%d:6443@loadbalancer", apiPort),
		"--timeout", "120s",
		"--wait",
	}

	cmd := exec.Command(m.k3dPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	kubeconfigPath := m.getKubeconfig(name)
	if err := exec.Command(m.k3dPath, "kubeconfig", "get", name).Run(); err != nil {
		return nil, err
	}

	return &Cluster{
		Name:       name,
		Type:       "k3d",
		Agents:     agents,
		APIPort:    apiPort,
		Kubeconfig: kubeconfigPath,
		Status:     "running",
		CreatedAt:  time.Now(),
	}, nil
}

func (m *ClusterManager) DeleteCluster(name string) error {
	cmd := exec.Command(m.k3dPath, "cluster", "delete", name)
	return cmd.Run()
}

func (m *ClusterManager) HealthCheck(name string) (map[string]interface{}, error) {
	ctx := context.Background()
	kubeconfigPath := m.getKubeconfig(name)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	allReady := true
	nodeStatuses := make([]map[string]string, 0)
	for _, n := range nodes.Items {
		status := "Unknown"
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == "True" {
					status = "Ready"
				} else {
					status = "NotReady"
					allReady = false
				}
			}
		}
		nodeStatuses = append(nodeStatuses, map[string]string{
			"name":   n.Name,
			"status": status,
		})
	}

	return map[string]interface{}{
		"connected": true,
		"ready":    allReady,
		"nodes":    nodeStatuses,
	}, nil
}

func (m *ClusterManager) GetWorkloads(name, namespace string) ([]map[string]interface{}, error) {
	ctx := context.Background()
	kubeconfigPath := m.getKubeconfig(name)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []map[string]interface{}
	for _, d := range deployments.Items {
		workloads = append(workloads, map[string]interface{}{
			"name":           d.Name,
			"namespace":      d.Namespace,
			"replicas":       d.Spec.Replicas,
			"ready_replicas": d.Status.ReadyReplicas,
		})
	}
	return workloads, nil
}

// Node Management - 节点管理
func (m *ClusterManager) GetNodes(clusterName string) ([]Node, error) {
	ctx := context.Background()
	kubeconfigPath := m.getKubeconfig(clusterName)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []Node
	for _, n := range nodes.Items {
		ready := false
		condition := "Unknown"
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" {
				ready = c.Status == "True"
				if ready {
					condition = "Ready"
				} else {
					condition = "NotReady"
				}
			}
		}

		role := "worker"
		if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
			role = "master"
		}

		cpu := n.Status.Capacity.Cpu().String()
		memory := n.Status.Capacity.Memory().String()

		var taints []string
		for _, t := range n.Spec.Taints {
			taints = append(taints, fmt.Sprintf("%s=%s:%v", t.Key, t.Value, t.Effect))
		}

		result = append(result, Node{
			Name:      n.Name,
			Ready:     ready,
			Role:      role,
			CPU:       cpu,
			Memory:    memory,
			Age:       time.Since(n.CreationTimestamp.Time).Round(time.Hour).String(),
			Taints:    taints,
			Labels:    n.Labels,
			Condition: condition,
		})
	}

	return result, nil
}

// Cordon - 标记节点为不可调度
func (m *ClusterManager) CordonNode(clusterName, nodeName string) error {
	cmd := exec.Command(m.kubectlPath, "--kubeconfig", m.getKubeconfig(clusterName),
		"cordon", nodeName)
	return cmd.Run()
}

// Uncordon - 标记节点为可调度
func (m *ClusterManager) UncordonNode(clusterName, nodeName string) error {
	cmd := exec.Command(m.kubectlPath, "--kubeconfig", m.getKubeconfig(clusterName),
		"uncordon", nodeName)
	return cmd.Run()
}

// Drain - 排空节点
func (m *ClusterManager) DrainNode(clusterName, nodeName string, force bool) error {
	args := []string{"--kubeconfig", m.getKubeconfig(clusterName),
		"drain", nodeName, "--ignore-daemonsets", "--delete-emptydir-data"}
	if force {
		args = append(args, "--force", "--grace-period=30")
	}
	cmd := exec.Command(m.kubectlPath, args...)
	return cmd.Run()
}

// Pod Management - Pod管理
func (m *ClusterManager) GetPods(clusterName, namespace string) ([]Pod, error) {
	ctx := context.Background()
	kubeconfigPath := m.getKubeconfig(clusterName)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []Pod
	for _, p := range pods.Items {
		ready := fmt.Sprintf("%d/%d", countReady(p.Status.Conditions), len(p.Spec.Containers))

		cpu := "0"
		if len(p.Spec.Containers) > 0 {
			if cpuReq := p.Spec.Containers[0].Resources.Requests.Cpu(); !cpuReq.IsZero() {
				cpu = cpuReq.String()
			}
		}
		memory := "0"
		if len(p.Spec.Containers) > 0 {
			if memReq := p.Spec.Containers[0].Resources.Requests.Memory(); !memReq.IsZero() {
				memory = memReq.String()
			}
		}

		restarts := 0
		if len(p.Status.ContainerStatuses) > 0 {
			restarts = int(p.Status.ContainerStatuses[0].RestartCount)
		}

		result = append(result, Pod{
			Name:      p.Name,
			Namespace: p.Namespace,
			Ready:     ready,
			Status:    string(p.Status.Phase),
			Restarts:  restarts,
			CPU:       cpu,
			Memory:    memory,
			Age:       time.Since(p.CreationTimestamp.Time).Round(time.Hour).String(),
			NodeName:  p.Spec.NodeName,
			IP:        p.Status.PodIP,
		})
	}

	return result, nil
}

// DeletePod - 删除 Pod
func (m *ClusterManager) DeletePod(clusterName, namespace, podName string) error {
	cmd := exec.Command(m.kubectlPath, "--kubeconfig", m.getKubeconfig(clusterName),
		"delete", "pod", podName, "-n", namespace)
	return cmd.Run()
}

// RestartPod - 重启 Pod (删除后自动重新创建)
func (m *ClusterManager) RestartPod(clusterName, namespace, podName string) error {
	if err := m.DeletePod(clusterName, namespace, podName); err != nil {
		return err
	}
	return nil
}

// GetPodLogs - 获取 Pod 日志
func (m *ClusterManager) GetPodLogs(clusterName, namespace, podName string, lines int) (string, error) {
	args := []string{"--kubeconfig", m.getKubeconfig(clusterName),
		"logs", podName, "-n", namespace}
	if lines > 0 {
		args = append(args, fmt.Sprintf("--tail=%d", lines))
	}

	cmd := exec.Command(m.kubectlPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// ScaleWorkload - 扩缩容
func (m *ClusterManager) ScaleWorkload(clusterName, namespace, kind, name string, replicas int) error {
	cmd := exec.Command(m.kubectlPath, "--kubeconfig", m.getKubeconfig(clusterName),
		"scale", fmt.Sprintf("%s/%s", kind, name), fmt.Sprintf("--replicas=%d", replicas), "-n", namespace)
	return cmd.Run()
}

// GetNamespaces - 获取所有命名空间
func (m *ClusterManager) GetNamespaces(clusterName string) ([]string, error) {
	ctx := context.Background()
	kubeconfigPath := m.getKubeconfig(clusterName)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	ns, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, n := range ns.Items {
		result = append(result, n.Name)
	}
	return result, nil
}

// ExecuteMaintenanceOp - 执行维护操作
func (m *ClusterManager) ExecuteMaintenanceOp(op *MaintenanceOp) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"cluster":   op.Cluster,
		"operation": op.Operation,
		"target":    op.Target,
		"success":   false,
	}

	switch op.Operation {
	case "cordon":
		err := m.CordonNode(op.Cluster, op.Target)
		result["success"] = err == nil
		result["message"] = "节点已标记为不可调度"
	case "uncordon":
		err := m.UncordonNode(op.Cluster, op.Target)
		result["success"] = err == nil
		result["message"] = "节点已恢复调度"
	case "drain":
		err := m.DrainNode(op.Cluster, op.Target, op.Force)
		result["success"] = err == nil
		result["message"] = "节点已排空"
	case "delete-pod":
		parts := splitNamespaceResource(op.Target)
		if len(parts) != 2 {
			result["message"] = "无效的Pod标识，格式: namespace/pod-name"
			return result, nil
		}
		err := m.DeletePod(op.Cluster, parts[0], parts[1])
		result["success"] = err == nil
		result["message"] = "Pod已删除"
	case "restart-pod":
		parts := splitNamespaceResource(op.Target)
		if len(parts) != 2 {
			result["message"] = "无效的Pod标识，格式: namespace/pod-name"
			return result, nil
		}
		err := m.RestartPod(op.Cluster, parts[0], parts[1])
		result["success"] = err == nil
		result["message"] = "Pod已重启"
	case "get-logs":
		parts := splitNamespaceResource(op.Target)
		if len(parts) != 2 {
			result["message"] = "无效的Pod标识，格式: namespace/pod-name"
			return result, nil
		}
		logs, err := m.GetPodLogs(op.Cluster, parts[0], parts[1], 100)
		result["success"] = err == nil
		result["logs"] = logs
		result["message"] = "日志获取成功"
	default:
		result["message"] = fmt.Sprintf("未知操作: %s", op.Operation)
	}

	return result, nil
}

func splitNamespaceResource(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func countReady(conditions []v1.PodCondition) int {
	for _, c := range conditions {
		if c.Type == "Ready" {
			if c.Status == "True" {
				return 1
			}
			return 0
		}
	}
	return 0
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// HTTP handlers
func (m *ClusterManager) ListClustersHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	clusters, err := m.ListClusters()
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	// Apply pagination in-memory
	total := len(clusters)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedClusters := clusters[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedClusters, total, limit, offset))
}

func (m *ClusterManager) CreateClusterHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Agents  int    `json:"agents"`
		APIPort int    `json:"api_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	if input.Agents == 0 {
		input.Agents = 3
	}
	if input.APIPort == 0 {
		input.APIPort = 31000
	}

	cluster, err := m.CreateCluster(input.Name, input.Agents, input.APIPort)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

func (m *ClusterManager) DeleteClusterHTTP(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if err := m.DeleteCluster(name); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *ClusterManager) HealthCheckHTTP(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	status, err := m.HealthCheck(name)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Maintenance HTTP handlers
func (m *ClusterManager) GetNodesHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := mux.Vars(r)["cluster"]
	limit, offset := parsePagination(r)
	nodes, err := m.GetNodes(cluster)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	// Apply pagination in-memory
	total := len(nodes)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedNodes := nodes[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedNodes, total, limit, offset))
}

func (m *ClusterManager) GetNamespacesHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := mux.Vars(r)["cluster"]
	namespaces, err := m.GetNamespaces(cluster)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaces)
}

func (m *ClusterManager) GetPodsHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := mux.Vars(r)["cluster"]
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	limit, offset := parsePagination(r)
	pods, err := m.GetPods(cluster, namespace)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	// Apply pagination in-memory
	total := len(pods)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedPods := pods[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedPods, total, limit, offset))
}

func (m *ClusterManager) MaintenanceOpHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		apierror.MethodNotAllowed(w)
		return
	}

	var op MaintenanceOp
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	result, err := m.ExecuteMaintenanceOp(&op)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (m *ClusterManager) GetPodLogsHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := mux.Vars(r)["cluster"]
	namespace := r.URL.Query().Get("namespace")
	podName := mux.Vars(r)["pod"]
	if namespace == "" || podName == "" {
		apierror.ValidationError(w, "namespace and pod are required")
		return
	}

	logs, err := m.GetPodLogs(cluster, namespace, podName, 100)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}