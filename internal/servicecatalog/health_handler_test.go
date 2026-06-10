package servicecatalog

import (
	"net/http"
	"testing"
)

// TestHandler_Health_NoRuns_ReturnsUnknown pins the
// HTTP envelope for the unknown-state case. The handler
// builds a stub RunSource internally because the test
// does not have access to the real pipeline.Repository.
func TestHandler_Health_NoRuns_ReturnsUnknown(t *testing.T) {
	r, _ := newTestHandler(t)

	// Create a service to get an ID.
	w, body := doJSON(t, r, "POST", "/api/v1/services",
		`{"name":"svc-with-no-runs","tier":"standard"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	id, _ := body["id"].(string)

	w, hbody := doJSON(t, r, "GET", "/api/v1/services/"+id+"/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("health: %d %v", w.Code, hbody)
	}
	if hbody["status"] != "unknown" {
		t.Errorf("status = %v, want unknown", hbody["status"])
	}
	if hbody["service_id"] != id {
		t.Errorf("service_id = %v, want %s", hbody["service_id"], id)
	}
}

// TestHandler_Health_NotFound_Returns404 pins the
// 404 path for an unknown service ID.
func TestHandler_Health_NotFound_Returns404(t *testing.T) {
	r, _ := newTestHandler(t)
	w, _ := doJSON(t, r, "GET", "/api/v1/services/missing/health", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
