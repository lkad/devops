package pipeline

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openPipelineDB builds a fresh in-memory sqlite DB, runs
// AutoMigrate for every model this package owns, and registers
// a Cleanup to close the underlying sql.DB. The shared-cache +
// one-connection pattern matches the device package so tests
// in this package and neighbouring packages do not deadlock.
func openPipelineDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:pipeline-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

// ptrTime is a small helper that returns a pointer to t. The
// run / step-run tables store StartedAt and FinishedAt as
// nullable columns; the helper avoids having to write &x{} at
// the call site.
func ptrTime(t time.Time) *time.Time { return &t }

// TestPipelineModel_AutoMigrate verifies that the schema lands
// without error. The model declarations must match the migration
// contract or every downstream test will misbehave.
func TestPipelineModel_AutoMigrate(t *testing.T) {
	db := openPipelineDB(t)
	for _, m := range AllModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
		if stmt.Schema == nil {
			t.Errorf("%T: schema is nil after parse", m)
		}
	}
}

// TestStepType_Valid pins the allowed values. The wire format
// is the constant string; renaming a value would be a breaking
// change for clients that branch on it.
func TestStepType_Valid(t *testing.T) {
	cases := []struct {
		t    StepType
		want bool
	}{
		{StepTypeShell, true},
		{StepTypeK8sApply, true},
		{StepTypeTerraform, true},
		{StepTypeHTTP, true},
		{StepType("python"), false},
		{StepType(""), false},
	}
	for _, c := range cases {
		if got := c.t.Valid(); got != c.want {
			t.Errorf("StepType(%q).Valid() = %v, want %v", c.t, got, c.want)
		}
	}
}

// TestTargetType_Valid pins the allowed values.
func TestTargetType_Valid(t *testing.T) {
	cases := []struct {
		t    TargetType
		want bool
	}{
		{TargetTypeDevice, true},
		{TargetTypeProject, true},
		{TargetTypeCluster, true},
		{TargetTypeWebhook, true},
		{TargetType("bogus"), false},
		{TargetType(""), false},
	}
	for _, c := range cases {
		if got := c.t.Valid(); got != c.want {
			t.Errorf("TargetType(%q).Valid() = %v, want %v", c.t, got, c.want)
		}
	}
}

// TestRunStatus_Valid covers the five run lifecycle states.
func TestRunStatus_Valid(t *testing.T) {
	for _, s := range []RunStatus{
		RunStatusPending, RunStatusRunning, RunStatusSucceeded,
		RunStatusFailed, RunStatusCancelled,
	} {
		if !s.Valid() {
			t.Errorf("RunStatus %q should be valid", s)
		}
	}
	for _, s := range []RunStatus{
		RunStatus("done"), RunStatus(""),
	} {
		if s.Valid() {
			t.Errorf("RunStatus %q should not be valid", s)
		}
	}
}

// TestRunStatus_Terminal reports whether a status is final
// (no more transitions allowed). Cancellation is terminal even
// though the pipeline is in flight.
func TestRunStatus_Terminal(t *testing.T) {
	terminal := []RunStatus{RunStatusSucceeded, RunStatusFailed, RunStatusCancelled}
	notTerminal := []RunStatus{RunStatusPending, RunStatusRunning}
	for _, s := range terminal {
		if !s.Terminal() {
			t.Errorf("RunStatus %q should be terminal", s)
		}
	}
	for _, s := range notTerminal {
		if s.Terminal() {
			t.Errorf("RunStatus %q should not be terminal", s)
		}
	}
}

// TestStepRunStatus_Valid covers the six step-run lifecycle states.
func TestStepRunStatus_Valid(t *testing.T) {
	for _, s := range []StepRunStatus{
		StepRunStatusPending, StepRunStatusRunning, StepRunStatusSucceeded,
		StepRunStatusFailed, StepRunStatusCancelled, StepRunStatusSkipped,
	} {
		if !s.Valid() {
			t.Errorf("StepRunStatus %q should be valid", s)
		}
	}
}

// TestStepRunStatus_Terminal mirrors the run-status test for step runs.
func TestStepRunStatus_Terminal(t *testing.T) {
	terminal := []StepRunStatus{
		StepRunStatusSucceeded, StepRunStatusFailed,
		StepRunStatusCancelled, StepRunStatusSkipped,
	}
	notTerminal := []StepRunStatus{StepRunStatusPending, StepRunStatusRunning}
	for _, s := range terminal {
		if !s.Terminal() {
			t.Errorf("StepRunStatus %q should be terminal", s)
		}
	}
	for _, s := range notTerminal {
		if s.Terminal() {
			t.Errorf("StepRunStatus %q should not be terminal", s)
		}
	}
}

