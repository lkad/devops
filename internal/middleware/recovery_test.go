package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TestRecovery_PanicReturns500Envelope verifies the spec scenario:
// "WHEN handler panics THEN middleware recovers and logs stack trace AND returns 500".
func TestRecovery_PanicReturns500Envelope(t *testing.T) {
	// GIVEN a logger that captures output and a route that panics
	var buf bytes.Buffer
	log := logger.New(logger.WithWriter(&buf), logger.WithLevel("error"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery(log))
	r.GET("/boom", func(c *gin.Context) {
		panic("kaboom")
	})

	// WHEN the request is sent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	// THEN the response is 500 with the standard error envelope
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (raw=%q)", err, w.Body.String())
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' object in body, got %v", body)
	}
	if errObj["code"] != "INTERNAL_ERROR" {
		t.Errorf("error.code = %v, want INTERNAL_ERROR", errObj["code"])
	}
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Error("expected non-empty error.message")
	}
	// AND the panic was logged (with the panic value in the message)
	out := buf.String()
	if !strings.Contains(out, "panic") {
		t.Errorf("expected log output to contain 'panic', got: %s", out)
	}
	if !strings.Contains(out, "kaboom") {
		t.Errorf("expected log output to mention panic value 'kaboom', got: %s", out)
	}
}

// TestRecovery_NoPanicPassesThrough verifies the recovery middleware is a
// no-op for normal handlers — they still run and the response is untouched.
func TestRecovery_NoPanicPassesThrough(t *testing.T) {
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery(log))
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"hello": "world"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "world") {
		t.Errorf("body = %q, want to contain 'world'", w.Body.String())
	}
}
