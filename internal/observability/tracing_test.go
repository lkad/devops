package observability

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestTracing_ResponseCarriesXTraceID pins the spec's
// "X-Trace-Id response header" rule. Every request,
// whether it carries traceparent or not, must come back
// with a 32-hex trace_id in the response.
func TestTracing_ResponseCarriesXTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tr := NewTracing(TracingConfig{SamplerRatio: 1.0})
	r := gin.New()
	r.Use(tr.Middleware())
	r.GET("/v1/items", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	r.ServeHTTP(w, req)

	got := w.Header().Get("X-Trace-Id")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(got) {
		t.Errorf("X-Trace-Id = %q, want 32 hex chars", got)
	}
}

// TestTracing_ExtractsIncomingTraceparent pins the spec
// "incoming request carries traceparent" rule. When the
// caller sends a valid traceparent, the response's
// X-Trace-Id must match the trace_id portion of the
// header (not a freshly generated one).
func TestTracing_ExtractsIncomingTraceparent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tr := NewTracing(TracingConfig{SamplerRatio: 1.0})
	r := gin.New()
	r.Use(tr.Middleware())
	r.GET("/v1/items", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	const traceID = "0af7651916cd43dd8448eb211c80319c"
	const parentID = "b7ad6b7169203331"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	req.Header.Set("traceparent", "00-"+traceID+"-"+parentID+"-01")
	r.ServeHTTP(w, req)

	got := w.Header().Get("X-Trace-Id")
	if got != traceID {
		t.Errorf("X-Trace-Id = %q, want %q (extracted from traceparent)", got, traceID)
	}
}

// TestTracing_NoEndpointConfigured_DoesNotPanic pins the
// "no OTLP endpoint configured" rule: the middleware must
// not panic when no exporter is attached.
func TestTracing_NoEndpointConfigured_DoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("tracing panicked with no endpoint: %v", r)
		}
	}()
	tr := NewTracing(TracingConfig{Stdout: false, OTLPEndpoint: ""})
	r := gin.New()
	r.Use(tr.Middleware())
	r.GET("/v1/items", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/items", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// TestTracing_MatchedRouteIsAttribute pins the spec
// "matched route is recorded on the span" rule. The
// recorder (returned by Recorder()) gets the span so the
// test can inspect attributes without coupling to the
// exporter.
func TestTracing_MatchedRouteIsAttribute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tr := NewTracing(TracingConfig{SamplerRatio: 1.0})
	rec := tr.Recorder()
	r := gin.New()
	r.Use(tr.Middleware())
	r.GET("/v1/items/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/items/42", nil))

	spans := rec.Collect()
	if len(spans) == 0 {
		t.Fatalf("recorder captured no spans")
	}
	attrs := map[string]string{}
	for _, a := range spans[0].Attributes {
		attrs[a.Key] = a.Value
	}
	if attrs["http.route"] != "/v1/items/:id" {
		t.Errorf("http.route = %q, want /v1/items/:id", attrs["http.route"])
	}
	if attrs["http.request.method"] != "GET" {
		t.Errorf("http.request.method = %q, want GET", attrs["http.request.method"])
	}
	if attrs["http.response.status_code"] != "200" {
		t.Errorf("http.response.status_code = %q, want 200", attrs["http.response.status_code"])
	}
}
