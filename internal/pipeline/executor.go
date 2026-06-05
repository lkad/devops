package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StepEvent is the unit of communication between the executor
// and the persistence layer. The service layer subscribes to
// events via the emitter callback supplied to Execute. Each
// event represents a single state transition for a single
// step: "step X moved to status Y with output Z and exit
// code N". The executor does not know how those events are
// stored — that is the service layer's job.
type StepEvent struct {
	StepName string
	Status   StepRunStatus
	Output   string
	ExitCode int
}

// StepEmitter is the callback signature passed to
// Executor.Execute. The service layer wires this to the
// repository; the executor calls it once per step transition.
// An error from Emit aborts execution and is returned to the
// caller of Execute.
type StepEmitter func(StepEvent) error

// Executor runs a pipeline's steps in order. The interface
// is intentionally narrow — Execute is the only method — so
// swapping the Fake for the Local (or a future K8s
// implementation) is a one-line change in the service
// constructor.
//
// The emitter callback is how the executor reports state
// transitions to the persistence layer. Each call is
// expected to be cheap (a single GORM Update); the executor
// does not batch or coalesce.
type Executor interface {
	Execute(ctx context.Context, run PipelineRun, steps []PipelineStep, emit StepEmitter) error
}

// Fake is a test-friendly Executor that records step
// transitions in-memory and never actually shells out. The
// FailStep field, when set to a step name, makes the Fake
// emit a failed status for that step and skip every
// subsequent step. The Fields are exported so test code can
// configure the Fake in one line.
type Fake struct {
	// FailStep is the name of the step that should fail. If
	// empty, every step succeeds.
	FailStep string
	// DelayPerStep sleeps the goroutine for this duration
	// between events. Tests can leave it at zero.
	DelayPerStep time.Duration
	// Block, if non-nil, makes the executor wait on this
	// channel before emitting the first event. Tests use
	// this to keep the run "in flight" long enough to call
	// Cancel and verify the cancelled status. Closing the
	// channel unblocks the executor.
	Block chan struct{}

	mu       sync.Mutex
	events   []StepEvent
	canceled bool
}

