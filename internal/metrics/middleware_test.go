package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// middlewareBundle groups the Gin engine and the
// repository the middleware writes to. Returning a single
// struct keeps the fixture call site short and avoids
// reaching into engine internals.
type middlewareBundle struct {
	Engine *gin.Engine
	Repo   *Repository
}

// middlewareFixture wires a Gin engine with the metrics
// middleware installed globally. The underlying Service is
// the real one over an in-memory repo.
func middlewareFixture(t *testing.T) *middlewareBundle {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openDB(t))
	svc := NewService(repo, NewFakeScraper())
	r := gin.New()
	r.Use(Middleware(svc))
	r.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"err": true})
	})
	return &middlewareBundle{Engine: r, Repo: repo}
}

// TestMiddleware_RecordsRequestLatency: a single request
// produces exactly one Metric row tagged with the
// matched path + method + status.
func TestMiddleware_RecordsRequestLatency(t *testing.T) {
	b := middlewareFixture(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	b.Engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("probe: %d", rr.Code)
	}

	rows, total, err := b.Repo.List(ListFilter{Name: "http_request_duration_ms"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	m := rows[0]
	if m.Value < 0 {
		t.Errorf("Value = %v, want >= 0", m.Value)
	}
	if m.Labels["endpoint"] != "/probe" {
		t.Errorf("Labels[endpoint] = %v, want /probe", m.Labels["endpoint"])
	}
	if m.Labels["method"] != "GET" {
		t.Errorf("Labels[method] = %v, want GET", m.Labels["method"])
	}
	if m.Labels["status"] != "200" {
		t.Errorf("Labels[status] = %v, want 200", m.Labels["status"])
	}
}

// TestMiddleware_RecordsErrorStatus: a 5xx still produces
// a metric — observability is more important than the
// request outcome.
func TestMiddleware_RecordsErrorStatus(t *testing.T) {
	b := middlewareFixture(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	b.Engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("error: %d", rr.Code)
	}

	rows, _, err := b.Repo.List(ListFilter{Name: "http_request_duration_ms"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Labels["status"] != "500" {
		t.Errorf("Labels[status] = %v, want 500", rows[0].Labels["status"])
	}
}

// TestMiddleware_DoesNotPanicOnUnmatchedRoute: a request
// to a route the engine has no handler for still records
// a metric labelled with the placeholder.
func TestMiddleware_DoesNotPanicOnUnmatchedRoute(t *testing.T) {
	b := middlewareFixture(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	b.Engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}

	rows, _, err := b.Repo.List(ListFilter{Name: "http_request_duration_ms"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Labels["endpoint"] != "unmatched" {
		t.Errorf("Labels[endpoint] = %v, want unmatched", rows[0].Labels["endpoint"])
	}
}

// TestMiddleware_TruncatesLongMethod is a safety net: a
// crafted method header is capped to keep label cardinality
// under control.
func TestMiddleware_TruncatesLongMethod(t *testing.T) {
	b := middlewareFixture(t)

	rr := httptest.NewRecorder()
	long := strings.Repeat("X", 256)
	req := httptest.NewRequest(long, "/probe", nil)
	b.Engine.ServeHTTP(rr, req)

	rows, _, err := b.Repo.List(ListFilter{Name: "http_request_duration_ms"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got, ok := rows[0].Labels["method"].(string); !ok || len(got) > 16 {
		t.Errorf("method label = %v, want <= 16 chars", rows[0].Labels["method"])
	}
}

// TestMiddleware_TimestampWithinTolerance: the recorded
// timestamp is roughly the request completion time, not
// something from the dark ages.
func TestMiddleware_TimestampWithinTolerance(t *testing.T) {
	b := middlewareFixture(t)

	before := time.Now().UTC().Add(-time.Second)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	b.Engine.ServeHTTP(rr, req)
	after := time.Now().UTC().Add(time.Second)

	rows, _, err := b.Repo.List(ListFilter{Name: "http_request_duration_ms"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	ts := rows[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("Timestamp = %v, want in [%v, %v]", ts, before, after)
	}
}
