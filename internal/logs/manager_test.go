// Package logs tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManager_AddLog(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	entry, err := m.AddLog("info", "test message", "test-source", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != "info" {
		t.Errorf("expected level 'info', got '%s'", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("expected message 'test message', got '%s'", entry.Message)
	}
	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_QueryLogs(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	m.AddLog("info", "first message", "app", nil)
	m.AddLog("error", "second message", "app", nil)
	m.AddLog("debug", "third message", "web", nil)

	// Query all
	entries, err := m.QueryLogs(QueryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Query by level
	entries, err = m.QueryLogs(QueryOptions{Level: "error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for level=error, got %d", len(entries))
	}

	// Query by source
	entries, err = m.QueryLogs(QueryOptions{Source: "web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for source=web, got %d", len(entries))
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	m.AddLog("info", "msg1", "app", nil)
	m.AddLog("error", "msg2", "app", nil)
	m.AddLog("info", "msg3", "web", nil)

	stats, err := m.GetStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}
	if stats.ByLevel["info"] != 2 {
		t.Errorf("expected 2 info logs, got %d", stats.ByLevel["info"])
	}
	if stats.ByLevel["error"] != 1 {
		t.Errorf("expected 1 error log, got %d", stats.ByLevel["error"])
	}
}

func TestManager_CreateAlertRule(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	rule := m.CreateAlertRule("high-error-rate", "error", "connection refused", 5)
	if rule.Name != "high-error-rate" {
		t.Errorf("expected name 'high-error-rate', got '%s'", rule.Name)
	}
	if rule.Level != "error" {
		t.Errorf("expected level 'error', got '%s'", rule.Level)
	}
	if rule.Threshold != 5 {
		t.Errorf("expected threshold 5, got %d", rule.Threshold)
	}
	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_ListAlertRules(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	m.CreateAlertRule("rule1", "error", "", 1)
	m.CreateAlertRule("rule2", "warning", "", 2)

	rules := m.ListAlertRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestManager_DeleteAlertRule(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	rule := m.CreateAlertRule("to-delete", "error", "", 1)

	deleted := m.DeleteAlertRule(rule.ID)
	if !deleted {
		t.Error("expected DeleteAlertRule to return true")
	}

	rules := m.ListAlertRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestManager_CheckAlerts(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	m.CreateAlertRule("err-alert", "error", "connection refused", 1)

	entry := &Entry{
		ID:      "test-1",
		Level:   "error",
		Message: "connection refused to database",
		Source:  "app",
	}

	// Should not panic - alert checking is implemented
	m.CheckAlerts(entry)
}

func TestContainsPattern(t *testing.T) {
	tests := []struct {
		message string
		pattern string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"", "world", false},
		{"hello world", "", false},
	}

	for _, tt := range tests {
		result := containsPattern(tt.message, tt.pattern)
		if result != tt.expected {
			t.Errorf("containsPattern(%q, %q) = %v, want %v",
				tt.message, tt.pattern, result, tt.expected)
		}
	}
}

func TestManager_QueryLogsHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)
	m.AddLog("info", "test message", "app", nil)

	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()

	m.QueryLogsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test message") {
		t.Error("expected response to contain log message")
	}
}

func TestManager_CreateLogHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	body := `{"level":"error","message":"http 500","source":"api"}`
	req := httptest.NewRequest("POST", "/api/logs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreateLogHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "http 500") {
		t.Error("expected response to contain message")
	}
}

