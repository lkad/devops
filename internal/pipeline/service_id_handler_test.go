package pipeline

import (
	"net/http"
	"testing"
)

// TestHandler_Create_AcceptsServiceID pins the wire
// shape: a POST /pipelines with a service_id stores the
// FK and the response body includes it.
func TestHandler_Create_AcceptsServiceID(t *testing.T) {
	r := pipelineHandlerFixture(t)
	rr, body := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"service_id":  "svc-abc",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%v", rr.Code, body)
	}
	if body["service_id"] != "svc-abc" {
		t.Errorf("service_id = %v, want svc-abc; body=%v", body["service_id"], body)
	}
}

// TestHandler_Get_IncludesServiceID pins the read path:
// the GET /pipelines/:id response includes service_id
// when the row has one.
func TestHandler_Get_IncludesServiceID(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"service_id":  "svc-abc",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body["service_id"] != "svc-abc" {
		t.Errorf("get service_id = %v, want svc-abc", body["service_id"])
	}
}

// TestHandler_Put_UpdatesServiceID pins the partial-update
// path that lets an operator attach a service to an
// existing pipeline.
func TestHandler_Put_UpdatesServiceID(t *testing.T) {
	r := pipelineHandlerFixture(t)
	_, created := doJSON(t, r, "POST", "/api/v1/pipelines", map[string]any{
		"name":        "ci",
		"project_id":  "proj-1",
		"target_type": "project",
		"steps":       []map[string]any{{"name": "build", "type": "shell"}},
	})
	id, _ := created["id"].(string)

	rr, _ := doJSON(t, r, "PUT", "/api/v1/pipelines/"+id, map[string]any{
		"service_id": "svc-late",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200", rr.Code)
	}
	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+id, nil)
	if body["service_id"] != "svc-late" {
		t.Errorf("after PUT, service_id = %v, want svc-late", body["service_id"])
	}
}
