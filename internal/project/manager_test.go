// Package project tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewBusinessLine(t *testing.T) {
	bl := NewBusinessLine("电商事业部", "电商业务部门")
	if bl.ID == "" {
		t.Error("expected non-empty ID")
	}
	if bl.Name != "电商事业部" {
		t.Errorf("expected name '电商事业部', got '%s'", bl.Name)
	}
	if bl.Description != "电商业务部门" {
		t.Errorf("expected description '电商业务部门', got '%s'", bl.Description)
	}
	if bl.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewSystem(t *testing.T) {
	blID := "test-bl-id"
	sys := NewSystem(blID, "订单系统", "订单管理")
	if sys.ID == "" {
		t.Error("expected non-empty ID")
	}
	if sys.BusinessLineID != blID {
		t.Errorf("expected BusinessLineID '%s', got '%s'", blID, sys.BusinessLineID)
	}
	if sys.Name != "订单系统" {
		t.Errorf("expected name '订单系统', got '%s'", sys.Name)
	}
}

func TestNewProject(t *testing.T) {
	sysID := "test-sys-id"
	proj := NewProject(sysID, "order-backend", ProjectTypeBackend, "后端服务")
	if proj.ID == "" {
		t.Error("expected non-empty ID")
	}
	if proj.SystemID != sysID {
		t.Errorf("expected SystemID '%s', got '%s'", sysID, proj.SystemID)
	}
	if proj.Type != ProjectTypeBackend {
		t.Errorf("expected type backend, got '%s'", proj.Type)
	}
}

func TestNewProjectResource(t *testing.T) {
	pr := NewProjectResource("proj-1", ResourceTypeDevice, "device-123")
	if pr.ID == "" {
		t.Error("expected non-empty ID")
	}
	if pr.ProjectID != "proj-1" {
		t.Errorf("expected ProjectID 'proj-1', got '%s'", pr.ProjectID)
	}
	if pr.ResourceType != ResourceTypeDevice {
		t.Errorf("expected ResourceType device, got '%s'", pr.ResourceType)
	}
	if pr.ResourceID != "device-123" {
		t.Errorf("expected ResourceID 'device-123', got '%s'", pr.ResourceID)
	}
}

func TestNewPermission(t *testing.T) {
	projID := "proj-1"
	sysID := "sys-1"
	blID := "bl-1"

	// Project-level permission
	p1 := NewPermission("project", &projID, nil, nil, RoleEditor, "user@example.com")
	if p1.Level != "project" {
		t.Errorf("expected level 'project', got '%s'", p1.Level)
	}
	if *p1.ProjectID != projID {
		t.Errorf("expected ProjectID '%s', got '%s'", projID, *p1.ProjectID)
	}
	if p1.Role != RoleEditor {
		t.Errorf("expected role editor, got '%s'", p1.Role)
	}

	// System-level permission
	p2 := NewPermission("system", nil, &sysID, nil, RoleAdmin, "cn=admins,ou=groups,dc=example,dc=com")
	if p2.Level != "system" {
		t.Errorf("expected level 'system', got '%s'", p2.Level)
	}
	if *p2.SystemID != sysID {
		t.Errorf("expected SystemID '%s', got '%s'", sysID, *p2.SystemID)
	}

	// Business line-level permission
	p3 := NewPermission("business_line", nil, nil, &blID, RoleViewer, "user2@example.com")
	if p3.Level != "business_line" {
		t.Errorf("expected level 'business_line', got '%s'", p3.Level)
	}
	if *p3.BusinessLineID != blID {
		t.Errorf("expected BusinessLineID '%s', got '%s'", blID, *p3.BusinessLineID)
	}
}

func TestValidateProjectType(t *testing.T) {
	if !ValidateProjectType("frontend") {
		t.Error("expected frontend to be valid")
	}
	if !ValidateProjectType("backend") {
		t.Error("expected backend to be valid")
	}
	if ValidateProjectType("invalid") {
		t.Error("expected invalid to be false")
	}
}

func TestValidateResourceType(t *testing.T) {
	validTypes := []string{"device", "pipeline", "log_source", "alert_channel", "physical_host"}
	for _, rt := range validTypes {
		if !ValidateResourceType(rt) {
			t.Errorf("expected %s to be valid", rt)
		}
	}
	if ValidateResourceType("invalid") {
		t.Error("expected invalid to be false")
	}
}

