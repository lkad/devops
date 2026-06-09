package pipeline

import (
	"testing"
	"time"
)

// TestStrategyType_Valid covers the 3-way enum validation
// from the spec: blue_green, canary, rolling.
func TestStrategyType_Valid(t *testing.T) {
	for _, s := range []StrategyType{StrategyBlueGreen, StrategyCanary, StrategyRolling} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []StrategyType{"", "unknown", "BLUE_GREEN"} {
		if s.Valid() {
			t.Errorf("%q should NOT be valid", s)
		}
	}
}

// TestBlueGreen_PlanPhases covers the spec's "switch traffic
// atomically" pattern. Blue-Green must:
//   1. Deploy to the INACTIVE environment
//   2. Run smoke tests against the new env
//   3. Switch traffic atomically (single phase)
//   4. Decommission the OLD environment
func TestBlueGreen_PlanPhases(t *testing.T) {
	s := NewBlueGreenStrategy(BlueGreenConfig{ActiveEnv: "blue", InactiveEnv: "green"})
	phases := s.Plan()

	wantNames := []string{
		"deploy-inactive", // 1. deploy to green (inactive)
		"smoke-inactive",  // 2. smoke test green
		"switch-traffic",  // 3. atomic cutover
		"decommission",    // 4. teardown old env
	}
	if len(phases) != len(wantNames) {
		t.Fatalf("len(phases) = %d, want %d", len(phases), len(wantNames))
	}
	for i, p := range phases {
		if p.Name != wantNames[i] {
			t.Errorf("phases[%d].Name = %q, want %q", i, p.Name, wantNames[i])
		}
		if p.Env != "green" && p.Name != "decommission" {
			t.Errorf("phases[%d] (%s) should target green, got %q", i, p.Name, p.Env)
		}
		if p.Name == "decommission" && p.Env != "blue" {
			t.Errorf("decommission phase should target blue, got %q", p.Env)
		}
	}
}

// TestCanary_PlanStages covers the spec's "1%, 5%, 25%, 100%"
// canary stages with default config; the planner emits 4
// stages plus a baseline "deploy" stage.
func TestCanary_PlanStages(t *testing.T) {
	s := NewCanaryStrategy(CanaryConfig{
		Baseline: "stable",
		Candidate: "canary",
		Stages:   []int{1, 5, 25, 100}, // spec's defaults
	})
	phases := s.Plan()

	if len(phases) != 5 {
		t.Fatalf("len(phases) = %d, want 5 (deploy + 4 canary stages)", len(phases))
	}
	if phases[0].Name != "deploy-candidate" {
		t.Errorf("phase 0 = %q, want deploy-candidate", phases[0].Name)
	}
	wantPcts := []int{1, 5, 25, 100}
	for i, p := range wantPcts {
		idx := i + 1
		if phases[idx].TrafficPct != p {
			t.Errorf("phase %d TrafficPct = %d, want %d", idx, phases[idx].TrafficPct, p)
		}
		if phases[idx].Env != "canary" {
			t.Errorf("phase %d Env = %q, want canary", idx, phases[idx].Env)
		}
	}
}

// TestCanary_RejectsMonotonicStages covers the validation:
// the spec implies stages must ascend (1% < 5% < 25% < 100%).
// A bogus [50, 25, 100] input must be rejected.
func TestCanary_RejectsMonotonicStages(t *testing.T) {
	_, err := NewCanaryStrategyValidate(CanaryConfig{
		Baseline:  "stable",
		Candidate: "canary",
		Stages:    []int{50, 25, 100}, // not strictly ascending
	})
	if err == nil {
		t.Fatal("expected validation error for non-ascending canary stages")
	}
}

// TestRolling_PlanBatches covers the spec's "max_surge 20%"
// batching. 10 instances with 20% surge = batches of 2.
func TestRolling_PlanBatches(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{
		TotalInstances: 10,
		MaxSurgePct:    20,
	})
	batches := s.Plan()

	// 10 instances, surge 20% means batch of ceil(10*0.2)=2, so 5 batches.
	if len(batches) != 5 {
		t.Fatalf("len(batches) = %d, want 5", len(batches))
	}
	// Each batch's command embeds the instance range; assert
	// they line up to a continuous partition of 1..10.
	wantRanges := []string{
		"1-2/10", "3-4/10", "5-6/10", "7-8/10", "9-10/10",
	}
	for i, b := range batches {
		if b.Command != "rolling-update.sh "+wantRanges[i] {
			t.Errorf("batch[%d].Command = %q, want rolling-update.sh %q",
				i, b.Command, wantRanges[i])
		}
	}
}

// TestRolling_UnevenDivision: 7 instances with 20% surge
// must produce batches that sum to 7 (last batch may be
// smaller).
func TestRolling_UnevenDivision(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{TotalInstances: 7, MaxSurgePct: 20})
	batches := s.Plan()
	// ceil(7*0.2) = 2 → 4 batches (2+2+2+1)
	if len(batches) != 4 {
		t.Fatalf("len(batches) = %d, want 4", len(batches))
	}
	// Each batch name increments; first 3 are full size 2,
	// the last one rolls 1 instance.
	wantNames := []string{"rolling-batch-1", "rolling-batch-2", "rolling-batch-3", "rolling-batch-4"}
	for i, b := range batches {
		if b.Name != wantNames[i] {
			t.Errorf("batch[%d].Name = %q, want %q", i, b.Name, wantNames[i])
		}
	}
}


// TestStrategyService_TriggerHonoursStrategy covers the
// end-to-end wiring: a pipeline with strategy=blue_green
// triggers 4 step runs (one per phase) and they appear in
// the repository in the right order.
func TestStrategyService_TriggerHonoursStrategy(t *testing.T) {
	svc, repo, _ := pipelineSvcFixture(t)
	p := &Pipeline{
		ProjectID:  "p1",
		Name:       "blue-green-deploy",
		TargetType: TargetTypeProject,
		Trigger:    "manual",
		Enabled:    true,
		Strategy:   StrategyBlueGreen,
		BlueGreenConfig: BlueGreenConfig{ActiveEnv: "blue", InactiveEnv: "green"},
	}
	if err := repo.CreatePipeline(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	run, err := svc.Trigger(p.ID, "tester")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	// Wait for the run to reach a terminal state. The
	// Fake executor completes every step instantly, so
	// 1s is generous.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.GetRun(run.ID)
		if got != nil && got.Status.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	_, steps, err := svc.GetRunWithSteps(run.ID)
	if err != nil {
		t.Fatalf("GetRunWithSteps: %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("len(steps) = %d, want 4 (BG phases)", len(steps))
	}
	wantOrder := []string{"deploy-inactive", "smoke-inactive", "switch-traffic", "decommission"}
	for i, s := range steps {
		if s.StepName != wantOrder[i] {
			t.Errorf("steps[%d].StepName = %q, want %q", i, s.StepName, wantOrder[i])
		}
	}
}
