package servicecatalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
)

// newTestHandler builds a router with the catalog handler
// attached under /api/v1. Used by the handler tests to drive
// real HTTP requests.
func newTestHandler(t *testing.T) (*gin.Engine, *Catalog) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	cat := NewCatalog(repo)
	r := gin.New()
	api := r.Group("/api/v1")
	NewHandler(cat).Register(api, rbac.NoopPermFactory())
	return r, cat
}

// doJSON sends a request and decodes the body into a
// map[string]any. Tests use the loose shape rather than a
// strict struct so the assertions are explicit and the
// envelope is visible.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	var req *http.Request
	if bodyReader != nil {
		req = httptest.NewRequest(method, path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

// TestHandler_Create_Returns201 pins the happy path: POST
// /api/v1/services with a valid body returns 201 + the row.
func TestHandler_Create_Returns201(t *testing.T) {
	r, _ := newTestHandler(t)
	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"payments-api","tier":"critical","owner":"team-payments@example.com"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %v", w.Code, body)
	}
	if body["name"] != "payments-api" {
		t.Errorf("name = %v, want payments-api", body["name"])
	}
	if body["tier"] != "critical" {
		t.Errorf("tier = %v, want critical", body["tier"])
	}
	if _, ok := body["id"]; !ok {
		t.Errorf("response missing id; body = %v", body)
	}
}

// TestHandler_Create_InvalidJSON_Returns400 pins the
// envelope shape on bad input.
func TestHandler_Create_InvalidJSON_Returns400(t *testing.T) {
	r, _ := newTestHandler(t)
	w, body := doJSON(t, r, "POST", "/api/v1/services", `{bad json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %v", w.Code, body)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("response missing error envelope; body = %v", body)
	}
}

// TestHandler_Create_BlankName_Returns400 pins that
// validation errors land on the spec's 400 path.
func TestHandler_Create_BlankName_Returns400(t *testing.T) {
	r, _ := newTestHandler(t)
	w, _ := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"","tier":"standard"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestHandler_Get_NotFound_Returns404 pins the not-found
// envelope for an unknown ID.
func TestHandler_Get_NotFound_Returns404(t *testing.T) {
	r, _ := newTestHandler(t)
	w, body := doJSON(t, r, "GET", "/api/v1/services/missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %v", w.Code, body)
	}
}

// TestHandler_List_EmptyEnvelope pins the spec's list
// envelope shape: {data: [...], pagination: ...}.
func TestHandler_List_EmptyEnvelope(t *testing.T) {
	r, _ := newTestHandler(t)
	w, body := doJSON(t, r, "GET", "/api/v1/services", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if _, ok := body["data"]; !ok {
		t.Errorf("list envelope missing 'data'; body = %v", body)
	}
	if _, ok := body["pagination"]; !ok {
		t.Errorf("list envelope missing 'pagination'; body = %v", body)
	}
}

// TestHandler_Delete_Returns204 pins the no-content
// success path on soft delete.
func TestHandler_Delete_Returns204(t *testing.T) {
	r, _ := newTestHandler(t)
	// Create one to delete.
	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"to-delete","tier":"standard"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	id, _ := body["id"].(string)

	w, _ = doJSON(t, r, "DELETE", "/api/v1/services/"+id, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", w.Code)
	}

	// Subsequent GET is 404.
	w, _ = doJSON(t, r, "GET", "/api/v1/services/"+id, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("after delete, GET status = %d, want 404", w.Code)
	}
}

// TestHandler_FilterByQuery pins that the q= filter
// narrows the list.
func TestHandler_FilterByQuery(t *testing.T) {
	r, _ := newTestHandler(t)
	for _, body := range []string{
		`{"name":"payments-api","tier":"critical"}`,
		`{"name":"payments-worker","tier":"standard"}`,
		`{"name":"auth-api","tier":"standard"}`,
	} {
		w, b := doJSON(t, r, "POST", "/api/v1/services", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("seed create: %d %v", w.Code, b)
		}
	}
	w, got := doJSON(t, r, "GET", "/api/v1/services?q=payment", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", w.Code)
	}
	data, _ := got["data"].([]any)
	if len(data) != 2 {
		t.Errorf("q=payment returned %d rows, want 2", len(data))
	}
}

// seedService is a tiny test helper that creates a
// service and returns its id. The body argument is the
// full JSON request body so callers can vary the
// tier/name without a builder.
func seedService(t *testing.T, r *gin.Engine, body string) string {
	t.Helper()
	w, b := doJSON(t, r, "POST", "/api/v1/services", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed service: %d %v", w.Code, b)
	}
	id, _ := b["id"].(string)
	if id == "" {
		t.Fatalf("seed service: response missing id; body = %v", b)
	}
	return id
}

// futureRange returns a [start, end) pair one and two
// hours from now, in RFC3339. Used by the write tests
// to avoid hard-coded dates that would rot.
func futureRange() (string, string) {
	now := time.Now()
	return now.Add(1 * time.Hour).UTC().Format(time.RFC3339),
		now.Add(2 * time.Hour).UTC().Format(time.RFC3339)
}

// TestHandler_CreateOnCall_Returns201 pins the happy
// path: POST /services/:id/oncall with a valid body
// returns 201 + the row. Timestamps are relative to
// now so the test does not rot.
func TestHandler_CreateOnCall_Returns201(t *testing.T) {
	r, _ := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-oncall-happy","tier":"standard"}`)
	start, end := futureRange()
	body := `{"user_email":"alice@example.com","shift_start":"` + start + `","shift_end":"` + end + `","scope":"service"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/"+id+"/oncall", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %v", w.Code, got)
	}
	if got["user"] != "alice@example.com" {
		t.Errorf("user = %v, want alice@example.com", got["user"])
	}
	if _, ok := got["id"]; !ok {
		t.Errorf("response missing id; body = %v", got)
	}
}

// TestHandler_CreateOnCall_BlankUser_Returns400 pins
// the validation envelope: an empty user_email rejects
// the request at the 400 layer.
func TestHandler_CreateOnCall_BlankUser_Returns400(t *testing.T) {
	r, _ := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-oncall-400","tier":"standard"}`)
	start, end := futureRange()
	body := `{"user_email":"","shift_start":"` + start + `","shift_end":"` + end + `"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/"+id+"/oncall", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %v", w.Code, got)
	}
}

// TestHandler_CreateOnCall_UnknownService_Returns404
// pins that an unknown service id is rejected with 404
// (NOT_FOUND) rather than 400.
func TestHandler_CreateOnCall_UnknownService_Returns404(t *testing.T) {
	r, _ := newTestHandler(t)
	start, end := futureRange()
	body := `{"user_email":"alice@example.com","shift_start":"` + start + `","shift_end":"` + end + `"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/missing/oncall", body)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %v", w.Code, got)
	}
}

