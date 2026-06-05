package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCORS_PreflightReturns204 covers the spec scenario:
// "WHEN OPTIONS request is received THEN CORS headers are set and 204 is returned".
func TestCORS_PreflightReturns204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"*"}))
	r.POST("/api/v1/projects", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	// WHEN a preflight OPTIONS is sent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/projects", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	r.ServeHTTP(w, req)

	// THEN status is 204
	if w.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", w.Code)
	}
	// AND the standard CORS headers are present
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("missing Access-Control-Allow-Origin")
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("missing Access-Control-Allow-Methods")
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("missing Access-Control-Allow-Headers")
	}
}

// TestCORS_AllowsConfiguredOrigin verifies the origin policy:
// a configured origin is echoed back (not "*") when an allowlist is provided.
func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://allowed.example.com"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://allowed.example.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example.com" {
		t.Errorf("ACAO = %q, want %q", got, "https://allowed.example.com")
	}
}

// TestCORS_RejectsUnknownOrigin verifies an origin not in the allowlist
// does NOT get the allow header echoed back.
func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://allowed.example.com"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "https://evil.example.com" {
		t.Errorf("ACAO = %q, want empty (origin not allowed)", got)
	}
}

// TestCORS_AddsHeaderToActualRequest verifies a real GET still gets
// the CORS header (not just preflight).
func TestCORS_AddsHeaderToActualRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"*"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://example.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("missing Access-Control-Allow-Origin on actual request")
	}
}
