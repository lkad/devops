package pipeline

import (
	"testing"
	"time"
)

// TestRepository_LastRunsForService pins the read path
// the service-catalog health rollup depends on: given a
// service_id, return up to N recent runs across all
// pipelines that deploy that service, newest first.
func TestRepository_LastRunsForService(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))
	svc := "svc-payments"

	// Two pipelines that deploy the same service.
	p1 := &Pipeline{ProjectID: "p1", Name: "deploy-p1", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s", Type: StepTypeShell}}, ServiceID: svc}
	p2 := &Pipeline{ProjectID: "p1", Name: "deploy-p2", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s", Type: StepTypeShell}}, ServiceID: svc}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if err := repo.CreatePipeline(p2); err != nil {
		t.Fatalf("create p2: %v", err)
	}

	// One unrelated pipeline.
	other := &Pipeline{ProjectID: "p1", Name: "other", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s", Type: StepTypeShell}}, ServiceID: "svc-other"}
	if err := repo.CreatePipeline(other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	now := time.Now()
	seed := func(p *Pipeline, status string, ago time.Duration) {
		run := &PipelineRun{
			PipelineID: p.ID,
			Status:     RunStatus(status),
			StartedAt:  ptrTime(now.Add(-ago)),
		}
		if err := repo.CreateRun(run); err != nil {
			t.Fatalf("seed run: %v", err)
		}
	}
	seed(p1, "succeeded", 10*time.Minute)
	seed(p1, "failed", 5*time.Minute)
	seed(p2, "succeeded", 2*time.Minute)
	seed(other, "succeeded", 1*time.Minute) // not in the service — must not show up

	runs, err := repo.LastRunsForService(svc, 10)
	if err != nil {
		t.Fatalf("LastRunsForService: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("got %d runs, want 3 (excluding the unrelated service)", len(runs))
	}
	// Newest first.
	if runs[0].PipelineID != p2.ID {
		t.Errorf("newest run should be from p2, got pipeline_id=%s", runs[0].PipelineID)
	}
	// Limit honored.
	runs, _ = repo.LastRunsForService(svc, 1)
	if len(runs) != 1 {
		t.Errorf("limit=1: got %d, want 1", len(runs))
	}
}
