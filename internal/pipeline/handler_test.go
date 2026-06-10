package pipeline

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// pipelineHandlerFixture builds a Gin engine wired to a real
// Service and a real Repository over an in-memory sqlite DB,
// with a Fake executor. The engine exposes the pipeline
// routes directly; auth / RBAC is tested in a separate layer
// where it can be exercised against rbac.RequirePermission.
//
// The service-catalog validator is wired with a permissive
// pass-through (any non-empty service_id is accepted) so
// legacy / unit tests that predate the catalog keep
// working. The catalog FK rule is exercised in its own
// test (TestService_Create_RejectsUnknownServiceID).
func pipelineHandlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openPipelineDB(t)
	repo := NewRepository(db)
	svc := NewService(repo, &Fake{}).
		WithServiceValidator(func(string) error { return nil })
	h := NewHandler(svc)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r
}

// pipelineHandlerFixtureWithFake is the same as
// pipelineHandlerFixture but also returns the Fake executor
// so tests that need to control execution timing (e.g. cancel
// during a run) can configure the Fake.Block channel.
func pipelineHandlerFixtureWithFake(t *testing.T) (*gin.Engine, *Fake) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openPipelineDB(t)
	repo := NewRepository(db)
	fake := &Fake{}
	svc := NewService(repo, fake).
		WithServiceValidator(func(string) error { return nil })
	h := NewHandler(svc)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r, fake
}

// doJSON is a thin helper around httptest.NewRecorder that
// marshals body as JSON, sends the request, and decodes the
// response into a generic map so individual tests do not
// repeat boilerplate.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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

// TestHandler_Create_201 is the spec's "Create pipeline"
// scenario. A 201 is returned with the freshly-stored
// pipeline (id, name, etc).
func TestHandler_Create_201(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps": []map[string]any{
			{"name": "build", "type": "shell"},
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("id is empty: %v", body)
	}
	if body["name"] != "ci" {
		t.Errorf("name = %v, want ci", body["name"])
	}
}

// TestHandler_Create_ValidationError maps to the spec's
// "Invalid YAML structure" scenario. An empty name returns a
// 400 VALIDATION_ERROR envelope.
func TestHandler_Create_ValidationError(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil {
		t.Fatalf("error envelope missing: %v", body)
	}
	if errEnv["code"] != string(contracts.CodeValidation) {
		t.Errorf("code = %v, want VALIDATION_ERROR", errEnv["code"])
	}
}

// TestHandler_List_EmptyReturnsEnvelope covers the spec's
// "List pipelines" scenario. A fresh DB renders data=[] and
// pagination.total=0.
func TestHandler_List_EmptyReturnsEnvelope(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines", nil)
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
		t.Fatalf("pagination missing: %T", body["pagination"])
	}
	if total, _ := page["total"].(float64); int(total) != 0 {
		t.Errorf("pagination.total = %v, want 0", page["total"])
	}
}

// TestHandler_Get_OK returns a single pipeline as the bare
// object (not wrapped in data/pagination). This is the
// "Get pipeline by ID" scenario.
func TestHandler_Get_OK(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] != id {
		t.Errorf("id = %v, want %v", body["id"], id)
	}
	// Full pipeline definition including stages (steps).
	steps, _ := body["steps"].([]any)
	if len(steps) == 0 {
		t.Errorf("expected at least one step, got %v", body["steps"])
	}
}

