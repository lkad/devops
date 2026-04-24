package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Agents    int       `json:"agents"`
	APIPort   int       `json:"api_port"`
	Kubeconfig string   `json:"kubeconfig"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
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
			Name:     line["name"],
			Provider: "k3d",
			Status:   "running",
		})
	}
	return clusters, nil
}

func parseK3dList(output string) []map[string]string {
	var result []map[string]string
	// Simple parsing - in production would be more robust
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

	kubeconfigPath := filepath.Join(m.kubeconfigDir, fmt.Sprintf("config-%s", name))
	if err := exec.Command(m.k3dPath, "kubeconfig", "get", name).Run(); err != nil {
		return nil, err
	}

	return &Cluster{
		Name:       name,
		Provider:   "k3d",
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
	kubeconfigPath := filepath.Join(m.kubeconfigDir, fmt.Sprintf("config-%s", name))

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

	var allReady bool
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
	kubeconfigPath := filepath.Join(m.kubeconfigDir, fmt.Sprintf("config-%s", name))

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

// HTTP handlers
func (m *ClusterManager) ListClustersHTTP(w http.ResponseWriter, r *http.Request) {
	clusters, err := m.ListClusters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

func (m *ClusterManager) CreateClusterHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Agents  int    `json:"agents"`
		APIPort int    `json:"api_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

func (m *ClusterManager) DeleteClusterHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")
	if err := m.DeleteCluster(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *ClusterManager) HealthCheckHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")
	status, err := m.HealthCheck(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
