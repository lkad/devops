package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Collector struct {
	mu       sync.RWMutex
	counters map[string]float64
	gauges   map[string]float64
	histograms map[string]*Histogram
}

type Histogram struct {
	Count  int
	Sum    float64
	Min    float64
	Max    float64
	Values []float64
}

type MetricsData struct {
	Counters   map[string]float64 `json:"counters"`
	Gauges     map[string]float64 `json:"gauges"`
	Histograms map[string]*Histogram `json:"histograms"`
	Timestamp  time.Time `json:"timestamp"`
}

func NewCollector() *Collector {
	return &Collector{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]*Histogram),
	}
}

func (c *Collector) IncrementCounter(name string, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.makeKey(name, labels)
	c.counters[key]++
}

func (c *Collector) SetGauge(name string, value float64, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.makeKey(name, labels)
	c.gauges[key] = value
}

func (c *Collector) ObserveHistogram(name string, value float64, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.makeKey(name, labels)
	h, ok := c.histograms[key]
	if !ok {
		h = &Histogram{Min: value, Max: value, Values: make([]float64, 0)}
		c.histograms[key] = h
	}
	h.Count++
	h.Sum += value
	if value < h.Min {
		h.Min = value
	}
	if value > h.Max {
		h.Max = value
	}
	h.Values = append(h.Values, value)
}

func (c *Collector) RecordLog(level string) {
	c.IncrementCounter("logs_total", map[string]string{"level": level})
	if level == "error" {
		c.IncrementCounter("log_errors_total", nil)
	}
}

func (c *Collector) RecordDeviceEvent(eventType, deviceType string) {
	c.IncrementCounter("device_events_total", map[string]string{"type": eventType, "device_type": deviceType})
}

func (c *Collector) RecordPipelineEvent(pipeline, eventType string) {
	c.IncrementCounter("pipeline_events_total", map[string]string{"pipeline": pipeline, "type": eventType})
}

func (c *Collector) RecordAlert(alertName, severity string) {
	c.IncrementCounter("alerts_total", map[string]string{"name": alertName, "severity": severity})
}

func (c *Collector) RecordHTTPRequest(endpoint, method, status string, durationMs float64) {
	c.IncrementCounter("http_requests_total", map[string]string{"endpoint": endpoint, "method": method, "status": status})
	c.ObserveHistogram("http_request_duration_ms", durationMs, map[string]string{"endpoint": endpoint, "method": method, "status": status})
}

func (c *Collector) ServePrometheus(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lines []string
	lines = append(lines, "# HELP http_requests_total Total HTTP requests")
	lines = append(lines, "# TYPE http_requests_total counter")
	for key, val := range c.counters {
		labels := c.parseLabels(key)
		lines = append(lines, fmt.Sprintf("http_requests_total{%s} %v", labels, val))
	}

	lines = append(lines, "# HELP http_request_duration_ms HTTP request duration in ms")
	lines = append(lines, "# TYPE http_request_duration_ms histogram")
	for key, h := range c.histograms {
		labels := c.parseLabels(key)
		avg := 0.0
		if h.Count > 0 {
			avg = h.Sum / float64(h.Count)
		}
		lines = append(lines, fmt.Sprintf("http_request_duration_ms_count{%s} %d", labels, h.Count))
		lines = append(lines, fmt.Sprintf("http_request_duration_ms_sum{%s} %v", labels, h.Sum))
		lines = append(lines, fmt.Sprintf("http_request_duration_ms_avg{%s} %v", labels, avg))
		lines = append(lines, fmt.Sprintf("http_request_duration_ms_min{%s} %v", labels, h.Min))
		lines = append(lines, fmt.Sprintf("http_request_duration_ms_max{%s} %v", labels, h.Max))
	}

	lines = append(lines, "# HELP logs_total Total log entries")
	lines = append(lines, "# TYPE logs_total counter")
	for key, val := range c.counters {
		if strings.HasPrefix(key, "logs_") {
			labels := c.parseLabels(key)
			lines = append(lines, fmt.Sprintf("%s{%s} %v", key, labels, val))
		}
	}

	lines = append(lines, "# HELP device_events_total Device events")
	lines = append(lines, "# TYPE device_events_total counter")
	for key, val := range c.counters {
		if strings.HasPrefix(key, "device_") {
			labels := c.parseLabels(key)
			lines = append(lines, fmt.Sprintf("%s{%s} %v", key, labels, val))
		}
	}

	lines = append(lines, "# HELP pipeline_events_total Pipeline events")
	lines = append(lines, "# TYPE pipeline_events_total counter")
	for key, val := range c.counters {
		if strings.HasPrefix(key, "pipeline_") {
			labels := c.parseLabels(key)
			lines = append(lines, fmt.Sprintf("%s{%s} %v", key, labels, val))
		}
	}

	lines = append(lines, "# HELP alerts_total Total alerts triggered")
	lines = append(lines, "# TYPE alerts_total counter")
	for key, val := range c.counters {
		if strings.HasPrefix(key, "alerts_") {
			labels := c.parseLabels(key)
			lines = append(lines, fmt.Sprintf("%s{%s} %v", key, labels, val))
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprint(w, strings.Join(lines, "\n"))
}

func (c *Collector) ServeJSON(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := MetricsData{
		Counters:   c.counters,
		Gauges:     c.gauges,
		Histograms: c.histograms,
		Timestamp:  time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (c *Collector) makeKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return name + "{" + strings.Join(parts, ",") + "}"
}

func (c *Collector) parseLabels(key string) string {
	start := strings.Index(key, "{")
	if start == -1 {
		return ""
	}
	end := strings.Index(key, "}")
	if end == -1 {
		return ""
	}
	return key[start+1 : end]
}
