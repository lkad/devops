package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
)

// recordingLogsEmitter captures every AuditEvent for assertion.
// It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingLogsEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingLogsEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingLogsEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newLogsAuditFixture wires a complete router for the logs
// module with a recording audit service so the saved-filter
// and alert-rule mutating handlers can be exercised
// end-to-end. The fixture mirrors newLogsHandlerFixture but
// threads the audit service in.
func newLogsAuditFixture(t *testing.T) (*gin.Engine, *recordingLogsEmitter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:logs-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&SavedFilter{}, &AlertRule{}, &RetentionPolicy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	local := NewLocal(LocalConfig{Dir: t.TempDir()})
	svc := NewService(local, ServiceConfig{})
	repo := NewExtraRepository(db)
	extra := NewExtraService(repo, svc)
	emitter := &recordingLogsEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})

	r := gin.New()
	api := r.Group("/api/v1")
	h := NewHandlerWithExtra(svc, extra, repo, auditSvc)
	h.Register(api, rbac.NoopPermFactory())
	return r, emitter
}

// doJSON sends a JSON request and decodes the response body
// into a generic map. Mirrors the helper used in the catalog
// tests.
func doJSONLogs(t *testing.T, r *gin.Engine, method, path string, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w.Code, out
}

// TestHandler_CreateSavedFilter_EmitsAudit pins the
// handler-level audit emission for saved-filter creation.
// The P0 #3 audit-trail commit added this emit alongside the
// alert-rule emit; both share the same "logs configuration"
// family of events.
func TestHandler_CreateSavedFilter_EmitsAudit(t *testing.T) {
	r, emitter := newLogsAuditFixture(t)
	code, body := doJSONLogs(t, r, "POST", "/api/v1/logs/saved-filters",
		`{"name":"errors-only","query":{"q":"ERROR"}}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, body)
	}
	id, _ := body["id"].(string)
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceSavedFilter {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceSavedFilter)
	}
	if events[0].ResourceID != id {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, id)
	}
}

// TestHandler_DeleteSavedFilter_EmitsAudit pins the delete
// path.
func TestHandler_DeleteSavedFilter_EmitsAudit(t *testing.T) {
	r, emitter := newLogsAuditFixture(t)
	code, body := doJSONLogs(t, r, "POST", "/api/v1/logs/saved-filters",
		`{"name":"errors-only","query":{"q":"ERROR"}}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, body)
	}
	id, _ := body["id"].(string)
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/logs/saved-filters/"+id, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, delReq)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %v", w.Code, w.Body.String())
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionDelete {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionDelete)
	}
	if last.ResourceID != id {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, id)
	}
}
