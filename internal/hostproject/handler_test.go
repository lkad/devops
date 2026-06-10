package hostproject

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
)

// newHandlerRig is the single fixture for handler tests:
// it builds a Gin engine wired to the HostProjectHandler
// over a fresh in-memory DB, and returns the DB + the
// service so the test can seed links directly. The
// handler does not know about the service, but the test
// does — this is the standard "real handler + service
// shortcut for seeding" pattern used across the repo.
func newHandlerRig(t *testing.T) (*gin.Engine, *gorm.DB, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	projRepo := projectpkg.NewRepository(db)
	projSvc := projectpkg.NewService(projRepo)
	devRepo := devicepkg.NewRepository(db)

	svc := NewService(repo, projSvc)
	svc.SetDeviceGetter(devRepo.Get)
	hpH := NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	hpH.Register(api, rbac.NoopPermFactory())
	return r, db, svc
}

// doJSON builds a request with a JSON body (or nil) and
// runs it through the supplied router.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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
	var decoded map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &decoded)
	}
	return w, decoded
}

// TestHandler_ListDeviceProjects_EmptyReturnsEmptyArray
// covers the spec scenario "No linked projects": the
// list endpoint must return data: [] for a device with
// no links, not 404.
func TestHandler_ListDeviceProjects_EmptyReturnsEmptyArray(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")

	rr, body := doJSON(t, r, "GET", "/api/v1/devices/"+dev.ID+"/projects", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(data) != 0 {
		t.Errorf("data len = %d, want 0", len(data))
	}
}

// TestHandler_LinkDeviceProject_201 covers the spec
// scenario "Link physical host to project from project
// page": POST /devices/:id/projects yields 201.
func TestHandler_LinkDeviceProject_201(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)

	rr, body := doJSON(t, r, "POST", "/api/v1/devices/"+dev.ID+"/projects",
		map[string]any{"project_id": prj.ID, "linked_by": "actor"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s, want 201", rr.Code, rr.Body.String())
	}
	if got, _ := body["device_id"].(string); got != dev.ID {
		t.Errorf("device_id = %q, want %q", got, dev.ID)
	}
	if got, _ := body["project_id"].(string); got != prj.ID {
		t.Errorf("project_id = %q, want %q", got, prj.ID)
	}
}

// TestHandler_LinkDeviceProject_Conflict covers the
// composite unique invariant.
func TestHandler_LinkDeviceProject_Conflict(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	body1 := map[string]any{"project_id": prj.ID, "linked_by": "actor"}
	_, _ = doJSON(t, r, "POST", "/api/v1/devices/"+dev.ID+"/projects", body1)
	rr, body := doJSON(t, r, "POST", "/api/v1/devices/"+dev.ID+"/projects", body1)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%v, want 409", rr.Code, body)
	}
	errEnv, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %v", body)
	}
	if code, _ := errEnv["code"].(string); code != "CONFLICT" {
		t.Errorf("code = %q, want CONFLICT", code)
	}
}

// TestHandler_LinkDeviceProject_MissingDevice is the
// 404 boundary on link.
func TestHandler_LinkDeviceProject_MissingDevice(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	prj := seedProjectDB(t, db, "p1", nil)
	rr, body := doJSON(t, r, "POST", "/api/v1/devices/ghost/projects",
		map[string]any{"project_id": prj.ID, "linked_by": "actor"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%v, want 404", rr.Code, body)
	}
}

// TestHandler_LinkDeviceProject_MissingActor covers the
// validation rule.
func TestHandler_LinkDeviceProject_MissingActor(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	rr, _ := doJSON(t, r, "POST", "/api/v1/devices/"+dev.ID+"/projects",
		map[string]any{"project_id": prj.ID})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestHandler_UnlinkDeviceProject_204 covers the
// "Unlink host from project" spec scenario.
func TestHandler_UnlinkDeviceProject_204(t *testing.T) {
	r, db, svc := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	rr, _ := doJSON(t, r, "DELETE", "/api/v1/devices/"+dev.ID+"/projects/"+prj.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}

	rr, _ = doJSON(t, r, "DELETE", "/api/v1/devices/"+dev.ID+"/projects/"+prj.ID, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 on missing unlink", rr.Code)
	}
}

// TestHandler_BulkLink_201 covers the bulk link endpoint.
func TestHandler_BulkLink_201(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	p1 := seedProjectDB(t, db, "p1", nil)
	p2 := seedProjectDB(t, db, "p2", nil)

	rr, body := doJSON(t, r, "POST", "/api/v1/devices/"+dev.ID+"/projects/bulk",
		map[string]any{"project_ids": []string{p1.ID, p2.ID}, "linked_by": "actor"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s, want 201", rr.Code, rr.Body.String())
	}
	links, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(links) != 2 {
		t.Errorf("data len = %d, want 2", len(links))
	}
}

// TestHandler_ListProjectDevices_HierarchyWalk covers
// the "GET /api/v1/projects/:id/devices" endpoint
// returning devices linked to the project or any
// descendant.
func TestHandler_ListProjectDevices_HierarchyWalk(t *testing.T) {
	r, db, svc := newHandlerRig(t)
	dev := seedDeviceDB(t, db, "h1")
	bl := seedProjectDB(t, db, "bl", nil)
	sys := seedProjectDB(t, db, "sys", bl)
	prj := seedProjectDB(t, db, "prj", sys)

	if _, err := svc.Link(dev.ID, bl.ID, "actor"); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	for _, p := range []*projectpkg.Project{bl, sys, prj} {
		rr, body := doJSON(t, r, "GET", "/api/v1/projects/"+p.ID+"/devices", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("project %s: status = %d, want 200", p.Code, rr.Code)
		}
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatalf("project %s: data is not an array: %T", p.Code, body["data"])
		}
		if len(data) != 1 {
			t.Errorf("project %s: data len = %d, want 1", p.Code, len(data))
		}
	}
}

// TestHandler_ListProjectDevices_EmptyReturnsEmptyArray
// covers the boundary: an empty list returns data:[],
// not 404.
func TestHandler_ListProjectDevices_EmptyReturnsEmptyArray(t *testing.T) {
	r, db, _ := newHandlerRig(t)
	prj := seedProjectDB(t, db, "p1", nil)
	rr, body := doJSON(t, r, "GET", "/api/v1/projects/"+prj.ID+"/devices", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(data) != 0 {
		t.Errorf("data len = %d, want 0", len(data))
	}
}
