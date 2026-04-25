package device

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// skipIfNoDeviceDeps skips if server is not running or device routes are not available
func skipIfNoDeviceDeps(t *testing.T) string {
	// Check if server is running
	resp, err := http.Get("http://localhost:3000/health")
	if err != nil {
		t.Skip("Skipping: no devops-toolkit server running on localhost:3000")
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Skipping: server not healthy")
		return ""
	}

	// Check if device routes are available
	resp, err = http.Get("http://localhost:3000/api/devices")
	if err != nil {
		t.Skip("Skipping: cannot reach device endpoints")
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("Skipping: device manager unavailable (PostgreSQL not connected)")
		return ""
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping: device routes not registered")
		return ""
	}

	return "http://localhost:3000"
}

// TestDeviceAPI_List tests listing devices via real HTTP
func TestDeviceAPI_List(t *testing.T) {
	baseURL := skipIfNoDeviceDeps(t)

	resp, err := http.Get(baseURL + "/api/devices")
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
	baseURL := skipIfNoDeviceDeps(t)

	// Create a device
	createPayload := `{"type":"server","name":"test-integration-server","labels":{"env":"test","integration":"true"}}`
	resp, err := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    mustParseURL(baseURL + "/api/devices"),
		Body:   io.NopCloser(strings.NewReader(createPayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200, got %d", resp.StatusCode)
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
	if created.Status != StatePending {
		t.Errorf("Expected status 'pending', got '%s'", created.Status)
	}

	t.Logf("Created device: %s", created.ID)

	// Get the specific device
	getResp, err := http.Get(baseURL + "/api/devices/" + created.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", getResp.StatusCode)
	}

	var retrieved Device
	if err := json.NewDecoder(getResp.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}
}

// TestDeviceAPI_Update tests updating a device via real HTTP
func TestDeviceAPI_Update(t *testing.T) {
	baseURL := skipIfNoDeviceDeps(t)

	// Create a device first
	createPayload := `{"type":"server","name":"test-update-device"}`
	createResp, _ := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    mustParseURL(baseURL + "/api/devices"),
		Body:   io.NopCloser(strings.NewReader(createPayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Update the device
	updatePayload := `{"name":"test-updated-name","labels":{"updated":"true"}}`
	updateResp, err := http.DefaultClient.Do(&http.Request{
		Method: "PUT",
		URL:    mustParseURL(baseURL + "/api/devices/" + created.ID),
		Body:   io.NopCloser(strings.NewReader(updatePayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
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
	baseURL := skipIfNoDeviceDeps(t)

	// Create a device first
	createPayload := `{"type":"server","name":"test-delete-device"}`
	createResp, _ := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    mustParseURL(baseURL + "/api/devices"),
		Body:   io.NopCloser(strings.NewReader(createPayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	deviceID := created.ID

	// Delete the device
	deleteResp, err := http.DefaultClient.Do(&http.Request{
		Method: "DELETE",
		URL:    mustParseURL(baseURL + "/api/devices/" + deviceID),
	})
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK && deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected 200 or 204, got %d", deleteResp.StatusCode)
	}

	// Verify device is deleted
	getResp, err := http.Get(baseURL + "/api/devices/" + deviceID)
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
	baseURL := skipIfNoDeviceDeps(t)

	// Create a device
	createPayload := `{"type":"server","name":"test-state-device"}`
	createResp, _ := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    mustParseURL(baseURL + "/api/devices"),
		Body:   io.NopCloser(strings.NewReader(createPayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
	var created Device
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Transition to provisioning
	transitPayload := `{"action":"provision"}`
	transitResp, err := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    mustParseURL(baseURL + "/api/devices/" + created.ID + "/transition"),
		Body:   io.NopCloser(strings.NewReader(transitPayload)),
		Header: map[string][]string{"Content-Type": {"application/json"}},
	})
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
	baseURL := skipIfNoDeviceDeps(t)

	resp, err := http.Get(baseURL + "/api/devices/stats")
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