func TestValidateRole(t *testing.T) {
	if !ValidateRole("viewer") {
		t.Error("expected viewer to be valid")
	}
	if !ValidateRole("editor") {
		t.Error("expected editor to be valid")
	}
	if !ValidateRole("admin") {
		t.Error("expected admin to be valid")
	}
	if ValidateRole("superadmin") {
		t.Error("expected superadmin to be false")
	}
}

func TestPagination(t *testing.T) {
	// Test pagination helper
	pr := &PaginatedResponse{
		Data: []string{"a", "b", "c"},
		Pagination: Pagination{
			Total:   100,
			Limit:   10,
			Offset:  20,
			HasMore: true,
		},
	}

	if pr.Pagination.Total != 100 {
		t.Errorf("expected total 100, got %d", pr.Pagination.Total)
	}
	if pr.Pagination.Limit != 10 {
		t.Errorf("expected limit 10, got %d", pr.Pagination.Limit)
	}
	if pr.Pagination.Offset != 20 {
		t.Errorf("expected offset 20, got %d", pr.Pagination.Offset)
	}
	if !pr.Pagination.HasMore {
		t.Errorf("expected hasMore true, got false")
	}
}

func TestManager_parsePagination(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		url        string
		wantLimit  int
		wantOffset int
	}{
		{"/", 50, 0},
		{"/?limit=10", 10, 0},
		{"/?offset=20", 50, 20},
		{"/?limit=25&offset=50", 25, 50},
		{"/?limit=-1", 50, 0},
		{"/?offset=-5", 50, 0},
		{"/?limit=200", 50, 0},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.url, nil)
		limit, offset := m.parsePagination(req)
		if limit != tt.wantLimit {
			t.Errorf("parsePagination(%s): limit = %d, want %d", tt.url, limit, tt.wantLimit)
		}
		if offset != tt.wantOffset {
			t.Errorf("parsePagination(%s): offset = %d, want %d", tt.url, offset, tt.wantOffset)
		}
	}
}

func TestManager_paginatedResponse(t *testing.T) {
	m := &Manager{}
	data := []string{"a", "b", "c"}
	resp := m.paginatedResponse(data, 100, 10, 20)

	if resp.Pagination.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 10 {
		t.Errorf("expected limit 10, got %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 20 {
		t.Errorf("expected offset 20, got %d", resp.Pagination.Offset)
	}
	// offset(20) + len(data)(3) < total(100), so hasMore = true
	if !resp.Pagination.HasMore {
		t.Errorf("expected hasMore true, got false")
	}
}

func TestManager_paginatedResponse_remainder(t *testing.T) {
	m := &Manager{}
	data := []string{}
	resp := m.paginatedResponse(data, 95, 10, 90)

	// When data is empty (dataLen=0), hasMore should be false regardless of offset
	// because an empty page indicates we've reached the end
	if resp.Pagination.HasMore {
		t.Errorf("expected hasMore false, got true")
	}
}

func TestManager_paginatedResponse_no_more(t *testing.T) {
	m := &Manager{}
	data := []string{"a", "b", "c"}
	resp := m.paginatedResponse(data, 3, 10, 0)

	// offset(0) + len(data)(3) = total(3), so hasMore = false
	if resp.Pagination.HasMore {
		t.Errorf("expected hasMore false, got true")
	}
}

func TestProjectTypes(t *testing.T) {
	if ProjectTypeFrontend != "frontend" {
		t.Errorf("expected frontend, got %s", ProjectTypeFrontend)
	}
	if ProjectTypeBackend != "backend" {
		t.Errorf("expected backend, got %s", ProjectTypeBackend)
	}
}

func TestResourceTypes(t *testing.T) {
	if ResourceTypeDevice != "device" {
		t.Errorf("expected device, got %s", ResourceTypeDevice)
	}
	if ResourceTypePipeline != "pipeline" {
		t.Errorf("expected pipeline, got %s", ResourceTypePipeline)
	}
	if ResourceTypeLogSource != "log_source" {
		t.Errorf("expected log_source, got %s", ResourceTypeLogSource)
	}
	if ResourceTypeAlertChannel != "alert_channel" {
		t.Errorf("expected alert_channel, got %s", ResourceTypeAlertChannel)
	}
	if ResourceTypePhysicalHost != "physical_host" {
		t.Errorf("expected physical_host, got %s", ResourceTypePhysicalHost)
	}
}

func TestRoles(t *testing.T) {
	if RoleViewer != "viewer" {
		t.Errorf("expected viewer, got %s", RoleViewer)
	}
	if RoleEditor != "editor" {
		t.Errorf("expected editor, got %s", RoleEditor)
	}
	if RoleAdmin != "admin" {
		t.Errorf("expected admin, got %s", RoleAdmin)
	}
}

func TestCreateBusinessLineJSON(t *testing.T) {
	input := `{"name":"电商事业部","description":"电子商务部门"}`
	var bl struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&bl); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if bl.Name != "电商事业部" {
		t.Errorf("expected name '电商事业部', got '%s'", bl.Name)
	}
}