// TestHandler_DeleteOnCall_Returns204 pins the
// success path on DELETE; subsequent delete surfaces
// the not-found envelope.
func TestHandler_DeleteOnCall_Returns204(t *testing.T) {
	r, cat := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-oncall-del","tier":"standard"}`)
	now := time.Now()
	row := &OnCall{
		ServiceID:  id,
		User:       "alice@example.com",
		ShiftStart: now.Add(1 * time.Hour),
		ShiftEnd:   now.Add(2 * time.Hour),
	}
	if err := cat.repo.CreateOnCall(row); err != nil {
		t.Fatalf("seed oncall: %v", err)
	}
	w, _ := doJSON(t, r, "DELETE", "/api/v1/services/"+id+"/oncall/"+row.ID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", w.Code)
	}
	w, _ = doJSON(t, r, "DELETE", "/api/v1/services/"+id+"/oncall/"+row.ID, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", w.Code)
	}
}

// TestHandler_CreateRunbook_Returns201 pins the happy
// path: POST /services/:id/runbook with valid title +
// body returns 201 + the row.
func TestHandler_CreateRunbook_Returns201(t *testing.T) {
	r, _ := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-rb-happy","tier":"standard"}`)
	body := `{"title":"Restart procedure","body":"1. ssh in\n2. systemctl restart"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/"+id+"/runbook", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %v", w.Code, got)
	}
	if got["title"] != "Restart procedure" {
		t.Errorf("title = %v, want Restart procedure", got["title"])
	}
}

// TestHandler_CreateRunbook_BlankTitle_Returns400 pins
// the validation envelope: an empty title is rejected
// at the 400 layer.
func TestHandler_CreateRunbook_BlankTitle_Returns400(t *testing.T) {
	r, _ := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-rb-400","tier":"standard"}`)
	body := `{"title":"","body":"x"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/"+id+"/runbook", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %v", w.Code, got)
	}
}

// TestHandler_CreateRunbook_UnknownService_Returns404
// pins the not-found envelope on an unknown service id.
func TestHandler_CreateRunbook_UnknownService_Returns404(t *testing.T) {
	r, _ := newTestHandler(t)
	body := `{"title":"x","body":"y"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/missing/runbook", body)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %v", w.Code, got)
	}
}

