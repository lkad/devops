package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollector_IncrementCounter(t *testing.T) {
	c := NewCollector()

	c.IncrementCounter("requests_total", nil)
	c.IncrementCounter("requests_total", nil)
	c.IncrementCounter("requests_get", map[string]string{"method": "GET"})

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.counters["requests_total"] != 2 {
		t.Errorf("expected counter 'requests_total' to be 2, got %v", c.counters["requests_total"])
	}
	// With labels, the key format is name{key=value} without quotes
	if c.counters["requests_get{method=GET}"] != 1 {
		t.Errorf("expected counter 'requests_get{method=GET}' to be 1, got %v", c.counters["requests_get{method=GET}"])
	}
}

func TestCollector_SetGauge(t *testing.T) {
	c := NewCollector()

	c.SetGauge("memory_usage_bytes", 1024, nil)
	c.SetGauge("memory_usage_bytes", 2048, nil)

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.gauges["memory_usage_bytes"] != 2048 {
		t.Errorf("expected gauge 'memory_usage_bytes' to be 2048, got %v", c.gauges["memory_usage_bytes"])
	}
}

func TestCollector_ObserveHistogram(t *testing.T) {
	c := NewCollector()

	c.ObserveHistogram("request_duration_seconds", 0.1, nil)
	c.ObserveHistogram("request_duration_seconds", 0.2, nil)
	c.ObserveHistogram("request_duration_seconds", 0.3, nil)

	c.mu.RLock()
	defer c.mu.RUnlock()

	h := c.histograms["request_duration_seconds"]
	if h == nil {
		t.Fatal("expected histogram to exist")
	}
	if h.Count != 3 {
		t.Errorf("expected histogram count to be 3, got %d", h.Count)
	}
	if h.Sum < 0.59 || h.Sum > 0.61 {
		t.Errorf("expected histogram sum to be approximately 0.6, got %v", h.Sum)
	}
	if h.Min != 0.1 {
		t.Errorf("expected histogram min to be 0.1, got %v", h.Min)
	}
	if h.Max != 0.3 {
		t.Errorf("expected histogram max to be 0.3, got %v", h.Max)
	}
}

func TestCollector_RecordLog(t *testing.T) {
	c := NewCollector()

	c.RecordLog("info")
	c.RecordLog("info")
	c.RecordLog("error")

	c.mu.RLock()
	defer c.mu.RUnlock()

	// The key format without quotes is logs_total{level=info}
	infoKey := "logs_total{level=info}"
	if c.counters[infoKey] != 2 {
		t.Errorf("expected logs_total{level=info} to be 2, got %v", c.counters[infoKey])
	}
	if c.counters["log_errors_total"] != 1 {
		t.Errorf("expected log_errors_total to be 1, got %v", c.counters["log_errors_total"])
	}
}

func TestCollector_GetMetricsJSON(t *testing.T) {
	c := NewCollector()
	c.IncrementCounter("test_counter", nil)
	c.SetGauge("test_gauge", 42, nil)

	// Test via ServeJSON which internally accesses the metrics
	req := httptest.NewRequest("GET", "/api/metrics", nil)
	w := httptest.NewRecorder()

	c.ServeJSON(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify the response contains expected values
	body := w.Body.String()
	if !strings.Contains(body, "test_counter") {
		t.Error("expected response to contain test_counter")
	}
}

func TestCollector_ServeJSON(t *testing.T) {
	c := NewCollector()
	c.IncrementCounter("test_counter", nil)

	req := httptest.NewRequest("GET", "/api/metrics", nil)
	w := httptest.NewRecorder()

	c.ServeJSON(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestCollector_ServePrometheus(t *testing.T) {
	c := NewCollector()
	c.IncrementCounter("test_counter", map[string]string{"method": "GET"})
	c.SetGauge("test_gauge", 42, nil)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	c.ServePrometheus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}
