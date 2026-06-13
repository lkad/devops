package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrUnauthenticated is the sentinel returned when a service
// method is invoked without a caller on the context. The
// handler maps it to a 401 UNAUTHORIZED APIError.
var ErrUnauthenticated = errors.New("pipeline: unauthenticated")

// ErrForbidden is the sentinel returned when the caller's
// tenant membership does not allow the requested operation.
// The handler maps it to a 403 FORBIDDEN APIError.
var ErrForbidden = errors.New("pipeline: forbidden")

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
	// ServiceID is the FK to a service in the
	// service-catalog. Empty for legacy / unassigned
	// pipelines. The FK is ON DELETE SET NULL so removing
	// a service does not cascade-delete the pipeline row.
	ServiceID   string
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
	// ServiceID: nil means "do not change"; non-nil with
	// an empty string means "clear the association".
	ServiceID   *string
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

	// serviceValidator is the optional function-typed seam
	// for service_id FK validation. It is nil by default;
	// when set, every Create / Update that supplies a
	// non-empty ServiceID calls it. The servicecatalog
	// package wires the real validator in main.go; tests
	// can inject a fake via WithServiceValidator.
	serviceValidator ServiceValidator

	// membershipChecker is the cross-tenant membership
	// check used by the v0.3.0.0 P0 #2 service-layer guard.
	// nil means "no memberships" (fail-closed). Production
	// wires repo.ListProjectIDsForUser via the
	// pipeline.NewService factory in main.go; tests
	// supply a fake via SetMembershipChecker.
	membershipChecker caller.MembershipChecker
}

// SetMembershipChecker wires the cross-tenant membership
// check used by the v0.3.0.0 P0 #2 service-layer guard.
func (s *Service) SetMembershipChecker(m caller.MembershipChecker) {
	s.membershipChecker = m
}

// ServiceValidator reports whether a service_id refers to
// an existing service. Returning a non-nil error fails
// the create / update with a 400.
type ServiceValidator func(serviceID string) error

// errUnknownService is the typed sentinel for "service
// does not exist". The handler maps it to a 400 with a
// readable message.
var errUnknownService = errors.New("pipeline: unknown service_id")

