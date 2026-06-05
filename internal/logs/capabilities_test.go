package logs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestCapabilitiesHandler_200 covers the spec scenario
// "Capabilities response" — GET /api/v1/logs/capabilities returns
// 200 with the backend's Capabilities and the standard envelope.
func TestCapabilitiesHandler_200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&fakeBackend{caps: Capabilities{
		BackendName:    "local",
		MaxTimeRange:   7 * 24 * time.Hour,
		MaxQueryLength: 1024,
	}}, ServiceConfig{})
	h := NewCapabilitiesHandler(svc)

	r := gin.New()
	r.GET("/api/v1/logs/capabilities", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/capabilities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp capabilitiesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Backend != "local" {
		t.Errorf("Backend = %q, want local", resp.Backend)
	}
	if resp.Capabilities.MaxTimeRange != "168h0m0s" {
		t.Errorf("Capabilities.MaxTimeRange = %q, want 168h0m0s", resp.Capabilities.MaxTimeRange)
	}
}

// TestCapabilitiesHandler_NeverFails: the spec scenario
// "Capabilities query never fails" — even when the backend
// reports degraded, the endpoint returns 200.
func TestCapabilitiesHandler_NeverFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "unavailable"}}, ServiceConfig{})
	h := NewCapabilitiesHandler(svc)

	r := gin.New()
	r.GET("/api/v1/logs/capabilities", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/capabilities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 even when backend unavailable", w.Code)
	}
}
