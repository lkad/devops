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

// TestChain_OrderAndComposition verifies the chain mounts in the documented
// order: Recovery → RequestID → CORS → Logger → Handler. Each middleware
// must see the right things at the right time.
func TestChain_OrderAndComposition(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.WithWriter(&buf), logger.WithLevel("info"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, m := range Chain(log, []string{"*"}) {
		r.Use(m)
	}

	var sawRequestIDInHandler bool
	r.GET("/api/v1/projects", func(c *gin.Context) {
		// Handler runs after RequestID, so request_id must be in context.
		if v, ok := c.Get("request_id"); ok {
			if s, _ := v.(string); s != "" {
				sawRequestIDInHandler = true
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// WHEN a normal request hits the chain
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r.ServeHTTP(w, req)

	// THEN: handler ran (200 + JSON body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !sawRequestIDInHandler {
		t.Error("handler did not see request_id in context")
	}
	// RequestID: header present
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("missing X-Request-ID response header")
	}
	// CORS: header present
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("missing Access-Control-Allow-Origin response header")
	}
	// Logger: a line was written
	if !strings.Contains(buf.String(), `"path":"/api/v1/projects"`) {
		t.Errorf("expected logger to record path; got: %s", buf.String())
	}
	var entry map[string]any
	lines := splitNonEmpty(buf.String())
	if len(lines) == 0 {
		t.Fatal("no log lines emitted")
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("log line not JSON: %v", err)
	}
	if entry["method"] != "GET" {
		t.Errorf("logged method = %v, want GET", entry["method"])
	}
}

// TestChain_RecoveryInsideChain covers the spec scenario for Recovery
// executing as the outermost middleware — a panic in the handler must
// return a 500 envelope, not crash the server.
func TestChain_RecoveryInsideChain(t *testing.T) {
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, m := range Chain(log, []string{"*"}) {
		r.Use(m)
	}
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (raw=%q)", err, w.Body.String())
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Errorf("expected error envelope, got %v", body)
	}
}

// TestChain_EmptyOrigins verifies the CORS middleware tolerates the
// common "no origins configured" case without panicking.
func TestChain_EmptyOrigins(t *testing.T) {
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, m := range Chain(log, nil) {
		r.Use(m)
	}
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
