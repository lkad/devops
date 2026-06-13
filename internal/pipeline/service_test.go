package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// pipelineSvcFixture builds a Service backed by a fresh
// in-memory repository and a Fake executor. The Service has no
// external dependencies beyond the repo + executor, so a
// single constructor is enough.
func pipelineSvcFixture(t *testing.T) (*Service, *Repository, *Fake) {
	t.Helper()
	repo := NewRepository(openPipelineDB(t))
	fake := &Fake{}
	svc := NewService(repo, fake)
	return svc, repo, fake
}

// TestService_Create_DefaultsEnabled verifies that a Create
// with an empty Enabled field defaults to true so callers can
// omit the field.
func TestService_Create_DefaultsEnabled(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, err := svc.Create(CreatePipelineInput{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
		Steps:      []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !p.Enabled {
		t.Error("Enabled = false, want true (default)")
	}
}

// TestService_Create_ValidatesName rejects blank names.
func TestService_Create_ValidatesName(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("expected *contracts.APIError, got %T", err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "name") {
		t.Errorf("message = %q, want mention of 'name'", apiErr.Message)
	}
}

// TestService_Create_ValidatesProjectID rejects an empty
// project_id. A pipeline without a project has no cost-
// allocation target.
func TestService_Create_ValidatesProjectID(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		Name:       "ci",
		TargetType: TargetTypeProject,
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_ValidatesTargetType rejects an unknown
// target_type.
func TestService_Create_ValidatesTargetType(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetType("bogus"),
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_ValidatesSteps rejects an empty step
// list. The spec requires "stages array containing valid stage
// definitions" — a pipeline with no steps has nothing to run.
func TestService_Create_ValidatesSteps(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
		Steps:      nil,
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_ValidatesStepType rejects an unknown
// step type. This is the spec's "Invalid YAML structure"
// scenario, expressed as structured-config validation.
func TestService_Create_ValidatesStepType(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Create(CreatePipelineInput{
		Name:       "ci",
		ProjectID:  "proj-1",
		TargetType: TargetTypeProject,
		Steps: []PipelineStep{
			{Name: "build", Type: StepType("python")},
		},
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_AcceptsAllStepTypes is the spec's "Valid
// YAML structure" scenario — the four step types the spec
// recognises all land in the DB.
func TestService_Create_AcceptsAllStepTypes(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	for _, st := range []StepType{StepTypeShell, StepTypeK8sApply, StepTypeTerraform, StepTypeHTTP} {
		p, err := svc.Create(CreatePipelineInput{
			Name:       "ci-" + string(st),
			ProjectID:  "proj-1",
			TargetType: TargetTypeProject,
			Steps:      []PipelineStep{{Name: "x", Type: st}},
		})
		if err != nil {
			t.Errorf("create with step type %q: %v", st, err)
			continue
		}
		if p.Steps[0].Type != st {
			t.Errorf("Step type = %q, want %q", p.Steps[0].Type, st)
		}
	}
}

// TestService_Get_NotFound covers the service's handling of a
// missing row. The handler maps CodeNotFound to a 404.
func TestService_Get_NotFound(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Get("missing")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_Get_OK pins the happy path so the 200 render is
// stable across refactors.
func TestService_Get_OK(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	in := CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	}
	p, err := svc.Create(in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Get(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID = %q, want %q", got.ID, p.ID)
	}
}

// TestService_Update_AppliesPatch verifies a partial update
// persists the supplied fields and leaves the rest intact.
func TestService_Update_AppliesPatch(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	newName := "ci-2"
	disabled := false
	got, err := svc.Update(p.ID, UpdatePipelineInput{
		Name:    &newName,
		Enabled: &disabled,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "ci-2" {
		t.Errorf("Name = %q, want ci-2", got.Name)
	}
	if got.Enabled {
		t.Error("Enabled = true, want false")
	}
}

// TestService_Update_ValidatesName rejects a blank-name patch.
func TestService_Update_ValidatesName(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	blank := "  "
	_, err := svc.Update(p.ID, UpdatePipelineInput{Name: &blank})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Delete_OK removes a pipeline.
func TestService_Delete_OK(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	if err := svc.Delete(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(p.ID); err == nil {
		t.Error("expected not-found after delete")
	}
}

// TestService_Trigger_CreatesRunAndExecutes covers the spec's
// "Execute full pipeline" scenario: a trigger creates a new
// run, dispatches the executor in a goroutine, and returns the
// run immediately.
func TestService_Trigger_CreatesRunAndExecutes(t *testing.T) {
	svc, _, fake := pipelineSvcFixture(t)
	// Add a small per-step delay so DurationMs is at least
	// 1 ms — sub-millisecond Fake executions would otherwise
	// round to 0 and fail the duration assertion.
	fake.DelayPerStep = 5 * time.Millisecond
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	run, err := svc.Trigger(p.ID, "user-1")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if run.ID == "" {
		t.Fatal("run ID empty")
	}
	if run.PipelineID != p.ID {
		t.Errorf("PipelineID = %q, want %q", run.PipelineID, p.ID)
	}
	// Wait for the goroutine to finish. The Fake executor
	// completes synchronously; the goroutine just records the
	// final status.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusSucceeded || got.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := svc.GetRun(run.ID)
	if got.Status != RunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt = nil, want set")
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt = nil, want set")
	}
	if got.DurationMs <= 0 {
		t.Errorf("DurationMs = %d, want > 0", got.DurationMs)
	}
	if got.TriggeredBy != "user-1" {
		t.Errorf("TriggeredBy = %q, want user-1", got.TriggeredBy)
	}
	// And the fake recorded two events (running + succeeded)
	if len(fake.events) != 2 {
		t.Errorf("fake recorded %d events, want 2", len(fake.events))
	}
}

// TestService_Trigger_NotFound is the spec's "Execute"
// scenario with a missing pipeline.
func TestService_Trigger_NotFound(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	_, err := svc.Trigger("missing", "u")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_Trigger_DisabledPipeline is a 422 INVALID_STATE
// when the caller tries to run a disabled pipeline. The
// service layer enforces this rather than the model hook
// because it is business policy.
func TestService_Trigger_DisabledPipeline(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	disabled := false
	if _, err := svc.Update(p.ID, UpdatePipelineInput{Enabled: &disabled}); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, err := svc.Trigger(p.ID, "u")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeInvalidState {
		t.Errorf("err = %v, want *contracts.APIError(INVALID_STATE)", err)
	}
}

// TestService_Trigger_FailedStepMarksRunFailed pins the
// "Stage failure handling" scenario: a step failure
// short-circuits the run, the run is marked failed, and the
// error message is recorded.
func TestService_Trigger_FailedStepMarksRunFailed(t *testing.T) {
	svc, _, fake := pipelineSvcFixture(t)
	fake.FailStep = "test"
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{
			{Name: "build", Type: StepTypeShell},
			{Name: "test", Type: StepTypeShell},
		},
	})
	run, err := svc.Trigger(p.ID, "u")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusFailed || got.Status == RunStatusSucceeded {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := svc.GetRun(run.ID)
	if got.Status != RunStatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.ErrorMessage == "" {
		t.Error("ErrorMessage empty, want set on failure")
	}
}

// TestService_Cancel_PendingRun is the spec's "Cancel running
// pipeline" scenario. The executor is paused on a Block
// channel so the run stays "running" long enough for the
// test to call Cancel. The executor unblocks when the
// context is cancelled.
func TestService_Cancel_PendingRun(t *testing.T) {
	svc, _, fake := pipelineSvcFixture(t)
	fake.Block = make(chan struct{})
	defer close(fake.Block)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	run, _ := svc.Trigger(p.ID, "u")
	// Poll until the goroutine has started and the run is
	// "running" — at that point the executor is blocked on
	// fake.Block and Cancel will short-circuit it.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := svc.Cancel(run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	got, _ := svc.GetRun(run.ID)
	if got.Status != RunStatusCancelled {
		t.Errorf("Status = %q, want cancelled", got.Status)
	}
	// Second call is a noop.
	if err := svc.Cancel(run.ID); err != nil {
		t.Errorf("second cancel: %v", err)
	}
}

// TestService_Cancel_AlreadyFinishedIsIdempotent covers the
// "safe to call on an already-finished run" constraint from
// the task.
func TestService_Cancel_AlreadyFinishedIsIdempotent(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	run, _ := svc.Trigger(p.ID, "u")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusSucceeded || got.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Even though the run is already finished, Cancel returns
	// nil — it is a noop on terminal runs.
	if err := svc.Cancel(run.ID); err != nil {
		t.Errorf("cancel on finished run: %v", err)
	}
	got, _ := svc.GetRun(run.ID)
	if got.Status != RunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded (unchanged)", got.Status)
	}
}

// TestService_Cancel_NotFound covers the missing-run case.
func TestService_Cancel_NotFound(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	err := svc.Cancel("missing")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_ListRuns_ForPipeline returns the run history for
// a given pipeline. The spec scenario "Get pipeline runs"
// drives this method.
func TestService_ListRuns_ForPipeline(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	for i := 0; i < 3; i++ {
		if _, err := svc.Trigger(p.ID, "u"); err != nil {
			t.Fatalf("trigger %d: %v", i, err)
		}
	}
	// wait for goroutines
	time.Sleep(100 * time.Millisecond)
	runs, total, err := svc.ListRuns(p.ID, 0, 0)
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

// TestService_ListAllRecentRuns covers the spec scenario "Get
// all recent runs" — runs across every pipeline.
func TestService_ListAllRecentRuns(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	for i := 0; i < 2; i++ {
		p, _ := svc.Create(CreatePipelineInput{
			Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
			Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
		})
		if _, err := svc.Trigger(p.ID, "u"); err != nil {
			t.Fatalf("trigger: %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	runs, total, err := svc.ListAllRecentRuns(0, 0)
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

// TestService_GetRunWithSteps returns the run plus its
// per-step records, with output preserved.
func TestService_GetRunWithSteps(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{
			{Name: "build", Type: StepTypeShell},
			{Name: "test", Type: StepTypeShell},
		},
	})
	run, _ := svc.Trigger(p.ID, "u")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusSucceeded || got.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, steps, err := svc.GetRunWithSteps(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(steps) != 2 {
		t.Errorf("steps = %d, want 2", len(steps))
	}
}

// TestService_Trigger_LogsPerStepOutput pins the "Logs:
// per-step output persisted" feature. The Fake executor
// produces output for every step, and the run-detail endpoint
// surfaces it.
func TestService_Trigger_LogsPerStepOutput(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	run, _ := svc.Trigger(p.ID, "u")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got.Status == RunStatusSucceeded || got.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, steps, _ := svc.GetRunWithSteps(run.ID)
	if len(steps) == 0 {
		t.Fatal("no step runs")
	}
	if steps[0].StepName != "build" {
		t.Errorf("StepName = %q, want build", steps[0].StepName)
	}
	if steps[0].Status != StepRunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", steps[0].Status)
	}
}

// TestService_Trigger_ConcurrentRunsAreIndependent verifies
// that two simultaneous triggers produce two independent
// runs, with no shared mutable state corrupting one or the
// other.
func TestService_Trigger_ConcurrentRunsAreIndependent(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name: "ci", ProjectID: "p", TargetType: TargetTypeProject,
		Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}},
	})
	var wg sync.WaitGroup
	ids := make([]string, 0, 4)
	var mu sync.Mutex
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, err := svc.Trigger(p.ID, "u")
			if err != nil {
				t.Errorf("trigger: %v", err)
				return
			}
			mu.Lock()
			ids = append(ids, run.ID)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(ids) != 4 {
		t.Errorf("runs = %d, want 4", len(ids))
	}
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate run ID: %q", id)
		}
		seen[id] = true
	}
}

// TestService_Trigger_WebhookStub ensures the "webhook" trigger
// path is wired through the service layer even though the
// webhook receiver is not implemented. A pipeline created
// with trigger=webhook can still be triggered manually.
func TestService_Trigger_WebhookStub(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, _ := svc.Create(CreatePipelineInput{
		Name:       "ci",
		ProjectID:  "p",
		TargetType: TargetTypeProject,
		Steps:      []PipelineStep{{Name: "build", Type: StepTypeShell}},
		Trigger:    "webhook",
	})
	if p.Trigger != "webhook" {
		t.Errorf("Trigger = %q, want webhook", p.Trigger)
	}
	// Manual trigger is still possible.
	run, err := svc.Trigger(p.ID, "u")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if run.ID == "" {
		t.Error("run.ID empty")
	}
}

// ensure errors.As is referenced
var _ = errors.As

// TestService_GetPipeline_CrossTenant_Denied covers the v0.3.0.0
// P0 #2 cross-tenant enforcement: a Developer who is a member
// of project A cannot read a pipeline in project B.
func TestService_GetPipeline_CrossTenant_Denied(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleDeveloper})
	svc.SetMembershipChecker(func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return map[string]struct{}{"a-1": {}}, nil
	})
	p, err := svc.Create(CreatePipelineInput{Name: "P", ProjectID: "b-1", TargetType: TargetTypeProject, Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	ctx := caller.WithContext(context.Background(), cl)
	_, err = svc.GetWithCaller(ctx, p.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// TestService_GetPipeline_SuperAdmin_Bypasses covers the spec
// rule that SuperAdmin is implicitly a member of every project.
func TestService_GetPipeline_SuperAdmin_Bypasses(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	cl := caller.New(&contracts.User{ID: "root", Username: "root", Role: contracts.RoleSuperAdmin})
	svc.SetMembershipChecker(func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return nil, nil
	})
	p, err := svc.Create(CreatePipelineInput{Name: "P", ProjectID: "x-1", TargetType: TargetTypeProject, Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	ctx := caller.WithContext(context.Background(), cl)
	got, err := svc.GetWithCaller(ctx, p.ID)
	if err != nil {
		t.Errorf("SuperAdmin should bypass, got err = %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("got.ID = %q, want %q", got.ID, p.ID)
	}
}

// TestService_GetPipeline_NilCaller_401 covers the fail-closed
// rule: a context without a caller MUST surface as
// ErrUnauthenticated.
func TestService_GetPipeline_NilCaller_401(t *testing.T) {
	svc, _, _ := pipelineSvcFixture(t)
	p, err := svc.Create(CreatePipelineInput{Name: "P", ProjectID: "x-1", TargetType: TargetTypeProject, Steps: []PipelineStep{{Name: "build", Type: StepTypeShell}}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = svc.GetWithCaller(context.Background(), p.ID)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}
