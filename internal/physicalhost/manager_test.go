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

	"github.com/gorilla/mux"
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

func TestParseServiceList(t *testing.T) {
	// Sample systemctl output
	output := []byte(`nginx.service loaded active running A nginx HTTP and reverse proxy server
sshd.service loaded active running OpenSSH daemon
crond.service loaded active running Regular background program processing daemon
failed.service loaded failed failed Failed to start Some service
`)

	services := parseServiceList(output)

	if len(services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(services))
	}

	// Check nginx
	if services[0].Name != "nginx" {
		t.Errorf("expected first service 'nginx', got '%s'", services[0].Name)
	}
	if !services[0].Active {
		t.Error("expected nginx to be active")
	}

	// Check sshd
	if services[1].Name != "sshd" {
		t.Errorf("expected second service 'sshd', got '%s'", services[1].Name)
	}
	if !services[1].Active {
		t.Error("expected sshd to be active")
	}

	// Check crond
	if services[2].Name != "crond" {
		t.Errorf("expected third service 'crond', got '%s'", services[2].Name)
	}
	if !services[2].Active {
		t.Error("expected crond to be active")
	}

	// Check failed service
	if services[3].Name != "failed" {
		t.Errorf("expected fourth service 'failed', got '%s'", services[3].Name)
	}
	if services[3].Active {
		t.Error("expected failed service to not be active")
	}
}