// TestHandler_DeleteRunbook_Returns204 pins the
// success path on DELETE; subsequent delete surfaces
// the not-found envelope.
func TestHandler_DeleteRunbook_Returns204(t *testing.T) {
	r, cat := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-rb-del","tier":"standard"}`)
	row := &RunbookEntry{ServiceID: id, Title: "x", Body: "y"}
	if err := cat.repo.CreateRunbook(row); err != nil {
		t.Fatalf("seed runbook: %v", err)
	}
	w, _ := doJSON(t, r, "DELETE", "/api/v1/services/"+id+"/runbook/"+row.ID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", w.Code)
	}
	w, _ = doJSON(t, r, "DELETE", "/api/v1/services/"+id+"/runbook/"+row.ID, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", w.Code)
	}
}

// TestHandler_CreateOnCall_ConflictOnOverlap pins the
// 409 CONFLICT path: a second shift strictly inside
// the first is rejected with 409.
func TestHandler_CreateOnCall_ConflictOnOverlap(t *testing.T) {
	r, _ := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-oncall-409","tier":"standard"}`)
	now := time.Now()
	f1 := now.Add(1 * time.Hour).UTC().Format(time.RFC3339)
	f2 := now.Add(2 * time.Hour).UTC().Format(time.RFC3339)
	f3 := now.Add(3 * time.Hour).UTC().Format(time.RFC3339)
	fMid := now.Add(90 * time.Minute).UTC().Format(time.RFC3339)
	first := `{"user_email":"alice@example.com","shift_start":"` + f1 + `","shift_end":"` + f2 + `"}`
	w, _ := doJSON(t, r, "POST", "/api/v1/services/"+id+"/oncall", first)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: %d", w.Code)
	}
	overlap := `{"user_email":"bob@example.com","shift_start":"` + fMid + `","shift_end":"` + f3 + `"}`
	w, got := doJSON(t, r, "POST", "/api/v1/services/"+id+"/oncall", overlap)
	if w.Code != http.StatusConflict {
		t.Fatalf("overlap status = %d, want 409; body = %v", w.Code, got)
	}
}

// TestHandler_CreateOnCall_ActorFromJWT pins the
// P0 #3 audit-trail-forgery fix: the actor captured
// by the handler is the dev-bypass X-User header (the
// surface that becomes the JWT subject in
// production). The actual audit emit is the P0 #3
// agent's job; this test pins the on-call row's
// rotation user stays separate from the actor.
//
// TODO(audit): once P0 #3 lands an audit.Service
// dependency, replace the persisted-row sanity check
// with a real audit log assertion.
func TestHandler_CreateOnCall_ActorFromJWT(t *testing.T) {
	r, cat := newTestHandler(t)
	id := seedService(t, r, `{"name":"svc-oncall-actor","tier":"standard"}`)
	now := time.Now()
	start := now.Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	end := now.Add(1 * time.Hour).UTC().Format(time.RFC3339)
	req := `{"user_email":"bob@example.com","shift_start":"` + start + `","shift_end":"` + end + `"}`
	w, _ := doJSONWithHeader(t, r, "POST", "/api/v1/services/"+id+"/oncall", req, "X-User", "alice@example.com")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d", w.Code)
	}
	got, err := cat.repo.CurrentOnCall(id, now)
	if err != nil || got == nil {
		t.Fatalf("current oncall: err=%v got=%v", err, got)
	}
	if got.User != "bob@example.com" {
		t.Errorf("oncall user = %q, want bob (actor goes to audit, not oncall.user)", got.User)
	}
}

// doJSONWithHeader is doJSON plus a single extra
// header. Used by the actor-from-JWT test to inject
// the dev-bypass X-User header.
func doJSONWithHeader(t *testing.T, r *gin.Engine, method, path, body, hdrKey, hdrVal string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	var req *http.Request
	if bodyReader != nil {
		req = httptest.NewRequest(method, path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set(hdrKey, hdrVal)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}
