package metrics

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// skipIfNoServer skips if no devops-toolkit server is running on port 3000
func skipIfNoServer(t *testing.T) string {
	// Check if server is running
	resp, err := http.Get("http://localhost:3000/health")
	if err != nil {
		t.Skip("Skipping: no devops-toolkit server running on localhost:3000")
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Skipping: devops-toolkit server not healthy on localhost:3000")
		return ""
	}
	return "http://localhost:3000"
}

func TestHTTPMetrics_RealPrometheusScrape(t *testing.T) {
	baseURL := skipIfNoServer(t)

	// Make several different requests to generate metrics
	requests := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/api/k8s/clusters"},
		{"GET", "/api/k8s/clusters/dev-cluster-1/nodes"},
		{"GET", "/api/k8s/clusters/dev-cluster-1/namespaces"},
		{"GET", "/metrics"},
	}

	for _, req := range requests {
		resp, err := http.DefaultClient.Do(&http.Request{
			Method: req.method,
			URL:    mustParseURL(baseURL + req.path),
			Header: make(http.Header),
		})
		if err != nil {
			t.Fatalf("Request %s %s failed: %v", req.method, req.path, err)
		}
		resp.Body.Close()
		// We expect 2xx or actual response codes, not errors
		t.Logf("%s %s -> %d", req.method, req.path, resp.StatusCode)
	}

	// Now scrape the actual /metrics endpoint
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to scrape /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected /metrics to return 200, got %d", resp.StatusCode)
	}

	buf := make([]byte, 32768)
	n, _ := resp.Body.Read(buf)
	metricsOutput := string(buf[:n])

	// Verify HTTP request metrics are recorded
	// These should be present because we made actual requests above
	if !strings.Contains(metricsOutput, "http_requests_total{") {
		t.Error("Expected http_requests_total metric to be present in Prometheus output")
	}

	// Verify at least one endpoint was recorded
	expectedEndpoints := []string{"/health", "/api/k8s/clusters", "/metrics"}
	found := 0
	for _, ep := range expectedEndpoints {
		if strings.Contains(metricsOutput, fmt.Sprintf("endpoint=%s", ep)) {
			found++
			t.Logf("Found metric for endpoint: %s", ep)
		}
	}

	if found == 0 {
		t.Error("No expected endpoints found in metrics output. Full output:")
		t.Log(metricsOutput)
	}

	// Verify Prometheus format is correct
	if !strings.Contains(metricsOutput, "# HELP http_requests_total") {
		t.Error("Expected Prometheus HELP line for http_requests_total")
	}
	if !strings.Contains(metricsOutput, "# TYPE http_requests_total counter") {
		t.Error("Expected Prometheus TYPE line for http_requests_total")
	}

	// Verify histogram metrics exist for request duration
	if !strings.Contains(metricsOutput, "http_request_duration_ms_count{") {
		t.Error("Expected http_request_duration_ms histogram to be present")
	}

	t.Logf("Prometheus metrics output sample:\n%s", metricsOutput[:min(1000, len(metricsOutput))])
}

func TestHTTPMetrics_RequestCounting(t *testing.T) {
	baseURL := skipIfNoServer(t)

	// Record initial count for a specific endpoint
	getInitialCount := func(endpoint string) float64 {
		resp, err := http.Get(baseURL + "/metrics")
		if err != nil {
			return -1
		}
		defer resp.Body.Close()
		buf := make([]byte, 32768)
		n, _ := resp.Body.Read(buf)
		metricsOutput := string(buf[:n])

		// Parse http_requests_total{endpoint=/health,...} X
		lines := strings.Split(metricsOutput, "\n")
		for _, line := range lines {
			if strings.Contains(line, "http_requests_total{") && strings.Contains(line, fmt.Sprintf("endpoint=%s", endpoint)) {
				// Format: http_requests_total{endpoint=/health,method=GET,status=200} 5
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					var val float64
					fmt.Sscanf(parts[len(parts)-1], "%f", &val)
					return val
				}
			}
		}
		return 0
	}

	initialCount := getInitialCount("/health")

	// Make 3 requests to /health
	for i := 0; i < 3; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}

	// Verify count increased by 3
	finalCount := getInitialCount("/health")
	expectedCount := initialCount + 3

	if finalCount < expectedCount {
		t.Errorf("Expected /health request count to be at least %.0f, got %.0f", expectedCount, finalCount)
	}

	t.Logf("/health request count: %.0f -> %.0f", initialCount, finalCount)
}

func TestHTTPMetrics_DurationHistogram(t *testing.T) {
	baseURL := skipIfNoServer(t)

	// Make a request that should have measurable duration
	resp, err := http.Get(baseURL + "/api/k8s/clusters")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	// Check /metrics for duration histogram
	metricsResp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer metricsResp.Body.Close()

	buf := make([]byte, 65536)
	n, _ := metricsResp.Body.Read(buf)
	metricsOutput := string(buf[:n])

	// Verify histogram has count > 0
	if !strings.Contains(metricsOutput, "http_request_duration_ms_count{endpoint=/api/k8s/clusters") {
		t.Error("Expected histogram count for /api/k8s/clusters")
	}

	// Verify we have sum recorded (means timing was captured)
	if !strings.Contains(metricsOutput, "http_request_duration_ms_sum{endpoint=/api/k8s/clusters") {
		t.Error("Expected histogram sum for /api/k8s/clusters")
	}

	t.Log("Duration histogram is being recorded correctly")
}

func TestHTTPMetrics_StatusCodeTracking(t *testing.T) {
	baseURL := skipIfNoServer(t)

	// Hit an endpoint that returns specific status
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	// Check metrics contain status=200
	metricsResp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer metricsResp.Body.Close()

	buf := make([]byte, 65536)
	n, _ := metricsResp.Body.Read(buf)
	metricsOutput := string(buf[:n])

	if !strings.Contains(metricsOutput, "status=200") {
		t.Error("Expected status=200 to be recorded in metrics")
	}

	t.Log("Status code tracking is working")
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
