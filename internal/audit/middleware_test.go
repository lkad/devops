package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMiddleware_ExtractsAllHeaders covers the happy path: the
// middleware reads X-User-Id, X-User-Name, X-Forwarded-For, and
// User-Agent and stores them on the gin.Context so the helper
// functions can read them.
func TestMiddleware_ExtractsAllHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/x", func(c *gin.Context) {
		got := RequestActorFromContext(c)
		if got.UserID != "u-1" {
			t.Errorf("user id: got %q want u-1", got.UserID)
		}
		if got.Username != "alice" {
			t.Errorf("user name: got %q want alice", got.Username)
		}
		if got.IPAddress != "10.0.0.1" {
			t.Errorf("ip: got %q want 10.0.0.1", got.IPAddress)
		}
		if got.UserAgent != "test/1.0" {
			t.Errorf("ua: got %q want test/1.0", got.UserAgent)
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-User-Id", "u-1")
	req.Header.Set("X-User-Name", "alice")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("User-Agent", "test/1.0")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
}

// TestMiddleware_MissingHeaders verifies missing headers result
// in zero values, never panics, and never short-circuit the
// handler. httptest sets a default RemoteAddr (192.0.2.1:1234),
// so we explicitly clear it to assert the headers are empty.
func TestMiddleware_MissingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/x", func(c *gin.Context) {
		got := RequestActorFromContext(c)
		if got.UserID != "" || got.Username != "" || got.IPAddress != "" || got.UserAgent != "" {
			t.Errorf("expected zero values, got %+v", got)
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ""
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
}

// TestMiddleware_XForwardedForFirstHop covers the common
// production case where X-Forwarded-For is a comma-separated
// chain; the middleware should pick the first hop (the original
// client).
func TestMiddleware_XForwardedForFirstHop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/x", func(c *gin.Context) {
		got := RequestActorFromContext(c)
		if got.IPAddress != "192.168.1.10" {
			t.Errorf("first-hop ip: got %q want 192.168.1.10", got.IPAddress)
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.10, 10.0.0.1, 10.0.0.2")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
}

// TestMiddleware_RemoteAddrFallback covers the case where no
// X-Forwarded-For is provided: the middleware should fall back
// to req.RemoteAddr (host:port) and strip the port.
func TestMiddleware_RemoteAddrFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/x", func(c *gin.Context) {
		got := RequestActorFromContext(c)
		if got.IPAddress != "203.0.113.7" {
			t.Errorf("remote ip: got %q want 203.0.113.7", got.IPAddress)
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "203.0.113.7:51234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
}

// TestMiddleware_PassesThrough covers the trivial case: the
// middleware must always call c.Next() so downstream handlers
// run.
func TestMiddleware_PassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	called := false
	r.GET("/x", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if !called {
		t.Error("downstream handler not called")
	}
}

// TestMiddleware_ContextAware verifies the context.Context
// returned by WithRequestActor carries the same values. This is
// the seam used by non-Gin callers (e.g. background jobs that
// synthesise an audit event without a Gin request).
func TestMiddleware_ContextAware(t *testing.T) {
	actor := RequestActor{UserID: "u-9", Username: "eve", IPAddress: "10.0.0.9", UserAgent: "ua/9"}
	ctx := WithRequestActor(context.Background(), actor)
	got2 := actorFromContext(ctx)
	if got2.UserID != "u-9" || got2.Username != "eve" {
		t.Errorf("context roundtrip: %+v", got2)
	}
}

// TestRequestActor_RenderForAudit covers the helper that
// composes the audit fields from the RequestActor. The middleware
// is a *helper*; the spec says modules call RecordAction from
// their service layer, and RecordAction expects an
// ActorID/ActorName pair.
func TestRequestActor_RenderForAudit(t *testing.T) {
	a := RequestActor{UserID: "u-1", Username: "alice", IPAddress: "10.0.0.1", UserAgent: "ua/1"}
	id, name, ip, ua := a.AuditFields()
	if id != "u-1" || name != "alice" || ip != "10.0.0.1" || ua != "ua/1" {
		t.Errorf("audit fields: %+v", a)
	}
}

// TestMiddleware_HeaderConstants pins the header names so
// refactors that rename a constant do not silently break the
// frontend / other modules that send these headers.
func TestMiddleware_HeaderConstants(t *testing.T) {
	if HeaderUserID != "X-User-Id" {
		t.Errorf("HeaderUserID: got %q", HeaderUserID)
	}
	if HeaderUserName != "X-User-Name" {
		t.Errorf("HeaderUserName: got %q", HeaderUserName)
	}
	if HeaderForwardedFor != "X-Forwarded-For" {
		t.Errorf("HeaderForwardedFor: got %q", HeaderForwardedFor)
	}
	if !strings.EqualFold(HeaderUserAgent, "User-Agent") {
		t.Errorf("HeaderUserAgent: got %q", HeaderUserAgent)
	}
}
