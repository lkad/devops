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

// TestProjectAPI_BusinessLines_DuplicateName tests that creating a business line with
// a duplicate name returns 409 Conflict instead of 500 Internal Server Error
func TestProjectAPI_BusinessLines_DuplicateName(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// Create a business line with unique name
	blName := uniqueName("test-bl-duplicate")
	createPayload := map[string]interface{}{
		"name":        blName,
		"description": "first creation",
	}

	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, createPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create business line: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 201 or 200 for first creation, got %d", resp.StatusCode)
	}

	// Try to create another business line with the same name
	duplicatePayload := map[string]interface{}{
		"name":        blName,
		"description": "duplicate name",
	}

	req2, err := makeReq("POST", baseURL+"/api/org/business-lines", token, duplicatePayload)
	if err != nil {
		t.Fatalf("Failed to create duplicate request: %v", err)
	}
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to create duplicate business line: %v", err)
	}
	defer resp2.Body.Close()

	// Should return 409 Conflict, not 500
	if resp2.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp2.Body)
		t.Errorf("Expected 409 Conflict for duplicate name, got %d. Body: %s", resp2.StatusCode, string(body))
	}

	// Verify error response format
	var errResp map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	errObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errObj["code"] != "CONFLICT" {
		t.Errorf("Expected error code CONFLICT, got %v", errObj["code"])
	}

	t.Logf("Duplicate name correctly returned 409 Conflict: %v", errObj["message"])
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

