package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// handlerDB returns a fresh sqlite db for handler tests.
func handlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "audit.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// newTestRouter builds a Gin engine with the audit routes
// mounted under /api/v1.
func newTestRouter(repo *Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := NewService(ServiceConfig{Repo: repo})
	h := NewHandler(svc)
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r
}

// TestHandler_List_NoEvents is the empty-list happy path: 200
// with an empty data array and total=0.
func TestHandler_List_NoEvents(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
	var body struct {
		Data       []AuditEvent `json:"data"`
		Pagination *struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Pagination == nil || body.Pagination.Total != 0 {
		t.Errorf("expected total=0, got %+v", body.Pagination)
	}
	if len(body.Data) != 0 {
		t.Errorf("expected empty data, got %+v", body.Data)
	}
}

// TestHandler_List_ActionFilter covers the spec scenario
// "Filter by entity type" generalised to any single filter
// dimension.
func TestHandler_List_ActionFilter(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionDelete, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now})
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=delete", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
	var body struct {
		Data       []AuditEvent `json:"data"`
		Pagination *struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Pagination == nil || body.Pagination.Total != 1 {
		t.Errorf("expected total=1, got %+v", body.Pagination)
	}
	if len(body.Data) != 1 || body.Data[0].Action != ActionDelete {
		t.Errorf("data mismatch: %+v", body.Data)
	}
}

// TestHandler_List_AllFilters covers the spec scenario where all
// filter dimensions are present simultaneously.
func TestHandler_List_AllFilters(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ActorID: "u-1", ActorUsername: "alice", ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ActorID: "u-2", ActorUsername: "bob", ResourceType: ResourceProject, ResourceID: "p-1", OccurredAt: now})
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	url := "/api/v1/audit?action=create&actor_id=u-1&actor_username=alice&resource_type=device&resource_id=d-1"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data       []AuditEvent `json:"data"`
		Pagination *struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Pagination == nil || body.Pagination.Total != 1 {
		t.Errorf("expected total=1, got %+v", body.Pagination)
	}
}

// TestHandler_List_Pagination covers the spec scenario
// "Pagination" — limit / offset are honoured.
func TestHandler_List_Pagination(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now.Add(time.Duration(i) * time.Second)})
	}
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?limit=2&offset=1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
	var body struct {
		Data       []AuditEvent `json:"data"`
		Pagination *struct {
			Total   int64 `json:"total"`
			Limit   int   `json:"limit"`
			Offset  int   `json:"offset"`
			HasMore bool  `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Pagination == nil || body.Pagination.Total != 5 {
		t.Errorf("expected total=5, got %+v", body.Pagination)
	}
	if body.Pagination.Limit != 2 || body.Pagination.Offset != 1 {
		t.Errorf("pagination fields: %+v", body.Pagination)
	}
	if !body.Pagination.HasMore {
		t.Error("expected has_more=true")
	}
}

// TestHandler_Get_Ok covers the GET /audit/:id happy path.
func TestHandler_Get_Ok(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	e := &AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: time.Now().UTC()}
	_ = repo.Create(e)
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/"+e.ID, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
	var got AuditEvent
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != e.ID {
		t.Errorf("id: got %q want %q", got.ID, e.ID)
	}
}

// TestHandler_Get_NotFound covers the 404 envelope for a missing
// id.
func TestHandler_Get_NotFound(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/nope", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", w.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("code: got %q want NOT_FOUND", body.Error.Code)
	}
}

// TestHandler_List_BadFilter covers the 400 envelope for a
// malformed time-range parameter.
func TestHandler_List_BadFilter(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?from=not-a-time", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", w.Code)
	}
}

// TestHandler_List_TimeRange covers the spec scenario for
// from / to query parameters.
func TestHandler_List_TimeRange(t *testing.T) {
	repo := NewRepository(handlerDB(t))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: base})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-2", OccurredAt: base.Add(48 * time.Hour)})
	from := base.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	to := base.Add(72 * time.Hour).UTC().Format(time.RFC3339)
	r := newTestRouter(repo)
	w := httptest.NewRecorder()
	url := "/api/v1/audit?from=" + from + "&to=" + to
	req := httptest.NewRequest(http.MethodGet, url, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data       []AuditEvent `json:"data"`
		Pagination *struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Pagination == nil || body.Pagination.Total != 1 {
		t.Errorf("expected total=1, got %+v", body.Pagination)
	}
}