// Execute implements Executor. It emits a running event, then
// a terminal event (succeeded / failed / skipped) for each
// step in order. A cancelled context short-circuits to a
// single cancelled event for the first step.
func (f *Fake) Execute(ctx context.Context, _ PipelineRun, steps []PipelineStep, emit StepEmitter) error {
	if len(steps) == 0 {
		return nil
	}
	if f.Block != nil {
		select {
		case <-f.Block:
		case <-ctx.Done():
			f.mu.Lock()
			f.canceled = true
			f.mu.Unlock()
			return ctx.Err()
		}
	}
	for i, step := range steps {
		if f.DelayPerStep > 0 {
			select {
			case <-time.After(f.DelayPerStep):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case <-ctx.Done():
			f.mu.Lock()
			f.canceled = true
			f.mu.Unlock()
			_ = emit(StepEvent{StepName: step.Name, Status: StepRunStatusCancelled})
			return ctx.Err()
		default:
		}

		if err := emit(StepEvent{StepName: step.Name, Status: StepRunStatusRunning, Output: ""}); err != nil {
			return err
		}

		if f.FailStep != "" && step.Name == f.FailStep {
			if err := emit(StepEvent{StepName: step.Name, Status: StepRunStatusFailed, Output: "simulated failure", ExitCode: 1}); err != nil {
				return err
			}
			f.mu.Lock()
			f.events = append(f.events,
				StepEvent{StepName: step.Name, Status: StepRunStatusRunning},
				StepEvent{StepName: step.Name, Status: StepRunStatusFailed},
			)
			f.mu.Unlock()
			// Skip every remaining step.
			for j := i + 1; j < len(steps); j++ {
				if err := emit(StepEvent{StepName: steps[j].Name, Status: StepRunStatusSkipped}); err != nil {
					return err
				}
				f.mu.Lock()
				f.events = append(f.events, StepEvent{StepName: steps[j].Name, Status: StepRunStatusSkipped})
				f.mu.Unlock()
			}
			return fmt.Errorf("pipeline: step %q failed", step.Name)
		}

		// Happy path: succeed the step.
		if err := emit(StepEvent{StepName: step.Name, Status: StepRunStatusSucceeded, Output: "ok"}); err != nil {
			return err
		}
		f.mu.Lock()
		f.events = append(f.events,
			StepEvent{StepName: step.Name, Status: StepRunStatusRunning},
			StepEvent{StepName: step.Name, Status: StepRunStatusSucceeded},
		)
		f.mu.Unlock()
	}
	return nil
}

// Local is the real-shell Executor. It runs shell steps via
// /bin/sh -c. HTTP, k8s-apply, and terraform steps are
// stubbed with a "not implemented" failure in v1 — the
// executor's seam is the interface, so a future K8s /
// Terraform executor can be added without touching the
// service layer.
//
// Per-step output is capped at MaxOutputBytes (default 1
// MiB) to keep a chatty process from blowing up the row.
type Local struct {
	// MaxOutputBytes caps the per-step output buffer. The
	// real-impl limit is 1 MiB; tests can shrink it.
	MaxOutputBytes int
	// ShellPath overrides /bin/sh for the executor. Tests
	// can leave it blank; production code is happy with the
	// default.
	ShellPath string
}

// NewLocal builds a Local with the default 1 MiB cap.
func NewLocal(opts ...LocalOption) *Local {
	l := &Local{MaxOutputBytes: defaultMaxOutputBytes, ShellPath: defaultShellPath}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LocalOption mutates a Local. Functional options keep the
// constructor signature stable as new knobs are added.
type LocalOption func(*Local)

// WithMaxOutputBytes overrides the per-step output cap.
func WithMaxOutputBytes(n int) LocalOption {
	return func(l *Local) { l.MaxOutputBytes = n }
}

// WithShellPath overrides the shell binary.
func WithShellPath(p string) LocalOption {
	return func(l *Local) { l.ShellPath = p }
}

const (
	defaultMaxOutputBytes = 1 << 20 // 1 MiB
	defaultShellPath      = "/bin/sh"
)

// Execute implements Executor. It runs each shell step via
// os/exec, with ctx cancellation killing the process. Output
// is streamed into a capped buffer; anything beyond the cap
// is dropped (we do not preserve the head).
func (l *Local) Execute(ctx context.Context, _ PipelineRun, steps []PipelineStep, emit StepEmitter) error {
	if len(steps) == 0 {
		return nil
	}
	for i, step := range steps {
		select {
		case <-ctx.Done():
			_ = emit(StepEvent{StepName: step.Name, Status: StepRunStatusCancelled})
			return ctx.Err()
		default:
		}

		if err := emit(StepEvent{StepName: step.Name, Status: StepRunStatusRunning, Output: ""}); err != nil {
			return err
		}

		switch step.Type {
		case StepTypeShell:
			output, exitCode, err := l.runShell(ctx, step)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				// Real error from cmd.Run (other than ctx
				// cancellation) — surface as failed step.
				ev := StepEvent{StepName: step.Name, Status: StepRunStatusFailed, Output: truncate(output, l.MaxOutputBytes), ExitCode: exitCode}
				_ = emit(ev)
				// Skip remaining steps.
				for j := i + 1; j < len(steps); j++ {
					_ = emit(StepEvent{StepName: steps[j].Name, Status: StepRunStatusSkipped})
				}
				return fmt.Errorf("pipeline: step %q failed: %w", step.Name, err)
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				_ = emit(StepEvent{StepName: step.Name, Status: StepRunStatusCancelled})
				return err
			}
			if exitCode != 0 {
				ev := StepEvent{StepName: step.Name, Status: StepRunStatusFailed, Output: truncate(output, l.MaxOutputBytes), ExitCode: exitCode}
				_ = emit(ev)
				for j := i + 1; j < len(steps); j++ {
					_ = emit(StepEvent{StepName: steps[j].Name, Status: StepRunStatusSkipped})
				}
				return fmt.Errorf("pipeline: step %q exited %d", step.Name, exitCode)
			}
			_ = emit(StepEvent{StepName: step.Name, Status: StepRunStatusSucceeded, Output: truncate(output, l.MaxOutputBytes), ExitCode: 0})
		default:
			// HTTP / k8s-apply / terraform are stubbed in v1.
			_ = emit(StepEvent{
				StepName: step.Name,
				Status:   StepRunStatusFailed,
				Output:   fmt.Sprintf("step type %q is not implemented in v1", step.Type),
				ExitCode: -1,
			})
			for j := i + 1; j < len(steps); j++ {
				_ = emit(StepEvent{StepName: steps[j].Name, Status: StepRunStatusSkipped})
			}
			return fmt.Errorf("pipeline: step %q: type %q not implemented", step.Name, step.Type)
		}
	}
	return nil
}

// runShell executes a single shell step. The command is taken
// from the step's "cmd" config. Output is captured into a
// buffer (capped at MaxOutputBytes inside Execute).
func (l *Local) runShell(ctx context.Context, step PipelineStep) (string, int, error) {
	cmdStr, _ := step.Config["cmd"].(string)
	if strings.TrimSpace(cmdStr) == "" {
		return "step config is missing 'cmd'", -1, errors.New("pipeline: shell step is missing 'cmd'")
	}
	cmd := exec.CommandContext(ctx, l.ShellPath, "-c", cmdStr)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return out.String(), exitCode, err
}

// truncate caps a string at max bytes. We use a simple byte
// cut because step output is ASCII in practice; a future
// enhancement could preserve the head and tail and drop the
// middle.
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