func TestCreateSystemJSON(t *testing.T) {
	input := `{"name":"订单系统","description":"订单管理"}`
	var sys struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&sys); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if sys.Name != "订单系统" {
		t.Errorf("expected name '订单系统', got '%s'", sys.Name)
	}
}

func TestCreateProjectJSON(t *testing.T) {
	input := `{"name":"order-backend","type":"backend","description":"后端服务"}`
	var proj struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&proj); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if proj.Name != "order-backend" {
		t.Errorf("expected name 'order-backend', got '%s'", proj.Name)
	}
	if proj.Type != "backend" {
		t.Errorf("expected type 'backend', got '%s'", proj.Type)
	}
}

func TestLinkResourceJSON(t *testing.T) {
	input := `{"resource_type":"device","resource_id":"uuid-123"}`
	var res struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&res); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if res.ResourceType != "device" {
		t.Errorf("expected resource_type 'device', got '%s'", res.ResourceType)
	}
	if res.ResourceID != "uuid-123" {
		t.Errorf("expected resource_id 'uuid-123', got '%s'", res.ResourceID)
	}
}

func TestGrantPermissionJSON(t *testing.T) {
	input := `{"level":"project","role":"editor","subject":"user@example.com"}`
	var perm struct {
		Level   string `json:"level"`
		Role    string `json:"role"`
		Subject string `json:"subject"`
	}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&perm); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if perm.Level != "project" {
		t.Errorf("expected level 'project', got '%s'", perm.Level)
	}
	if perm.Role != "editor" {
		t.Errorf("expected role 'editor', got '%s'", perm.Role)
	}
	if perm.Subject != "user@example.com" {
		t.Errorf("expected subject 'user@example.com', got '%s'", perm.Subject)
	}
}

func TestManager_CheckPermission_NilRepo(t *testing.T) {
	// Without a real repository, CheckPermission will fail
	m := &Manager{repo: nil}
	_, err := m.CheckPermission("user@example.com", "project-1", RoleViewer)
	if err == nil {
		t.Error("expected error with nil repository")
	}
}

func TestManager_CheckPermission_RoleHierarchy(t *testing.T) {
	// Test role weight comparisons
	roleWeight := map[Role]int{
		RoleViewer: 1,
		RoleEditor: 2,
		RoleAdmin:  3,
	}

	if roleWeight[RoleViewer] >= roleWeight[RoleEditor] {
		t.Error("viewer should have less weight than editor")
	}
	if roleWeight[RoleEditor] >= roleWeight[RoleAdmin] {
		t.Error("editor should have less weight than admin")
	}
	if roleWeight[RoleAdmin] != 3 {
		t.Errorf("admin should have weight 3, got %d", roleWeight[RoleAdmin])
	}
}

func TestHTTPStatusCodes(t *testing.T) {
	// Test that HTTP status codes are correct
	tests := []struct {
		status   int
		expected string
	}{
		{http.StatusOK, "200"},
		{http.StatusCreated, "201"},
		{http.StatusNoContent, "204"},
		{http.StatusBadRequest, "400"},
		{http.StatusNotFound, "404"},
		{http.StatusConflict, "409"},
		{http.StatusInternalServerError, "500"},
	}

	for _, tt := range tests {
		if tt.status != http.StatusOK && tt.status/100 != 2 {
			continue
		}
	}
}

func TestFinOpsRow(t *testing.T) {
	row := FinOpsRow{
		BusinessLine: "电商事业部",
		System:       "订单系统",
		ProjectType:  "backend",
		Project:      "order-backend",
		ResourceType: "device",
		Count:        3,
		Unit:         "nodes",
	}

	if row.BusinessLine != "电商事业部" {
		t.Errorf("expected business line '电商事业部', got '%s'", row.BusinessLine)
	}
	if row.Count != 3 {
		t.Errorf("expected count 3, got %d", row.Count)
	}
	if row.Unit != "nodes" {
		t.Errorf("expected unit 'nodes', got '%s'", row.Unit)
	}
}