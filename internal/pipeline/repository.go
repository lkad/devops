package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"
)

// ErrPipelineNotFound is the typed sentinel returned by
// pipeline-repository methods when a pipeline row is missing.
// The service layer wraps it in a 404 NOT_FOUND APIError; the
// handler does the rendering.
var ErrPipelineNotFound = errors.New("pipeline: not found")

// ErrRunNotFound is the typed sentinel for missing runs.
var ErrRunNotFound = errors.New("pipeline run: not found")

// IsNotFound reports whether err is (or wraps) one of the
// package's not-found sentinels. The service and handler both
// call this rather than == comparisons so wrapped errors are
// still recognised.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrPipelineNotFound) || errors.Is(err, ErrRunNotFound)
}

// Repository is the GORM-only data-access layer for the
// pipeline subsystem. Per the layering rules it knows nothing
// about Gin, contracts, or business validation — it just
// translates method calls into queries.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository. The DB is shared
// with the rest of the application; the repository does not
// own its lifecycle.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// PipelineFilter narrows the result of ListPipelines. All
// fields are optional; the zero value returns every pipeline.
type PipelineFilter struct {
	ProjectID string
	Search    string
	// ServiceID restricts to pipelines that deploy the
	// given microservice. Used by the service-catalog
	// health rollup to answer "what deploys service X".
	ServiceID string
	// Limit caps the page size; 0 returns every row.
	Limit  int
	Offset int
}

// CreatePipeline inserts a new pipeline row. The ID is filled
// in by BaseModel.BeforeCreate; the caller can read p.ID
// immediately after CreatePipeline returns.
func (r *Repository) CreatePipeline(p *Pipeline) error {
	if err := r.db.Create(p).Error; err != nil {
		return fmt.Errorf("pipeline.Create: %w", err)
	}
	return nil
}

// GetPipeline returns the pipeline with the given ID, or
// ErrPipelineNotFound if no such row exists. Soft-deleted
// rows are hidden.
func (r *Repository) GetPipeline(id string) (*Pipeline, error) {
	var p Pipeline
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrPipelineNotFound)
		return nil, fmt.Errorf("pipeline.Get: %w", err)
	}
	return &p, nil
}

// ListPipelines returns a page of pipelines plus the
// unfiltered total count. All filter fields are optional; the
// zero value returns every pipeline.
func (r *Repository) ListPipelines(f PipelineFilter) ([]Pipeline, int64, error) {
	q := r.db.Model(&Pipeline{})
	if f.ProjectID != "" {
		q = q.Where("project_id = ?", f.ProjectID)
	}
	if f.ServiceID != "" {
		q = q.Where("service_id = ?", f.ServiceID)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("name LIKE ?", like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.List count: %w", err)
	}
	rows := []Pipeline{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.List find: %w", err)
	}
	return rows, total, nil
}

// UpdatePipeline persists the entire pipeline record. The
// row must already exist; an Update on a missing ID returns
// ErrPipelineNotFound. We verify existence explicitly rather
// than relying on Save's upsert behaviour so the contract is
// "update only".
func (r *Repository) UpdatePipeline(p *Pipeline) error {
	var existing Pipeline
	err := r.db.First(&existing, "id = ?", p.ID).Error
	if err != nil {
		return database.MapNotFound(err, ErrPipelineNotFound)
		return fmt.Errorf("pipeline.Update lookup: %w", err)
	}
	if err := r.db.Save(p).Error; err != nil {
		return fmt.Errorf("pipeline.Update: %w", err)
	}
	return nil
}

// DeletePipeline soft-deletes the pipeline. The row stays
// in the table with DeletedAt set; subsequent
// GetPipeline/ListPipelines calls will not see it.
func (r *Repository) DeletePipeline(id string) error {
	res := r.db.Delete(&Pipeline{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("pipeline.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrPipelineNotFound
	}
	return nil
}

// CreateRun inserts a new pipeline run. The ID is filled in
// by BaseModel.BeforeCreate.
func (r *Repository) CreateRun(run *PipelineRun) error {
	if err := r.db.Create(run).Error; err != nil {
		return fmt.Errorf("pipeline.CreateRun: %w", err)
	}
	return nil
}

// GetRun returns a single run by ID.
func (r *Repository) GetRun(id string) (*PipelineRun, error) {
	var run PipelineRun
	if err := r.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrRunNotFound)
		return nil, fmt.Errorf("pipeline.GetRun: %w", err)
	}
	return &run, nil
}

// GetRunWithSteps returns the run plus the ordered list of
// its step runs. The handler uses this for the run-detail
// endpoint.
func (r *Repository) GetRunWithSteps(id string) (*PipelineRun, []PipelineStepRun, error) {
	run, err := r.GetRun(id)
	if err != nil {
		return nil, nil, err
	}
	var steps []PipelineStepRun
	if err := r.db.Where("run_id = ?", id).Order("step_order ASC, id ASC").Find(&steps).Error; err != nil {
		return nil, nil, fmt.Errorf("pipeline.GetRunWithSteps: %w", err)
	}
	return run, steps, nil
}

// ListRunsForPipeline returns the run history for a given
// pipeline, ordered by created_at desc so the most recent run
// is first. limit/offset are 0 for "no pagination".
func (r *Repository) ListRunsForPipeline(pipelineID string, limit, offset int) ([]PipelineRun, int64, error) {
	q := r.db.Model(&PipelineRun{}).Where("pipeline_id = ?", pipelineID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.ListRunsForPipeline count: %w", err)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	rows := []PipelineRun{}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.ListRunsForPipeline find: %w", err)
	}
	return rows, total, nil
}

