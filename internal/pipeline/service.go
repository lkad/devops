package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// CreatePipelineInput is the input DTO for pipeline creation.
// The handler decodes the wire JSON into this struct; the
// service then validates and maps it onto a Pipeline.
// Pointer fields are used sparingly — only when the value
// must be optional.
type CreatePipelineInput struct {
	Name        string
	Description string
	ProjectID   string
	TargetType  TargetType
	TargetID    string
	Trigger     string
	Steps       []PipelineStep
	Enabled     *bool
}

// UpdatePipelineInput is the partial-update DTO. A nil
// pointer means "do not change"; a non-nil pointer to a zero
// value means "set to zero". The service only exposes a few
// updatable fields; widening the surface is additive.
type UpdatePipelineInput struct {
	Name        *string
	Description *string
	TargetType  *TargetType
	TargetID    *string
	Enabled     *bool
}

// Service is the business-logic layer for the pipeline
// subsystem. It owns validation, state-transition rules, and
// the goroutine that drives the Executor. It is
// framework-agnostic (no Gin) so it can be reused by gRPC
// handlers, CLI tools, or background workers in the future.
type Service struct {
	repo     *Repository
	executor Executor

	// runMu protects the in-memory registry of in-flight
	// runs. Triggering a pipeline creates a context that
	// the cancel endpoint looks up to cancel the goroutine.
	runMu   sync.Mutex
	running map[string]context.CancelFunc
}

// NewService builds a Service. The repository and executor
// are the only dependencies; everything else (clocks, audit
// hooks) can be injected later by extending the constructor.
func NewService(repo *Repository, exec Executor) *Service {
	return &Service{
		repo:     repo,
		executor: exec,
		running:  make(map[string]context.CancelFunc),
	}
}

// Create validates the input and persists a new pipeline.
// The returned Pipeline is the freshly-stored row, including
// its generated ID and timestamps.
func (s *Service) Create(in CreatePipelineInput) (*Pipeline, error) {
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	trigger := in.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	p := &Pipeline{
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		ProjectID:   strings.TrimSpace(in.ProjectID),
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		Trigger:     trigger,
		Steps:       StepList(in.Steps),
		Enabled:     enabled,
	}
	if err := s.repo.CreatePipeline(p); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create pipeline",
			Cause:   err,
		}
	}
	return p, nil
}

// Get returns a single pipeline or a 404 APIError.
func (s *Service) Get(id string) (*Pipeline, error) {
	p, err := s.repo.GetPipeline(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load pipeline",
			Cause:   err,
		}
	}
	return p, nil
}

// List returns a page of pipelines plus the unfiltered total.
func (s *Service) List(f PipelineFilter) ([]Pipeline, int64, error) {
	rows, total, err := s.repo.ListPipelines(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list pipelines",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Update applies a partial update.
func (s *Service) Update(id string, in UpdatePipelineInput) (*Pipeline, error) {
	p, err := s.repo.GetPipeline(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load pipeline",
			Cause:   err,
		}
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "name cannot be blank",
			}
		}
		p.Name = trimmed
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.TargetType != nil {
		if !in.TargetType.Valid() {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("target_type %q is not valid", *in.TargetType),
			}
		}
		p.TargetType = *in.TargetType
	}
	if in.TargetID != nil {
		p.TargetID = *in.TargetID
	}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if err := s.repo.UpdatePipeline(p); err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update pipeline",
			Cause:   err,
		}
	}
	return p, nil
}

// Delete soft-deletes a pipeline. A 404 APIError is returned
// when the row does not exist.
func (s *Service) Delete(id string) error {
	if err := s.repo.DeletePipeline(id); err != nil {
		if IsNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", id),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete pipeline",
			Cause:   err,
		}
	}
	return nil
}

