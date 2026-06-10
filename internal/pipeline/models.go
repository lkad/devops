// Package pipeline implements the CI/CD pipeline subsystem. A
// Pipeline is a named, versioned list of steps that can be
// triggered manually (via API) or by a webhook (stubbed). Each
// execution produces a PipelineRun plus one PipelineStepRun
// per step; runs are cancelable and idempotent on already-
// finished runs.
//
// The package follows the project-wide layering rules:
//
//	handler -> service -> (executor | repository) -> model
//
// Handler is thin: parse, call, render. Service owns validation,
// orchestration, and the goroutine that drives the executor.
// The Executor is an interface so the Local shell-backed
// implementation can be swapped for a Fake in tests.
//
// Pipeline steps are persisted as a JSON column on the
// pipelines table. The choice trades queryability (you cannot
// say "find pipelines that contain a step named X") for
// simplicity (no extra table or join). Step-level runs ARE a
// separate table (pipeline_step_runs) because the spec
// requires per-step timing and logs.
package pipeline

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// StepType categorises the kind of step. The string values
// are part of the public API contract — clients branch on
// them, so renaming a value is a breaking change.
type StepType string

const (
	StepTypeShell     StepType = "shell"
	StepTypeK8sApply  StepType = "k8s-apply"
	StepTypeTerraform StepType = "terraform"
	StepTypeHTTP      StepType = "http"
)

// Valid reports whether t is one of the recognised step
// types. Used by the service layer to reject bogus create
// requests with a 400 VALIDATION_ERROR.
func (t StepType) Valid() bool {
	switch t {
	case StepTypeShell, StepTypeK8sApply, StepTypeTerraform, StepTypeHTTP:
		return true
	}
	return false
}

// StepConfig is the per-step configuration blob. The shape
// depends on the StepType: shell expects {"cmd": "..."},
// k8s-apply expects {"manifest": "..."}, etc. The column is a
// TEXT (JSON) blob; queryability is intentionally not
// supported in v1.
type StepConfig map[string]any

// Value renders the config as a JSON byte slice. A nil map
// renders as NULL so the column stays clean for steps that
// take no parameters.
func (s StepConfig) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any(s))
}

// Scan parses the column value into a fresh map. A nil input
// (NULL column) produces a nil map; empty strings are treated
// as "no value" — some drivers return "" for NULL on certain
// column types.
func (s *StepConfig) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		if v == "" {
			*s = nil
			return nil
		}
		b = []byte(v)
	default:
		return errors.New("pipeline: StepConfig: unsupported scan source type")
	}
	out := StepConfig{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// GormDataType pins the column type so AutoMigrate lands
// TEXT (sqlite) / JSONB (postgres) consistently.
func (StepConfig) GormDataType() string { return "text" }

// TargetType categorises what the pipeline runs against.
// Pipelines always belong to a project; the target identifies
// the unit of execution.
type TargetType string

const (
	TargetTypeProject TargetType = "project"
	TargetTypeDevice  TargetType = "device"
	TargetTypeCluster TargetType = "cluster"
	TargetTypeWebhook TargetType = "webhook"
)

// Valid reports whether t is one of the recognised target
// types.
func (t TargetType) Valid() bool {
	switch t {
	case TargetTypeProject, TargetTypeDevice, TargetTypeCluster, TargetTypeWebhook:
		return true
	}
	return false
}

// Pipeline is the CI/CD pipeline definition. The Steps column
// is a JSON blob (StepList) that survives AutoMigrate as
// TEXT/JSONB. Enabled is a soft-disable flag; a disabled
// pipeline can still be GETed but Trigger returns
// INVALID_STATE.
type Pipeline struct {
	database.BaseModel
	ProjectID   string     `gorm:"column:project_id;type:text;not null;index" json:"project_id"`
	Name        string     `gorm:"column:name;size:128;not null" json:"name"`
	Description string     `gorm:"column:description;size:512" json:"description"`
	TargetType  TargetType `gorm:"column:target_type;size:32;not null" json:"target_type"`
	// TargetID is the FK to the unit of execution (device,
	// cluster, ...). Optional; many pipelines just run
	// against the project as a whole.
	TargetID string `gorm:"column:target_id;type:text;index" json:"target_id,omitempty"`
	// ServiceID is the FK to the Service entity
	// (servicecatalog.Service). Nullable: pipelines created
	// before the service catalog was introduced, or
	// pipelines whose deploys are not associated with a
	// specific microservice, leave this as the empty
	// string. ON DELETE SET NULL at the DB level.
	ServiceID  string     `gorm:"column:service_id;type:text;size:64;index" json:"service_id,omitempty"`
	// Trigger is "manual" (default) or "webhook" (stub for
	// the planned webhook receiver).
	Trigger string `gorm:"column:trigger;size:32;not null;default:manual" json:"trigger"`
	Steps   StepList `gorm:"column:steps;type:text" json:"steps"`
	Enabled bool   `gorm:"column:enabled;default:true" json:"enabled"`

	// Deployment strategy (spec: blue-green / canary /
	// rolling). Empty string means "no strategy — use the
	// linear Steps list as-is". The config sub-structs
	// are JSON-serialized into dedicated columns so the
	// planner can fetch them without a JOIN.
	Strategy        StrategyType    `gorm:"column:strategy;size:32;index" json:"strategy,omitempty"`
	BlueGreenConfig BlueGreenConfig  `gorm:"column:blue_green_config;type:text" json:"blue_green_config,omitempty"`
	CanaryConfig    CanaryConfig     `gorm:"column:canary_config;type:text" json:"canary_config,omitempty"`
	RollingConfig   RollingConfig    `gorm:"column:rolling_config;type:text" json:"rolling_config,omitempty"`
}

// TableName pins the GORM-generated table name.
func (Pipeline) TableName() string { return "pipelines" }

// BeforeCreate wires the GORM hook. We default Enabled to
// true so a service-layer caller can leave it blank, and
// explicitly call the embedded BaseModel's BeforeCreate so
// the UUID PK is assigned — GORM does not invoke promoted
// hooks from embedded structs when a top-level method of the
// same name exists.
func (p *Pipeline) BeforeCreate(tx *gorm.DB) error {
	if p.Trigger == "" {
		p.Trigger = "manual"
	}
	return p.BaseModel.BeforeCreate(tx)
}

// PipelineStep is a single step in a pipeline. It is a value
// type (not a row) because we persist steps as a JSON column
// rather than a related table — see package doc for the
// trade-off.
type PipelineStep struct {
	Name   string     `json:"name"`
	Type   StepType   `json:"type"`
	Config StepConfig `json:"config,omitempty"`
}

// StepList is the persistence type for the list of steps. We
// keep it as a named type so the (Valuer, Scanner) pair is
// discoverable to GORM.
type StepList []PipelineStep

// Value renders the list as a JSON byte slice. An empty or
// nil slice renders as NULL — "no steps" is a validation
// error caught at the service layer, but a NULL column is
// still safe.
func (s StepList) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return json.Marshal([]PipelineStep(s))
}

