package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/ginadapter"
	"github.com/devops-toolkit/internal/pagination"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"gorm.io/gorm"
)

type ClusterManager struct {
	provider      string
	kubeconfigDir string
	k3dPath       string
	kindPath      string
	kubectlPath   string
	db           *gorm.DB
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

// PodLogsOptions contains options for pod log retrieval
type PodLogsOptions struct {
	Namespace string
	PodName   string
	Container string
	Lines     int64
	Previous  bool
}

// ExecOptions contains options for pod exec
type ExecOptions struct {
	Namespace string
	PodName   string
	Container string
	Command   []string
}

// ExecResult contains the result of pod exec
type ExecResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// NodeMetrics contains CPU and memory metrics for a node
type NodeMetrics struct {
	Name      string `json:"name"`
	CPUUsage  string `json:"cpuUsage"`
	CPUCap    string `json:"cpuCap"`
	MemUsage  string `json:"memUsage"`
	MemCap    string `json:"memCap"`
}

func NewClusterManager(db *gorm.DB) *ClusterManager {
	return &ClusterManager{
		provider:      "k3d",
		kubeconfigDir: filepath.Join(os.Getenv("HOME"), ".kube"),
		k3dPath:       "k3d",
		kindPath:      "kind",
		kubectlPath:   "kubectl",
		db:            db,
	}
}

func (m *ClusterManager) getKubeconfig(clusterName string) string {
	return filepath.Join(m.kubeconfigDir, fmt.Sprintf("config-%s", clusterName))
}

// getKubeconfigFromDB retrieves kubeconfig content from database
func (m *ClusterManager) getKubeconfigFromDB(clusterName string) (string, error) {
	if m.db == nil {
		return "", fmt.Errorf("database not initialized")
	}
	var cluster GORMCluster
	if err := m.db.Where("name = ?", clusterName).First(&cluster).Error; err != nil {
		return "", fmt.Errorf("cluster not found: %w", err)
	}
	return cluster.Kubeconfig, nil
}

// ImportExistingK3dClusters imports existing k3d clusters from ~/.kube/config-* files into DB
// This is called during startup to migrate from file-based to database-backed registry
func (m *ClusterManager) ImportExistingK3dClusters() error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	// Run k3d cluster list to get existing clusters
	cmd := exec.Command(m.k3dPath, "cluster", "list")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list k3d clusters: %w", err)
	}

	lines := parseK3dList(string(output))
	imported := 0
	for _, line := range lines {
		clusterName := line["name"]
		if clusterName == "" {
			continue
		}

		// Check if cluster already exists in DB
		var existing GORMCluster
		if err := m.db.Where("name = ?", clusterName).First(&existing).Error; err == nil {
			// Cluster already in DB, skip
			continue
		}

		// Export kubeconfig for this cluster
		kubeconfigCmd := exec.Command(m.k3dPath, "kubeconfig", "get", clusterName)
		kubeconfigOutput, err := kubeconfigCmd.Output()
		if err != nil {
			log.Printf("Failed to export kubeconfig for cluster %s: %v", clusterName, err)
			continue
		}

		// Register cluster in DB
		cluster := &GORMCluster{
			Name:       clusterName,
			Type:       ClusterTypeK3d,
			Env:        ClusterEnvDev,
			Kubeconfig: string(kubeconfigOutput),
			Status:     ClusterStatusUnknown,
		}
		if err := m.db.Create(cluster).Error; err != nil {
			log.Printf("Failed to register cluster %s in DB: %v", clusterName, err)
			continue
		}
		imported++
	}
	if imported > 0 {
		log.Printf("Imported %d existing k3d clusters into database", imported)
	}
	return nil
}

// getClusterByName retrieves a cluster from database by name
func (m *ClusterManager) getClusterByName(clusterName string) (*GORMCluster, error) {
	if m.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var cluster GORMCluster
	if err := m.db.Where("name = ?", clusterName).First(&cluster).Error; err != nil {
		return nil, err
	}
	return &cluster, nil
}

