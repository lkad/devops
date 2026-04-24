package alerts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devops-toolkit/internal/metrics"
)

func TestManager_AddChannel(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)

	ch := &Channel{Name: "webhook-1", Type: "webhook", WebhookURL: "https://example.com/hook"}
	err := m.AddChannel(ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	channels := m.ListChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "webhook-1" {
		t.Errorf("expected channel name 'webhook-1', got '%s'", channels[0].Name)
	}
}

func TestManager_ListChannels(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)

	m.AddChannel(&Channel{Name: "ch1", Type: "log"})
	m.AddChannel(&Channel{Name: "ch2", Type: "slack"})

	channels := m.ListChannels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
}

func TestManager_DeleteChannel(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)

	m.AddChannel(&Channel{Name: "to-delete", Type: "log"})

	deleted := m.DeleteChannel("to-delete")
	if !deleted {
		t.Error("expected DeleteChannel to return true")
	}

	channels := m.ListChannels()
	if len(channels) != 0 {
		t.Errorf("expected 0 channels after delete, got %d", len(channels))
	}

	deleted = m.DeleteChannel("non-existent")
	if deleted {
		t.Error("expected DeleteChannel to return false for non-existent")
	}
}

func TestManager_TriggerAlert(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)
	m.AddChannel(&Channel{Name: "test-alert", Type: "log"})

	err := m.TriggerAlert("test-alert", "critical", "test message", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history := m.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 alert in history, got %d", len(history))
	}
	if history[0].Severity != "critical" {
		t.Errorf("expected severity 'critical', got '%s'", history[0].Severity)
	}
}

func TestManager_TriggerAlert_RateLimitExceeded(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)
	m.AddChannel(&Channel{Name: "rate-test", Type: "log"})

	// Fill the rate limiter (max is 10 per minute)
	for i := 0; i < 10; i++ {
		err := m.TriggerAlert("rate-test", "info", "test", nil)
		if err != nil {
			t.Fatalf("unexpected error on attempt %d: %v", i, err)
		}
	}

	// 11th should be rate limited
	err := m.TriggerAlert("rate-test", "info", "test", nil)
	if err == nil {
		t.Error("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected rate limit error message, got: %v", err)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(60000, 3)

	if !rl.Allow("test") {
		t.Error("expected Allow to return true for first request")
	}
	if !rl.Allow("test") {
		t.Error("expected Allow to return true for second request")
	}
	if !rl.Allow("test") {
		t.Error("expected Allow to return true for third request")
	}
	if rl.Allow("test") {
		t.Error("expected Allow to return false after exceeding limit")
	}
}

func TestManager_GetHistory(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)
	m.AddChannel(&Channel{Name: "history-test", Type: "log"})

	m.TriggerAlert("history-test", "info", "msg1", nil)
	m.TriggerAlert("history-test", "error", "msg2", nil)

	history := m.GetHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(history))
	}
}

func TestManager_ListChannelsHTTP(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)
	m.AddChannel(&Channel{Name: "http-test", Type: "webhook"})

	req := httptest.NewRequest("GET", "/api/alerts/channels", nil)
	w := httptest.NewRecorder()

	m.ListChannelsHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "http-test") {
		t.Error("expected response to contain channel name")
	}
}

func TestManager_AddChannelHTTP(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)

	body := `{"name":"new-channel","type":"slack"}`
	req := httptest.NewRequest("POST", "/api/alerts/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.AddChannelHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestManager_GetHistoryHTTP(t *testing.T) {
	metricsCollector := metrics.NewCollector()
	m := NewManager(metricsCollector)
	m.AddChannel(&Channel{Name: "hist-http", Type: "log"})
	m.TriggerAlert("hist-http", "warning", "test", nil)

	req := httptest.NewRequest("GET", "/api/alerts/history", nil)
	w := httptest.NewRecorder()

	m.GetHistoryHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}