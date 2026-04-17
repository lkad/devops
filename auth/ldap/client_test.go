package ldap

import (
	"os"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				URL:          "ldap://localhost:389",
				BaseDN:       "dc=example,dc=com",
				AdminDN:      "cn=admin,dc=example,dc=com",
				AdminPassword: "admin",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			config: &Config{
				URL:    "",
				BaseDN: "dc=example,dc=com",
				AdminDN: "cn=admin,dc=example,dc=com",
			},
			wantErr: true,
		},
		{
			name: "missing BaseDN",
			config: &Config{
				URL:    "ldap://localhost:389",
				BaseDN: "",
				AdminDN: "cn=admin,dc=example,dc=com",
			},
			wantErr: true,
		},
		{
			name: "missing AdminDN",
			config: &Config{
				URL:    "ldap://localhost:389",
				BaseDN: "dc=example,dc=com",
				AdminDN: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("LDAP_URL")
	os.Unsetenv("LDAP_BASE_DN")
	os.Unsetenv("LDAP_ADMIN_DN")
	os.Unsetenv("LDAP_ADMIN_PASSWORD")

	config := DefaultConfig()

	if config.URL != "ldap://localhost:389" {
		t.Errorf("expected URL 'ldap://localhost:389', got '%s'", config.URL)
	}
	if config.BaseDN != "dc=example,dc=com" {
		t.Errorf("expected BaseDN 'dc=example,dc=com', got '%s'", config.BaseDN)
	}
	if config.AdminDN != "cn=admin,dc=example,dc=com" {
		t.Errorf("expected AdminDN 'cn=admin,dc=example,dc=com', got '%s'", config.AdminDN)
	}
	if config.PoolSize != 10 {
		t.Errorf("expected PoolSize 10, got %d", config.PoolSize)
	}
	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", config.MaxRetries)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10) // 10 requests per second

	// Should allow first request
	if !rl.Allow() {
		t.Error("expected first request to be allowed")
	}

	// Should allow multiple requests up to limit
	for i := 0; i < 9; i++ {
		if !rl.Allow() {
			t.Errorf("request %d should be allowed", i+2)
		}
	}

	// Reset and test again
	rl.Reset()
	if !rl.Allow() {
		t.Error("expected request after reset to be allowed")
	}
}

func TestNormalizeDN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CN=Admin,DC=Example,DC=COM", "cn=admin,dc=example,dc=com"},
		{" cn=admin,dc=example,dc=com ", "cn=admin,dc=example,dc=com"},
		{"cn=admin,dc=example,dc=com", "cn=admin,dc=example,dc=com"},
	}

	for _, tt := range tests {
		result := normalizeDN(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeDN(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGroupRoleMapping(t *testing.T) {
	// Test that all predefined mappings exist (keys are lowercase in the map)
	expectedMappings := map[string]string{
		"cn=it_ops,ou=groups,dc=example,dc=com":            "Operator",
		"cn=devteam_payments,ou=groups,dc=example,dc=com":  "Developer",
		"cn=security_auditors,ou=groups,dc=example,dc=com": "Auditor",
		"cn=sre_lead,ou=groups,dc=example,dc=com":           "SuperAdmin",
	}

	for dn, role := range expectedMappings {
		if GroupRoleMapping[dn] != role {
			t.Errorf("GroupRoleMapping[%q] = %q, want %q", dn, GroupRoleMapping[dn], role)
		}
	}
}

func TestAuthError(t *testing.T) {
	err := &AuthError{
		UserDN: "cn=test,dc=example,dc=com",
		Reason: "invalid credentials",
	}

	expected := "auth error for cn=test,dc=example,dc=com: invalid credentials"
	if err.Error() != expected {
		t.Errorf("AuthError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Field:   "URL",
		Message: "LDAP_URL is required",
	}

	expected := "config error: URL - LDAP_URL is required"
	if err.Error() != expected {
		t.Errorf("ConfigError.Error() = %q, want %q", err.Error(), expected)
	}
}

// Integration tests require Docker Compose - see docker-compose.yml
// Run with: docker compose up -d && go test -run TestIntegration ./...

func TestIntegrationAuthenticate(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	config := &Config{
		URL:           "ldap://localhost:389",
		BaseDN:        "dc=example,dc=com",
		AdminDN:       "cn=admin,dc=example,dc=com",
		AdminPassword: "admin",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Test valid authentication
	success, err := client.Authenticate("cn=john,ou=Users,dc=example,dc=com", "password123")
	if err != nil {
		t.Fatalf("authentication error: %v", err)
	}
	if !success {
		t.Error("expected authentication to succeed")
	}

	// Test invalid authentication
	success, err = client.Authenticate("cn=john,ou=Users,dc=example,dc=com", "wrongpassword")
	if err != nil {
		t.Fatalf("authentication error: %v", err)
	}
	if success {
		t.Error("expected authentication to fail")
	}
}

func TestIntegrationGetRoles(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	config := &Config{
		URL:           "ldap://localhost:389",
		BaseDN:        "dc=example,dc=com",
		AdminDN:       "cn=admin,dc=example,dc=com",
		AdminPassword: "admin",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Test role resolution for a user in IT_Ops group
	roles, err := client.GetRoles("cn=john,ou=Users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("failed to get roles: %v", err)
	}

	found := false
	for _, role := range roles {
		if role == "Operator" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Operator role to be found")
	}
}

// Performance tests

func TestRateLimiterPerformance(t *testing.T) {
	// Test rate limiter works correctly at configured rate
	rl := NewRateLimiter(1000) // 1k requests per second

	// First 1000 should be allowed
	count := 0
	for i := 0; i < 1000; i++ {
		if rl.Allow() {
			count++
		}
	}

	// Should allow roughly the configured number of requests
	if count < 900 || count > 1100 {
		t.Errorf("expected ~1000 allowed requests, got %d", count)
	}
}

func BenchmarkNormalizeDN(b *testing.B) {
	for i := 0; i < b.N; i++ {
		normalizeDN("CN=Admin,DC=Example,DC=COM")
	}
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(10000)
	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}