// Trigger creates a new PipelineRun for the supplied
// pipeline ID and dispatches the executor in a goroutine.
// The returned run is the freshly-stored row in "pending"
// state; the goroutine will transition it through
// running → (succeeded|failed|cancelled) and record per-step
// status updates. The trigger is the spec's "Execute full
// pipeline" scenario.
//
// triggeredBy is recorded on the run for audit. The
// service-layer default is the caller's user ID; an empty
// string is allowed for system-triggered runs.
func (s *Service) Trigger(pipelineID, triggeredBy string) (*PipelineRun, error) {
	p, err := s.repo.GetPipeline(pipelineID)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", pipelineID),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load pipeline",
			Cause:   err,
		}
	}
	if !p.Enabled {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: "pipeline is disabled",
		}
	}
	run := &PipelineRun{
		PipelineID:  p.ID,
		Status:      RunStatusPending,
		TriggeredBy: triggeredBy,
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create run",
			Cause:   err,
		}
	}

	// Dispatch the executor in a goroutine. The cancel
	// function is registered in the in-memory map so the
	// Cancel endpoint can stop it.
	ctx, cancel := context.WithCancel(context.Background())
	s.runMu.Lock()
	s.running[run.ID] = cancel
	s.runMu.Unlock()

	go s.executeRun(ctx, p, run)
	return run, nil
}

// executeRun drives the executor for a single run. The
// goroutine updates the run + step rows in the repository
// and removes the run from the in-memory map on completion.
// The state machine here is:
//
//	pending → running → succeeded|failed|cancelled
//
// On executor error (other than ctx.Cancel) the run is
// marked failed with the executor's error message.
func (s *Service) executeRun(ctx context.Context, p *Pipeline, run *PipelineRun) {
	defer func() {
		s.runMu.Lock()
		delete(s.running, run.ID)
		s.runMu.Unlock()
	}()

	startedAt := time.Now().UTC()
	if err := s.repo.UpdateRunStartedAt(run.ID, &startedAt); err != nil {
		// The row is gone (or never landed); give up.
		return
	}
	if err := s.repo.UpdateRunStatus(run.ID, RunStatusRunning, 0, ""); err != nil {
		return
	}

	// Build the emitter. The executor is the only writer
	// for per-step status; we look up the step run ID by
	// step name when a transition comes in.
	emitter := func(e StepEvent) error {
		return s.recordStepEvent(run.ID, e)
	}

	execErr := s.executor.Execute(ctx, *run, []PipelineStep(p.Steps), emitter)
	finishedAt := time.Now().UTC()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()

	switch {
	case execErr != nil && errors.Is(execErr, context.Canceled):
		_ = s.repo.UpdateRunFinished(run.ID, RunStatusCancelled, durationMs, "")
	case execErr != nil:
		_ = s.repo.UpdateRunFinished(run.ID, RunStatusFailed, durationMs, truncateMsg(execErr.Error()))
	default:
		_ = s.repo.UpdateRunFinished(run.ID, RunStatusSucceeded, durationMs, "")
	}
}

// recordStepEvent persists a single step transition. The
// first call for a given (run, step) creates the step run
// row; subsequent calls update it. We use a (run, step)
// lookup table in memory so we can find the step run ID
// quickly without an extra DB round-trip per event.
func (s *Service) recordStepEvent(runID string, e StepEvent) error {
	id, err := s.ensureStepRun(runID, e.StepName)
	if err != nil {
		return err
	}
	return s.repo.UpdateStepRunStatus(id, e.Status, e.ExitCode, e.Output)
}

// ensureStepRun returns the step-run ID for (run, step) —
// creating the row on the first call. The map is scoped
// to this goroutine, so it is not shared; no locking needed.
var (
	stepRunCacheMu sync.Mutex
	stepRunCache   = make(map[string]string)
)

func cacheKey(runID, stepName string) string {
	return runID + "|" + stepName
}

