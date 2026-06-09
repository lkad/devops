package pipeline

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// sharedFixture returns a router + repo backed by the same
// sqlite DB. The standard pipelineHandlerFixture and
// pipelineSvcFixture each open a fresh DB, so a test that
// wants the handler to see what the repo wrote must use
// this one instead.
func sharedFixture(t *testing.T) (*gin.Engine, *Repository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openPipelineDB(t)
	repo := NewRepository(db)
	svc := NewService(repo, &Fake{})
	h := NewHandler(svc)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r, repo
}

// TestService_Stats_Empty covers the cold-start path: a fresh
// pipeline with no runs reports zero counts and an empty
// recent-runs list.
func TestService_Stats_Empty(t *testing.T) {
	svc, repo, _ := pipelineSvcFixture(t)
	p := &Pipeline{ProjectID: "p1", Name: "stats-empty", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s1", Type: StepTypeShell}}}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	stats, err := svc.Stats(p.ID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.PipelineID != p.ID {
		t.Errorf("PipelineID = %q, want %q", stats.PipelineID, p.ID)
	}
	if stats.TotalRuns != 0 {
		t.Errorf("TotalRuns = %d, want 0", stats.TotalRuns)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", stats.SuccessRate)
	}
	if stats.AverageDurationMs != 0 {
		t.Errorf("AverageDurationMs = %d, want 0", stats.AverageDurationMs)
	}
	if len(stats.RecentRuns) != 0 {
		t.Errorf("RecentRuns len = %d, want 0", len(stats.RecentRuns))
	}
}

// TestService_Stats_MixedOutcomes: 3 runs (2 succeeded, 1
// failed) report success rate ~0.667 and recent runs newest-
// first.
func TestService_Stats_MixedOutcomes(t *testing.T) {
	svc, repo, _ := pipelineSvcFixture(t)
	p := &Pipeline{ProjectID: "p1", Name: "stats-mixed", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s1", Type: StepTypeShell}}}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	now := time.Now().UTC()
	mkRun := func(status RunStatus, dur time.Duration, startedAt time.Time) {
		finished := startedAt.Add(dur)
		run := &PipelineRun{
			PipelineID:  p.ID,
			Status:      status,
			StartedAt:   &startedAt,
			FinishedAt:  &finished,
			DurationMs:  dur.Milliseconds(),
			TriggeredBy: "test",
		}
		if err := repo.CreateRun(run); err != nil {
			t.Fatalf("create run: %v", err)
		}
	}
	mkRun(RunStatusSucceeded, 100*time.Millisecond, now.Add(-30*time.Minute))
	mkRun(RunStatusSucceeded, 200*time.Millisecond, now.Add(-20*time.Minute))
	mkRun(RunStatusFailed, 50*time.Millisecond, now.Add(-10*time.Minute))

	stats, err := svc.Stats(p.ID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", stats.TotalRuns)
	}
	if stats.SuccessfulRuns != 2 {
		t.Errorf("SuccessfulRuns = %d, want 2", stats.SuccessfulRuns)
	}
	if stats.FailedRuns != 1 {
		t.Errorf("FailedRuns = %d, want 1", stats.FailedRuns)
	}
	if stats.SuccessRate < 0.66 || stats.SuccessRate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.667", stats.SuccessRate)
	}
	// (100+200+50)/3 = 116ms
	if stats.AverageDurationMs < 110 || stats.AverageDurationMs > 120 {
		t.Errorf("AverageDurationMs = %d, want ~116", stats.AverageDurationMs)
	}
	if len(stats.RecentRuns) != 3 {
		t.Errorf("RecentRuns len = %d, want 3", len(stats.RecentRuns))
	}
	// Newest first → the failed run
	if stats.RecentRuns[0].Status != RunStatusFailed {
		t.Errorf("most recent = %q, want failed (newest first)", stats.RecentRuns[0].Status)
	}
}

// TestHandler_StatsEndpoint end-to-end via the HTTP surface.
func TestHandler_StatsEndpoint(t *testing.T) {
	r, repo := sharedFixture(t)
	p := &Pipeline{ProjectID: "p1", Name: "stats-http", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s1", Type: StepTypeShell}}}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	seedRunViaRepo(t, repo, p.ID, RunStatusSucceeded, 100*time.Millisecond, time.Now().Add(-1*time.Minute))
	seedRunViaRepo(t, repo, p.ID, RunStatusFailed, 50*time.Millisecond, time.Now())

	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+p.ID+"/stats", nil)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %v", rr.Code, body)
	}
	if body["pipeline_id"] != p.ID {
		t.Errorf("pipeline_id = %v, want %s", body["pipeline_id"], p.ID)
	}
	if total, _ := body["total_runs"].(float64); int(total) != 2 {
		t.Errorf("total_runs = %v, want 2", body["total_runs"])
	}
	if sr, _ := body["success_rate"].(float64); sr < 0.49 || sr > 0.51 {
		t.Errorf("success_rate = %v, want ~0.5", body["success_rate"])
	}
	recent, _ := body["recent_runs"].([]any)
	if len(recent) != 2 {
		t.Errorf("recent_runs len = %v, want 2", recent)
	}
}

