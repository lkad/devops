package servicecatalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
	NewHandler(cat).Register(api)
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
