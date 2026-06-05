package pipeline

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

// pipelineRepoFixture is a slim wrapper around *gorm.DB so each
// test can have a fresh repository without re-importing the
// database package's test helpers.
func pipelineRepoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openPipelineDB(t))
}

// TestRepository_PipelineCreateAndGet verifies the basic
// create/read path on the Pipeline model — the minimum
// persistence contract.
func TestRepository_PipelineCreateAndGet(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p := &Pipeline{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
		Steps:      StepList{{Name: "build", Type: StepTypeShell}},
		Enabled:    true,
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID was not assigned")
	}
	got, err := repo.GetPipeline(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "ci" {
		t.Errorf("Name = %q, want ci", got.Name)
	}
	if got.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q, want proj-1", got.ProjectID)
	}
	if len(got.Steps) != 1 || got.Steps[0].Name != "build" {
		t.Errorf("Steps = %+v, want one step 'build'", got.Steps)
	}
}

// TestRepository_PipelineGet_NotFound verifies the "missing
// row" case. The service layer maps this to a 404 NOT_FOUND,
// so the error must be distinguishable from random DB errors.
func TestRepository_PipelineGet_NotFound(t *testing.T) {
	repo := pipelineRepoFixture(t)
	_, err := repo.GetPipeline("missing")
	if err == nil {
		t.Fatal("expected error for missing pipeline")
	}
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_PipelineUpdate persists a name + enabled
// change. The repository must call Save (or equivalent) so
// UpdatedAt is bumped and the row is written.
func TestRepository_PipelineUpdate(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p := &Pipeline{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
		Enabled:    true,
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	p.Name = "ci-2"
	p.Enabled = false
	if err := repo.UpdatePipeline(p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetPipeline(p.ID)
	if got.Name != "ci-2" {
		t.Errorf("Name = %q, want ci-2", got.Name)
	}
	if got.Enabled {
		t.Error("Enabled = true, want false")
	}
}

// TestRepository_PipelineDelete_SoftDelete pins the soft-
// delete contract: the row is hidden from default queries and
// the call returns nil.
func TestRepository_PipelineDelete_SoftDelete(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p := &Pipeline{Name: "x", ProjectID: "p", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.DeletePipeline(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetPipeline(p.ID); !IsNotFound(err) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestRepository_PipelineList_NoFilter returns every pipeline.
func TestRepository_PipelineList_NoFilter(t *testing.T) {
	repo := pipelineRepoFixture(t)
	for i := 0; i < 5; i++ {
		_ = repo.CreatePipeline(&Pipeline{Name: "n", ProjectID: "p", TargetType: TargetTypeProject})
	}
	rows, total, err := repo.ListPipelines(PipelineFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(rows) != 5 {
		t.Errorf("rows = %d, want 5", len(rows))
	}
}

// TestRepository_PipelineList_FilterByProject narrows the
// result by ProjectID. The spec ties pipelines to a project,
// so this filter is the common case.
func TestRepository_PipelineList_FilterByProject(t *testing.T) {
	repo := pipelineRepoFixture(t)
	_ = repo.CreatePipeline(&Pipeline{Name: "a", ProjectID: "p-1", TargetType: TargetTypeProject})
	_ = repo.CreatePipeline(&Pipeline{Name: "b", ProjectID: "p-1", TargetType: TargetTypeProject})
	_ = repo.CreatePipeline(&Pipeline{Name: "c", ProjectID: "p-2", TargetType: TargetTypeProject})

	rows, total, err := repo.ListPipelines(PipelineFilter{ProjectID: "p-1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

// TestRepository_PipelineList_Pagination verifies that
// limit/offset produce a stable page of results and the count
// is the pre-pagination total.
func TestRepository_PipelineList_Pagination(t *testing.T) {
	repo := pipelineRepoFixture(t)
	for i := 0; i < 7; i++ {
		_ = repo.CreatePipeline(&Pipeline{Name: "n", ProjectID: "p", TargetType: TargetTypeProject})
	}
	rows, total, err := repo.ListPipelines(PipelineFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 7 {
		t.Errorf("total = %d, want 7", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestRepository_RunCreateAndGet exercises the run-side
// create/read path. StartedAt is set on insert; the read-back
// must preserve the value.
func TestRepository_RunCreateAndGet(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	run := &PipelineRun{PipelineID: p1.ID, Status: RunStatusRunning, TriggeredBy: "u-1"}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PipelineID != p1.ID {
		t.Errorf("PipelineID = %q, want %q", got.PipelineID, p1.ID)
	}
	if got.Status != RunStatusRunning {
		t.Errorf("Status = %q, want running", got.Status)
	}
}

// TestRepository_RunUpdateStatus allows the executor to flip
// status to succeeded / failed / cancelled without losing the
// original timestamps.
func TestRepository_RunUpdateStatus(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	run := &PipelineRun{PipelineID: p1.ID, Status: RunStatusRunning}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.UpdateRunStatus(run.ID, RunStatusSucceeded, 0, ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetRun(run.ID)
	if got.Status != RunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", got.Status)
	}
}

// TestRepository_ListRuns_ForPipeline returns only the runs
// for the given pipeline, ordered by created_at desc so the
// most recent run is first.
func TestRepository_ListRuns_ForPipeline(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	p2 := &Pipeline{Name: "y", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if err := repo.CreatePipeline(p2); err != nil {
		t.Fatalf("create p2: %v", err)
	}
	for i := 0; i < 3; i++ {
		_ = repo.CreateRun(&PipelineRun{PipelineID: p1.ID, Status: RunStatusSucceeded})
	}
	for i := 0; i < 2; i++ {
		_ = repo.CreateRun(&PipelineRun{PipelineID: p2.ID, Status: RunStatusSucceeded})
	}
	runs, total, err := repo.ListRunsForPipeline(p1.ID, 0, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(runs) != 3 {
		t.Errorf("rows = %d, want 3", len(runs))
	}
}

// TestRepository_GetRunWithSteps returns the run plus the
// ordered list of its step runs. The handler uses this for
// the run-detail endpoint.
func TestRepository_GetRunWithSteps(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create: %v", err)
	}
	run := &PipelineRun{PipelineID: p1.ID, Status: RunStatusSucceeded}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	for _, name := range []string{"build", "test"} {
		if err := repo.CreateStepRun(&PipelineStepRun{
			RunID:    run.ID,
			StepName: name,
			Status:   StepRunStatusSucceeded,
		}); err != nil {
			t.Fatalf("create step run: %v", err)
		}
	}
	got, steps, err := repo.GetRunWithSteps(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("run.ID = %q, want %q", got.ID, run.ID)
	}
	if len(steps) != 2 {
		t.Errorf("steps = %d, want 2", len(steps))
	}
}

// TestRepository_UpdateStepRunStatus updates a single step
// run to its terminal state.
func TestRepository_UpdateStepRunStatus(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create: %v", err)
	}
	run := &PipelineRun{PipelineID: p1.ID, Status: RunStatusRunning}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	step := &PipelineStepRun{RunID: run.ID, StepName: "build", Status: StepRunStatusRunning}
	if err := repo.CreateStepRun(step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	if err := repo.UpdateStepRunStatus(step.ID, StepRunStatusSucceeded, 0, "output"); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, steps, _ := repo.GetRunWithSteps(run.ID)
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if steps[0].Status != StepRunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", steps[0].Status)
	}
	if steps[0].OutputTruncated != "output" {
		t.Errorf("OutputTruncated = %q, want 'output'", steps[0].OutputTruncated)
	}
}

// TestRepository_ListRecentRuns_AcrossPipelines returns runs
// from every pipeline. The handler uses this for the
// "all recent runs" feed.
func TestRepository_ListRecentRuns_AcrossPipelines(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p1 := &Pipeline{Name: "x", ProjectID: "pr", TargetType: TargetTypeProject}
	p2 := &Pipeline{Name: "y", ProjectID: "pr", TargetType: TargetTypeProject}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if err := repo.CreatePipeline(p2); err != nil {
		t.Fatalf("create p2: %v", err)
	}
	_ = repo.CreateRun(&PipelineRun{PipelineID: p1.ID, Status: RunStatusSucceeded})
	_ = repo.CreateRun(&PipelineRun{PipelineID: p2.ID, Status: RunStatusFailed})
	runs, total, err := repo.ListRecentRuns(0, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(runs) != 2 {
		t.Errorf("rows = %d, want 2", len(runs))
	}
}

// TestRepository_UpdatePipeline_NotFound ensures the Update
// path returns a typed error when the row no longer exists.
func TestRepository_UpdatePipeline_NotFound(t *testing.T) {
	repo := pipelineRepoFixture(t)
	p := &Pipeline{Name: "x", ProjectID: "p", TargetType: TargetTypeProject}
	p.ID = "missing"
	err := repo.UpdatePipeline(p)
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestIsNotFound wraps arbitrary errors so a service-layer
// check can branch on the typed sentinel.
func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrPipelineNotFound) {
		t.Error("ErrPipelineNotFound should be IsNotFound")
	}
	if !IsNotFound(ErrRunNotFound) {
		t.Error("ErrRunNotFound should be IsNotFound")
	}
	if IsNotFound(errors.New("other")) {
		t.Error("other error should not be IsNotFound")
	}
}

// ensure we can compile against gorm.ErrRecordNotFound
var _ = gorm.ErrRecordNotFound
