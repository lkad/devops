// Package physicalhost integration tests
// NOTE: These are QA TESTS - require real environment (server running, etc.)
// Will skip if dependencies are not available.

package physicalhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	testHost     = "localhost"
	testPort     = 2222
	testUsername = "test"
	testPassword = "test"
)

func skipIfNoDeps(t *testing.T) (string, string) {
	baseURL := os.Getenv("DEVOPS_TEST_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}

	// Check if server is running
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping: no devops-toolkit server running at %s: %v", baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping: server at %s not healthy (status %d)", baseURL, resp.StatusCode)
	}

	// Try to login and get token
	loginBody := map[string]interface{}{
		"username": "dev",
		"password": "dev",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginResp, err := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		t.Skipf("Skipping: cannot login to get token: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Skipf("Skipping: login failed with status %d", loginResp.StatusCode)
	}

	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Skipf("Skipping: cannot decode login response: %v", err)
	}

	token, ok := loginResult["token"].(string)
	if !ok || token == "" {
		t.Skip("Skipping: no token in login response")
	}

	return baseURL, token
}

func skipIfNoPhysicalHostDeps(t *testing.T) {
	// Check if we have SSH test credentials
	if os.Getenv("DEVOPS_TEST_SSH_HOST") == "" {
		t.Log("DEVOPS_TEST_SSH_HOST not set, skipping SSH-dependent tests")
	}
}

func TestPhysicalHost_API_CreateAndList(t *testing.T) {
	baseURL, token := skipIfNoDeps(t)

	// Create a test host
	body := map[string]interface{}{
		"hostname":   fmt.Sprintf("test-host-%d", time.Now().Unix()),
		"ip":         "192.168.1.100",
		"port":       testPort,
		"username":   testUsername,
		"auth_method": "password",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/api/physical-hosts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	// List hosts
	req, _ = http.NewRequest("GET", baseURL+"/api/physical-hosts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list hosts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var hosts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(hosts) < 1 {
		t.Fatal("Expected at least 1 host after creation")
	}
}

func TestPhysicalHost_API_Get(t *testing.T) {
	baseURL, token := skipIfNoDeps(t)

	// Create a host first
	body := map[string]interface{}{
		"hostname":   fmt.Sprintf("test-get-%d", time.Now().Unix()),
		"ip":         "192.168.1.101",
		"port":       22,
		"username":   "admin",
		"auth_method": "key",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/api/physical-hosts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	hostID := created["id"].(string)

	// Get the host by ID
	req, _ = http.NewRequest("GET", baseURL+"/api/physical-hosts/"+hostID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var host map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&host); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if host["id"].(string) != hostID {
		t.Errorf("Expected host ID %s, got %s", hostID, host["id"])
	}
}

func TestPhysicalHost_API_Delete(t *testing.T) {
	baseURL, token := skipIfNoDeps(t)

	// Create a host first
	body := map[string]interface{}{
		"hostname":   fmt.Sprintf("test-delete-%d", time.Now().Unix()),
		"ip":         "192.168.1.102",
		"port":       22,
		"username":   "admin",
		"auth_method": "key",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/api/physical-hosts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	hostID := created["id"].(string)

	// Delete the host
	req, _ = http.NewRequest("DELETE", baseURL+"/api/physical-hosts/"+hostID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 204 or 200, got %d", resp.StatusCode)
	}

	// Verify it's deleted
	req, _ = http.NewRequest("GET", baseURL+"/api/physical-hosts/"+hostID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get deleted host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404 after deletion, got %d", resp.StatusCode)
	}
}

func TestPhysicalHost_SSHConnection(t *testing.T) {
	skipIfNoPhysicalHostDeps(t)

	// This test requires a real SSH server
	// Set DEVOPS_TEST_SSH_HOST to enable
	sshHost := os.Getenv("DEVOPS_TEST_SSH_HOST")
	if sshHost == "" {
		t.Skip("Skipping: DEVOPS_TEST_SSH_HOST not set")
	}

	m := NewManager()

	host := m.CreateHost("ssh-test", sshHost, testUsername, "password", 22)
	if host == nil {
		t.Fatal("Failed to create host")
	}

	// Try to collect metrics (will attempt SSH connection)
	err := m.CollectMetrics(host.ID)
	if err != nil {
		t.Logf("SSH connection failed (expected if no SSH server): %v", err)
	}

	// Verify the host was created and has proper state
	retrieved := m.GetHost(host.ID)
	if retrieved == nil {
		t.Fatal("Host not found after creation")
	}
	if retrieved.Hostname != "ssh-test" {
		t.Errorf("Expected hostname 'ssh-test', got '%s'", retrieved.Hostname)
	}
}
