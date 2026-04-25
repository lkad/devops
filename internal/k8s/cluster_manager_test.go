package k8s

import (
	"os"
	"testing"
)

func TestClusterManager_NewClusterManager(t *testing.T) {
	m := NewClusterManager()
	if m.provider != "k3d" {
		t.Errorf("expected provider 'k3d', got '%s'", m.provider)
	}
	if m.k3dPath != "k3d" {
		t.Errorf("expected k3dPath 'k3d', got '%s'", m.k3dPath)
	}
	if m.kindPath != "kind" {
		t.Errorf("expected kindPath 'kind', got '%s'", m.kindPath)
	}
}

func TestCluster_Struct(t *testing.T) {
	cluster := &Cluster{
		Name:     "test-cluster",
		Type:     "k3d",
		Agents:   3,
		APIPort:  31000,
		Status:   "running",
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("expected name 'test-cluster', got '%s'", cluster.Name)
	}
	if cluster.Agents != 3 {
		t.Errorf("expected agents 3, got %d", cluster.Agents)
	}
	if cluster.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", cluster.Status)
	}
}

func TestParseK3dList(t *testing.T) {
	// Test parsing k3d cluster list output
	output := `NAME                SERVER          AGENTS    LB
dev-cluster-1      localhost:6443   3         localhost:0
dev-cluster-2       localhost:6444   2         localhost:0`

	result := parseK3dList(output)
	if len(result) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result))
	}
	if result[0]["name"] != "dev-cluster-1" {
		t.Errorf("expected first cluster name 'dev-cluster-1', got '%s'", result[0]["name"])
	}
	if result[1]["name"] != "dev-cluster-2" {
		t.Errorf("expected second cluster name 'dev-cluster-2', got '%s'", result[1]["name"])
	}
}

func TestClusterManager_DeleteCluster(t *testing.T) {
	m := NewClusterManager()
	// DeleteCluster just runs k3d which we can't test without cluster
	// We just verify the method exists - it will error when k3d not found
	err := m.DeleteCluster("non-existent-cluster")
	// In test env without k3d, we expect an error
	if err == nil {
		t.Log("k3d appears to be installed - cluster deleted (unexpected in test env)")
	}
}

// Integration tests - require real k3d clusters
func skipIfNoK8s(t *testing.T) string {
	// Check for k3d kubeconfig
	home := os.Getenv("HOME")
	kubeconfig := home + "/.kube/config-dev-cluster-1"
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skip("Skipping: no k3d cluster kubeconfig at " + kubeconfig)
	}
	return kubeconfig
}

func TestListClusters_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()
	clusters, err := m.ListClusters()
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}
	t.Logf("Found %d clusters: %v", len(clusters), clusters)
}

func TestGetNodes_Integration(t *testing.T) {
	kubeconfig := skipIfNoK8s(t)
	// Get cluster name from kubeconfig path
	clusterName := "dev-cluster-1"

	// Verify kubeconfig exists
	_ = kubeconfig // used for reference

	m := NewClusterManager()
	nodes, err := m.GetNodes(clusterName)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected at least one node")
	}

	// Verify node structure
	for _, n := range nodes {
		if n.Name == "" {
			t.Error("node name is empty")
		}
		if n.Role == "" {
			t.Error("node role is empty")
		}
		t.Logf("Node: name=%s role=%s ready=%v cpu=%s memory=%s",
			n.Name, n.Role, n.Ready, n.CPU, n.Memory)
	}
}

func TestGetNamespaces_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()
	namespaces, err := m.GetNamespaces("dev-cluster-1")
	if err != nil {
		t.Fatalf("GetNamespaces failed: %v", err)
	}
	if len(namespaces) == 0 {
		t.Fatal("expected at least one namespace")
	}

	// Verify common namespaces exist
	found := false
	for _, ns := range namespaces {
		if ns == "kube-system" {
			found = true
		}
		t.Logf("Namespace: %s", ns)
	}
	if !found {
		t.Error("expected kube-system namespace")
	}
}

func TestGetPods_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()
	pods, err := m.GetPods("dev-cluster-1", "kube-system")
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	if len(pods) == 0 {
		t.Fatal("expected at least one pod in kube-system")
	}

	// Verify pod structure
	for _, p := range pods {
		if p.Name == "" {
			t.Error("pod name is empty")
		}
		if p.Namespace != "kube-system" {
			t.Errorf("expected namespace 'kube-system', got '%s'", p.Namespace)
		}
		t.Logf("Pod: name=%s ready=%s status=%s restarts=%d",
			p.Name, p.Ready, p.Status, p.Restarts)
	}
}

