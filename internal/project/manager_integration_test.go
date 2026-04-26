package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// uniqueName generates a unique name for test data
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), rand.Intn(9999))
}

// skipIfNoDeps skips if server is not running or project routes are not available
func skipIfNoProjectDeps(t *testing.T) (string, string) {
	baseURL, token := checkProjectServer(t)
	if baseURL == "" {
		return "", ""
	}
	cleanupTestData(t, baseURL, token)
	return baseURL, token
}

// cleanupTestData removes test data before running tests
func cleanupTestData(t *testing.T, baseURL, token string) {
	// Get all business lines
	req, _ := http.NewRequest("GET", baseURL+"/api/org/business-lines?per_page=100", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Warning: could not fetch business lines for cleanup: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Logf("Warning: could not decode response for cleanup: %v", err)
		return
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return
	}

	// Delete business lines starting with "test-"
	client := &http.Client{}
	for _, item := range data {
		bl, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := bl["name"].(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(name, "test-") {
			continue
		}
		id, ok := bl["id"].(string)
		if !ok {
			continue
		}
		req, err := http.NewRequest("DELETE", baseURL+"/api/org/business-lines/"+id, nil)
		if err != nil {
			continue
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		client.Do(req)
	}
}

// checkProjectServer verifies the server is running and project routes are available
func checkProjectServer(t *testing.T) (string, string) {
	baseURL := os.Getenv("DEVOPS_TEST_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}

	// Check if server is running
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping: no devops-toolkit server running at %s: %v", baseURL, err)
		return "", ""
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping: server at %s not healthy (status %d)", baseURL, resp.StatusCode)
		return "", ""
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
		return "", ""
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Skipf("Skipping: login failed with status %d", loginResp.StatusCode)
		return "", ""
	}

	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Skipf("Skipping: cannot decode login response: %v", err)
		return "", ""
	}

	token, ok := loginResult["token"].(string)
	if !ok || token == "" {
		t.Skip("Skipping: no token in login response")
		return "", ""
	}

	// Check if project routes are available
	req, _ := http.NewRequest("GET", baseURL+"/api/org/business-lines", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Skipping: cannot reach project endpoints")
		return "", ""
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("Skipping: project manager unavailable (PostgreSQL not connected)")
		return "", ""
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping: project routes not registered")
		return "", ""
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


// TestProjectAPI_BusinessLines_CreateAndList tests creating and listing business lines via real HTTP
func TestProjectAPI_BusinessLines_CreateAndList(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// Create a business line with unique name
	blName := uniqueName("test-bl-integration")
	createPayload := map[string]interface{}{
		"name":        blName,
		"description": "integration test business line",
	}

	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
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
	req, err = makeReq("GET", baseURL+"/api/org/business-lines", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
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
	baseURL, token := skipIfNoProjectDeps(t)

	// Create a business line first with unique name
	blName := uniqueName("test-bl-get")
	createPayload := map[string]interface{}{
		"name":        blName,
		"description": "test get",
	}

	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
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
	req, err = makeReq("GET", baseURL+"/api/org/business-lines/"+blID, token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
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

	if result["name"] != blName {
		t.Errorf("Expected name '%s', got '%v'", blName, result["name"])
	}
}

// TestProjectAPI_Systems_CRUD tests system CRUD operations via real HTTP
func TestProjectAPI_Systems_CRUD(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// First create a business line with unique name
	blName := uniqueName("test-bl-for-system")
	blPayload := map[string]interface{}{
		"name":        blName,
		"description": "test",
	}

	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, blPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	blResp, err := http.DefaultClient.Do(req)
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

	// Create a system with unique name
	sysName := uniqueName("test-system")
	sysPayload := map[string]interface{}{
		"name":              sysName,
		"description":       "integration test system",
		"business_line_id": blID,
	}

	req, err = makeReq("POST", baseURL+"/api/org/business-lines/"+blID+"/systems", token, sysPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	sysResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create system: %v", err)
	}
	sysResp.Body.Close()

	if sysResp.StatusCode != http.StatusCreated && sysResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200, got %d", sysResp.StatusCode)
	}

	// List systems under the business line
	req, err = makeReq("GET", baseURL+"/api/org/business-lines/"+blID+"/systems", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	listResp, err := http.DefaultClient.Do(req)
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
	baseURL, token := skipIfNoProjectDeps(t)

	// Create business line -> system -> project hierarchy with unique names
	blName := uniqueName("test-bl-hierarchy")
	blPayload := map[string]interface{}{
		"name":        blName,
		"description": "test",
	}

	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, blPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	blResp, err := http.DefaultClient.Do(req)
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

	sysName := uniqueName("test-sys-hierarchy")
	sysPayload := map[string]interface{}{
		"name":              sysName,
		"description":       "test",
		"business_line_id": blID,
	}

	req, err = makeReq("POST", baseURL+"/api/org/business-lines/"+blID+"/systems", token, sysPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	sysResp, err := http.DefaultClient.Do(req)
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

	// Create a project with unique name
	projName := uniqueName("test-project")
	projPayload := map[string]interface{}{
		"name":        projName,
		"type":        "backend",
		"description": "integration test",
		"system_id":   sysID,
	}

	req, err = makeReq("POST", baseURL+"/api/org/systems/"+sysID+"/projects", token, projPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	projResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	projResp.Body.Close()

	if projResp.StatusCode != http.StatusCreated && projResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200, got %d", projResp.StatusCode)
	}

	// List projects
	req, err = makeReq("GET", baseURL+"/api/org/systems/"+sysID+"/projects", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	listResp, err := http.DefaultClient.Do(req)
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
	baseURL, token := skipIfNoProjectDeps(t)

	// Test with pagination params
	req, err := makeReq("GET", baseURL+"/api/org/business-lines?page=1&per_page=5", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
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
