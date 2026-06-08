// Boundary / edge-case tests for the pipeline service.
//
// Covers the gaps around trigger/cancel semantics and step
// validation that the original TDD pass didn't pin explicitly.
package pipeline

import (
	"testing"
	"time"
)

// ─── trigger edge cases ────────────────────────────────────────

func TestBoundary_Trigger_OnCompletedRunIsIdempotent(t *testing.T) {
	// A user might re-trigger a pipeline that already has a
	// terminal-state run. The service should create a NEW run,
	// not refuse. (This is what users expect: a re-run is not
	// the same as resuming a failed run.)
	svc, _, _ := pipelineSvcFixture(t)
	p, err := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	first, err := svc.Trigger(p.ID, "admin")
	if err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	// Wait for it to finish.
	deadline := time.Now().Add(2 * time.Second)
	for first.Status == "pending" || first.Status == "running" {
		if time.Now().After(deadline) {
			t.Fatalf("run did not finish in 2s: status=%s", first.Status)
		}
		time.Sleep(10 * time.Millisecond)
		first, _ = svc.GetRun(first.ID)
	}
	// Second trigger should succeed and produce a new run id.
	second, err := svc.Trigger(p.ID, "admin")
	if err != nil {
		t.Errorf("second trigger should succeed, got %v", err)
	}
	if second.ID == first.ID {
		t.Errorf("re-trigger must produce a new run, got same id %s", first.ID)
	}
}

func TestBoundary_Trigger_OnMissingPipelineIsNotFound(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Trigger("does-not-exist", "admin")
	if err == nil {
		t.Errorf("missing pipeline should 404")
	}
}

func TestBoundary_Cancel_OnMissingRunIsNotFound(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	err := svc.Cancel("does-not-exist")
	if err == nil {
		t.Errorf("missing run should 404 on cancel")
	}
}

// ─── create edge cases ─────────────────────────────────────────

func TestBoundary_Create_WithSingleStepIsAllowed(t *testing.T) {
	// A trivial "echo" pipeline with a single shell step is a
	// legitimate sanity-check workflow. Rejecting it would force
	// users to add a no-op second step.
	svc, _, _ := pipelineSvcFixture(t)
	p, err := svc.Create(CreatePipelineInput{
		Name: "echo", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "say-hi", Type: StepTypeShell, Config: map[string]any{"cmd": "echo hi"}}},
	})
	if err != nil {
		t.Errorf("single-step pipeline should be accepted, got %v", err)
	}
	if len(p.Steps) != 1 {
		t.Errorf("steps = %d, want 1", len(p.Steps))
	}
}

func TestBoundary_Create_WithVeryLongNameIsAccepted(t *testing.T) {
	// No name length cap today. Pin the behaviour — if a cap is
	// added later, this test will start failing and the team can
	// decide on the right value.
	svc, _, _ := pipelineSvcFixture(t)
	longName := ""
	for i := 0; i < 500; i++ {
		longName += "n"
	}
	p, err := svc.Create(CreatePipelineInput{
		Name: longName, ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	if err != nil {
		t.Errorf("500-char name should be accepted, got %v", err)
	}
	if p.Name != longName {
		t.Errorf("name was not round-tripped verbatim")
	}
}

func TestBoundary_Create_WithNoProjectIDIsRejected(t *testing.T) {
	// project_id is required for any pipeline — even project-
	// scoped pipelines need to know which project to charge.
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		Name: "orphan", ProjectID: "", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	if err == nil {
		t.Errorf("empty project_id must be rejected")
	}
}
