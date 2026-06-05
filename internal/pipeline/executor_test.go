package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// collectEmitter is a StepEmitter used by the executor tests.
// It records every event on a thread-safe slice and returns a
// pre-set error (if any) to the executor so we can test error
// propagation from the persistence layer.
type collectEmitter struct {
	mu     sync.Mutex
	events []StepEvent
	err    error
}

func (c *collectEmitter) Emit(e StepEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.events = append(c.events, e)
	return nil
}

func (c *collectEmitter) Snapshot() []StepEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]StepEvent, len(c.events))
	copy(out, c.events)
	return out
}

// TestFake_Execute_RunsAllStepsInOrder checks that the Fake
// executor emits a running → succeeded pair for every step in
// the supplied slice, in the slice order. This is the
// "Execute full pipeline" scenario from the spec.
func TestFake_Execute_RunsAllStepsInOrder(t *testing.T) {
	f := &Fake{}
	steps := []PipelineStep{
		{Name: "build", Type: StepTypeShell},
		{Name: "test", Type: StepTypeShell},
		{Name: "deploy", Type: StepTypeShell},
	}
	em := &collectEmitter{}
	err := f.Execute(context.Background(), PipelineRun{}, steps, em.Emit)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	evs := em.Snapshot()
	if len(evs) != 6 {
		t.Fatalf("event count = %d, want 6 (running+succeeded per step)", len(evs))
	}
	for i, step := range steps {
		if evs[2*i].StepName != step.Name || evs[2*i].Status != StepRunStatusRunning {
			t.Errorf("event[%d] = %+v, want running for %q", 2*i, evs[2*i], step.Name)
		}
		if evs[2*i+1].StepName != step.Name || evs[2*i+1].Status != StepRunStatusSucceeded {
			t.Errorf("event[%d] = %+v, want succeeded for %q", 2*i+1, evs[2*i+1], step.Name)
		}
	}
}

// TestFake_Execute_StopsOnFailedStep is the spec's "Stage
// failure handling" scenario: when a step is configured to
// fail, the executor stops execution, the failed step gets a
// failed event, and subsequent steps get a skipped event.
func TestFake_Execute_StopsOnFailedStep(t *testing.T) {
	f := &Fake{FailStep: "test"}
	steps := []PipelineStep{
		{Name: "build", Type: StepTypeShell},
		{Name: "test", Type: StepTypeShell},
		{Name: "deploy", Type: StepTypeShell},
	}
	em := &collectEmitter{}
	err := f.Execute(context.Background(), PipelineRun{}, steps, em.Emit)
	if err == nil {
		t.Fatal("expected error from failed step")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error = %v, want to mention the failing step", err)
	}
	evs := em.Snapshot()
	// build runs and succeeds, test runs and fails, deploy is skipped.
	names := make([]string, 0, len(evs))
	for _, e := range evs {
		names = append(names, e.StepName+":"+string(e.Status))
	}
	want := []string{
		"build:running", "build:succeeded",
		"test:running", "test:failed",
		"deploy:skipped",
	}
	if len(names) != len(want) {
		t.Fatalf("events = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("event[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

// TestFake_Execute_ContextCancel maps to the "Cancel running
// pipeline" scenario. A cancelled context must short-circuit
// the executor: subsequent steps get a cancelled event, and
// the executor returns ctx.Err().
func TestFake_Execute_ContextCancel(t *testing.T) {
	f := &Fake{}
	steps := []PipelineStep{
		{Name: "a", Type: StepTypeShell},
		{Name: "b", Type: StepTypeShell},
		{Name: "c", Type: StepTypeShell},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	em := &collectEmitter{}
	err := f.Execute(ctx, PipelineRun{}, steps, em.Emit)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	evs := em.Snapshot()
	if len(evs) == 0 {
		t.Fatal("expected at least one event before cancellation observed")
	}
	if evs[0].Status != StepRunStatusCancelled {
		t.Errorf("first event = %+v, want cancelled", evs[0])
	}
}

// TestFake_Execute_EmptyStepsIsNoop ensures an empty slice
// does not raise an error and emits no events.
func TestFake_Execute_EmptyStepsIsNoop(t *testing.T) {
	f := &Fake{}
	em := &collectEmitter{}
	if err := f.Execute(context.Background(), PipelineRun{}, nil, em.Emit); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := em.Snapshot(); len(got) != 0 {
		t.Errorf("events = %v, want none", got)
	}
}

// TestFake_Execute_EmitterErrorPropagates ensures the executor
// surfaces persistence errors instead of silently swallowing
// them.
func TestFake_Execute_EmitterErrorPropagates(t *testing.T) {
	f := &Fake{}
	wantErr := errors.New("db is down")
	em := &collectEmitter{err: wantErr}
	steps := []PipelineStep{{Name: "build", Type: StepTypeShell}}
	err := f.Execute(context.Background(), PipelineRun{}, steps, em.Emit)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

// TestExecutor_Interface_Satisfied ensures both Fake and Local
// satisfy the Executor interface. This is a compile-time
// guarantee that the service layer can swap them at will.
func TestExecutor_Interface_Satisfied(t *testing.T) {
	var _ Executor = (*Fake)(nil)
	var _ Executor = (*Local)(nil)
}
