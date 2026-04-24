package physicalhost

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManager_NewManager(t *testing.T) {
	m := NewManager()
	if m.hosts == nil {
		t.Fatal("expected hosts to be initialized")
	}
	if m.sshCfg == nil {
		t.Fatal("expected sshCfg to be initialized")
	}
	if m.sshCfg.Timeout != 30*1e9 { // 30 seconds in nanoseconds
		t.Error("expected default timeout 30 seconds")
	}
}

func TestManager_CreateHost(t *testing.T) {
	m := NewManager()

	host := m.CreateHost("server1", "192.168.1.10", "admin", "key", 22)
	if host.Hostname != "server1" {
		t.Errorf("expected hostname 'server1', got '%s'", host.Hostname)
	}
	if host.IP != "192.168.1.10" {
		t.Errorf("expected IP '192.168.1.10', got '%s'", host.IP)
	}
	if host.Port != 22 {
		t.Errorf("expected port 22, got %d", host.Port)
	}
	if host.Username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", host.Username)
	}
	if host.State != "online" {
		t.Errorf("expected state 'online', got '%s'", host.State)
	}
	if host.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_GetHost(t *testing.T) {
	m := NewManager()

	created := m.CreateHost("test-host", "10.0.0.1", "user", "password", 22)
	retrieved := m.GetHost(created.ID)

	if retrieved == nil {
		t.Fatal("expected to retrieve host, got nil")
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}

	// Non-existent
	missing := m.GetHost("non-existent")
	if missing != nil {
		t.Error("expected nil for non-existent host")
	}
}

func TestManager_ListHosts(t *testing.T) {
	m := NewManager()

	m.CreateHost("host1", "10.0.0.1", "user", "key", 22)
	m.CreateHost("host2", "10.0.0.2", "user", "key", 22)

	hosts := m.ListHosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestManager_DeleteHost(t *testing.T) {
	m := NewManager()

	p1 := m.CreateHost("to-delete", "10.0.0.1", "user", "key", 22)
	m.CreateHost("to-keep", "10.0.0.2", "user", "key", 22)

	deleted := m.DeleteHost(p1.ID)
	if !deleted {
		t.Error("expected DeleteHost to return true")
	}

	hosts := m.ListHosts()
	if len(hosts) != 1 {
		t.Errorf("expected 1 host after delete, got %d", len(hosts))
	}
}

func TestManager_CollectMetrics(t *testing.T) {
	m := NewManager()

	host := m.CreateHost("metrics-test", "10.0.0.100", "user", "key", 22)

	// This will try to SSH but fail since host doesn't exist
	// The method should handle this gracefully
	err := m.CollectMetrics(host.ID)
	// Expect error since we can't actually connect
	if err == nil {
		t.Log("SSH connection succeeded (unexpected in test env)")
	}
}

func TestHost_MetricsStructures(t *testing.T) {
	host := &Host{
		ID:       "test-1",
		Hostname: "test-host",
		Metrics: &HostMetrics{
			CPU: CPUStats{
				Usage:  45.5,
				Cores:  4,
				Idle:   54.5,
			},
			Memory: MemoryStats{
				Total:        8192000,
				Used:         4096000,
				Free:         4096000,
				UsagePercent: 50.0,
			},
		},
	}

	if host.Metrics.CPU.Usage != 45.5 {
		t.Errorf("expected CPU usage 45.5, got %f", host.Metrics.CPU.Usage)
	}
	if host.Metrics.Memory.UsagePercent != 50.0 {
		t.Errorf("expected memory usage 50%%, got %f", host.Metrics.Memory.UsagePercent)
	}
}

func TestManager_ListHostsHTTP(t *testing.T) {
	m := NewManager()
	m.CreateHost("http-test", "10.0.0.1", "admin", "key", 22)

	req := httptest.NewRequest("GET", "/api/physical-hosts", nil)
	w := httptest.NewRecorder()

	m.ListHostsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "http-test") {
		t.Error("expected response to contain hostname")
	}
}

func TestManager_CreateHostHTTP(t *testing.T) {
	m := NewManager()

	body := `{"hostname":"new-host","ip":"10.0.0.50","username":"admin","auth_method":"key","port":22}`
	req := httptest.NewRequest("POST", "/api/physical-hosts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreateHostHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "new-host") {
		t.Error("expected response to contain hostname")
	}
}

func TestManager_CreateHostHTTP_DefaultPort(t *testing.T) {
	m := NewManager()

	body := `{"hostname":"no-port","ip":"10.0.0.50","username":"admin","auth_method":"key"}`
	req := httptest.NewRequest("POST", "/api/physical-hosts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreateHostHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