// WithServiceValidator injects the validator and returns
// the receiver for fluent chaining. Called once at
// startup, after NewService, before serving traffic.
func (s *Service) WithServiceValidator(v ServiceValidator) *Service {
	s.serviceValidator = v
	return s
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
	if err := s.validateServiceID(in.ServiceID); err != nil {
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
		ServiceID:   in.ServiceID,
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

// GetWithCaller is the v0.3.0.0 P0 #2 cross-tenant variant of
// Get. The caller MUST be attached to the context; an absent
// caller surfaces as ErrUnauthenticated (401). A non-SuperAdmin
// caller without membership in the pipeline's project_id
// surfaces as ErrForbidden (403). The ungoverned Get is
// retained for legacy code paths (e.g. the run executor's
// internal lookups).
func (s *Service) GetWithCaller(ctx context.Context, id string) (*Pipeline, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	p, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if !cl.IsSuperAdmin() {
		if !cl.IsMemberOf(ctx, p.ProjectID, s.membershipChecker) {
			return nil, ErrForbidden
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
	if in.ServiceID != nil {
		if err := s.validateServiceID(*in.ServiceID); err != nil {
			return nil, err
		}
	}
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
	if in.ServiceID != nil {
		p.ServiceID = *in.ServiceID
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

// CreateWithCaller is the v0.3.0.0 P0 #2 cross-tenant
// variant of Create. The caller MUST be attached to the
// context; an absent caller surfaces as ErrUnauthenticated
// (401). A non-SuperAdmin caller without membership in
// in.ProjectID (or an empty ProjectID) is denied. The
// ungoverned Create is retained for legacy code paths.
func (s *Service) CreateWithCaller(ctx context.Context, in CreatePipelineInput) (*Pipeline, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		if !cl.IsMemberOf(ctx, in.ProjectID, s.membershipChecker) {
			return nil, ErrForbidden
		}
	}
	return s.Create(in)
}

// UpdateWithCaller is the v0.3.0.0 P0 #2 cross-tenant
// variant of Update. The caller MUST be attached to the
// context; an absent caller surfaces as ErrUnauthenticated
// (401). A non-SuperAdmin caller without membership in the
// pipeline's current project_id is denied. The ungoverned
// Update is retained for legacy code paths.
func (s *Service) UpdateWithCaller(ctx context.Context, id string, in UpdatePipelineInput) (*Pipeline, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	current, err := s.repo.GetPipeline(id)
	if err != nil {
		return nil, err
	}
	if !cl.IsSuperAdmin() {
		if !cl.IsMemberOf(ctx, current.ProjectID, s.membershipChecker) {
			return nil, ErrForbidden
		}
	}
	return s.Update(id, in)
}

// DeleteWithCaller is the v0.3.0.0 P0 #2 cross-tenant
// variant of Delete. The caller MUST be attached to the
// context; an absent caller surfaces as ErrUnauthenticated
// (401). A non-SuperAdmin caller without membership in the
// pipeline's project_id is denied. The ungoverned Delete is
// retained for legacy code paths.
func (s *Service) DeleteWithCaller(ctx context.Context, id string) error {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return ErrUnauthenticated
	}
	current, err := s.repo.GetPipeline(id)
	if err != nil {
		return err
	}
	if !cl.IsSuperAdmin() {
		if !cl.IsMemberOf(ctx, current.ProjectID, s.membershipChecker) {
			return ErrForbidden
		}
	}
	return s.Delete(id)
}

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

	// Resolve the actual step list: if a strategy is
	// configured, the planner's phases go BEFORE the
	// user-supplied Steps so the linear steps only run
	// after a successful strategy rollout.
	steps := resolveSteps(p)
	// Prime the step→order cache so ensureStepRun can
	// record the right order even when a strategy
	// re-ordered the steps away from p.Steps.
	s.primeStepOrder(run.ID, steps)

	// Build the emitter. The executor is the only writer
	// for per-step status; we look up the step run ID by
	// step name when a transition comes in.
	emitter := func(e StepEvent) error {
		return s.recordStepEvent(run.ID, e)
	}

	execErr := s.executor.Execute(ctx, *run, steps, emitter)
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

// primeStepOrder pre-populates the step-order cache with
// the (name, order) pairs from the resolved steps. The
// cache is what ensureStepRun consults to record the
// right Order; without this prime pass a strategy-
// managed step would land at order=0.
func (s *Service) primeStepOrder(runID string, steps []PipelineStep) {
	stepRunCacheMu.Lock()
	defer stepRunCacheMu.Unlock()
	for i, st := range steps {
		// We store the (step name → order index) so
		// ensureStepRun can find it without scanning the
		// pipeline row again.
		key := runID + "::order::" + st.Name
		stepOrderCache[key] = i
	}
}

// stepOrderCache is keyed by "<runID>::order::<stepName>".
// A package-level map keeps the test surface narrow. The
// RWMutex is RWMutex because the hot path (ensureStepRun)
// only reads; primeStepOrder holds the write lock.
var (
	stepOrderCacheMu sync.RWMutex
	stepOrderCache   = make(map[string]int)
)

// resolveSteps turns a pipeline into the actual step slice
// the executor will run. A pipeline with a strategy set has
// its planned phases prepended to the user-defined Steps
// (strategy phases first, then linear steps). A pipeline
// without a strategy runs the user-defined Steps verbatim.
//
// The strategy is a no-op when the planner returns no
// phases (e.g. an empty config) — the linear steps still
// run.
func resolveSteps(p *Pipeline) []PipelineStep {
	phases, err := PlanForPipeline(p)
	if err != nil || len(phases) == 0 {
		return []PipelineStep(p.Steps)
	}
	out := make([]PipelineStep, 0, len(phases)+len(p.Steps))
	for _, ph := range phases {
		out = append(out, phaseToStep(ph))
	}
	out = append(out, p.Steps...)
	return out
}

// phaseToStep turns a strategy Phase into a PipelineStep
// the existing executor can run. The "shell" step type
// carries the rendered command in Config["cmd"]; the
// env is a label so audit + UI can render the target
// environment.
func phaseToStep(ph Phase) PipelineStep {
	cfg := StepConfig{
		"cmd":     ph.Command,
		"phase":   ph.Name,
		"traffic": ph.TrafficPct,
		"health":  ph.Healthcheck,
	}
	return PipelineStep{
		Name:   ph.Name,
		Type:   StepTypeShell,
		Config: cfg,
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
	// The strategy service pre-populates the stepOrderCache
	// (see primeStepOrder) so a strategy-managed step is
	// not pinned to order=0. The cache is keyed by
	// runID + step name; a fall-back to p.Steps handles
	// non-strategy pipelines.
	order := -1
	stepOrderCacheMu.RLock()
	if v, ok := stepOrderCache[runID+"::order::"+stepName]; ok {
		order = v
	}
	stepOrderCacheMu.RUnlock()
	if order < 0 {
		p, err := s.repo.GetPipeline(run.PipelineID)
		if err != nil {
			return "", err
		}
		for i, st := range p.Steps {
			if st.Name == stepName {
				order = i
				break
			}
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

// PipelineStats is the aggregated view of a pipeline's
// execution history. The wire shape mirrors the spec's
// "Get pipeline stats" scenario: total / successful / failed
// counts, success rate, average duration, and the last 10
// runs (newest first).
type PipelineStats struct {
	PipelineID         string         `json:"pipeline_id"`
	TotalRuns          int            `json:"total_runs"`
	SuccessfulRuns     int            `json:"successful_runs"`
	FailedRuns         int            `json:"failed_runs"`
	CancelledRuns      int            `json:"cancelled_runs"`
	SuccessRate        float64        `json:"success_rate"`
	AverageDurationMs  int64          `json:"average_duration_ms"`
	RecentRuns         []PipelineRun  `json:"recent_runs"`
}

// Stats computes the aggregate statistics for one pipeline.
// The implementation walks the underlying SQLite result set
// in-process (no SQL aggregation functions) so it works on
// every GORM driver the dev tier uses, including the
// in-memory sqlite the tests rely on.
func (s *Service) Stats(pipelineID string) (*PipelineStats, error) {
	stats := &PipelineStats{PipelineID: pipelineID}

	// Pull every run for the pipeline; for a "last 10" view
	// the dataset is bounded by the user's retention policy
	// and a full table scan is acceptable in v1. A future
	// iteration can push the aggregation into SQL.
	limit := 10000
	rows, total, err := s.repo.ListRunsForPipeline(pipelineID, limit, 0)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load runs",
			Cause:   err,
		}
	}
	stats.TotalRuns = int(total)

	var totalDuration int64
	for _, r := range rows {
		switch r.Status {
		case RunStatusSucceeded:
			stats.SuccessfulRuns++
			totalDuration += r.DurationMs
		case RunStatusFailed:
			stats.FailedRuns++
			totalDuration += r.DurationMs
		case RunStatusCancelled:
			stats.CancelledRuns++
		}
	}
	if stats.TotalRuns > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalRuns)
	}
	// Average over the runs that actually completed (succeeded
	// or failed) — cancelled runs are not measured here
	// because their DurationMs is usually 0.
	completed := stats.SuccessfulRuns + stats.FailedRuns
	if completed > 0 {
		stats.AverageDurationMs = totalDuration / int64(completed)
	}

	// Last 10 runs in DESC started_at order. The repository
	// already returns DESC; we just trim.
	if len(rows) > 10 {
		stats.RecentRuns = rows[:10]
	} else {
		stats.RecentRuns = rows
	}
	return stats, nil
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

// validateServiceID is the function-typed seam the
// servicecatalog package uses. When the validator is
// configured (production wiring in main.go) every non-empty
// service_id is checked. When the validator is nil the
// function fails closed: a non-empty service_id is rejected.
// This is deliberate — the alternative ("accept anything
// when no validator is wired") is a footgun. Existing
// pipelines that did not set service_id keep working
// because we only check when the caller supplies a value.
func (s *Service) validateServiceID(id string) error {
	if id == "" {
		return nil
	}
	if s.serviceValidator == nil {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("service_id %q is not validated (no catalog wired)", id),
		}
	}
	if err := s.serviceValidator(id); err != nil {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("service_id %q does not exist", id),
			Cause:   err,
		}
	}
	return nil
}