func (s *Service) ensureStepRun(runID, stepName string) (string, error) {
	stepRunCacheMu.Lock()
	defer stepRunCacheMu.Unlock()
	if id, ok := stepRunCache[cacheKey(runID, stepName)]; ok {
		return id, nil
	}
	// Determine order from the parent pipeline. We look up
	// the run to find the pipeline, then the pipeline's
	// steps, then the index. This is O(steps); the cache
	// keeps subsequent transitions O(1).
	run, err := s.repo.GetRun(runID)
	if err != nil {
		return "", err
	}
	p, err := s.repo.GetPipeline(run.PipelineID)
	if err != nil {
		return "", err
	}
	order := 0
	for i, st := range p.Steps {
		if st.Name == stepName {
			order = i
			break
		}
	}
	step := &PipelineStepRun{
		RunID:    runID,
		StepName: stepName,
		Status:   StepRunStatusPending,
		Order:    order,
	}
	if err := s.repo.CreateStepRun(step); err != nil {
		return "", err
	}
	stepRunCache[cacheKey(runID, stepName)] = step.ID
	return step.ID, nil
}

// Cancel transitions a run to "cancelled". The operation is
// idempotent: calling Cancel on an already-finished run is a
// noop. This is the spec's "Cancel running pipeline"
// scenario.
func (s *Service) Cancel(runID string) error {
	run, err := s.repo.GetRun(runID)
	if err != nil {
		if IsNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline run %q not found", runID),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load run",
			Cause:   err,
		}
	}
	if run.Status.Terminal() {
		// Idempotent: already finished.
		return nil
	}

	// Signal the goroutine to stop, then mark the run
	// cancelled. The goroutine may have already updated
	// the row; the final state wins.
	s.runMu.Lock()
	cancel, ok := s.running[runID]
	s.runMu.Unlock()
	if ok {
		cancel()
	}
	now := time.Now().UTC()
	durationMs := int64(0)
	if run.StartedAt != nil {
		durationMs = now.Sub(*run.StartedAt).Milliseconds()
	}
	if err := s.repo.UpdateRunFinished(runID, RunStatusCancelled, durationMs, ""); err != nil {
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to cancel run",
			Cause:   err,
		}
	}
	return nil
}

// GetRun returns a single run by ID, wrapped in a
// service-layer APIError envelope.
func (s *Service) GetRun(id string) (*PipelineRun, error) {
	run, err := s.repo.GetRun(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline run %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load run",
			Cause:   err,
		}
	}
	return run, nil
}

// GetRunWithSteps returns the run plus the per-step records.
func (s *Service) GetRunWithSteps(id string) (*PipelineRun, []PipelineStepRun, error) {
	run, steps, err := s.repo.GetRunWithSteps(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline run %q not found", id),
			}
		}
		return nil, nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load run",
			Cause:   err,
		}
	}
	return run, steps, nil
}

// ListRuns returns the run history for a given pipeline.
func (s *Service) ListRuns(pipelineID string, limit, offset int) ([]PipelineRun, int64, error) {
	// Verify the pipeline exists so a 404 can be rendered
	// at the API boundary rather than as an empty page.
	if _, err := s.repo.GetPipeline(pipelineID); err != nil {
		if IsNotFound(err) {
			return nil, 0, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("pipeline %q not found", pipelineID),
			}
		}
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load pipeline",
			Cause:   err,
		}
	}
	rows, total, err := s.repo.ListRunsForPipeline(pipelineID, limit, offset)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list runs",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// ListAllRecentRuns returns runs across every pipeline.
func (s *Service) ListAllRecentRuns(limit, offset int) ([]PipelineRun, int64, error) {
	rows, total, err := s.repo.ListRecentRuns(limit, offset)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list runs",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// validateCreate is the small block of field-level rules
// shared between create and (eventually) bulk import. The
// checks are intentionally conservative: empty name, empty
// project, unknown target type, no steps, unknown step type.
func validateCreate(in CreatePipelineInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "project_id is required",
		}
	}
	if !in.TargetType.Valid() {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("target_type %q is not valid", in.TargetType),
		}
	}
	if len(in.Steps) == 0 {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "at least one step is required",
		}
	}
	for i, step := range in.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("step[%d].name is required", i),
			}
		}
		if !step.Type.Valid() {
			return &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("step[%d].type %q is not valid", i, step.Type),
			}
		}
	}
	return nil
}

// truncateMsg caps the error message length so the
// error_message column does not blow up.
func truncateMsg(s string) string {
	const max = 1024
	if len(s) <= max {
		return s
	}
	return s[:max]
}
