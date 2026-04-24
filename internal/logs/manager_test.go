package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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