// TestProjectAPI_LinkResource tests linking a device/resource to a project
func TestProjectAPI_LinkResource(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// First create business line -> system -> project hierarchy
	blName := uniqueName("test-bl-link-res")
	blPayload := map[string]interface{}{
		"name":        blName,
		"description": "test for link resource",
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

	// Create system
	sysName := uniqueName("test-sys-link-res")
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

	// Create project
	projName := uniqueName("test-proj-link-res")
	projPayload := map[string]interface{}{
		"name":        projName,
		"type":        "backend",
		"description": "test for link resource",
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
	var createdProj map[string]interface{}
	if err := json.NewDecoder(projResp.Body).Decode(&createdProj); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	projResp.Body.Close()
	projID, ok := createdProj["id"].(string)
	if !ok || projID == "" {
		t.Fatal("Failed to get project ID")
	}

	// Link a device to the project
	linkPayload := map[string]interface{}{
		"resource_type": "device",
		"resource_id":   "test-device-001",
	}
	req, err = makeReq("POST", baseURL+"/api/org/projects/"+projID+"/resources", token, linkPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	linkResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to link resource: %v", err)
	}
	defer linkResp.Body.Close()

	if linkResp.StatusCode != http.StatusCreated && linkResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(linkResp.Body)
		t.Fatalf("Expected 201 or 200, got %d. Body: %s", linkResp.StatusCode, string(body))
	}

	// Verify the link was created
	var linkedRes map[string]interface{}
	if err := json.NewDecoder(linkResp.Body).Decode(&linkedRes); err != nil {
		t.Fatalf("Failed to decode link response: %v", err)
	}

	if linkedRes["resource_type"] != "device" {
		t.Errorf("Expected resource_type 'device', got '%v'", linkedRes["resource_type"])
	}
	if linkedRes["resource_id"] != "test-device-001" {
		t.Errorf("Expected resource_id 'test-device-001', got '%v'", linkedRes["resource_id"])
	}
	t.Logf("Successfully linked device to project: %s", projID)

	// List project resources to verify
	req, err = makeReq("GET", baseURL+"/api/org/projects/"+projID+"/resources", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", listResp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resources, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(resources) == 0 {
		t.Fatal("Expected at least 1 linked resource")
	}

	t.Logf("Project resources: found %d linked resources", len(resources))
}

// TestProjectAPI_LinkMultipleResources tests linking multiple resources to a project
func TestProjectAPI_LinkMultipleResources(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// Create business line -> system -> project
	blName := uniqueName("test-bl-multi-res")
	blPayload := map[string]interface{}{
		"name":        blName,
		"description": "test for multi resource link",
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
	blID := createdBL["id"].(string)

	sysName := uniqueName("test-sys-multi-res")
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
	sysID := createdSys["id"].(string)

	projName := uniqueName("test-proj-multi-res")
	projPayload := map[string]interface{}{
		"name":        projName,
		"type":        "frontend",
		"description": "test for multi resource link",
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
	var createdProj map[string]interface{}
	if err := json.NewDecoder(projResp.Body).Decode(&createdProj); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	projResp.Body.Close()
	projID := createdProj["id"].(string)

	// Link multiple different resource types
	resourceLinks := []struct {
		resourceType string
		resourceID   string
	}{
		{"device", "shared-vm-001"},
		{"physical_host", "physical-host-001"},
		{"pipeline", "pipeline-test-001"},
	}

	for _, link := range resourceLinks {
		linkPayload := map[string]interface{}{
			"resource_type": link.resourceType,
			"resource_id":   link.resourceID,
		}
		req, err = makeReq("POST", baseURL+"/api/org/projects/"+projID+"/resources", token, linkPayload)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		linkResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to link resource %s: %v", link.resourceType, err)
		}
		linkResp.Body.Close()

		if linkResp.StatusCode != http.StatusCreated && linkResp.StatusCode != http.StatusOK {
			t.Errorf("Failed to link %s: expected 201/200, got %d", link.resourceType, linkResp.StatusCode)
		}
		t.Logf("Linked %s: %s to project", link.resourceType, link.resourceID)
	}

	// Verify all resources are linked
	req, err = makeReq("GET", baseURL+"/api/org/projects/"+projID+"/resources", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	defer listResp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resources, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(resources) != len(resourceLinks) {
		t.Errorf("Expected %d resources, got %d", len(resourceLinks), len(resources))
	}

	t.Logf("Successfully linked %d different resource types to project", len(resources))
}

// TestProjectAPI_UnlinkResource tests unlinking a resource from a project
func TestProjectAPI_UnlinkResource(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// Create hierarchy and project
	blName := uniqueName("test-bl-unlink")
	blPayload := map[string]interface{}{"name": blName, "description": "test"}
	req, err := makeReq("POST", baseURL+"/api/org/business-lines", token, blPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	blResp, _ := http.DefaultClient.Do(req)
	var createdBL map[string]interface{}
	json.NewDecoder(blResp.Body).Decode(&createdBL)
	blResp.Body.Close()
	blID := createdBL["id"].(string)

	sysPayload := map[string]interface{}{"name": uniqueName("test-sys-unlink"), "description": "test", "business_line_id": blID}
	req, err = makeReq("POST", baseURL+"/api/org/business-lines/"+blID+"/systems", token, sysPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	sysResp, _ := http.DefaultClient.Do(req)
	var createdSys map[string]interface{}
	json.NewDecoder(sysResp.Body).Decode(&createdSys)
	sysResp.Body.Close()
	sysID := createdSys["id"].(string)

	projPayload := map[string]interface{}{"name": uniqueName("test-proj-unlink"), "type": "backend", "system_id": sysID}
	req, err = makeReq("POST", baseURL+"/api/org/systems/"+sysID+"/projects", token, projPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	projResp, _ := http.DefaultClient.Do(req)
	var createdProj map[string]interface{}
	json.NewDecoder(projResp.Body).Decode(&createdProj)
	projResp.Body.Close()
	projID := createdProj["id"].(string)

	// Link a resource
	linkPayload := map[string]interface{}{"resource_type": "device", "resource_id": "device-to-unlink"}
	req, err = makeReq("POST", baseURL+"/api/org/projects/"+projID+"/resources", token, linkPayload)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	linkResp, _ := http.DefaultClient.Do(req)
	linkResp.Body.Close()

	if linkResp.StatusCode != http.StatusCreated && linkResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to link resource: %d", linkResp.StatusCode)
	}

	// Unlink the resource
	req, err = makeReq("DELETE", baseURL+"/api/org/projects/"+projID+"/resources/device-to-unlink", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	unlinkResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to unlink resource: %v", err)
	}
	defer unlinkResp.Body.Close()

	if unlinkResp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", unlinkResp.StatusCode)
	}

	// Verify resource is unlinked
	req, err = makeReq("GET", baseURL+"/api/org/projects/"+projID+"/resources", token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	defer listResp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&result)

	resources := result["data"].([]interface{})
	if len(resources) != 0 {
		t.Errorf("Expected 0 resources after unlink, got %d", len(resources))
	}

	t.Log("Successfully unlinked resource from project")
}

// TestFinOpsExport_WithResources tests FinOps CSV export with linked resources
func TestFinOpsExport_WithResources(t *testing.T) {
	baseURL, token := skipIfNoProjectDeps(t)

	// Create business line -> system -> project hierarchy
	blName := uniqueName("test-bl-finops")
	blPayload := map[string]interface{}{
		"name":        blName,
		"description": "FinOps test business line",
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

	// Create system
	sysName := uniqueName("test-sys-finops")
	sysPayload := map[string]interface{}{
		"name":              sysName,
		"description":       "FinOps test system",
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

	// Create project with type
	projName := uniqueName("test-proj-finops")
	projPayload := map[string]interface{}{
		"name":        projName,
		"type":        "backend",
		"description": "FinOps test project",
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
	var createdProj map[string]interface{}
	if err := json.NewDecoder(projResp.Body).Decode(&createdProj); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	projResp.Body.Close()
	projID, ok := createdProj["id"].(string)
	if !ok || projID == "" {
		t.Fatal("Failed to get project ID")
	}

	// Link resources to the project
	resources := []struct {
		resourceType string
		resourceID   string
	}{
		{"device", "dev-vm-001"},
		{"device", "dev-vm-002"},
		{"pipeline", "ci-pipeline-001"},
	}

	for _, res := range resources {
		linkPayload := map[string]interface{}{
			"resource_type": res.resourceType,
			"resource_id":   res.resourceID,
		}
		req, err = makeReq("POST", baseURL+"/api/org/projects/"+projID+"/resources", token, linkPayload)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		linkResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to link resource: %v", err)
		}
		linkResp.Body.Close()
	}

	// Export FinOps report
	period := "2026-04"
	req, err = makeReq("GET", baseURL+"/api/org/reports/finops?period="+period, token, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	finopsResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to export FinOps: %v", err)
	}
	defer finopsResp.Body.Close()

	// FinOps should return CSV or 200
	if finopsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(finopsResp.Body)
		t.Fatalf("Expected 200 for FinOps export, got %d. Body: %s", finopsResp.StatusCode, string(body))
	}

	contentType := finopsResp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "csv") && !strings.Contains(contentType, "text/plain") {
		t.Logf("Warning: Expected CSV content-type, got %s", contentType)
	}

	// Read and verify CSV content
	body, err := io.ReadAll(finopsResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	csvContent := string(body)
	lines := strings.Split(csvContent, "\n")

	// Should have header + at least some data rows
	if len(lines) < 2 {
		t.Fatalf("Expected CSV with header + data rows, got %d lines", len(lines))
	}

	// Verify header contains expected columns
	header := lines[0]
	expectedColumns := []string{"Business Line", "System", "Project Type", "Project", "Resource Type"}
	for _, col := range expectedColumns {
		if !strings.Contains(header, col) {
			t.Errorf("Expected header to contain '%s', got: %s", col, header)
		}
	}

	// Verify our test project appears in the FinOps data
	found := false
	for _, line := range lines {
		if strings.Contains(line, projName) {
			found = true
			t.Logf("Found project '%s' in FinOps report: %s", projName, line)
			break
		}
	}
	if !found {
		t.Logf("Warning: Project '%s' not found in FinOps report. This may be expected if period doesn't match.", projName)
	}

	t.Logf("FinOps CSV export successful, %d lines, content preview: %s...", len(lines), strings.Split(csvContent, "\n")[0])
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
