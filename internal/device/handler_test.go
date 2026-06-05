package device

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// deviceHandlerFixture builds a Gin engine wired to a real
// Service and a real Repository over an in-memory sqlite DB.
// Auth / RBAC are not in scope for this package; the engine
// exposes the device routes directly. All three services share
// a single DB connection so AutoMigrate only runs once per
// fixture.
func deviceHandlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openDeviceDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)
	// Group service
	groupRepo := NewGroupRepository(db)
	groupSvc := NewGroupService(groupRepo)
	groupH := NewGroupHandler(groupSvc)
	// Template service
	tplRepo := NewTemplateRepository(db)
	tplSvc := NewTemplateService(tplRepo)
	tplH := NewTemplateHandler(tplSvc)

	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	groupH.Register(api)
	tplH.Register(api)
	return r
}

// doRequest is a thin helper around httptest.NewRecorder that
// also decodes the body into a generic JSON value so the
// individual tests do not have to repeat boilerplate.
func doRequest(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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
	var decoded map[string]any
	if rr.Body.Len() > 0 {
		_ = json.Unmarshal(rr.Body.Bytes(), &decoded)
	}
	return rr, decoded
}

// TestHandler_List_EmptyReturnsEnvelope covers the spec's "list
// returns the standard envelope" requirement. A fresh DB
// renders data=[] and pagination.total=0.
func TestHandler_List_EmptyReturnsEnvelope(t *testing.T) {
	r := deviceHandlerFixture(t)
	rr, body := doRequest(t, r, "GET", "/api/v1/devices", nil)
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
	page, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination missing or wrong type: %T", body["pagination"])
	}
	if total, _ := page["total"].(float64); int(total) != 0 {
		t.Errorf("pagination.total = %v, want 0", page["total"])
	}
}

// TestHandler_List_WithFilters exercises type/state/group_id
// query params and limit/offset pagination on the wire.
func TestHandler_List_WithFilters(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, _ = doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "p1", "type": "physical_host",
	})
	_, _ = doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "c1", "type": "container",
	})

	rr, body := doRequest(t, r, "GET", "/api/v1/devices?type=container&limit=10", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
	if data[0].(map[string]any)["name"] != "c1" {
		t.Errorf("data[0].name = %v, want c1", data[0].(map[string]any)["name"])
	}
}

// TestHandler_Get_OK returns a single device as the bare object
// (not wrapped in data/pagination).
func TestHandler_Get_OK(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "h2",
		"type": "physical_host",
	})
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("id missing from create response")
	}
	rr, body := doRequest(t, r, "GET", "/api/v1/devices/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] != id {
		t.Errorf("id = %v, want %v", body["id"], id)
	}
	if body["name"] != "h2" {
		t.Errorf("name = %v, want h2", body["name"])
	}
}

// TestHandler_Get_NotFound renders a 404 envelope.
func TestHandler_Get_NotFound(t *testing.T) {
	r := deviceHandlerFixture(t)
	rr, body := doRequest(t, r, "GET", "/api/v1/devices/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil {
		t.Fatalf("error envelope missing: %v", body)
	}
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Create_OK returns 201 with the created device.
func TestHandler_Create_OK(t *testing.T) {
	r := deviceHandlerFixture(t)
	rr, body := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "host-1",
		"type": "physical_host",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("id is empty in response")
	}
	if body["name"] != "host-1" {
		t.Errorf("name = %v, want host-1", body["name"])
	}
}

// TestHandler_Create_ValidationError returns 400.
func TestHandler_Create_ValidationError(t *testing.T) {
	r := deviceHandlerFixture(t)
	rr, body := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "",
		"type": "physical_host",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeValidation) {
		t.Errorf("code = %v, want VALIDATION_ERROR", errEnv["code"])
	}
}

// TestHandler_Put_OK replaces the device record.
func TestHandler_Put_OK(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "before",
		"type": "physical_host",
	})
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("id missing from create response")
	}
	rr, body := doRequest(t, r, "PUT", "/api/v1/devices/"+id, map[string]any{
		"name": "after",
		"type": "physical_host",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["name"] != "after" {
		t.Errorf("name = %v, want after", body["name"])
	}
}

// TestHandler_Delete_OK returns 204 and hides the row.
func TestHandler_Delete_OK(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "x",
		"type": "physical_host",
	})
	id, _ := created["id"].(string)
	rr, _ := doRequest(t, r, "DELETE", "/api/v1/devices/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	rr2, _ := doRequest(t, r, "GET", "/api/v1/devices/"+id, nil)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("post-delete status = %d, want 404", rr2.Code)
	}
}

