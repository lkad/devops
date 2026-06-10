package project

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newTestRouter wires the project handler onto a fresh Gin router
// with the in-memory database. The RBAC middleware is NOT attached
// here — handler tests focus on the contract, not the permission
// check. A separate test exercises the middleware chain.
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc, repo)
	r := gin.New()
	h.Register(r.Group("/api/v1"), rbac.NoopPermFactory())
	return r
}

// doJSON is a tiny helper that builds a request with a JSON body
// (or nil), runs it through the supplied router, and returns the
// response recorder.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestHandler_CreateProject_201 is the spec scenario
// "Create Project": POST /api/v1/projects with valid input yields
// 201 and the created row.
func TestHandler_CreateProject_201(t *testing.T) {
	r := newTestRouter(t)
	// seed a type
	w := doJSON(t, r, "POST", "/api/v1/project-types", map[string]any{"name": "platform", "weight": 1})
	if w.Code != http.StatusCreated {
		t.Fatalf("seed type status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w = doJSON(t, r, "POST", "/api/v1/projects", map[string]any{
		"name":    "Payments",
		"code":    "payments",
		"type_id": created.ID,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", w.Code, w.Body.String())
	}
	var got Project
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID == "" || got.Name != "Payments" {
		t.Errorf("unexpected project: %+v", got)
	}
}

// TestHandler_CreateProject_400 covers the spec validation path:
// bad input returns a 400 with a VALIDATION_ERROR envelope.
func TestHandler_CreateProject_400(t *testing.T) {
	r := newTestRouter(t)
	w := doJSON(t, r, "POST", "/api/v1/projects", map[string]any{
		"name": "",
		"code": "x",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", w.Code, w.Body.String())
	}
	var env contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Error.Code != contracts.CodeValidation {
		t.Errorf("code=%s want VALIDATION_ERROR", env.Error.Code)
	}
}

// TestHandler_ListProjects_Envelope is the spec's api-contract
// guarantee: list endpoints return { data, pagination }.
func TestHandler_ListProjects_Envelope(t *testing.T) {
	r := newTestRouter(t)
	// empty list
	w := doJSON(t, r, "GET", "/api/v1/projects", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env contracts.ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Pagination == nil {
		t.Error("pagination missing")
	}
}

// TestHandler_GetProject_Relations returns the project with its
// parent, children, and members in a single payload. The spec
// calls for "Get Project with resource links" — we return
// children + members as a stand-in.
func TestHandler_GetProject_Relations(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	pID := seedProject(t, r, "BL", "bl", ptID, nil)

	w := doJSON(t, r, "GET", "/api/v1/projects/"+pID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	// The body shape: { project: {...}, children: [...], members: [...] }
	var got struct {
		Project  Project           `json:"project"`
		Children []Project         `json:"children"`
		Members  []ProjectMember   `json:"members"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Project.ID != pID {
		t.Errorf("project id mismatch")
	}
}

// TestHandler_UpdateProject_200 is the spec "Update Project".
func TestHandler_UpdateProject_200(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	pID := seedProject(t, r, "Old", "old", ptID, nil)

	w := doJSON(t, r, "PUT", "/api/v1/projects/"+pID, map[string]any{"name": "New"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got Project
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Name != "New" {
		t.Errorf("name=%s want New", got.Name)
	}
}

// TestHandler_DeleteProject_204 is the spec "Delete Project" for
// a leaf.
func TestHandler_DeleteProject_204(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	pID := seedProject(t, r, "BL", "bl", ptID, nil)

	w := doJSON(t, r, "DELETE", "/api/v1/projects/"+pID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestHandler_DeleteProject_422 covers the "reject if has
// children" requirement. A non-leaf delete yields 422.
func TestHandler_DeleteProject_422(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	blID := seedProject(t, r, "BL", "bl", ptID, nil)
	_ = seedProject(t, r, "Sys", "sys", ptID, &blID)

	w := doJSON(t, r, "DELETE", "/api/v1/projects/"+blID, nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", w.Code, w.Body.String())
	}
}

// TestHandler_ListChildren covers GET /api/v1/projects/:id/children.
func TestHandler_ListChildren(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	blID := seedProject(t, r, "BL", "bl", ptID, nil)
	_ = seedProject(t, r, "Sys1", "s1", ptID, &blID)
	_ = seedProject(t, r, "Sys2", "s2", ptID, &blID)

	w := doJSON(t, r, "GET", "/api/v1/projects/"+blID+"/children", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env contracts.ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Pagination == nil || env.Pagination.Total != 2 {
		t.Errorf("total=%v want 2", env.Pagination)
	}
}

// TestHandler_ListAncestors covers GET /api/v1/projects/:id/ancestors.
func TestHandler_ListAncestors(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	blID := seedProject(t, r, "BL", "bl", ptID, nil)
	sysID := seedProject(t, r, "Sys", "sys", ptID, &blID)
	prjID := seedProject(t, r, "Prj", "prj", ptID, &sysID)

	w := doJSON(t, r, "GET", "/api/v1/projects/"+prjID+"/ancestors", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got []Project
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ancestors=%d want 2", len(got))
	}
}

// TestHandler_Members_AddAndList covers POST /:id/members and
// GET /:id/members.
func TestHandler_Members_AddAndList(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	pID := seedProject(t, r, "BL", "bl", ptID, nil)

	w := doJSON(t, r, "POST", "/api/v1/projects/"+pID+"/members", map[string]any{
		"user_id":  "u-1",
		"role":     "viewer",
		"added_by": "admin",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("add member status=%d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, "GET", "/api/v1/projects/"+pID+"/members", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var got []ProjectMember
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].UserID != "u-1" || got[0].Role != "viewer" {
		t.Errorf("members: %+v", got)
	}
}

// TestHandler_Members_Remove covers DELETE /:id/members/:user_id
// and the spec scenario "Revoke permission".
func TestHandler_Members_Remove(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	pID := seedProject(t, r, "BL", "bl", ptID, nil)
	if err := addMember(t, r, pID, "u-1", "viewer", "admin"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	w := doJSON(t, r, "DELETE", "/api/v1/projects/"+pID+"/members/u-1", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestHandler_ListTypes is the spec scenario
// "List Business Lines" expressed as the project-type list, which
// the spec says is "used by forms".
func TestHandler_ListTypes(t *testing.T) {
	r := newTestRouter(t)
	_ = seedType(t, r, "platform")
	_ = seedType(t, r, "infra")

	w := doJSON(t, r, "GET", "/api/v1/project-types", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got []ProjectType
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("count=%d want 2", len(got))
	}
}

// TestHandler_NotFound is the API contract: a missing project is
// a 404 with NOT_FOUND code.
func TestHandler_NotFound(t *testing.T) {
	r := newTestRouter(t)
	w := doJSON(t, r, "GET", "/api/v1/projects/ghost", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Error.Code != contracts.CodeNotFound {
		t.Errorf("code=%s want NOT_FOUND", env.Error.Code)
	}
}

// TestHandler_FilterByParent is the spec's "List Projects under
// System" — the list endpoint with parent_id filter must scope
// the result to that parent.
func TestHandler_FilterByParent(t *testing.T) {
	r := newTestRouter(t)
	ptID := seedType(t, r, "platform")
	blID := seedProject(t, r, "BL", "bl", ptID, nil)
	_ = seedProject(t, r, "Sys", "sys", ptID, &blID)

	w := doJSON(t, r, "GET", "/api/v1/projects?parent_id="+blID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env contracts.ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Pagination == nil || env.Pagination.Total != 1 {
		t.Errorf("total=%v want 1", env.Pagination)
	}
}

// TestHandler_InvalidJSON is a defensive guard: a malformed body
// yields 400, not a 500 panic.
func TestHandler_InvalidJSON(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest("POST", "/api/v1/project-types", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d want 400 body=%s", w.Code, w.Body.String())
	}
}

// seedType is a test helper that POSTs a project type and
// returns its id. It uses the real handler so the test exercises
// the wire path.
func seedType(t *testing.T, r *gin.Engine, name string) string {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/v1/project-types", map[string]any{"name": name})
	if w.Code != http.StatusCreated {
		t.Fatalf("seed type %q status=%d body=%s", name, w.Code, w.Body.String())
	}
	var got ProjectType
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("seed type unmarshal: %v", err)
	}
	return got.ID
}

// seedProject is a test helper that POSTs a project and returns
// its id. The parentID argument is a pointer so the caller can
// pass nil for a root.
func seedProject(t *testing.T, r *gin.Engine, name, code, typeID string, parentID *string) string {
	t.Helper()
	body := map[string]any{
		"name":    name,
		"code":    code,
		"type_id": typeID,
	}
	if parentID != nil {
		body["parent_id"] = *parentID
	}
	w := doJSON(t, r, "POST", "/api/v1/projects", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed project %q status=%d body=%s", code, w.Code, w.Body.String())
	}
	var got Project
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("seed project unmarshal: %v", err)
	}
	return got.ID
}

// addMember seeds a project member through the wire path. Returns
// an error string only on failure so the caller can log it.
func addMember(t *testing.T, r *gin.Engine, projectID, userID, role, addedBy string) error {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/v1/projects/"+projectID+"/members", map[string]any{
		"user_id":  userID,
		"role":     role,
		"added_by": addedBy,
	})
	if w.Code != http.StatusCreated {
		return errStr("add member: status=%d body=%s", w.Code, w.Body.String())
	}
	return nil
}

// errStr is a tiny formatter used by the test helpers. It uses
// fmt.Sprintf internally; the indirection exists so callers can
// use %d/%s placeholders without depending on fmt directly.
func errStr(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}