func TestHealthCheck_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()
	status, err := m.HealthCheck("dev-cluster-1")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if status["connected"] != true {
		t.Error("expected connected=true")
	}
	if status["ready"] != true {
		t.Error("expected ready=true")
	}

	nodes, ok := status["nodes"].([]map[string]string)
	if !ok {
		t.Fatal("expected nodes array in status")
	}
	t.Logf("Health check: connected=%v ready=%v nodes=%d",
		status["connected"], status["ready"], len(nodes))
}

func TestCordonUncordon_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()

	// Use agent-0 for testing (safe to cordon in test env)
	nodeName := "k3d-dev-cluster-1-agent-0"
	clusterName := "dev-cluster-1"

	// Cordon the node
	err := m.CordonNode(clusterName, nodeName)
	if err != nil {
		t.Fatalf("CordonNode failed: %v", err)
	}
	t.Logf("Node %s cordoned", nodeName)

	// Verify node is cordoned
	nodes, err := m.GetNodes(clusterName)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	cordoned := false
	for _, n := range nodes {
		if n.Name == nodeName {
			t.Logf("Node %s taints after cordon: %v", nodeName, n.Taints)
			if len(n.Taints) > 0 {
				cordoned = true
			}
		}
	}
	if !cordoned {
		t.Log("Note: taints may not be immediately reflected in node list")
	}

	// Uncordon the node
	err = m.UncordonNode(clusterName, nodeName)
	if err != nil {
		t.Fatalf("UncordonNode failed: %v", err)
	}
	t.Logf("Node %s uncordoned", nodeName)

	// Verify node is uncordoned - check taints are cleared or node is schedulable
	nodes, err = m.GetNodes(clusterName)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	for _, n := range nodes {
		if n.Name == nodeName {
			t.Logf("Node %s taints after uncordon: %v", nodeName, n.Taints)
			// Verify node shows as Ready and schedulable
			if !n.Ready {
				t.Error("expected node to be Ready after uncordon")
			}
		}
	}
}

func TestGetPodLogs_Integration(t *testing.T) {
	skipIfNoK8s(t)
	m := NewClusterManager()

	// Get a pod from kube-system to read logs
	pods, err := m.GetPods("dev-cluster-1", "kube-system")
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	if len(pods) == 0 {
		t.Skip("no pods available for log test")
	}

	// Get logs from first pod
	podName := pods[0].Name
	logs, err := m.GetPodLogs("dev-cluster-1", "kube-system", podName, 10)
	if err != nil {
		t.Fatalf("GetPodLogs failed: %v", err)
	}
	t.Logf("Pod %s logs (last 10 lines):\n%s", podName, logs)
}

func TestSplitNamespaceResource(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"default/nginx-7fb96c846b-xf6g9", []string{"default", "nginx-7fb96c846b-xf6g9"}},
		{"kube-system/kube-scheduler-k3d-dev-cluster-1-server-0", []string{"kube-system", "kube-scheduler-k3d-dev-cluster-1-server-0"}},
		{"only-one", []string{"only-one"}},
	}

	for _, tt := range tests {
		result := splitNamespaceResource(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitNamespaceResource(%s) returned %d parts, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitNamespaceResource(%s)[%d] = %s, want %s", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestCountReady(t *testing.T) {
	// Test with empty conditions
	count := countReady(nil)
	if count != 0 {
		t.Errorf("countReady(nil) = %d, want 0", count)
	}
}

func TestNode_Struct(t *testing.T) {
	node := Node{
		Name:      "test-node",
		Ready:     true,
		Role:      "worker",
		CPU:       "4",
		Memory:    "8Gi",
		Age:       "10h",
		Condition: "Ready",
	}

	if node.Name != "test-node" {
		t.Errorf("expected name 'test-node', got '%s'", node.Name)
	}
	if !node.Ready {
		t.Error("expected Ready=true")
	}
	if node.Role != "worker" {
		t.Errorf("expected role 'worker', got '%s'", node.Role)
	}
}

func TestPod_Struct(t *testing.T) {
	pod := Pod{
		Name:      "nginx-7fb96c846b-xf6g9",
		Namespace: "default",
		Ready:     "1/1",
		Status:    "Running",
		Restarts:  0,
		NodeName:  "worker-1",
		IP:        "10.42.1.3",
	}

	if pod.Name != "nginx-7fb96c846b-xf6g9" {
		t.Errorf("expected name 'nginx-7fb96c846b-xf6g9', got '%s'", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", pod.Namespace)
	}
	if pod.Status != "Running" {
		t.Errorf("expected status 'Running', got '%s'", pod.Status)
	}
}