func TestManager_GetStatsHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	req := httptest.NewRequest("GET", "/api/logs/stats", nil)
	w := httptest.NewRecorder()

	m.GetStatsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestManager_ListAlertRulesHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)
	m.CreateAlertRule("test-rule", "error", "", 1)

	req := httptest.NewRequest("GET", "/api/logs/alerts", nil)
	w := httptest.NewRecorder()

	m.ListAlertRulesHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestManager_CreateAlertRuleHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	body := `{"name":"new-rule","level":"error","pattern":"fail","threshold":3}`
	req := httptest.NewRequest("POST", "/api/logs/alerts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreateAlertRuleHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// Retention policy tests
func TestManager_GetRetentionPolicy(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	policy := m.GetRetentionPolicy()
	if policy.Days != 30 {
		t.Errorf("expected days 30, got %d", policy.Days)
	}
}

func TestManager_UpdateRetentionPolicy(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	policy := m.UpdateRetentionPolicy(60, 10000, true)
	if policy.Days != 60 {
		t.Errorf("expected days 60, got %d", policy.Days)
	}
	if policy.MaxLogs != 10000 {
		t.Errorf("expected max_logs 10000, got %d", policy.MaxLogs)
	}
	if !policy.ApplyEnabled {
		t.Error("expected apply_enabled to be true")
	}
}

func TestManager_ApplyRetentionPolicy(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	// Add some old logs
	m.AddLog("info", "old log", "test", nil)
	// Override timestamp to be old
	m.entries[0].Timestamp = time.Now().AddDate(0, 0, -60)

	deleted, err := m.ApplyRetentionPolicy()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

func TestManager_GetRetentionPolicyHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	req := httptest.NewRequest("GET", "/api/logs/retention", nil)
	w := httptest.NewRecorder()

	m.GetRetentionPolicyHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"days"`) {
		t.Error("expected response to contain days field")
	}
}

func TestManager_UpdateRetentionPolicyHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	body := `{"days":60,"max_logs":5000,"apply_enabled":true}`
	req := httptest.NewRequest("PUT", "/api/logs/retention", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.UpdateRetentionPolicyHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestManager_ApplyRetentionPolicyHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local", RetentionDays: 30}
	m := NewManager(cfg, nil)

	req := httptest.NewRequest("POST", "/api/logs/retention/apply", nil)
	w := httptest.NewRecorder()

	m.ApplyRetentionPolicyHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"deleted"`) {
		t.Error("expected response to contain deleted field")
	}
}

// Saved filter tests
func TestManager_CreateSavedFilter(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	filter := m.CreateSavedFilter("my-filter", "error", "api", "fail", "", nil)
	if filter.Name != "my-filter" {
		t.Errorf("expected name 'my-filter', got '%s'", filter.Name)
	}
	if filter.Level != "error" {
		t.Errorf("expected level 'error', got '%s'", filter.Level)
	}
	if filter.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_ListSavedFilters(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	m.CreateSavedFilter("filter1", "info", "", "", "", nil)
	m.CreateSavedFilter("filter2", "error", "", "", "", nil)

	filters := m.ListSavedFilters()
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}
}

func TestManager_DeleteSavedFilter(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	filter := m.CreateSavedFilter("to-delete", "info", "", "", "", nil)

	deleted := m.DeleteSavedFilter(filter.ID)
	if !deleted {
		t.Error("expected DeleteSavedFilter to return true")
	}

	filters := m.ListSavedFilters()
	if len(filters) != 0 {
		t.Errorf("expected 0 filters after delete, got %d", len(filters))
	}
}

func TestManager_ListSavedFiltersHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)
	m.CreateSavedFilter("test-filter", "info", "", "", "", nil)

	req := httptest.NewRequest("GET", "/api/logs/filters", nil)
	w := httptest.NewRecorder()

	m.ListSavedFiltersHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestManager_CreateSavedFilterHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	body := `{"name":"new-filter","level":"warning","source":"api","search":"timeout"}`
	req := httptest.NewRequest("POST", "/api/logs/filters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreateSavedFilterHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// Generate sample logs tests
func TestManager_GenerateSampleLogs(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	entries := m.GenerateSampleLogs(5)
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.ID == "" {
			t.Error("expected non-empty ID")
		}
		if e.Level == "" {
			t.Error("expected non-empty level")
		}
	}
}

func TestManager_GenerateSampleLogsHTTP(t *testing.T) {
	cfg := LogsConfig{Backend: "local"}
	m := NewManager(cfg, nil)

	body := `{"count":3}`
	req := httptest.NewRequest("POST", "/api/logs/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.GenerateSampleLogsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"generated"`) {
		t.Error("expected response to contain generated field")
	}
}