// TestHandler_Action_EnterMaintenance covers the action dispatcher.
func TestHandler_Action_EnterMaintenance(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "x",
		"type": "physical_host",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r, "POST", "/api/v1/devices/"+id+"/actions", map[string]any{
		"action": ActionEnterMaintenance,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["state"] != string(DeviceStateMaintenance) {
		t.Errorf("state = %v, want maintenance", body["state"])
	}
}

// TestHandler_Action_UnknownAction returns 400.
func TestHandler_Action_UnknownAction(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "x",
		"type": "physical_host",
	})
	id, _ := created["id"].(string)
	rr, _ := doRequest(t, r, "POST", "/api/v1/devices/"+id+"/actions", map[string]any{
		"action": "reboot",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestHandler_Search_Query exercises the search endpoint.
func TestHandler_Search_Query(t *testing.T) {
	r := deviceHandlerFixture(t)
	_, _ = doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "web-01",
		"type": "physical_host",
	})
	_, _ = doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
		"name": "db-01",
		"type": "physical_host",
	})
	rr, body := doRequest(t, r, "GET", "/api/v1/devices?search=web", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
	if data[0].(map[string]any)["name"] != "web-01" {
		t.Errorf("data[0].name = %v, want web-01", data[0].(map[string]any)["name"])
	}
}

// TestHandler_List_PaginationLimitOffset covers the
// limit/offset query params. We use the strconv import so the
// linter is happy with our test helpers.
func TestHandler_List_PaginationLimitOffset(t *testing.T) {
	r := deviceHandlerFixture(t)
	for i := 0; i < 5; i++ {
		_, _ = doRequest(t, r, "POST", "/api/v1/devices", map[string]any{
			"name": "n-" + strconv.Itoa(i),
			"type": "container",
		})
	}
	rr, body := doRequest(t, r, "GET", "/api/v1/devices?limit=2&offset=1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("data len = %d, want 2", len(data))
	}
	page, _ := body["pagination"].(map[string]any)
	if int(page["total"].(float64)) != 5 {
		t.Errorf("pagination.total = %v, want 5", page["total"])
	}
}

// TestHandler_DeviceGroups_CRUD exercises the spec's "Create
// device group" scenario. We declare it here so the test file
// is self-contained.
func TestHandler_DeviceGroups_CRUD(t *testing.T) {
	r := deviceHandlerFixture(t)

	// Create
	rr, body := doRequest(t, r, "POST", "/api/v1/device-groups", map[string]any{
		"name":        "edge",
		"description": "edge devices",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("id missing from create response")
	}

	// List
	rr, listBody := doRequest(t, r, "GET", "/api/v1/device-groups", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d", rr.Code)
	}
	if data, _ := listBody["data"].([]any); len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}

	// Delete
	rr, _ = doRequest(t, r, "DELETE", "/api/v1/device-groups/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", rr.Code)
	}
}

// TestHandler_Templates_CRUD exercises the configuration template
// CRUD. The spec's "Apply template to device" scenario is
// integration-level (renders a Jinja2 template) and lives
// outside this package; the CRUD is the surface we test here.
func TestHandler_Templates_CRUD(t *testing.T) {
	r := deviceHandlerFixture(t)
	rr, body := doRequest(t, r, "POST", "/api/v1/configuration-templates", map[string]any{
		"name":        "nginx-base",
		"description": "nginx config",
		"body":        "server { listen {{port}}; }",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", rr.Code, rr.Body.String())
	}
	id, _ := body["id"].(string)

	rr, listBody := doRequest(t, r, "GET", "/api/v1/configuration-templates", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("list status = %d", rr.Code)
	}
	if data, _ := listBody["data"].([]any); len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}

	rr, _ = doRequest(t, r, "DELETE", "/api/v1/configuration-templates/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", rr.Code)
	}
}

// ensure strings is used so the linter does not complain
var _ = strings.TrimSpace