// TestHandler_PhasesEndpoint_BlueGreen covers the new
// /pipelines/:id/phases route. A blue-green pipeline must
// return the 4-phase plan (deploy-inactive, smoke-inactive,
// switch-traffic, decommission) so the frontend can render
// the deployment flow without re-implementing the planner.
func TestHandler_PhasesEndpoint_BlueGreen(t *testing.T) {
	r, repo := sharedFixture(t)
	p := &Pipeline{
		ProjectID: "p1", Name: "bg-phases-http", TargetType: TargetTypeProject,
		Trigger: "manual", Enabled: true,
		Steps:          StepList{{Name: "s1", Type: StepTypeShell}},
		Strategy:       StrategyBlueGreen,
		BlueGreenConfig: BlueGreenConfig{ActiveEnv: "blue", InactiveEnv: "green"},
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+p.ID+"/phases", nil)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %v", rr.Code, body)
	}
	if body["strategy"] != "blue_green" {
		t.Errorf("strategy = %v, want blue_green", body["strategy"])
	}
	phases, _ := body["phases"].([]any)
	if len(phases) != 4 {
		t.Fatalf("phases len = %d, want 4; got %v", len(phases), phases)
	}
	wantNames := []string{"deploy-inactive", "smoke-inactive", "switch-traffic", "decommission"}
	for i, name := range wantNames {
		p, _ := phases[i].(map[string]any)
		if p["name"] != name {
			t.Errorf("phases[%d].name = %v, want %s", i, p["name"], name)
		}
	}
}

// TestHandler_PhasesEndpoint_NoStrategy: pipelines without
// a strategy have empty phases (the frontend renders the
// raw Steps list instead of the strategy flow).
func TestHandler_PhasesEndpoint_NoStrategy(t *testing.T) {
	r, repo := sharedFixture(t)
	p := &Pipeline{
		ProjectID: "p1", Name: "no-strategy-http", TargetType: TargetTypeProject,
		Trigger: "manual", Enabled: true,
		Steps: StepList{{Name: "build", Type: StepTypeShell}, {Name: "test", Type: StepTypeShell}},
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	rr, body := doJSON(t, r, "GET", "/api/v1/pipelines/"+p.ID+"/phases", nil)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body["strategy"] != "" {
		t.Errorf("strategy = %v, want empty", body["strategy"])
	}
	phases, _ := body["phases"].([]any)
	if len(phases) != 0 {
		t.Errorf("phases len = %d, want 0", len(phases))
	}
}

// TestHandler_ListAllRunsEndpoint: the spec's "GET /api/runs"
// route. Pinned in TDD so a regression in the route map is
// caught by a unit test, not a manual curl.
func TestHandler_ListAllRunsEndpoint(t *testing.T) {
	r, repo := sharedFixture(t)
	p1 := &Pipeline{ProjectID: "p1", Name: "r1", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s1", Type: StepTypeShell}}}
	p2 := &Pipeline{ProjectID: "p1", Name: "r2", TargetType: TargetTypeProject, Trigger: "manual", Enabled: true, Steps: StepList{{Name: "s1", Type: StepTypeShell}}}
	if err := repo.CreatePipeline(p1); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if err := repo.CreatePipeline(p2); err != nil {
		t.Fatalf("create p2: %v", err)
	}

	seedRunViaRepo(t, repo, p1.ID, RunStatusSucceeded, 10*time.Millisecond, time.Now().Add(-1*time.Minute))
	seedRunViaRepo(t, repo, p2.ID, RunStatusFailed, 20*time.Millisecond, time.Now())

	rr, body := doJSON(t, r, "GET", "/api/v1/runs?limit=10", nil)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %v", rr.Code, body)
	}
	data, _ := body["data"].([]any)
	if len(data) < 2 {
		t.Errorf("data len = %d, want >= 2", len(data))
	}
	page, _ := body["pagination"].(map[string]any)
	if total, _ := page["total"].(float64); int(total) < 2 {
		t.Errorf("pagination.total = %v, want >= 2", page["total"])
	}
}

// seedRunViaRepo creates a run with a controlled
// StartedAt/FinishedAt so the stats test is deterministic.
func seedRunViaRepo(t *testing.T, repo *Repository, pipelineID string, status RunStatus, dur time.Duration, startedAt time.Time) {
	t.Helper()
	finished := startedAt.Add(dur)
	run := &PipelineRun{
		PipelineID:  pipelineID,
		Status:      status,
		StartedAt:   &startedAt,
		FinishedAt:  &finished,
		DurationMs:  dur.Milliseconds(),
		TriggeredBy: "seed",
	}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("seed run: %v", err)
	}
}
