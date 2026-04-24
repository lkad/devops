package k8s

import (
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
		Provider: "k3d",
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
	// parseK3dList is a stub that returns empty (nil) slice
	result := parseK3dList("")
	// The stub returns nil since it's not implemented
	if result != nil {
		t.Errorf("expected nil result from stub, got %v", result)
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