// TestStepList_RoundTrip persists a slice of steps as a JSON
// column and reads it back. This is the per-step persistence
// contract: the wire shape is []PipelineStep, the storage shape
// is a JSON blob.
func TestStepList_RoundTrip(t *testing.T) {
	db := openPipelineDB(t)
	steps := StepList{
		{Name: "build", Type: StepTypeShell, Config: StepConfig{"cmd": "go build"}},
		{Name: "test", Type: StepTypeShell, Config: StepConfig{"cmd": "go test"}},
	}
	p := Pipeline{Name: "ci", ProjectID: "p-1", TargetType: TargetTypeProject, Steps: steps}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got Pipeline
	if err := db.First(&got, "id = ?", p.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(got.Steps))
	}
	if got.Steps[0].Name != "build" || got.Steps[0].Type != StepTypeShell {
		t.Errorf("Steps[0] = %+v", got.Steps[0])
	}
	if got.Steps[1].Config["cmd"] != "go test" {
		t.Errorf("Steps[1].Config[cmd] = %v", got.Steps[1].Config["cmd"])
	}
}

// TestStepConfig_NilRendersAsNULL pins the empty-config contract:
// a step with no config stored as nil in Go must land as NULL in
// the column so the consumer can distinguish "no config" from
// "empty config object".
func TestStepConfig_NilRendersAsNULL(t *testing.T) {
	// nil map -> driver returns nil
	var c StepConfig
	v, err := c.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if v != nil {
		t.Errorf("nil StepConfig.Value() = %v, want nil", v)
	}
}

// TestPipeline_RoundTrip walks a pipeline through create, read,
// update, soft-delete. This is the minimum persistence contract.
func TestPipeline_RoundTrip(t *testing.T) {
	db := openPipelineDB(t)
	p := Pipeline{
		Name:       "ci",
		ProjectID:  "p-1",
		TargetType: TargetTypeProject,
		Enabled:    true,
		Steps:      StepList{{Name: "build", Type: StepTypeShell}},
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID was not assigned")
	}

	var got Pipeline
	if err := db.First(&got, "id = ?", p.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Name != "ci" {
		t.Errorf("Name = %q, want ci", got.Name)
	}
	if got.ProjectID != "p-1" {
		t.Errorf("ProjectID = %q, want p-1", got.ProjectID)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}

	got.Name = "ci-2"
	got.Enabled = false
	if err := db.Save(&got).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "ci-2" {
		t.Errorf("post-update Name = %q", got.Name)
	}

	if err := db.Delete(&got).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.First(&Pipeline{}, "id = ?", p.ID).Error; err == nil {
		t.Error("soft-deleted pipeline should be hidden from default query")
	}
	if err := db.Unscoped().First(&Pipeline{}, "id = ?", p.ID).Error; err != nil {
		t.Errorf("soft-deleted pipeline should be visible to Unscoped: %v", err)
	}
}

// TestPipelineRun_RoundTrip pins the run + step-run storage
// contract. The model uses nullable timestamps so the service
// layer can record StartedAt at run kickoff and FinishedAt at
// completion.
func TestPipelineRun_RoundTrip(t *testing.T) {
	db := openPipelineDB(t)
	now := time.Now()
	run := PipelineRun{
		PipelineID:   "p-1",
		Status:       RunStatusRunning,
		StartedAt:    ptrTime(now),
		TriggeredBy:  "user-1",
		DurationMs:   0,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.ID == "" {
		t.Fatal("ID was not assigned")
	}

	stepRun := PipelineStepRun{
		RunID:    run.ID,
		StepName: "build",
		Status:   StepRunStatusSucceeded,
		StartedAt:  ptrTime(now),
		FinishedAt: ptrTime(now.Add(time.Second)),
		ExitCode: 0,
	}
	if err := db.Create(&stepRun).Error; err != nil {
		t.Fatalf("create step run: %v", err)
	}

	var got PipelineRun
	if err := db.First(&got, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.PipelineID != "p-1" {
		t.Errorf("PipelineID = %q, want p-1", got.PipelineID)
	}
	if got.Status != RunStatusRunning {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt = nil, want set")
	}

	var gotStep PipelineStepRun
	if err := db.First(&gotStep, "id = ?", stepRun.ID).Error; err != nil {
		t.Fatalf("read step run: %v", err)
	}
	if gotStep.StepName != "build" {
		t.Errorf("StepName = %q, want build", gotStep.StepName)
	}
	if gotStep.Status != StepRunStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", gotStep.Status)
	}
	if gotStep.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", gotStep.ExitCode)
	}
}

// TestPipelineRun_TimestampsNullable pins that FinishedAt is
// stored as NULL when the run is still in flight. A future
// optimization that switched it to a zero-time sentinel would
// break status-bar rendering.
func TestPipelineRun_TimestampsNullable(t *testing.T) {
	db := openPipelineDB(t)
	run := PipelineRun{
		PipelineID: "p-1",
		Status:     RunStatusPending,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got PipelineRun
	if err := db.First(&got, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.StartedAt != nil {
		t.Errorf("StartedAt should be nil, got %v", *got.StartedAt)
	}
	if got.FinishedAt != nil {
		t.Errorf("FinishedAt should be nil, got %v", *got.FinishedAt)
	}
}
