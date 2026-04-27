package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/devops-toolkit/pkg/config"
)

func TestPoolConfigDefaults(t *testing.T) {
	// Test that pool config defaults are correctly applied
	poolConfig := PoolConfig{}

	if poolConfig.MaxConnections != 0 {
		t.Errorf("expected MaxConnections to be 0 initially, got %d", poolConfig.MaxConnections)
	}
}

func TestNewLDAPClient(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled:      true,
		Host:         "localhost",
		Port:         389,
		BaseDN:       "dc=example,dc=com",
		UserFilter:   "(uid=%s)",
		GroupMapping: map[string]string{},
	}

	client := NewLDAPClient(cfg)

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.config != cfg {
		t.Error("client config does not match")
	}

	if client.pool != nil {
		t.Error("expected nil pool for client created with NewLDAPClient")
	}
}

func TestNewLDAPClientWithPool(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled:      true,
		Host:         "localhost",
		Port:         389,
		BaseDN:       "dc=example,dc=com",
		UserFilter:   "(uid=%s)",
		GroupMapping: map[string]string{},
	}

	poolConfig := PoolConfig{
		MaxConnections: 2,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}
	defer pool.Close()

	client := NewLDAPClientWithPool(cfg, pool)

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.pool != pool {
		t.Error("client pool does not match")
	}
}

func TestLDAPClientSetPool(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	client := NewLDAPClient(cfg)
	if client.pool != nil {
		t.Error("expected nil pool initially")
	}

	poolConfig := PoolConfig{
		MaxConnections: 2,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}
	defer pool.Close()

	client.SetPool(pool)

	if client.pool != pool {
		t.Error("pool not set correctly")
	}
}

func TestLDAPPoolClose(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	poolConfig := PoolConfig{
		MaxConnections: 2,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}

	// Close should not panic
	pool.Close()
}

func TestLDAPPoolPutNilOrClosing(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	poolConfig := PoolConfig{
		MaxConnections: 2,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}
	defer pool.Close()

	// Putting nil should not panic
	pool.Put(nil)
}

func TestMaxRetriesConstant(t *testing.T) {
	if maxRetries != 3 {
		t.Errorf("expected maxRetries to be 3, got %d", maxRetries)
	}
}

func TestBaseRetryDelayConstant(t *testing.T) {
	expected := 100 * time.Millisecond
	if baseRetryDelay != expected {
		t.Errorf("expected baseRetryDelay to be %v, got %v", expected, baseRetryDelay)
	}
}

func TestRetryExponentialBackoff(t *testing.T) {
	// Verify exponential backoff calculation
	delays := []time.Duration{
		baseRetryDelay * time.Duration(1<<0), // 100ms
		baseRetryDelay * time.Duration(1<<1), // 200ms
		baseRetryDelay * time.Duration(1<<2), // 400ms
	}

	expectedDelays := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
	}

	for i, expected := range expectedDelays {
		if delays[i] != expected {
			t.Errorf("delay[%d] = %v, want %v", i, delays[i], expected)
		}
	}
}

func TestIsTransientError(t *testing.T) {
	// Test nil error returns false
	if isTransientError(nil) {
		t.Error("isTransientError(nil) should return false")
	}
}

func TestLDAPClientDisabled(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: false,
	}

	client := NewLDAPClient(cfg)

	user, err := client.Authenticate("testuser", "testpass")

	if err == nil {
		t.Error("expected error when LDAP is disabled")
	}

	if user != nil {
		t.Error("expected nil user when LDAP is disabled")
	}
}

func TestLDAPPoolConcurrency(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	poolConfig := PoolConfig{
		MaxConnections: 5,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	concurrency := 10
	iterations := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				conn, err := pool.Get()
				if err != nil {
					// Connection failures are expected without LDAP server
					return
				}
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				pool.Put(conn)
			}
		}()
	}

	wg.Wait()
}

func TestLDAPPoolGetNonBlocking(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	poolConfig := PoolConfig{
		MaxConnections: 1,
		MaxAge:         5 * time.Minute,
	}

	pool, err := NewLDAPPool(cfg, poolConfig)
	if err != nil {
		t.Skipf("LDAP server not available: %v", err)
	}
	defer pool.Close()

	// Get the only connection
	conn1, err := pool.Get()
	if err != nil {
		t.Skip("LDAP server not available")
	}

	// Get should still work even when pool is empty (creates new connection)
	conn2, err := pool.Get()
	if err != nil {
		t.Skip("LDAP server not available")
	}

	// Return both
	pool.Put(conn1)
	pool.Put(conn2)
}
func TestHealthCheckDisabled(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: false,
	}

	client := NewLDAPClient(cfg)
	healthy, reason := client.HealthCheck()

	if healthy {
		t.Error("expected unhealthy when LDAP is disabled")
	}
	if reason != "LDAP is disabled" {
		t.Errorf("expected 'LDAP is disabled', got '%s'", reason)
	}
}

func TestHealthCheckUnreachable(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    389,
	}

	client := NewLDAPClient(cfg)
	healthy, reason := client.HealthCheck()

	// Without an LDAP server running, we expect unhealthy
	if healthy {
		t.Skip("LDAP server appears to be running, skipping unreachable test")
	}
	if reason == "" {
		t.Error("expected non-empty reason when LDAP is unreachable")
	}
}

func TestHealthCheckWithBindDN(t *testing.T) {
	cfg := &config.LDAPConfig{
		Enabled:      true,
		Host:         "localhost",
		Port:         389,
		BindDN:       "cn=admin,dc=example,dc=com",
		BindPassword: "admin",
	}

	client := NewLDAPClient(cfg)
	healthy, reason := client.HealthCheck()

	// Without an LDAP server running, we expect unhealthy
	if healthy {
		t.Skip("LDAP server appears to be running, skipping bind test")
	}
	// The reason should indicate connection failure
	if reason == "" {
		t.Error("expected non-empty reason when LDAP is unreachable")
	}
}