// buildClientFromKubeconfig builds a kubernetes client from kubeconfig content (string)
func (m *ClusterManager) buildClientFromKubeconfig(kubeconfigContent string) (*kubernetes.Clientset, error) {
	// Write kubeconfig to a temp file since clientcmd.Load requires a file path
	tmpFile, err := os.CreateTemp("", "kubeconfig-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp kubeconfig file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(kubeconfigContent); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig content: %w", err)
	}
	tmpFile.Close()

	cfg, err := clientcmd.BuildConfigFromFlags("", tmpFile.Name())
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// buildClient builds a kubernetes client for the given cluster
// If DB is available, reads kubeconfig from DB; otherwise falls back to file system
func (m *ClusterManager) buildClient(clusterName string) (*kubernetes.Clientset, error) {
	cfg, err := m.buildConfig(clusterName)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// buildConfig builds a rest config for the given cluster
// If DB is available, reads kubeconfig from DB; otherwise falls back to file system
func (m *ClusterManager) buildConfig(clusterName string) (*rest.Config, error) {
	if m.db != nil {
		kubeconfig, err := m.getKubeconfigFromDB(clusterName)
		if err == nil && kubeconfig != "" {
			// Write kubeconfig to a temp file since clientcmd.Load requires a file path
			tmpFile, err := os.CreateTemp("", "kubeconfig-*")
			if err != nil {
				return nil, fmt.Errorf("failed to create temp kubeconfig file: %w", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			if _, err := tmpFile.WriteString(kubeconfig); err != nil {
				return nil, fmt.Errorf("failed to write kubeconfig content: %w", err)
			}
			tmpFile.Close()
			return clientcmd.BuildConfigFromFlags("", tmpFile.Name())
		}
	}
	// Fallback to file system
	kubeconfigPath := m.getKubeconfig(clusterName)
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

func (m *ClusterManager) ListClusters() ([]*Cluster, error) {
	// If DB is available, read from database
	if m.db != nil {
		var dbClusters []GORMCluster
		if err := m.db.Find(&dbClusters).Error; err != nil {
			return nil, fmt.Errorf("failed to list clusters from DB: %w", err)
		}
		var clusters []*Cluster
		for _, c := range dbClusters {
			clusters = append(clusters, &Cluster{
				Name:   c.Name,
				Type:   string(c.Type),
				Status: string(c.Status),
			})
		}
		return clusters, nil
	}
	// Fallback to k3d CLI if DB not available
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

// RegisterCluster adds a new cluster to the database
func (m *ClusterManager) RegisterCluster(name string, clusterType ClusterType, env ClusterEnv, kubeconfig string) (*GORMCluster, error) {
	if m.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	cluster := &GORMCluster{
		Name:       name,
		Type:       clusterType,
		Env:        env,
		Kubeconfig: kubeconfig,
		Status:     ClusterStatusUnknown,
	}
	if err := m.db.Create(cluster).Error; err != nil {
		return nil, fmt.Errorf("failed to register cluster: %w", err)
	}
	return cluster, nil
}

// UnregisterCluster removes a cluster from the database
func (m *ClusterManager) UnregisterCluster(name string) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := m.db.Where("name = ?", name).Delete(&GORMCluster{}).Error; err != nil {
		return fmt.Errorf("failed to unregister cluster: %w", err)
	}
	return nil
}

// UpdateClusterStatus updates the status of a cluster in the database
func (m *ClusterManager) UpdateClusterStatus(name string, status ClusterStatus) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := m.db.Model(&GORMCluster{}).Where("name = ?", name).Update("status", status).Error; err != nil {
		return fmt.Errorf("failed to update cluster status: %w", err)
	}
	return nil
}

func (m *ClusterManager) HealthCheck(name string) (map[string]interface{}, error) {
	ctx := context.Background()

	clientset, err := m.buildClient(name)
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

	clientset, err := m.buildClient(name)
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

	clientset, err := m.buildClient(clusterName)
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
	kubeconfigPath := m.getKubeconfig(clusterName)
	cmd := exec.Command(m.kubectlPath, "--kubeconfig", kubeconfigPath,
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

	clientset, err := m.buildClient(clusterName)
	if err != nil {
		return nil, err
	}

	// Empty namespace means all namespaces (like kubectl get pods -A)
	if namespace == "" {
		namespace = metav1.NamespaceAll
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
	ctx := context.Background()
	clientset, err := m.buildClient(clusterName)
	if err != nil {
		return err
	}
	return clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
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
	clientset, err := m.buildClient(clusterName)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	// Get first container if not specified
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	container := ""
	if len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

	limit := int64(lines)
	if limit <= 0 {
		limit = 100
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: container,
		TailLines: &limit,
	})
	result, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer result.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(result)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GetPodLogsWithOptions retrieves pod logs using the Kubernetes client API with support for previous logs
func (m *ClusterManager) GetPodLogsWithOptions(clusterName string, opts PodLogsOptions) (string, error) {
	ctx := context.Background()

	clientset, err := m.buildClient(clusterName)
	if err != nil {
		return "", err
	}

	limit := int64(opts.Lines)
	if limit <= 0 {
		limit = 100 // default
	}

	req := clientset.CoreV1().Pods(opts.Namespace).GetLogs(opts.PodName, &v1.PodLogOptions{
		Container:  opts.Container,
		TailLines:  &limit,
		Previous:   opts.Previous,
	})

	result, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer result.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(result)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// PodExec executes a command in a pod container using the Kubernetes exec API
func (m *ClusterManager) PodExec(clusterName string, opts ExecOptions) (*ExecResult, error) {
	ctx := context.Background()

	clientset, err := m.buildClient(clusterName)
	if err != nil {
		return nil, err
	}

	container := opts.Container
	if container == "" {
		// Get first container if not specified
		pod, err := clientset.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}
	}

	// Build the exec URL manually with query parameters
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		Param("container", container)

	// Add command as query parameters
	for _, cmd := range opts.Command {
		req.Param("command", cmd)
	}

	// Also set stdin/stdout/stderr via query params for proper execution
	req.Param("stdin", "false")
	req.Param("stdout", "true")
	req.Param("stderr", "true")
	req.Param("tty", "false")

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, err
	}

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	result := &ExecResult{
		Output: stdout.String(),
	}
	if err != nil {
		result.Error = stderr.String()
	}

	return result, nil
}

// GetClusterMetrics retrieves CPU and memory metrics for all nodes in a cluster
func (m *ClusterManager) GetClusterMetrics(clusterName string) ([]NodeMetrics, error) {
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

	var result []NodeMetrics
	for _, n := range nodes.Items {
		cpuCap := n.Status.Capacity.Cpu().String()
		memCap := n.Status.Capacity.Memory().String()

		// Calculate usage (Capacity - Allocated)
		cpuAlloc := n.Status.Allocatable.Cpu()
		memAlloc := n.Status.Allocatable.Memory()

		cpuUsage := "0"
		memUsage := "0"

		if cpuAlloc != nil && n.Status.Capacity.Cpu() != nil {
			cpuUsed := n.Status.Capacity.Cpu().DeepCopy()
			cpuUsed.Sub(*cpuAlloc)
			cpuUsage = cpuUsed.String()
		}

		if memAlloc != nil && n.Status.Capacity.Memory() != nil {
			memUsed := n.Status.Capacity.Memory().DeepCopy()
			memUsed.Sub(*memAlloc)
			memUsage = memUsed.String()
		}

		// If we couldn't calculate usage, use allocatable as approximation
		if cpuUsage == "0" && n.Status.Allocatable.Cpu() != nil {
			cpuUsage = n.Status.Allocatable.Cpu().String()
		}
		if memUsage == "0" && n.Status.Allocatable.Memory() != nil {
			memUsage = n.Status.Allocatable.Memory().String()
		}

		result = append(result, NodeMetrics{
			Name:     n.Name,
			CPUUsage: cpuUsage,
			CPUCap:   cpuCap,
			MemUsage: memUsage,
			MemCap:   memCap,
		})
	}

	return result, nil
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
	name := ginfadapter.Vars(r)["name"]
	if err := m.UnregisterCluster(name); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterClusterHTTP handles POST /api/k8s/clusters
func (m *ClusterManager) RegisterClusterHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		apierror.MethodNotAllowed(w)
		return
	}
	var input struct {
		Name       string `json:"name" binding:"required"`
		Type       string `json:"type" binding:"required"`
		Env        string `json:"environment"`
		Kubeconfig string `json:"kubeconfig" binding:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	env := ClusterEnvDev
	if input.Env != "" {
		env = ClusterEnv(input.Env)
	}

	cluster, err := m.RegisterCluster(input.Name, ClusterType(input.Type), env, input.Kubeconfig)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

// GetClusterHTTP handles GET /api/k8s/clusters/:name
func (m *ClusterManager) GetClusterHTTP(w http.ResponseWriter, r *http.Request) {
	name := ginfadapter.Vars(r)["name"]
	cluster, err := m.getClusterByName(name)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster)
}

// UpdateClusterHTTP handles PUT /api/k8s/clusters/:name
func (m *ClusterManager) UpdateClusterHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		apierror.MethodNotAllowed(w)
		return
	}
	name := ginfadapter.Vars(r)["name"]
	var input struct {
		Type   string `json:"type,omitempty"`
		Env    string `json:"environment,omitempty"`
		Status string `json:"status,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if input.Type != "" {
		updates["type"] = input.Type
	}
	if input.Env != "" {
		updates["env"] = input.Env
	}
	if input.Status != "" {
		updates["status"] = input.Status
	}

	if len(updates) > 0 {
		if err := m.db.Model(&GORMCluster{}).Where("name = ?", name).Updates(updates).Error; err != nil {
			apierror.InternalErrorFromErr(w, err)
			return
		}
	}

	cluster, err := m.getClusterByName(name)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster)
}

func (m *ClusterManager) HealthCheckHTTP(w http.ResponseWriter, r *http.Request) {
	name := ginfadapter.Vars(r)["name"]
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
	cluster := ginfadapter.Vars(r)["name"]
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
	cluster := ginfadapter.Vars(r)["name"]
	namespaces, err := m.GetNamespaces(cluster)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaces)
}

func (m *ClusterManager) GetPodsHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := ginfadapter.Vars(r)["name"]
	namespace := r.URL.Query().Get("namespace")
	// Empty namespace means all namespaces (not just "default")
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
	cluster := ginfadapter.Vars(r)["name"]
	namespace := r.URL.Query().Get("namespace")
	podName := ginfadapter.Vars(r)["pod"]
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

// GetPodLogsWithNamespaceHTTP handles GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs
// Supports ?previous=true for previous container logs and ?lines=N for tail count
func (m *ClusterManager) GetPodLogsWithNamespaceHTTP(w http.ResponseWriter, r *http.Request) {
	vars := ginfadapter.Vars(r)
	cluster := vars["name"]
	namespace := vars["ns"]
	podName := vars["pod"]

	previous := r.URL.Query().Get("previous") == "true"
	lines := int64(100)
	if linesStr := r.URL.Query().Get("lines"); linesStr != "" {
		if l, err := strconv.ParseInt(linesStr, 10, 64); err == nil && l > 0 {
			lines = l
		}
	}

	opts := PodLogsOptions{
		Namespace: namespace,
		PodName:   podName,
		Lines:     lines,
		Previous:  previous,
	}

	logs, err := m.GetPodLogsWithOptions(cluster, opts)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

// PodExecHTTP handles POST /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec
func (m *ClusterManager) PodExecHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		apierror.MethodNotAllowed(w)
		return
	}

	vars := ginfadapter.Vars(r)
	cluster := vars["name"]
	namespace := vars["ns"]
	podName := vars["pod"]

	var input struct {
		Command   []string `json:"command"`
		Container string   `json:"container,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	if len(input.Command) == 0 {
		apierror.ValidationError(w, "command is required")
		return
	}

	opts := ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: input.Container,
		Command:   input.Command,
	}

	result, err := m.PodExec(cluster, opts)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetClusterMetricsHTTP handles GET /api/k8s/clusters/:id/metrics
func (m *ClusterManager) GetClusterMetricsHTTP(w http.ResponseWriter, r *http.Request) {
	cluster := ginfadapter.Vars(r)["name"]

	metrics, err := m.GetClusterMetrics(cluster)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cluster": cluster,
		"nodes":   metrics,
	})
}