// LastRunsForService returns the most recent N pipeline
// runs across all pipelines that deploy the given service,
// newest first. limit <= 0 means "no limit". Used by the
// service-catalog health rollup.
//
// We join PipelineRun -> Pipeline on pipeline_id; the
// WHERE service_id = ? filters to just the deploys for
// the service. The two-step query (sub-select for matching
// pipeline IDs, then run query) is the cleanest portable
// shape across SQLite + Postgres.
func (r *Repository) LastRunsForService(serviceID string, limit int) ([]PipelineRun, error) {
	if serviceID == "" {
		return nil, nil
	}
	var pipelineIDs []string
	if err := r.db.Model(&Pipeline{}).
		Where("service_id = ?", serviceID).
		Pluck("id", &pipelineIDs).Error; err != nil {
		return nil, fmt.Errorf("pipeline.LastRunsForService pluck: %w", err)
	}
	if len(pipelineIDs) == 0 {
		return nil, nil
	}
	q := r.db.Model(&PipelineRun{}).
		Where("pipeline_id IN ?", pipelineIDs).
		Order("started_at DESC, id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	rows := []PipelineRun{}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("pipeline.LastRunsForService find: %w", err)
	}
	return rows, nil
}

// ListRecentRuns returns runs across every pipeline,
// ordered by created_at desc. The handler uses this for the
// "all recent runs" feed.
func (r *Repository) ListRecentRuns(limit, offset int) ([]PipelineRun, int64, error) {
	q := r.db.Model(&PipelineRun{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.ListRecentRuns count: %w", err)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	rows := []PipelineRun{}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("pipeline.ListRecentRuns find: %w", err)
	}
	return rows, total, nil
}

// UpdateRunStatus updates a run's status, duration, and
// optional error message. The StartedAt/FinishedAt
// timestamps are also set if not already populated — the
// executor goroutine typically sets them in the surrounding
// service code.
func (r *Repository) UpdateRunStatus(id string, status RunStatus, durationMs int64, errMsg string) error {
	updates := map[string]any{
		"status":        status,
		"duration_ms":   durationMs,
		"error_message": errMsg,
	}
	if status == RunStatusSucceeded || status == RunStatusFailed || status == RunStatusCancelled {
		now := time.Now().UTC()
		updates["finished_at"] = &now
	}
	res := r.db.Model(&PipelineRun{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("pipeline.UpdateRunStatus: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// UpdateRunStartedAt sets the run's StartedAt timestamp and
// transitions the status to running in one call. The service
// goroutine uses this on executor kickoff.
func (r *Repository) UpdateRunStartedAt(id string, startedAt *time.Time) error {
	res := r.db.Model(&PipelineRun{}).Where("id = ?", id).Updates(map[string]any{
		"started_at": startedAt,
		"status":     RunStatusRunning,
	})
	if res.Error != nil {
		return fmt.Errorf("pipeline.UpdateRunStartedAt: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// UpdateRunFinished sets the terminal status, duration, and
// optional error message in one call. The FinishedAt
// timestamp is set to now.
func (r *Repository) UpdateRunFinished(id string, status RunStatus, durationMs int64, errMsg string) error {
	now := time.Now().UTC()
	res := r.db.Model(&PipelineRun{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"duration_ms":   durationMs,
		"finished_at":   &now,
		"error_message": errMsg,
	})
	if res.Error != nil {
		return fmt.Errorf("pipeline.UpdateRunFinished: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// CreateStepRun inserts a new step run record.
func (r *Repository) CreateStepRun(step *PipelineStepRun) error {
	if err := r.db.Create(step).Error; err != nil {
		return fmt.Errorf("pipeline.CreateStepRun: %w", err)
	}
	return nil
}

// UpdateStepRunStatus updates a single step run to its
// terminal state. The service layer uses this from the
// executor's emitter callback.
func (r *Repository) UpdateStepRunStatus(id string, status StepRunStatus, exitCode int, output string) error {
	updates := map[string]any{
		"status":           status,
		"exit_code":        exitCode,
		"output_truncated": output,
	}
	if status == StepRunStatusSucceeded || status == StepRunStatusFailed ||
		status == StepRunStatusCancelled || status == StepRunStatusSkipped {
		now := time.Now().UTC()
		updates["finished_at"] = &now
	}
	res := r.db.Model(&PipelineStepRun{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("pipeline.UpdateStepRunStatus: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("pipeline step run: %w", ErrRunNotFound)
	}
	return nil
}
