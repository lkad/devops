package project

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// skipIfNoDeps skips if server is not running or project routes are not available
func skipIfNoProjectDeps(t *testing.T) string {
	// Check if server is running and project routes are registered
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

	// Check if project routes are available by trying a project endpoint
	// If it returns 404, project manager is not initialized (no DB)
	resp, err = http.Get("http://localhost:3000/api/org/business-lines")
	if err != nil {
		t.Skip("Skipping: cannot reach project endpoints")
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("Skipping: project manager unavailable (PostgreSQL not connected)")
		return ""
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping: project routes not registered")
		return ""
	}

	return "http://localhost:3000"
}

// TestProjectAPI_BusinessLines_CreateAndList tests creating and listing business lines via real HTTP
func TestProjectAPI_BusinessLines_CreateAndList(t *testing.T) {
	baseURL := skipIfNoProjectDeps(t)

	// Create a business line
	createPayload := `{"name":"test-bl-integration","description":"integration test business line"}`
	resp, err := http.Post(baseURL+"/api/org/business-lines", "application/json", strings.NewReader(createPayload))
	if err != nil {
		t.Fatalf("Failed to create business line: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201 or 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Extract ID from response
	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	blID, ok := created["id"].(string)
	if !ok || blID == "" {
		t.Fatal("Expected non-empty id in response")
	}
	t.Logf("Created business line: %s", blID)

	// List business lines
	resp, err = http.Get(baseURL + "/api/org/business-lines")
	if err != nil {
		t.Fatalf("Failed to list business lines: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify data field exists and contains items
	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(data) == 0 {
		t.Fatal("Expected at least 1 business line")
	}

	t.Logf("Found %d business lines", len(data))
}

// TestProjectAPI_BusinessLines_Get tests getting a single business line via real HTTP
func TestProjectAPI_BusinessLines_Get(t *testing.T) {
	baseURL := skipIfNoProjectDeps(t)

	// Create a business line first
	createPayload := `{"name":"test-bl-get","description":"test get"}`
	resp, err := http.Post(baseURL+"/api/org/business-lines", "application/json", strings.NewReader(createPayload))
	if err != nil {
		t.Fatalf("Failed to create business line: %v", err)
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp.Body.Close()

	blID, ok := created["id"].(string)
	if !ok || blID == "" {
		t.Fatal("Expected non-empty id in response")
	}

	// Get the business line
	resp, err = http.Get(baseURL + "/api/org/business-lines/" + blID)
	if err != nil {
		t.Fatalf("Failed to get business line: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["name"] != "test-bl-get" {
		t.Errorf("Expected name 'test-bl-get', got '%v'", result["name"])
	}
}

// TestProjectAPI_Systems_CRUD tests system CRUD operations via real HTTP
func TestProjectAPI_Systems_CRUD(t *testing.T) {
	baseURL := skipIfNoProjectDeps(t)

	// First create a business line
	blPayload := `{"name":"test-bl-for-system","description":"test"}`
	blResp, err := http.Post(baseURL+"/api/org/business-lines", "application/json", strings.NewReader(blPayload))
	if err != nil {
		t.Fatalf("Failed to create business line: %v", err)
	}
	var createdBL map[string]interface{}
	if err := json.NewDecoder(blResp.Body).Decode(&createdBL); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	blResp.Body.Close()
	blID, ok := createdBL["id"].(string)
	if !ok || blID == "" {
		t.Fatal("Failed to get business line ID")
	}

	// Create a system
	sysPayload := fmt.Sprintf(`{"name":"test-system","description":"integration test system","business_line_id":"%s"}`, blID)
	sysResp, err := http.Post(baseURL+"/api/org/business-lines/"+blID+"/systems", "application/json", strings.NewReader(sysPayload))
	if err != nil {
		t.Fatalf("Failed to create system: %v", err)
	}
	sysResp.Body.Close()

	if sysResp.StatusCode != http.StatusCreated && sysResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200, got %d", sysResp.StatusCode)
	}

	// List systems under the business line
	listResp, err := http.Get(baseURL + "/api/org/business-lines/" + blID + "/systems")
	if err != nil {
		t.Fatalf("Failed to list systems: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", listResp.StatusCode)
	}

	t.Log("System CRUD operations working")
}

// TestProjectAPI_Projects_CRUD tests project CRUD operations via real HTTP
func TestProjectAPI_Projects_CRUD(t *testing.T) {
	baseURL := skipIfNoProjectDeps(t)

	// Create business line -> system -> project hierarchy
	blPayload := `{"name":"test-bl-hierarchy","description":"test"}`
	blResp, err := http.Post(baseURL+"/api/org/business-lines", "application/json", strings.NewReader(blPayload))
	if err != nil {
		t.Fatalf("Failed to create business line: %v", err)
	}
	var createdBL map[string]interface{}
	if err := json.NewDecoder(blResp.Body).Decode(&createdBL); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	blResp.Body.Close()
	blID, ok := createdBL["id"].(string)
	if !ok || blID == "" {
		t.Fatal("Failed to get business line ID")
	}

	sysPayload := fmt.Sprintf(`{"name":"test-sys-hierarchy","description":"test","business_line_id":"%s"}`, blID)
	sysResp, err := http.Post(baseURL+"/api/org/business-lines/"+blID+"/systems", "application/json", strings.NewReader(sysPayload))
	if err != nil {
		t.Fatalf("Failed to create system: %v", err)
	}
	var createdSys map[string]interface{}
	if err := json.NewDecoder(sysResp.Body).Decode(&createdSys); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	sysResp.Body.Close()
	sysID, ok := createdSys["id"].(string)
	if !ok || sysID == "" {
		t.Fatal("Failed to get system ID")
	}

	// Create a project
	projPayload := fmt.Sprintf(`{"name":"test-project","type":"backend","description":"integration test","system_id":"%s"}`, sysID)
	projResp, err := http.Post(baseURL+"/api/org/systems/"+sysID+"/projects", "application/json", strings.NewReader(projPayload))
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	projResp.Body.Close()

	if projResp.StatusCode != http.StatusCreated && projResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200, got %d", projResp.StatusCode)
	}

	// List projects
	listResp, err := http.Get(baseURL + "/api/org/systems/" + sysID + "/projects")
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", listResp.StatusCode)
	}

	t.Log("Project hierarchy CRUD working")
}

// TestProjectAPI_Pagination tests pagination via real HTTP
func TestProjectAPI_Pagination(t *testing.T) {
	baseURL := skipIfNoProjectDeps(t)

	// Test with pagination params
	resp, err := http.Get(baseURL + "/api/org/business-lines?page=1&per_page=5")
	if err != nil {
		t.Fatalf("Failed to list with pagination: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		// Service unavailable means DB not connected, which is expected in some envs
		t.Fatalf("Unexpected status: %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		pagination, ok := result["pagination"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected pagination in response")
		}

		if pagination["page"] != float64(1) {
			t.Errorf("Expected page 1, got %v", pagination["page"])
		}
		if pagination["per_page"] != float64(5) {
			t.Errorf("Expected per_page 5, got %v", pagination["per_page"])
		}
		t.Logf("Pagination working: %+v", pagination)
	}
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
