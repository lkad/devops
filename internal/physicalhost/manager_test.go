// Package physicalhost tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package physicalhost

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
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
	if m.pool == nil {
		t.Fatal("expected pool to be initialized")
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

// SSH Connection Pool Tests

func TestDefaultSSHConnPoolConfig(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.MaxConns != 5 {
		t.Errorf("expected MaxConns 5, got %d", cfg.MaxConns)
	}
	if cfg.ConnTTL != 5*time.Minute {
		t.Errorf("expected ConnTTL 5m, got %v", cfg.ConnTTL)
	}
}

func TestNewSSHConnPool(t *testing.T) {
	cfg := &SSHConfig{
		Timeout:  10 * time.Second,
		MaxConns: 3,
		ConnTTL:  2 * time.Minute,
	}
	dialer := func(host *Host) (*ssh.Client, error) {
		return nil, nil
	}
	pool := NewSSHConnPool(cfg, dialer)
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if pool.maxConns != 3 {
		t.Errorf("expected maxConns 3, got %d", pool.maxConns)
	}
	if pool.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", pool.timeout)
	}
}

func TestSSHConnPool_PoolKey(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	pool := NewSSHConnPool(cfg, nil)

	host := &Host{
		IP:       "192.168.1.10",
		Port:     22,
		Username: "admin",
	}

	key := pool.poolKey(host)
	expected := "192.168.1.10:22:admin"
	if key != expected {
		t.Errorf("expected key '%s', got '%s'", expected, key)
	}
}

func TestSSHConnPool_PutNilClient(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	pool := NewSSHConnPool(cfg, nil)

	host := &Host{
		IP:       "192.168.1.10",
		Port:     22,
		Username: "admin",
	}

	// Should not panic
	pool.Put(host, nil)
}

func TestSSHConnPool_StatsEmptyPool(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	pool := NewSSHConnPool(cfg, nil)

	host := &Host{
		IP:       "192.168.1.10",
		Port:     22,
		Username: "admin",
	}

	avail, inUse := pool.Stats(host)
	if avail != 0 || inUse != 0 {
		t.Errorf("expected 0 avail, 0 inUse; got %d, %d", avail, inUse)
	}
}

func TestSSHConnPool_CloseHostPool(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	pool := NewSSHConnPool(cfg, nil)

	host := &Host{
		IP:       "192.168.1.10",
		Port:     22,
		Username: "admin",
	}

	// Should not panic on empty pool
	pool.CloseHostPool(host)
}

func TestSSHConnPool_Close(t *testing.T) {
	cfg := DefaultSSHConnPoolConfig()
	pool := NewSSHConnPool(cfg, nil)

	// Should not panic on empty pool
	pool.Close()
}

func TestManager_SSHPutConnection(t *testing.T) {
	m := NewManager()

	host := &Host{
		IP:       "192.168.1.10",
		Port:     22,
		Username: "admin",
	}

	// Should not panic with nil client
	m.sshPutConnection(host, nil)
}

func TestManager_PoolInitialized(t *testing.T) {
	m := NewManager()
	if m.pool == nil {
		t.Fatal("expected pool to be initialized")
	}
	// Pool should be functional
	cfg := DefaultSSHConnPoolConfig()
	if m.pool.maxConns != cfg.MaxConns {
		t.Errorf("expected pool maxConns %d, got %d", cfg.MaxConns, m.pool.maxConns)
	}
}