// TestHandler_Get_NotFound renders a 404 envelope.
func TestHandler_Get_NotFound(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil {
		t.Fatalf("error envelope missing")
	}
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Put_OK covers the "Update pipeline" scenario. A
// partial update persists the supplied fields.
func TestHandler_Put_OK(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	rr, body := doJSON(t, r, "PUT", "/api/v1/pipelines/"+id, map[string]any{
		"name": "ci-2",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["name"] != "ci-2" {
		t.Errorf("name = %v, want ci-2", body["name"])
	}
}

// TestHandler_Delete_204 covers the "Delete pipeline"
// scenario. A 204 is returned on success; a 404 on missing
// rows.
func TestHandler_Delete_204(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	rr, _ := doJSON(t, r, "DELETE", "/api/v1/pipelines/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	rr2, _ := doJSON(t, r, "GET", "/api/v1/pipelines/"+id, nil)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("post-delete status = %d, want 404", rr2.Code)
	}
}

// TestHandler_Trigger_201 covers the spec's "Execute full
// pipeline" scenario. A trigger creates a new run and returns
// the run object.
func TestHandler_Trigger_201(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	rr, body := doJSON(t, r, "POST", "/api/v1/pipelines/"+id+"/trigger", nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("run id is empty: %v", body)
	}
	if body["pipeline_id"] != id {
		t.Errorf("pipeline_id = %v, want %v", body["pipeline_id"], id)
	}
}

// TestHandler_ListRuns_ForPipeline covers the spec's "Get
// pipeline runs" scenario. The endpoint returns the run
// history for a given pipeline.
func TestHandler_ListRuns_ForPipeline(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	// Two triggers
	for i := 0; i < 2; i++ {
		_, _ = doJSON(t, r, "POST", "/api/v1/pipelines/"+id+"/trigger", nil)
	}
	// Wait for both runs to finish.
	time.Sleep(200 * time.Millisecond)
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+id+"/runs", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("data len = %d, want 2", len(data))
	}
	page, _ := body["pagination"].(map[string]any)
	if total, _ := page["total"].(float64); int(total) != 2 {
		t.Errorf("pagination.total = %v, want 2", page["total"])
	}
}

// TestHandler_GetRun_WithSteps covers the spec's "Get all
// recent runs" scenario, re-shaped as a single-run detail
// endpoint. The response includes the per-step records.
func TestHandler_GetRun_WithSteps(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	pid, _ := created["id"].(string)
	_, trig := doJSON(t, r, "POST", "/api/v1/pipelines/"+pid+"/trigger", nil)
	rid, _ := trig["id"].(string)
	time.Sleep(200 * time.Millisecond)
	rr, body := doJSON(t, r, "GET", "/api/v1/runs/"+rid, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] != rid {
		t.Errorf("id = %v, want %v", body["id"], rid)
	}
	steps, _ := body["steps"].([]any)
	if len(steps) == 0 {
		t.Errorf("expected at least one step run, got %v", body["steps"])
	}
}

// TestHandler_Cancel_200 covers the spec's "Cancel running
// pipeline" scenario. A 200 is returned with the updated run.
// The Fake executor is blocked on a channel so the run stays
// "running" while the cancel endpoint is called.
func TestHandler_Cancel_200(t *testing.T) {
	r, fake := pipelineHandlerFixtureWithFake(t)
	fake.Block = make(chan struct{})
	defer close(fake.Block)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	pid, _ := created["id"].(string)
	_, trig := doJSON(t, r, "POST", "/api/v1/pipelines/"+pid+"/trigger", nil)
	rid, _ := trig["id"].(string)
	// Poll until the run is "running" — the goroutine has
	// called UpdateRunStartedAt and the executor is blocked
	// on fake.Block.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr, body := doJSON(t, r, "GET", "/api/v1/runs/"+rid, nil)
		if rr.Code == http.StatusOK && body["status"] == string(RunStatusRunning) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	rr, body := doJSON(t, r, "POST", "/api/v1/runs/"+rid+"/cancel", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["status"] != string(RunStatusCancelled) {
		t.Errorf("status = %v, want cancelled", body["status"])
	}
}

// TestHandler_Cancel_Idempotent verifies the spec invariant
// that cancellation must be safe to call on an already-
// finished run. We let the run complete before the first
// cancel, then call cancel a second time.
func TestHandler_Cancel_Idempotent(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	pid, _ := created["id"].(string)
	_, trig := doJSON(t, r, "POST", "/api/v1/pipelines/"+pid+"/trigger", nil)
	rid, _ := trig["id"].(string)
	// Wait for the run to finish.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr, body := doJSON(t, r, "GET", "/api/v1/runs/"+rid, nil)
		if rr.Code == http.StatusOK && (body["status"] == string(RunStatusSucceeded) || body["status"] == string(RunStatusFailed)) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// First cancel — runs on a finished run.
	rr1, _ := doJSON(t, r, "POST", "/api/v1/runs/"+rid+"/cancel", nil)
	if rr1.Code != http.StatusOK {
		t.Errorf("first cancel status = %d, want 200", rr1.Code)
	}
	// Second cancel — must still be 200.
	rr2, _ := doJSON(t, r, "POST", "/api/v1/runs/"+rid+"/cancel", nil)
	if rr2.Code != http.StatusOK {
		t.Errorf("second cancel status = %d, want 200", rr2.Code)
	}
}

// TestHandler_GetRun_NotFound renders a 404 envelope.
func TestHandler_GetRun_NotFound(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "GET", "/api/v1/runs/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil {
		t.Fatalf("error envelope missing")
	}
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Trigger_NotFound is the 404 path of the trigger
// endpoint.
func TestHandler_Trigger_NotFound(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, _ := doJSON(t, r, "POST", "/api/v1/pipelines/missing/trigger", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}
