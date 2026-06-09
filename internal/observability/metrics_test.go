package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetrics_HandlerExposesCounters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := New()
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/v1/items", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/v1/items/:id", func(c *gin.Context) { c.String(http.StatusNotFound, "no") })
	r.GET("/metrics", gin.WrapH(promHandlerFor(m)))

	// Drive a few requests.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("200 expected, got %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("404 expected, got %d", w.Code)
	}

	// Scrape the /metrics endpoint body and assert the
	// counter we expect is present.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	body := w.Body.String()

	wantSubstrings := []string{
		`devops_toolkit_http_requests_total{method="GET",route="/v1/items",status="200"} 3`,
		`devops_toolkit_http_requests_total{method="GET",route="/v1/items/:id",status="404"} 1`,
		`devops_toolkit_http_request_duration_seconds_bucket`,
		`go_goroutines`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(body, s) {
			t.Errorf("metrics body missing %q\n--- body ---\n%s", s, body)
		}
	}
}

func TestMetrics_UnknownRouteLabelled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := New()
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/v1/known", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", gin.WrapH(promHandlerFor(m)))

	// Hit an unmatched route — should land in
	// devops_toolkit_http_requests_total{route="unmatched",...}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("404 expected, got %d", w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `route="unmatched"`) {
		t.Errorf("expected unmatched route label, got:\n%s", w.Body.String())
	}
}

// promHandlerFor wraps the metrics handler with the
// prometheus.HandlerFor signature; kept here so the test
// doesn't depend on the unexported field.
func promHandlerFor(m *Metrics) http.Handler {
	return m.Handler()
}