// Scan parses the column value into a fresh slice. A nil
// input produces a nil slice; empty strings are treated as
// "no value".
func (s *StepList) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		if v == "" {
			*s = nil
			return nil
		}
		b = []byte(v)
	default:
		return errors.New("pipeline: StepList: unsupported scan source type")
	}
	out := StepList{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// GormDataType pins the column type so AutoMigrate lands
// TEXT (sqlite) / JSONB (postgres) consistently.
func (StepList) GormDataType() string { return "text" }

// =============================================================================
// Strategy config Valuer/Scanner pairs. Each config is
// persisted as a JSON column; the GormDataType pins the
// column type so AutoMigrate lands TEXT (sqlite) / JSONB
// (postgres) consistently. A zero-value config renders as
// NULL so empty strategy columns stay compact.
// =============================================================================

// Value renders BlueGreenConfig as JSON. A zero-value
// config renders NULL.
func (b BlueGreenConfig) Value() (driver.Value, error) {
	if b.ActiveEnv == "" && b.InactiveEnv == "" {
		return nil, nil
	}
	return json.Marshal(b)
}

// Scan parses the column value into a fresh BlueGreenConfig.
func (b *BlueGreenConfig) Scan(src any) error {
	if src == nil {
		*b = BlueGreenConfig{}
		return nil
	}
	raw, err := scanJSONBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*b = BlueGreenConfig{}
		return nil
	}
	return json.Unmarshal(raw, b)
}

// Value renders CanaryConfig as JSON. A zero-value config
// renders NULL.
func (c CanaryConfig) Value() (driver.Value, error) {
	if c.Baseline == "" && c.Candidate == "" && len(c.Stages) == 0 {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan parses the column value into a fresh CanaryConfig.
func (c *CanaryConfig) Scan(src any) error {
	if src == nil {
		*c = CanaryConfig{}
		return nil
	}
	raw, err := scanJSONBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*c = CanaryConfig{}
		return nil
	}
	return json.Unmarshal(raw, c)
}

// Value renders RollingConfig as JSON. A zero-value config
// renders NULL.
func (r RollingConfig) Value() (driver.Value, error) {
	if r.TotalInstances == 0 && r.MaxSurgePct == 0 {
		return nil, nil
	}
	return json.Marshal(r)
}

// Scan parses the column value into a fresh RollingConfig.
func (r *RollingConfig) Scan(src any) error {
	if src == nil {
		*r = RollingConfig{}
		return nil
	}
	raw, err := scanJSONBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*r = RollingConfig{}
		return nil
	}
	return json.Unmarshal(raw, r)
}