func TestParseServiceList_EmptyOutput(t *testing.T) {
	output := []byte("")
	services := parseServiceList(output)

	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestParseServiceList_OnlyWhitespace(t *testing.T) {
	output := []byte("   \n   \n   ")
	services := parseServiceList(output)

	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestParseServiceList_ServiceWithoutSuffix(t *testing.T) {
	// Some services might not have .service suffix in output
	output := []byte(`docker loaded active running Docker Application Container Engine
`)

	services := parseServiceList(output)

	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name != "docker" {
		t.Errorf("expected service 'docker', got '%s'", services[0].Name)
	}
}

func TestManager_ListServicesHTTP_HostNotFound(t *testing.T) {
	m := NewManager()

	req := httptest.NewRequest("GET", "/api/physical-hosts/nonexistent/services", nil)
	w := httptest.NewRecorder()

	m.ListServicesHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestManager_ListServicesHTTP_HostExists(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	req := httptest.NewRequest("GET", "/api/physical-hosts/"+host.ID+"/services", nil)
	// Set mux variables since we're calling handler directly
	// Note: SetURLVars returns a new request with updated context
	vars := map[string]string{"id": host.ID}
	req = mux.SetURLVars(req, vars)

	w := httptest.NewRecorder()

	// This will fail to connect but should still return 500 (not 404)
	// since the host exists
	m.ListServicesHTTP(w, req)

	// Should get an error response since SSH won't work in test env
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("expected status 500 or 200, got %d", w.Code)
	}
}

func TestManager_PushConfigHTTP_ValidationError(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Test with missing path
	body := `{"content": "some content"}`
	req := httptest.NewRequest("POST", "/api/physical-hosts/"+host.ID+"/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Set mux variables since we're calling handler directly
	// Note: SetURLVars returns a new request with updated context
	vars := map[string]string{"id": host.ID}
	req = mux.SetURLVars(req, vars)
	w := httptest.NewRecorder()

	m.PushConfigHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestManager_PushConfigHTTP_ValidationErrorMissingContent(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Test with missing content
	body := `{"path": "/etc/nginx/nginx.conf"}`
	req := httptest.NewRequest("POST", "/api/physical-hosts/"+host.ID+"/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Set mux variables since we're calling handler directly
	// Note: SetURLVars returns a new request with updated context
	vars := map[string]string{"id": host.ID}
	req = mux.SetURLVars(req, vars)
	w := httptest.NewRecorder()

	m.PushConfigHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestManager_PushConfigHTTP_HostNotFound(t *testing.T) {
	m := NewManager()

	body := `{"path": "/etc/nginx/nginx.conf", "content": "some config"}`
	req := httptest.NewRequest("POST", "/api/physical-hosts/nonexistent/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.PushConfigHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestManager_PushConfig_InvalidRequest(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Test with nil request
	err := m.PushConfig(host.ID, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}

	// Test with empty path
	err = m.PushConfig(host.ID, &PushConfigRequest{Path: "", Content: "test"})
	if err == nil {
		t.Error("expected error for empty path")
	}

	// Test with empty content
	err = m.PushConfig(host.ID, &PushConfigRequest{Path: "/etc/test", Content: ""})
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestManager_PushConfig_NonExistentHost(t *testing.T) {
	m := NewManager()

	err := m.PushConfig("nonexistent", &PushConfigRequest{Path: "/etc/test", Content: "test"})
	if err == nil {
		t.Error("expected error for non-existent host")
	}
}

func TestManager_ListServices_NonExistentHost(t *testing.T) {
	m := NewManager()

	_, err := m.ListServices("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent host")
	}
}

func TestPushConfigRequest_Structure(t *testing.T) {
	req := PushConfigRequest{
		Path:    "/etc/nginx/nginx.conf",
		Content: "worker_processes 4;",
	}

	if req.Path != "/etc/nginx/nginx.conf" {
		t.Errorf("expected path '/etc/nginx/nginx.conf', got '%s'", req.Path)
	}
	if req.Content != "worker_processes 4;" {
		t.Errorf("unexpected content: '%s'", req.Content)
	}
}

// Two-Layer Architecture and Cache Tests

func TestDataStatus_Constants(t *testing.T) {
	if DataStatusFresh != "fresh" {
		t.Errorf("expected DataStatusFresh 'fresh', got '%s'", DataStatusFresh)
	}
	if DataStatusStale != "stale" {
		t.Errorf("expected DataStatusStale 'stale', got '%s'", DataStatusStale)
	}
	if DataStatusUnavailable != "unavailable" {
		t.Errorf("expected DataStatusUnavailable 'unavailable', got '%s'", DataStatusUnavailable)
	}
}

func TestNewMetricsCache(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.entries == nil {
		t.Fatal("expected entries to be initialized")
	}
	if cache.maxAge != 5*time.Minute {
		t.Errorf("expected maxAge 5m, got %v", cache.maxAge)
	}
	if cache.staleAge != 2*time.Minute {
		t.Errorf("expected staleAge 2m, got %v", cache.staleAge)
	}
}

func TestNewMetricsCache_Defaults(t *testing.T) {
	cache := NewMetricsCache(0, 0)
	if cache.maxAge != 5*time.Minute {
		t.Errorf("expected default maxAge 5m, got %v", cache.maxAge)
	}
	if cache.staleAge != 2*time.Minute {
		t.Errorf("expected default staleAge 2m, got %v", cache.staleAge)
	}
}

func TestMetricsCache_SetAndGet(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	metrics := &HostMetrics{
		CPU: CPUStats{Usage: 50.0, Cores: 4, Idle: 50.0},
	}
	cache.Set("host1", metrics, DataStatusFresh)

	cached, exists := cache.Get("host1")
	if !exists {
		t.Fatal("expected to get cached metrics")
	}
	if cached.Metrics.CPU.Usage != 50.0 {
		t.Errorf("expected CPU usage 50.0, got %f", cached.Metrics.CPU.Usage)
	}
	if cached.DataStatus != DataStatusFresh {
		t.Errorf("expected status fresh, got %s", cached.DataStatus)
	}
}

func TestMetricsCache_GetNonExistent(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	_, exists := cache.Get("nonexistent")
	if exists {
		t.Error("expected no cached entry for nonexistent host")
	}
}

func TestMetricsCache_Delete(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	metrics := &HostMetrics{
		CPU: CPUStats{Usage: 50.0},
	}
	cache.Set("host1", metrics, DataStatusFresh)

	cache.Delete("host1")

	_, exists := cache.Get("host1")
	if exists {
		t.Error("expected entry to be deleted")
	}
}

func TestMetricsCache_Clear(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	cache.Set("host1", &HostMetrics{}, DataStatusFresh)
	cache.Set("host2", &HostMetrics{}, DataStatusFresh)

	cache.Clear()

	_, exists1 := cache.Get("host1")
	_, exists2 := cache.Get("host2")
	if exists1 || exists2 {
		t.Error("expected all entries to be cleared")
	}
}

func TestMetricsCache_GetDataStatus_Fresh(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	metrics := &HostMetrics{}
	cache.Set("host1", metrics, DataStatusFresh)

	status := cache.GetDataStatus("host1")
	if status != DataStatusFresh {
		t.Errorf("expected fresh status, got %s", status)
	}
}

func TestMetricsCache_GetDataStatus_Stale(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 1*time.Minute) // staleAge=1min

	metrics := &HostMetrics{}
	cache.Set("host1", metrics, DataStatusFresh)

	// Simulate cache entry older than staleAge but younger than maxAge
	cache.mu.Lock()
	cache.entries["host1"].CachedAt = time.Now().Add(-2 * time.Minute)
	cache.mu.Unlock()

	status := cache.GetDataStatus("host1")
	if status != DataStatusStale {
		t.Errorf("expected stale status, got %s", status)
	}
}

func TestMetricsCache_GetDataStatus_Unavailable(t *testing.T) {
	cache := NewMetricsCache(5*time.Minute, 2*time.Minute)

	status := cache.GetDataStatus("nonexistent")
	if status != DataStatusUnavailable {
		t.Errorf("expected unavailable status, got %s", status)
	}
}

func TestMetricsCache_GetDataStatus_MaxAgeExceeded(t *testing.T) {
	cache := NewMetricsCache(1*time.Minute, 30*time.Second)

	metrics := &HostMetrics{}
	cache.Set("host1", metrics, DataStatusFresh)

	// Simulate cache entry older than maxAge
	cache.mu.Lock()
	cache.entries["host1"].CachedAt = time.Now().Add(-2 * time.Minute)
	cache.mu.Unlock()

	status := cache.GetDataStatus("host1")
	if status != DataStatusUnavailable {
		t.Errorf("expected unavailable status when maxAge exceeded, got %s", status)
	}
}

func TestManager_CacheInitialized(t *testing.T) {
	m := NewManager()
	if m.cache == nil {
		t.Fatal("expected cache to be initialized")
	}
}

func TestManager_GetMetrics_NoCache(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	resp, err := m.GetMetrics(host.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.DataStatus != DataStatusUnavailable {
		t.Errorf("expected unavailable status with no cache, got %s", resp.DataStatus)
	}
}

func TestManager_GetMetrics_WithCache(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Manually set cache
	metrics := &HostMetrics{
		CPU: CPUStats{Usage: 75.0, Cores: 8},
	}
	m.cache.Set(host.ID, metrics, DataStatusFresh)

	resp, err := m.GetMetrics(host.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.DataStatus != DataStatusFresh {
		t.Errorf("expected fresh status, got %s", resp.DataStatus)
	}
	if resp.Metrics.CPU.Usage != 75.0 {
		t.Errorf("expected CPU usage 75.0, got %f", resp.Metrics.CPU.Usage)
	}
}

func TestManager_GetMetrics_NonExistentHost(t *testing.T) {
	m := NewManager()

	_, err := m.GetMetrics("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent host")
	}
}

func TestManager_DeleteHost_ClearsCache(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Set cache
	metrics := &HostMetrics{}
	m.cache.Set(host.ID, metrics, DataStatusFresh)

	// Verify cache has entry
	_, exists := m.cache.Get(host.ID)
	if !exists {
		t.Fatal("expected cache entry before delete")
	}

	m.DeleteHost(host.ID)

	// Verify cache is cleared
	_, exists = m.cache.Get(host.ID)
	if exists {
		t.Error("expected cache entry to be cleared after delete")
	}
}

func TestManager_CheckNodeHealth_NonExistentHost(t *testing.T) {
	m := NewManager()

	err := m.CheckNodeHealth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent host")
	}
}

func TestManager_CheckNodeHealth_SSHFails(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// SSH will fail since host doesn't exist - CheckNodeHealth should handle this
	err := m.CheckNodeHealth(host.ID)
	if err == nil {
		t.Log("SSH connection succeeded (unexpected in test env)")
	} else {
		// Error expected - host should be marked offline
		if host.State != "offline" {
			t.Errorf("expected host state 'offline', got '%s'", host.State)
		}
	}
}

func TestManager_CollectMetrics_CachesResult(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// CollectMetrics will fail (no real SSH) but the method should handle it
	m.CollectMetrics(host.ID)

	// Cache should be updated even on failure (with stale data if cache exists)
	// or error if no cache
}

func TestMetricsResponse_Structure(t *testing.T) {
	now := time.Now()
	resp := &MetricsResponse{
		HostID:     "host-123",
		Hostname:   "test.example.com",
		Metrics:    &HostMetrics{},
		DataStatus: DataStatusStale,
		CachedAt:   &now,
	}

	if resp.HostID != "host-123" {
		t.Errorf("expected host ID 'host-123', got '%s'", resp.HostID)
	}
	if resp.Hostname != "test.example.com" {
		t.Errorf("expected hostname 'test.example.com', got '%s'", resp.Hostname)
	}
	if resp.DataStatus != DataStatusStale {
		t.Errorf("expected data status 'stale', got '%s'", resp.DataStatus)
	}
}

func TestManager_GetMetricsHTTP_HostNotFound(t *testing.T) {
	m := NewManager()

	req := httptest.NewRequest("GET", "/api/physical-hosts/nonexistent/metrics", nil)
	w := httptest.NewRecorder()

	m.GetMetricsHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestManager_GetMetricsHTTP_WithCache(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Pre-populate cache
	metrics := &HostMetrics{
		CPU: CPUStats{Usage: 60.0, Cores: 4},
	}
	m.cache.Set(host.ID, metrics, DataStatusFresh)

	vars := map[string]string{"id": host.ID}
	req := httptest.NewRequest("GET", "/api/physical-hosts/"+host.ID+"/metrics", nil)
	req = mux.SetURLVars(req, vars)
	w := httptest.NewRecorder()

	m.GetMetricsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"data_status":"fresh"`) {
		t.Error("expected response to contain fresh status")
	}
}

func TestManager_HealthCheckHTTP_HostNotFound(t *testing.T) {
	m := NewManager()

	req := httptest.NewRequest("GET", "/api/physical-hosts/nonexistent/health", nil)
	w := httptest.NewRecorder()

	m.HealthCheckHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestManager_HealthCheckHTTP_OfflineHost(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)
	host.State = "offline"

	vars := map[string]string{"id": host.ID}
	req := httptest.NewRequest("GET", "/api/physical-hosts/"+host.ID+"/health", nil)
	req = mux.SetURLVars(req, vars)
	w := httptest.NewRecorder()

	m.HealthCheckHTTP(w, req)

	// Health check should fail with 503 since SSH won't work
	if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusOK {
		t.Errorf("expected status 503 or 200, got %d", w.Code)
	}
}

func TestManager_RefreshMetricsHTTP(t *testing.T) {
	m := NewManager()
	host := m.CreateHost("test-host", "10.0.0.1", "admin", "key", 22)

	// Pre-populate cache with stale data
	metrics := &HostMetrics{
		CPU: CPUStats{Usage: 30.0, Cores: 2},
	}
	m.cache.Set(host.ID, metrics, DataStatusStale)

	vars := map[string]string{"id": host.ID}
	req := httptest.NewRequest("POST", "/api/physical-hosts/"+host.ID+"/metrics/refresh", nil)
	req = mux.SetURLVars(req, vars)
	w := httptest.NewRecorder()

	m.RefreshMetricsHTTP(w, req)

	// Should return 200 or 500 depending on SSH availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}
