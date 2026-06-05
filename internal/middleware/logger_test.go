package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TestLogger_LogsRequestFields covers the spec scenario:
// "WHEN HTTP request completes THEN request is logged with method, path, status, duration".
func TestLogger_LogsRequestFields(t *testing.T) {
	// GIVEN a logger writing to a buffer
	var buf bytes.Buffer
	log := logger.New(logger.WithWriter(&buf), logger.WithLevel("info"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger(log))
	r.GET("/api/v1/projects/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	// WHEN a request completes
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42", nil)
	r.ServeHTTP(w, req)

	// THEN exactly one log line is emitted with method, path, status, duration
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	lines := splitNonEmpty(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %q", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line not JSON: %v (line=%q)", err, lines[0])
	}
	if entry["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry["method"])
	}
	if entry["path"] != "/api/v1/projects/42" {
		t.Errorf("path = %v, want /api/v1/projects/42", entry["path"])
	}
	// JSON numbers decode to float64
	if status, _ := entry["status"].(float64); int(status) != http.StatusOK {
		t.Errorf("status = %v, want 200", entry["status"])
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Errorf("expected duration_ms field, got keys %v", keysOf(entry))
	}
}

// TestLogger_LogsNon2xx verifies the status field reflects the real response.
func TestLogger_LogsNon2xx(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.WithWriter(&buf), logger.WithLevel("info"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger(log))
	r.GET("/missing", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "nope"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	r.ServeHTTP(w, req)

	lines := splitNonEmpty(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	var entry map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &entry)
	if status, _ := entry["status"].(float64); int(status) != http.StatusNotFound {
		t.Errorf("status = %v, want 404", entry["status"])
	}
}

// TestLogger_DurationIsNonNegative ensures we always emit a numeric duration.
func TestLogger_DurationIsNonNegative(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.WithWriter(&buf), logger.WithLevel("info"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger(log))
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(2 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	r.ServeHTTP(w, req)

	lines := splitNonEmpty(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	var entry map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &entry)
	d, ok := entry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms missing or wrong type: %v", entry["duration_ms"])
	}
	if d < 0 {
		t.Errorf("duration_ms = %v, want >= 0", d)
	}
}

// splitNonEmpty trims trailing whitespace and returns non-empty lines.
func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
