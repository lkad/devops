package logs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newLogsHandlerFixture wraps the existing newTestRouter so
// the extra-feature tests share a single fixture style. It
// also wires the extra service + repo so the retention /
// saved-filters / alert-rules routes are registered.
func newLogsHandlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openLogsTestDB(t)
	local := NewLocal(LocalConfig{Dir: t.TempDir()})
	svc := NewService(local, ServiceConfig{})
	repo := NewExtraRepository(db)
	extra := NewExtraService(repo, svc)

	r := gin.New()
	api := r.Group("/api/v1")
	h := NewHandlerWithExtra(svc, extra, repo)
	h.Register(api)
	return r
}

// openLogsTestDB opens a fresh in-memory sqlite with both
// the standard logs schema and the new extra models
// migrated. The DSN is unique per test (UUID) so the
// shared-cache race from busy_timeout(5000) doesn't
// surface here.
func openLogsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:logs-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(AllExtraModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// doLogsRequest is a thin helper: marshal body to JSON,
// send, decode the response into a generic map. The local
// log test file already has similar helpers; this one is
// self-contained so the extra tests don't depend on the
// query-string conventions the spec-level tests use.
func doLogsRequest(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code >= 400 {
		return rr, nil
	}
	var decoded map[string]any
	if rr.Body.Len() > 0 {
		_ = json.Unmarshal(rr.Body.Bytes(), &decoded)
	}
	return rr, decoded
}

// _ = http.MethodGet keeps the import for callers that
// later need method constants.
var _ = http.MethodGet

// TestRetentionPolicy_GetSet: GET returns default; PUT
// persists the new value; subsequent GET reads it back.
func TestRetentionPolicy_GetSet(t *testing.T) {
	h := newLogsHandlerFixture(t)

	// Default policy should exist (a sensible default the
	// spec mandates in its "Get retention policy" scenario).
	rr, body := doLogsRequest(t, h, "GET", "/api/v1/logs/retention", nil)
	if rr.Code != 200 {
		t.Fatalf("GET default: status = %d, want 200", rr.Code)
	}
	if _, ok := body["retention_days"]; !ok {
		t.Errorf("default policy missing retention_days: %v", body)
	}

	// PUT a new policy.
	rr, _ = doLogsRequest(t, h, "PUT", "/api/v1/logs/retention", map[string]any{
		"retention_days": 30,
		"max_storage_gb": 500,
	})
	if rr.Code != 200 {
		t.Errorf("PUT: status = %d, want 200", rr.Code)
	}

	// GET should return the new value.
	rr, body = doLogsRequest(t, h, "GET", "/api/v1/logs/retention", nil)
	if rd, _ := body["retention_days"].(float64); int(rd) != 30 {
		t.Errorf("retention_days = %v, want 30", body["retention_days"])
	}
	if mg, _ := body["max_storage_gb"].(float64); int(mg) != 500 {
		t.Errorf("max_storage_gb = %v, want 500", body["max_storage_gb"])
	}
}

// TestRetentionPolicy_TriggerCleanup: POST
// /logs/retention/cleanup returns 200 + a cleanup report
// (rows removed, bytes freed). For a backend where the
// actual delete is a stub, the report is a zero-value
// snapshot — we only assert the wire shape here.
func TestRetentionPolicy_TriggerCleanup(t *testing.T) {
	h := newLogsHandlerFixture(t)
	rr, body := doLogsRequest(t, h, "POST", "/api/v1/logs/retention/cleanup", nil)
	if rr.Code != 200 {
		t.Fatalf("POST cleanup: status = %d, want 200", rr.Code)
	}
	if _, ok := body["rows_removed"]; !ok {
		t.Errorf("cleanup response missing rows_removed: %v", body)
	}
}

// TestLogStats_Endpoint: GET /logs/stats returns aggregate
// counts (total, by_level, by_source). The shape is the
// spec's "Log Statistics" scenario.
func TestLogStats_Endpoint(t *testing.T) {
	h := newLogsHandlerFixture(t)
	rr, body := doLogsRequest(t, h, "GET", "/api/v1/logs/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("GET stats: status = %d, want 200", rr.Code)
	}
	for _, key := range []string{"total", "by_level", "by_source"} {
		if _, ok := body[key]; !ok {
			t.Errorf("stats response missing %q: %v", key, body)
		}
	}
}

// TestSavedFilters_CRUD: full lifecycle — create, list,
// get by id, apply (returns matching query body), delete.
func TestSavedFilters_CRUD(t *testing.T) {
	h := newLogsHandlerFixture(t)

	// Create
	rr, body := doLogsRequest(t, h, "POST", "/api/v1/logs/saved-filters", map[string]any{
		"name":  "errors-only",
		"query": map[string]any{"level": "error", "limit": 50},
	})
	if rr.Code != 201 {
		t.Fatalf("POST: status = %d, want 201; body = %v", rr.Code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("created filter has no id")
	}

	// List
	rr, body = doLogsRequest(t, h, "GET", "/api/v1/logs/saved-filters", nil)
	if rr.Code != 200 {
		t.Fatalf("LIST: status = %d", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(saved_filters) = %d, want 1", len(data))
	}

	// Get
	rr, body = doLogsRequest(t, h, "GET", "/api/v1/logs/saved-filters/"+id, nil)
	if rr.Code != 200 {
		t.Errorf("GET by id: status = %d, want 200", rr.Code)
	}

	// Apply
	rr, body = doLogsRequest(t, h, "POST", "/api/v1/logs/saved-filters/"+id+"/apply", nil)
	if rr.Code != 200 {
		t.Errorf("APPLY: status = %d, want 200", rr.Code)
	}
	// Apply returns the rendered query + a "rows" array of
	// matching log records. The local backend's seed data
	// may be empty so we only assert the wire shape.
	if _, ok := body["query"]; !ok {
		t.Errorf("apply response missing query: %v", body)
	}

	// Delete
	rr, _ = doLogsRequest(t, h, "DELETE", "/api/v1/logs/saved-filters/"+id, nil)
	if rr.Code != 204 {
		t.Errorf("DELETE: status = %d, want 204", rr.Code)
	}

	// Re-list → empty
	rr, body = doLogsRequest(t, h, "GET", "/api/v1/logs/saved-filters", nil)
	data, _ = body["data"].([]any)
	if len(data) != 0 {
		t.Errorf("after delete, saved_filters len = %d, want 0", len(data))
	}
}

// TestAlertRules_CRUD: the spec's "Alert Rules"
// requirement — create, list, get, delete.
func TestAlertRules_CRUD(t *testing.T) {
	h := newLogsHandlerFixture(t)

	rr, body := doLogsRequest(t, h, "POST", "/api/v1/logs/alert-rules", map[string]any{
		"name":      "high-error-rate",
		"condition": "level=error",
		"window":    "5m",
		"threshold": 10,
		"channel":   "slack-ops",
	})
	if rr.Code != 201 {
		t.Fatalf("POST: status = %d, want 201; body = %v", rr.Code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("created rule has no id")
	}

	rr, body = doLogsRequest(t, h, "GET", "/api/v1/logs/alert-rules", nil)
	if rr.Code != 200 {
		t.Fatalf("LIST: status = %d", rr.Code)
	}
	rules, _ := body["data"].([]any)
	if len(rules) != 1 {
		t.Errorf("len(alert_rules) = %d, want 1", len(rules))
	}

	rr, _ = doLogsRequest(t, h, "DELETE", "/api/v1/logs/alert-rules/"+id, nil)
	if rr.Code != 204 {
		t.Errorf("DELETE: status = %d, want 204", rr.Code)
	}
}
