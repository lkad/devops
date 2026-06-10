package metrics

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// handlerFixture builds a Gin engine wired to a real Service
// over an in-memory sqlite DB. The Scraper is the Fake.
// We do not exercise RBAC in these tests — the auth/rbac
// spec is its own phase and lives in internal/auth/rbac/.
func handlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openDB(t))
	scraper := NewFakeScraper()
	svc := NewService(repo, scraper)
	h := NewHandler(svc)
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1, rbac.NoopPermFactory())
	return r
}

// doReq is a small helper that mirrors the physicalhost
// package's helper. It Marshals body to JSON, fires the
// request, and decodes the body into a generic map.
func doReq(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewBuffer(raw)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var decoded map[string]any
	if rr.Body.Len() > 0 {
		_ = json.Unmarshal(rr.Body.Bytes(), &decoded)
	}
	return rr, decoded
}

// TestHandler_PostMetric_Created covers the spec scenario
// "POST /api/v1/metrics — one Metric at a time".
func TestHandler_PostMetric_Created(t *testing.T) {
	r := handlerFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	rr, decoded := doReq(t, r, http.MethodPost, "/api/v1/metrics", map[string]any{
		"name":        "cpu",
		"target_type": "physical_host",
		"target_id":   "dev-1",
		"value":       0.42,
		"timestamp":   ts.Format(time.RFC3339Nano),
		"labels":      map[string]any{"host": "dev-1"},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if decoded["name"] != "cpu" {
		t.Errorf("name = %v, want cpu", decoded["name"])
	}
	if decoded["id"] == nil || decoded["id"] == "" {
		t.Error("id missing from response")
	}
}

// TestHandler_PostMetric_Validation covers the bad-payload
// branch: a 400 with a VALIDATION_ERROR code.
func TestHandler_PostMetric_Validation(t *testing.T) {
	r := handlerFixture(t)
	rr, decoded := doReq(t, r, http.MethodPost, "/api/v1/metrics", map[string]any{
		"target_type": "physical_host",
		"target_id":   "dev-1",
		"value":       0.42,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errBody, _ := decoded["error"].(map[string]any)
	if errBody == nil {
		t.Fatalf("expected error envelope, got %v", decoded)
	}
	if errBody["code"] != "VALIDATION_ERROR" {
		t.Errorf("code = %v, want VALIDATION_ERROR", errBody["code"])
	}
}

// TestHandler_PostMetric_BadJSON covers a malformed body —
// 400.
func TestHandler_PostMetric_BadJSON(t *testing.T) {
	r := handlerFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestHandler_List_FiltersAndEnvelope covers GET
// /api/v1/metrics with the name + target_type + from + to +
// limit filters. The response is the standard ListResponse
// envelope.
func TestHandler_List_FiltersAndEnvelope(t *testing.T) {
	r := handlerFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	// Seed two cpu rows and one mem row via the API.
	for _, m := range []map[string]any{
		{
			"name": "cpu", "target_type": "physical_host", "target_id": "dev-1",
			"value": 0.1, "timestamp": ts.Format(time.RFC3339Nano),
		},
		{
			"name": "cpu", "target_type": "physical_host", "target_id": "dev-2",
			"value": 0.2, "timestamp": ts.Add(time.Minute).Format(time.RFC3339Nano),
		},
		{
			"name": "mem", "target_type": "physical_host", "target_id": "dev-1",
			"value": 0.5, "timestamp": ts.Add(2 * time.Minute).Format(time.RFC3339Nano),
		},
	} {
		rr, _ := doReq(t, r, http.MethodPost, "/api/v1/metrics", m)
		if rr.Code != http.StatusCreated {
			t.Fatalf("seed: status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}
	}

	// List cpu only.
	rr, decoded := doReq(t, r, http.MethodGet, "/api/v1/metrics?name=cpu&from="+ts.Add(-time.Hour).Format(time.RFC3339Nano)+"&to="+ts.Add(time.Hour).Format(time.RFC3339Nano)+"&limit=10", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, _ := decoded["data"].([]any)
	if len(data) != 2 {
		t.Errorf("data = %d, want 2", len(data))
	}
	if decoded["pagination"] == nil {
		t.Error("pagination missing from envelope")
	}
}

// TestHandler_List_RejectsRangeOverCap covers the 90-day
// cap at the HTTP layer: 422 INVALID_STATE.
func TestHandler_List_RejectsRangeOverCap(t *testing.T) {
	r := handlerFixture(t)
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	to := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	rr, decoded := doReq(t, r, http.MethodGet, "/api/v1/metrics?from="+from+"&to="+to, nil)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rr.Code, rr.Body.String())
	}
	errBody, _ := decoded["error"].(map[string]any)
	if errBody == nil {
		t.Fatalf("expected error envelope, got %v", decoded)
	}
	if errBody["code"] != "INVALID_STATE" {
		t.Errorf("code = %v, want INVALID_STATE", errBody["code"])
	}
}

// TestHandler_List_BadTimestamp covers a 400 on an
// unparseable from / to query.
func TestHandler_List_BadTimestamp(t *testing.T) {
	r := handlerFixture(t)
	rr, _ := doReq(t, r, http.MethodGet, "/api/v1/metrics?from=not-a-date", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestHandler_ListSeries_HappyPath covers GET
// /api/v1/metrics/series.
func TestHandler_ListSeries_HappyPath(t *testing.T) {
	r := handlerFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for _, m := range []map[string]any{
		{
			"name": "cpu", "target_type": "physical_host", "target_id": "dev-1",
			"value": 0.1, "timestamp": ts.Format(time.RFC3339Nano),
		},
		{
			"name": "mem", "target_type": "physical_host", "target_id": "dev-1",
			"value": 0.5, "timestamp": ts.Format(time.RFC3339Nano),
		},
	} {
		rr, _ := doReq(t, r, http.MethodPost, "/api/v1/metrics", m)
		if rr.Code != http.StatusCreated {
			t.Fatalf("seed: %d", rr.Code)
		}
	}
	rr, decoded := doReq(t, r, http.MethodGet, "/api/v1/metrics/series", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, _ := decoded["data"].([]any)
	if len(data) != 2 {
		t.Errorf("data = %d, want 2", len(data))
	}
}

// TestHandler_GetSeriesByName_HappyPath covers GET
// /api/v1/metrics/series/:name. The handler resolves the
// remaining key (target_type, target_id) from query
// parameters.
func TestHandler_GetSeriesByName_HappyPath(t *testing.T) {
	r := handlerFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		rr, _ := doReq(t, r, http.MethodPost, "/api/v1/metrics", map[string]any{
			"name": "cpu", "target_type": "physical_host", "target_id": "dev-1",
			"value": float64(i), "timestamp": ts.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano),
		})
		if rr.Code != http.StatusCreated {
			t.Fatalf("seed: %d", rr.Code)
		}
	}
	rr, decoded := doReq(t, r, http.MethodGet, "/api/v1/metrics/series/cpu?target_type=physical_host&target_id=dev-1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if decoded["name"] != "cpu" {
		t.Errorf("name = %v, want cpu", decoded["name"])
	}
	points, _ := decoded["points"].([]any)
	if len(points) != 3 {
		t.Errorf("points = %d, want 3", len(points))
	}
}

// TestHandler_GetSeriesByName_RequiresTarget covers the
// 400 when the caller omits target_type or target_id.
func TestHandler_GetSeriesByName_RequiresTarget(t *testing.T) {
	r := handlerFixture(t)
	rr, _ := doReq(t, r, http.MethodGet, "/api/v1/metrics/series/cpu", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}
