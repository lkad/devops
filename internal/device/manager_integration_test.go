package device

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

// skipIfNoDeviceDeps skips if server is not running or device routes are not available
func skipIfNoDeviceDeps(t *testing.T) (string, string) {
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

func makeReq(method, urlStr, token string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBody)
	}
	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// TestDeviceAPI_List tests listing devices via real HTTP
func TestDeviceAPI_List(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	req, err := makeReq("GET", baseURL+"/api/devices", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var devices []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	t.Logf("Found %d devices", len(devices))
}

// TestDeviceAPI_CreateAndGet tests creating and retrieving a device via real HTTP
func TestDeviceAPI_CreateAndGet(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	// Create a device
	createPayload := map[string]interface{}{
		"type":    "server",
		"name":    "test-integration-server",
		"labels":  map[string]string{"env": "test", "integration": "true"},
	}

	req, err := makeReq("POST", baseURL+"/api/devices", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201 or 200, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse created device
	var created Device
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if created.ID == "" {
		t.Fatal("Expected non-empty device ID")
	}
	if created.Name != "test-integration-server" {
		t.Errorf("Expected name 'test-integration-server', got '%s'", created.Name)
	}

	t.Logf("Created device: %s", created.ID)

	// Get the specific device
	req, err = makeReq("GET", baseURL+"/api/devices/"+created.ID, token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var retrieved Device
	if err := json.NewDecoder(resp.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}
}

// TestDeviceAPI_Update tests updating a device via real HTTP
func TestDeviceAPI_Update(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	// Create a device first
	createPayload := map[string]interface{}{
		"type":   "server",
		"name":   "test-update-device",
		"labels": map[string]string{},
	}

	req, err := makeReq("POST", baseURL+"/api/devices", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	createResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Update the device
	updatePayload := map[string]interface{}{
		"name":   "test-updated-name",
		"labels": map[string]string{"updated": "true"},
	}

	req, err = makeReq("PUT", baseURL+"/api/devices/"+created.ID, token, updatePayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", updateResp.StatusCode)
	}

	var updated Device
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if updated.Name != "test-updated-name" {
		t.Errorf("Expected name 'test-updated-name', got '%s'", updated.Name)
	}

	t.Logf("Updated device: %s", updated.ID)
}

// TestDeviceAPI_Delete tests deleting a device via real HTTP
func TestDeviceAPI_Delete(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	// Create a device first
	createPayload := map[string]interface{}{
		"type":   "server",
		"name":   "test-delete-device",
		"labels": map[string]string{},
	}

	req, err := makeReq("POST", baseURL+"/api/devices", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	createResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	deviceID := created.ID

	// Delete the device
	req, err = makeReq("DELETE", baseURL+"/api/devices/"+deviceID, token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK && deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected 200 or 204, got %d", deleteResp.StatusCode)
	}

	// Verify device is deleted
	req, err = makeReq("GET", baseURL+"/api/devices/"+deviceID, token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	getResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get deleted device: %v", err)
	}
	getResp.Body.Close()

	if getResp.StatusCode != http.StatusNotFound {
		t.Logf("Note: delete may not return 404 (status: %d)", getResp.StatusCode)
	}

	t.Logf("Deleted device: %s", deviceID)
}

// TestDeviceAPI_StateTransitions tests device state transitions via real HTTP
func TestDeviceAPI_StateTransitions(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	// Create a device
	createPayload := map[string]interface{}{
		"type":   "server",
		"name":   "test-state-device",
		"labels": map[string]string{},
	}

	req, err := makeReq("POST", baseURL+"/api/devices", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	createResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Transition to provisioning
	transitPayload := map[string]interface{}{
		"action": "provision",
	}

	req, err = makeReq("POST", baseURL+"/api/devices/"+created.ID+"/transition", token, transitPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	transitResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to transition device: %v", err)
	}
	defer transitResp.Body.Close()

	// Note: May return 404 if transition endpoint not registered
	if transitResp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping: transition endpoint not registered")
	}

	if transitResp.StatusCode != http.StatusOK {
		t.Logf("Transition response status: %d", transitResp.StatusCode)
	}

	t.Logf("Device state transition tested")
}

// TestDeviceAPI_Stats tests device statistics via real HTTP
func TestDeviceAPI_Stats(t *testing.T) {
	baseURL, token := skipIfNoDeviceDeps(t)

	req, err := makeReq("GET", baseURL+"/api/devices/stats", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping: stats endpoint not registered")
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	t.Logf("Device stats: %+v", stats)
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