// scanJSONBytes is the small adapter shared by every
// config Scanner: accept []byte / string, drop empty, return.
func scanJSONBytes(src any) ([]byte, error) {
	switch v := src.(type) {
	case []byte:
		return v, nil
	case string:
		if v == "" {
			return nil, nil
		}
		return []byte(v), nil
	default:
		return nil, errors.New("pipeline: unsupported scan source type")
	}
}

// RunStatus is the lifecycle state of a PipelineRun. The
// five states mirror the task spec: pending, running,
// succeeded, failed, cancelled.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// Valid reports whether s is one of the recognised run
// statuses.
func (s RunStatus) Valid() bool {
	switch s {
	case RunStatusPending, RunStatusRunning, RunStatusSucceeded,
		RunStatusFailed, RunStatusCancelled:
		return true
	}
	return false
}

// Terminal reports whether the run is in a final state — no
// further transitions are allowed. Cancellation is terminal
// even though the run may not have completed.
func (s RunStatus) Terminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCancelled:
		return true
	}
	return false
}

// StepRunStatus is the lifecycle state of a PipelineStepRun.
// The "skipped" state is emitted by the executor when a
// step is bypassed because a prior step failed.
type StepRunStatus string

const (
	StepRunStatusPending   StepRunStatus = "pending"
	StepRunStatusRunning   StepRunStatus = "running"
	StepRunStatusSucceeded StepRunStatus = "succeeded"
	StepRunStatusFailed    StepRunStatus = "failed"
	StepRunStatusCancelled StepRunStatus = "cancelled"
	StepRunStatusSkipped   StepRunStatus = "skipped"
)

// Valid reports whether s is one of the recognised step-run
// statuses.
func (s StepRunStatus) Valid() bool {
	switch s {
	case StepRunStatusPending, StepRunStatusRunning, StepRunStatusSucceeded,
		StepRunStatusFailed, StepRunStatusCancelled, StepRunStatusSkipped:
		return true
	}
	return false
}

// Terminal mirrors RunStatus.Terminal for step runs.
func (s StepRunStatus) Terminal() bool {
	switch s {
	case StepRunStatusSucceeded, StepRunStatusFailed,
		StepRunStatusCancelled, StepRunStatusSkipped:
		return true
	}
	return false
}

// PipelineRun is a single execution of a pipeline. The
// timestamps are nullable so the service layer can record
// StartedAt at run kickoff and FinishedAt at completion.
type PipelineRun struct {
	database.BaseModel
	PipelineID   string `gorm:"column:pipeline_id;type:text;not null;index" json:"pipeline_id"`
	Status       RunStatus `gorm:"column:status;size:16;not null;index;default:pending" json:"status"`
	StartedAt    *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	DurationMs   int64  `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	TriggeredBy  string `gorm:"column:triggered_by;type:text" json:"triggered_by"`
	// ErrorMessage carries the failure reason when Status ==
	// failed. Empty otherwise. Capped at 1024 chars to keep
	// the row narrow.
	ErrorMessage string `gorm:"column:error_message;size:1024" json:"error_message,omitempty"`
}

// TableName pins the GORM-generated table name.
func (PipelineRun) TableName() string { return "pipeline_runs" }

// PipelineStepRun is the per-step execution record. The
// OutputTruncated column holds the (possibly truncated) step
// output — the Local executor caps the buffer at 1 MiB to
// keep the row from blowing up.
type PipelineStepRun struct {
	database.BaseModel
	RunID    string    `gorm:"column:run_id;type:text;not null;index" json:"run_id"`
	StepName string    `gorm:"column:step_name;size:128;not null" json:"step_name"`
	Status   StepRunStatus `gorm:"column:status;size:16;not null;index;default:pending" json:"status"`
	// Order is the 0-based step index from the original
	// pipeline definition. The handler uses it to render the
	// step list in execution order.
	Order       int        `gorm:"column:step_order" json:"order"`
	StartedAt   *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt  *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	OutputTruncated string `gorm:"column:output_truncated;type:text" json:"output_truncated,omitempty"`
	// ExitCode is the process exit code. 0 means success;
	// non-zero means the step failed. We intentionally do
	// not use a `default:-1` tag here because GORM would
	// then skip the column when the caller sets it to 0
	// (the zero value), defeating the test that pins the
	// happy-path exit code.
	ExitCode    int        `gorm:"column:exit_code" json:"exit_code"`
}

// TableName pins the GORM-generated table name.
func (PipelineStepRun) TableName() string { return "pipeline_step_runs" }

// AllModels returns every GORM model this package owns. The
// pipeline.AutoMigrate entry point uses this so callers do
// not have to repeat the model list every time the package
// grows.
func AllModels() []any {
	return []any{
		&Pipeline{},
		&PipelineRun{},
		&PipelineStepRun{},
	}
}
