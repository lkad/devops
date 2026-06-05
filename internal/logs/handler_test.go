package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newTestRouter wires a complete router with a Local backend and
// the full set of routes. Tests use this to exercise the handler
// layer end-to-end without booting a listener.
func newTestRouter(t *testing.T) (*gin.Engine, *Local) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	local := NewLocal(LocalConfig{Dir: dir})
	svc := NewService(local, ServiceConfig{RejectStructuredQueryOnLocal: true})
	h := NewHandler(svc, local)
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	return r, local
}

// newTestRouterWithBackend lets a test swap in any LogBackend
// implementation (e.g. a backend with a custom MaxTimeRange).
func newTestRouterWithBackend(t *testing.T, b LogBackend) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := NewService(b, ServiceConfig{RejectStructuredQueryOnLocal: true})
	h := NewHandler(svc, b)
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	return r
}

// TestHandler_Query_200 is the spec scenario "Query logs with
// search term" — a universal-subset GET /query yields a 200.
func TestHandler_Query_200(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/query?q=anything", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp Result
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Meta.Backend != "local" {
		t.Errorf("Meta.Backend = %q, want local", resp.Meta.Backend)
	}
}

// TestHandler_Query_TimeRange_Defaults: a request with no time
// range is accepted (24h default filled in by the service).
func TestHandler_Query_TimeRange_Defaults(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/query", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
}

// TestHandler_Query_TimeRange_Invalid: a bad from/to is a 400.
func TestHandler_Query_TimeRange_Invalid(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/query?from=not-a-time", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

// TestHandler_Query_Limit_Invalid: non-integer limit is a 400.
func TestHandler_Query_Limit_Invalid(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/query?limit=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

// TestHandler_Query_TimeRangeExceeded_422: a 60-day range against
// a backend with a 1-day cap yields 422.
func TestHandler_Query_TimeRangeExceeded_422(t *testing.T) {
	b := &shortMaxBackend{}
	r := newTestRouterWithBackend(t, b)
	from := time.Now().Add(-60 * 24 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/logs/query?from=%s&to=%s", from, to)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status=%d, want 422", w.Code)
	}
	var env contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("Error.Code = %v, want %v", env.Error.Code, contracts.CodeTimeRangeExceeded)
	}
}

// shortMaxBackend reports a 1-day MaxTimeRange so the 422 path is
// reachable without standing up a Loki dependency.
type shortMaxBackend struct{}

func (b *shortMaxBackend) Capabilities() Capabilities {
	return Capabilities{
		BackendName:    "test",
		MaxTimeRange:   24 * time.Hour,
		MaxQueryLength: 1024,
	}
}
func (b *shortMaxBackend) Query(_ context.Context, q Query) (Result, error) {
	return Result{Entries: []LogEntry{}, Total: 0, Meta: Meta{Backend: "test"}}, nil
}
func (b *shortMaxBackend) Streams(_ context.Context) ([]Stream, error) {
	return []Stream{{Name: "test"}}, nil
}

// TestHandler_Streams_200: GET /streams returns a list of streams.
func TestHandler_Streams_200(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/streams", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
}

// TestHandler_Echo_AppendsEntry: POST /_test/echo with a JSON
// LogEntry body appends to the Local backend and is visible in
// subsequent queries.
func TestHandler_Echo_AppendsEntry(t *testing.T) {
	r, _ := newTestRouter(t)
	now := time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC)
	body, _ := json.Marshal(LogEntry{
		ID:        "echo-1",
		Timestamp: now,
		Level:     "info",
		Source:    "echo",
		Message:   "hello",
		Host:      "test",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/_test/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	// Now query and assert the entry is visible.
	url := fmt.Sprintf("/api/v1/logs/query?q=hello&from=%s&to=%s",
		now.Add(-time.Hour).UTC().Format(time.RFC3339),
		now.Add(time.Hour).UTC().Format(time.RFC3339))
	req = httptest.NewRequest(http.MethodGet, url, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("query status=%d body=%s", w.Code, w.Body.String())
	}
	var resp Result
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1 (echo not visible): %+v", resp.Total, resp.Entries)
	}
}

// TestHandler_Echo_400_OnBadJSON: malformed body yields 400.
func TestHandler_Echo_400_OnBadJSON(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/_test/echo", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

// TestHandler_Query_StructuredQueryOnLocal_400: the spec scenario
// "Lucene query on Local backend" — 400 with the right hint.
func TestHandler_Query_StructuredQueryOnLocal_400(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/query?q="+urlEncode("level:error AND host:web-*"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
	var env contracts.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if !strings.Contains(env.Error.Message, "Local") {
		t.Errorf("Message = %q, want it to mention Local", env.Error.Message)
	}
}

// urlEncode is a tiny helper so we don't pull net/url into the
// test file just for one call.
func urlEncode(s string) string {
	r := strings.NewReplacer(":", "%3A", " ", "%20", "*", "%2A", "/", "%2F")
	return r.Replace(s)
}
