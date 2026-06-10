package physicalhost

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// handlerFixture builds a Gin engine wired to a real MaintenanceService
// + MonitorService + Repository over an in-memory sqlite DB. The
// Prober is the Fake. We do not exercise RBAC in these tests —
// the auth/rbac spec is its own phase and lives in
// internal/auth/rbac/.
func handlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openDB(t))
	prober := NewFake()
	mon := NewMonitorService(MonitorConfig{
		Repo:                repo,
		Prober:              prober,
		ConsecutiveFailures: 3,
		CheckInterval:       time.Minute,
	})
	maint := NewMaintenanceService(MaintenanceConfig{
		Repo:    repo,
		Auditor: &fakeAuditor{},
	})
	// Bridge: the monitor needs to delegate Enter/Exit to the
	// maintenance service so audit emission lives in one place.
	mon.SetMaintenance(maint)
	h := NewHandler(HandlerConfig{
		Repo:       repo,
		Monitor:    mon,
		Maintenance: maint,
	})
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1, rbac.NoopPermFactory())
	return r
}

// doRequest is a small helper that mirrors the device package's
// helper. Kept local because importing device would create a
// cycle (device is upstream of physicalhost, not the other way
// around, but the package is "leaf" and the test helper is
// trivial to duplicate).
func doPHRequest(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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

// TestHandler_List_EmptyReturnsEnvelope covers the spec's
// "list" requirement. A fresh DB renders data=[] and
// pagination.total=0.
func TestHandler_List_EmptyReturnsEnvelope(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "GET", "/api/v1/physical-hosts", nil)
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
	page, _ := body["pagination"].(map[string]any)
	if total, _ := page["total"].(float64); int(total) != 0 {
		t.Errorf("pagination.total = %v, want 0", page["total"])
	}
}

// TestHandler_List_FilterByState — the "List hosts in
// maintenance" scenario. GET /api/v1/physical-hosts?state=maintenance
// returns the hosts currently in maintenance, plus their reason
// and start time in the JSON.
func TestHandler_List_FilterByState(t *testing.T) {
	r := handlerFixture(t)
	// Seed: one online, one maintenance.
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d2", "ip_address": "10.0.0.2", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{
		"reason": "kernel upgrade",
	})

	rr, body := doPHRequest(t, r, "GET", "/api/v1/physical-hosts?state=maintenance", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
	if data[0].(map[string]any)["maintenance_reason"] != "kernel upgrade" {
		t.Errorf("reason = %v, want kernel upgrade", data[0].(map[string]any)["maintenance_reason"])
	}
}

// TestHandler_Get_OK returns a single host as the bare object.
func TestHandler_Get_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	rr, body := doPHRequest(t, r, "GET", "/api/v1/physical-hosts/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] != id {
		t.Errorf("id = %v, want %v", body["id"], id)
	}
	if body["ip_address"] != "10.0.0.1" {
		t.Errorf("ip_address = %v, want 10.0.0.1", body["ip_address"])
	}
}

// TestHandler_Get_NotFound renders a 404 envelope.
func TestHandler_Get_NotFound(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "GET", "/api/v1/physical-hosts/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Create_OK returns 201 with the created host.
func TestHandler_Create_OK(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("id is empty in response")
	}
	if body["state"] != string(StateOnline) {
		t.Errorf("state = %v, want online", body["state"])
	}
}

// TestHandler_Create_ValidationError returns 400 on a missing
// device_id.
func TestHandler_Create_ValidationError(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"ip_address": "10.0.0.1", "ssh_user": "root",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeValidation) {
		t.Errorf("code = %v, want VALIDATION_ERROR", errEnv["code"])
	}
}

// TestHandler_Put_OK replaces the host record.
func TestHandler_Put_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	rr, body := doPHRequest(t, r, "PUT", "/api/v1/physical-hosts/"+id, map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.99", "ssh_user": "root", "ssh_port": 2222,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["ip_address"] != "10.0.0.99" {
		t.Errorf("ip_address = %v, want 10.0.0.99", body["ip_address"])
	}
}

// TestHandler_Delete_OK returns 204 and hides the row.
func TestHandler_Delete_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	rr, _ := doPHRequest(t, r, "DELETE", "/api/v1/physical-hosts/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	rr2, _ := doPHRequest(t, r, "GET", "/api/v1/physical-hosts/"+id, nil)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("post-delete status = %d, want 404", rr2.Code)
	}
}

// TestHandler_Probe_OK runs a manual probe against a healthy host
// and reports the result inline in the response body.
func TestHandler_Probe_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/probe", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] != id {
		t.Errorf("id = %v, want %v", body["id"], id)
	}
}

// TestHandler_Maintenance_EnterExit covers the happy path through
// both endpoints.
func TestHandler_Maintenance_EnterExit(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)

	// Enter
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{
		"reason": "patching",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("enter status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["state"] != string(StateMaintenance) {
		t.Errorf("state = %v, want maintenance", body["state"])
	}
	if body["maintenance_reason"] != "patching" {
		t.Errorf("reason = %v, want patching", body["maintenance_reason"])
	}

	// Exit
	rr, body = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance/exit", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("exit status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["state"] != string(StateOnline) {
		t.Errorf("state = %v, want online", body["state"])
	}
}

// TestHandler_Maintenance_AlreadyMaintained returns 422.
func TestHandler_Maintenance_AlreadyMaintained(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{
		"reason": "first",
	})
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{
		"reason": "second",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInvalidState) {
		t.Errorf("code = %v, want INVALID_STATE", errEnv["code"])
	}
}

// TestHandler_Maintenance_ExitWhenNotInMaintenance returns 422.
func TestHandler_Maintenance_ExitWhenNotInMaintenance(t *testing.T) {
	r := handlerFixture(t)
	_, created := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	id, _ := created["id"].(string)
	rr, _ := doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance/exit", nil)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

// ensure strconv is used so the linter stays happy when no other
// test happens to need it.
var _ = strconv.Itoa
