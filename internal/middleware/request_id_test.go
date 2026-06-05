package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestID_GeneratesWhenAbsent covers the spec scenario for
// "generates a request ID when none is provided".
func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	var seen string
	r.GET("/x", func(c *gin.Context) {
		// THEN the id is available in the gin context as "request_id"
		v, ok := c.Get("request_id")
		if !ok {
			t.Fatal("expected request_id key in gin.Context")
		}
		seen, _ = v.(string)
		if seen == "" {
			t.Fatal("expected non-empty request_id in context")
		}
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	// AND the same id is echoed in the response header
	hdr := w.Header().Get("X-Request-ID")
	if hdr == "" {
		t.Fatal("expected X-Request-ID response header")
	}
	if hdr != seen {
		t.Errorf("header %q != context value %q", hdr, seen)
	}
}

// TestRequestID_PropagatesIncoming covers the spec scenario for
// "propagates X-Request-ID from the incoming request".
func TestRequestID_PropagatesIncoming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	var seen string
	r.GET("/x", func(c *gin.Context) {
		v, _ := c.Get("request_id")
		seen, _ = v.(string)
		c.Status(http.StatusOK)
	})

	const incoming = "abc-123-xyz"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", incoming)
	r.ServeHTTP(w, req)

	if seen != incoming {
		t.Errorf("context request_id = %q, want %q", seen, incoming)
	}
	if got := w.Header().Get("X-Request-ID"); got != incoming {
		t.Errorf("response X-Request-ID = %q, want %q", got, incoming)
	}
}

// TestRequestID_NonEmpty verifies generated IDs are non-trivial (UUID-shaped).
func TestRequestID_NonEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		v, _ := c.Get("request_id")
		s, _ := v.(string)
		// UUIDs are 36 chars (8-4-4-4-12). Anything long enough to be a
		// real id is fine; we just want a non-empty opaque token.
		if len(s) < 8 {
			t.Errorf("request id too short: %q", s)
		}
		if strings.TrimSpace(s) == "" {
			t.Error("request id is whitespace")
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
}
