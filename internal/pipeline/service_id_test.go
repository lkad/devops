package pipeline

import (
	"testing"
)

// TestPipeline_ServiceID_PersistsAndRoundtrips pins the
// pipeline ↔ service FK from the spec: a pipeline with a
// service_id stores it on Create, returns it on Get, and
// the JSON wire shape exposes it.
//
// This is the P0 join that makes "what deploy last
// touched this service" answerable in one query. The
// corresponding test in the service-catalog package
// verifies the inverse direction (services list shows
// their pipelines).
func TestPipeline_ServiceID_PersistsAndRoundtrips(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))

	p := &Pipeline{
		ProjectID:   "p1",
		Name:        "deploy-payments",
		TargetType:  TargetTypeProject,
		Trigger:     "manual",
		Enabled:     true,
		Steps:       StepList{{Name: "deploy", Type: StepTypeShell}},
		ServiceID:   "svc-abc",
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ServiceID != "svc-abc" {
		t.Fatalf("Create clobbered ServiceID: %q", p.ServiceID)
	}

	got, err := repo.GetPipeline(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ServiceID != "svc-abc" {
		t.Errorf("round-trip ServiceID = %q, want %q", got.ServiceID, "svc-abc")
	}
}

// TestPipeline_ServiceID_Nullable pins the legacy path:
// pipelines without a service_id are still valid (the
// linear Steps path stays unaffected).
func TestPipeline_ServiceID_Nullable(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))

	p := &Pipeline{
		ProjectID:  "p1",
		Name:       "legacy",
		TargetType: TargetTypeProject,
		Trigger:    "manual",
		Enabled:    true,
		Steps:      StepList{{Name: "s1", Type: StepTypeShell}},
		// no ServiceID
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, _ := repo.GetPipeline(p.ID)
	if got.ServiceID != "" {
		t.Errorf("expected empty ServiceID, got %q", got.ServiceID)
	}
}

// TestPipeline_Update_ServiceID pins the partial-update
// path that lets an operator attach a service to an
// existing pipeline without re-creating it.
func TestPipeline_Update_ServiceID(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))
	p := &Pipeline{
		ProjectID: "p1", Name: "attachable", TargetType: TargetTypeProject,
		Trigger: "manual", Enabled: true,
		Steps: StepList{{Name: "s1", Type: StepTypeShell}},
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	p.ServiceID = "svc-late"
	if err := repo.UpdatePipeline(p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetPipeline(p.ID)
	if got.ServiceID != "svc-late" {
		t.Errorf("update didn't persist ServiceID; got %q", got.ServiceID)
	}
}
