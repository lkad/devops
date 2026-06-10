package servicecatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestHandler_Get_IncludesOnCall pins the wire shape:
// GET /api/v1/services/:id embeds the current on-call
// (or null) so the detail page can render "currently on
// call" without a second round trip.
func TestHandler_Get_IncludesOnCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	cat := NewCatalog(repo)
	h := NewHandler(cat)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)

	// Create the service and a current shift.
	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"svc-with-oncall","tier":"critical"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	id := body["id"].(string)
	now := time.Now()
	if err := repo.CreateOnCall(&OnCall{
		ServiceID:  id,
		User:       "alice@example.com",
		ShiftStart: now.Add(-1 * time.Hour),
		ShiftEnd:   now.Add(1 * time.Hour),
	}); err != nil {
		t.Fatalf("create oncall: %v", err)
	}

	// GET the service.
	rr, getBody := doJSON(t, r, "GET", "/api/v1/services/"+id, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: %d %v", rr.Code, getBody)
	}
	raw, _ := json.Marshal(getBody["oncall"])
	if !contains(string(raw), "alice@example.com") {
		t.Fatalf("oncall block missing user: %s", string(raw))
	}
}

// TestHandler_Get_OnCallAbsent pins the "no current shift"
// branch: the field is present and null (not omitted)
// so the frontend can render "no one on call" without a
// missing-key check.
func TestHandler_Get_OnCallAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	cat := NewCatalog(repo)
	h := NewHandler(cat)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)

	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"svc-no-oncall","tier":"standard"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	id := body["id"].(string)

	rr, getBody := doJSON(t, r, "GET", "/api/v1/services/"+id, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: %d", rr.Code)
	}
	if _, ok := getBody["oncall"]; !ok {
		t.Fatalf("oncall field should be present (null OK)")
	}
	if getBody["oncall"] != nil {
		t.Fatalf("oncall = %v, want nil", getBody["oncall"])
	}
}

// TestHandler_Get_IncludesRunbook pins the runbook wire
// shape: GET /api/v1/services/:id embeds the runbook
// array (empty array if no entries).
func TestHandler_Get_IncludesRunbook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	cat := NewCatalog(repo)
	h := NewHandler(cat)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)

	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"svc-rb","tier":"standard"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	id := body["id"].(string)
	if err := repo.CreateRunbook(&RunbookEntry{
		ServiceID: id,
		Title:     "Restart procedure",
		Body:      "1. ssh in 2. systemctl restart",
	}); err != nil {
		t.Fatalf("create runbook: %v", err)
	}

	rr, getBody := doJSON(t, r, "GET", "/api/v1/services/"+id, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: %d", rr.Code)
	}
	rb, ok := getBody["runbook"].([]any)
	if !ok {
		t.Fatalf("runbook not an array: %T %v", getBody["runbook"], getBody["runbook"])
	}
	if len(rb) != 1 {
		t.Errorf("len(runbook) = %d, want 1", len(rb))
	}
}

// contains is a tiny substring helper kept local so
// the test file has no extra imports.
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub ||
		indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
var _ = context.Background
