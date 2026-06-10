package discovery

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// discoveryHandlerFixture builds a Gin engine with the
// discovery routes wired. We seed a few hosts via the fakes
// so the scan endpoints have something to return.
func discoveryHandlerFixture(t *testing.T, hosts []Host, results map[string]ProbeResult) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openDiscoveryWithDeviceDB(t)
	repo := NewRepository(db)
	devRepo := devicepkg.NewRepository(db)
	svc := NewService(repo, devRepo, NewFakeScanner(hosts, nil), NewFakeProber(results, nil))
	h := NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api, rbac.NoopPermFactory())
	return r
}

func doDiscRequest(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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

// TestHandler_CreateRun covers the POST /runs happy path.
// The body is {cidr, ports, snmp}; the response is 201 with
// the freshly-stored run.
func TestHandler_CreateRun(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "h1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	r := discoveryHandlerFixture(t, hosts, results)
	rr, body := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{
		"cidr":  "10.0.0.0/24",
		"ports": []int{22, 80},
		"snmp":  true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rr.Code, rr.Body.String())
	}
	if body["cidr"] != "10.0.0.0/24" {
		t.Errorf("cidr = %v, want 10.0.0.0/24", body["cidr"])
	}
	if body["status"] != string(RunStatusCompleted) {
		t.Errorf("status = %v, want completed", body["status"])
	}
}

// TestHandler_CreateRun_ValidationError covers the
// spec's "validation" path: missing cidr is a 400.
func TestHandler_CreateRun_ValidationError(t *testing.T) {
	r := discoveryHandlerFixture(t, nil, nil)
	rr, body := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{
		"ports": []int{22},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error body missing: %v", body)
	}
	if errObj["code"] != string(contracts.CodeValidation) {
		t.Errorf("error.code = %v, want VALIDATION_ERROR", errObj["code"])
	}
}

// TestHandler_ListRuns covers GET /runs. The response is the
// standard ListResponse envelope.
func TestHandler_ListRuns(t *testing.T) {
	r := discoveryHandlerFixture(t, nil, nil)
	rr, body := doDiscRequest(t, r, "GET", "/api/v1/discovery/runs", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(data) != 0 {
		t.Errorf("data = %d, want 0", len(data))
	}
	if _, ok := body["pagination"].(map[string]any); !ok {
		t.Errorf("pagination missing")
	}
}

// TestHandler_ListRuns_Pagination covers the limit query param.
func TestHandler_ListRuns_Pagination(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "h1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	r := discoveryHandlerFixture(t, hosts, results)
	for i := 0; i < 3; i++ {
		_, _ = doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{"cidr": "10.0.0.0/24"})
	}
	rr, body := doDiscRequest(t, r, "GET", "/api/v1/discovery/runs?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("data = %d, want 2", len(data))
	}
}

// TestHandler_GetRun covers GET /runs/:id. The response is
// the RunDetail (run + hosts).
func TestHandler_GetRun(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "h1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	r := discoveryHandlerFixture(t, hosts, results)
	_, body := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{"cidr": "10.0.0.0/24"})
	runID, _ := body["id"].(string)
	if runID == "" {
		t.Fatal("create did not return id")
	}
	rr, got := doDiscRequest(t, r, "GET", "/api/v1/discovery/runs/"+runID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	run, ok := got["run"].(map[string]any)
	if !ok {
		t.Fatalf("run missing: %v", got)
	}
	if run["id"] != runID {
		t.Errorf("run.id = %v, want %s", run["id"], runID)
	}
	hostsList, ok := got["hosts"].([]any)
	if !ok {
		t.Fatalf("hosts is not an array: %T", got["hosts"])
	}
	if len(hostsList) != 1 {
		t.Errorf("hosts = %d, want 1", len(hostsList))
	}
}

// TestHandler_GetRun_NotFound covers the 404 path.
func TestHandler_GetRun_NotFound(t *testing.T) {
	r := discoveryHandlerFixture(t, nil, nil)
	rr, _ := doDiscRequest(t, r, "GET", "/api/v1/discovery/runs/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// TestHandler_Promote covers the POST /runs/:id/promote
// happy path: an empty body promotes every host.
func TestHandler_Promote(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "h1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	r := discoveryHandlerFixture(t, hosts, results)
	_, body := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{"cidr": "10.0.0.0/24"})
	runID, _ := body["id"].(string)
	if runID == "" {
		t.Fatal("create did not return id")
	}
	rr, got := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs/"+runID+"/promote", map[string]any{})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	devices, ok := got["devices"].([]any)
	if !ok {
		t.Fatalf("devices is not an array: %T", got["devices"])
	}
	if len(devices) != 1 {
		t.Errorf("devices = %d, want 1", len(devices))
	}
}

// TestHandler_Promote_SelectedHosts covers the body shape
// `{host_ids: [...]}` for partial promote.
func TestHandler_Promote_SelectedHosts(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "h1"},
		{IPAddress: "10.0.0.2", Hostname: "h2"},
	}
	results := map[string]ProbeResult{
		"10.0.0.1": {Reachable: true},
		"10.0.0.2": {Reachable: true},
	}
	r := discoveryHandlerFixture(t, hosts, results)
	_, body := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs", map[string]any{"cidr": "10.0.0.0/24"})
	runID, _ := body["id"].(string)
	// Get the run detail to find a host_id.
	_, detail := doDiscRequest(t, r, "GET", "/api/v1/discovery/runs/"+runID, nil)
	hostsList, _ := detail["hosts"].([]any)
	if len(hostsList) != 2 {
		t.Fatalf("hosts = %d, want 2", len(hostsList))
	}
	firstHost, _ := hostsList[0].(map[string]any)
	hostID, _ := firstHost["id"].(string)
	if hostID == "" {
		t.Fatal("host id missing")
	}

	rr, got := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs/"+runID+"/promote",
		map[string]any{"host_ids": []string{hostID}})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	devices, _ := got["devices"].([]any)
	if len(devices) != 1 {
		t.Errorf("devices = %d, want 1", len(devices))
	}
}

// TestHandler_Promote_RunNotFound covers the 404 path on
// promote.
func TestHandler_Promote_RunNotFound(t *testing.T) {
	r := discoveryHandlerFixture(t, nil, nil)
	rr, _ := doDiscRequest(t, r, "POST", "/api/v1/discovery/runs/missing/promote", map[string]any{})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// _ = strconv keeps the import live for tests that may add
// numeric parsing in the future.
var _ = strconv.Atoi
