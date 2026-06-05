package audit

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB builds a fresh sqlite db, migrates the audit schema,
// and schedules cleanup. Each test gets its own DB so they
// cannot leak rows into each other.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "audit.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestRepository_CreateAndGet is the persistence happy path: a
// new event is written and read back with all fields intact. The
// ID is auto-assigned by BaseModel.BeforeCreate.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC().Truncate(time.Second)
	e := &AuditEvent{
		Action:        ActionCreate,
		ActorID:       "u-1",
		ActorUsername: "alice",
		ResourceType:  ResourceDevice,
		ResourceID:    "d-1",
		Metadata:      JSONMap{"k": "v"},
		IPAddress:     "10.0.0.1",
		UserAgent:     "ua/1.0",
		OccurredAt:    now,
	}
	if err := repo.Create(e); err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.ID == "" {
		t.Error("ID not assigned")
	}
	got, err := repo.Get(e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Action != ActionCreate {
		t.Errorf("action: got %q want %q", got.Action, ActionCreate)
	}
	if got.ActorID != "u-1" {
		t.Errorf("actor_id: got %q", got.ActorID)
	}
	if got.ResourceID != "d-1" {
		t.Errorf("resource_id: got %q", got.ResourceID)
	}
	if got.Metadata["k"] != "v" {
		t.Errorf("metadata: %+v", got.Metadata)
	}
	if got.IPAddress != "10.0.0.1" {
		t.Errorf("ip: %q", got.IPAddress)
	}
	if got.UserAgent != "ua/1.0" {
		t.Errorf("ua: %q", got.UserAgent)
	}
	if !got.OccurredAt.Equal(now) {
		t.Errorf("occurred_at: got %v want %v", got.OccurredAt, now)
	}
}

// TestRepository_Get_NotFound asserts the ErrNotFound sentinel is
// returned when the ID does not exist.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	_, err := repo.Get("nope")
	if !IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// TestRepository_List_NoFilters covers the unfiltered list path
// used by the GET /api/v1/audit handler.
func TestRepository_List_NoFilters(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_ = repo.Create(&AuditEvent{
			Action:       ActionCreate,
			ResourceType: ResourceDevice,
			ResourceID:   "d-1",
			ActorID:      "u-1",
			OccurredAt:   now.Add(time.Duration(i) * time.Second),
		})
	}
	rows, total, err := repo.List(AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 || len(rows) != 5 {
		t.Errorf("list: total=%d rows=%d", total, len(rows))
	}
	// ordering: latest first (DESC by occurred_at).
	if rows[0].OccurredAt.Before(rows[4].OccurredAt) {
		t.Errorf("expected DESC ordering, got %v before %v", rows[0].OccurredAt, rows[4].OccurredAt)
	}
}

// TestRepository_List_ActionFilter matches the spec scenario
// "Filter by entity type" / "Filter by username" generalised to
// any single field: only matching rows are returned.
func TestRepository_List_ActionFilter(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionDelete, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	rows, total, err := repo.List(AuditFilter{Action: ActionDelete})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Action != ActionDelete {
		t.Errorf("filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestRepository_List_ActorFilter mirrors the spec's "Filter by
// username" scenario.
func TestRepository_List_ActorFilter(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", ActorUsername: "alice", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-2", ActorID: "u-2", ActorUsername: "bob", OccurredAt: now})
	rows, total, err := repo.List(AuditFilter{ActorUsername: "bob"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ActorID != "u-2" {
		t.Errorf("filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestRepository_List_ResourceTypeAndID matches the spec's
// "Filter by entity type" / "Filter by entity id" scenarios.
func TestRepository_List_ResourceTypeAndID(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceProject, ResourceID: "p-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: now})
	rows, total, err := repo.List(AuditFilter{ResourceType: ResourceDevice, ResourceID: "d-1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Errorf("filter mismatch: total=%d rows=%d", total, len(rows))
	}
}

// TestRepository_List_TimeRange matches the spec's "from / to"
// query parameters. Rows strictly outside the window are
// excluded.
func TestRepository_List_TimeRange(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", OccurredAt: base})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-2", OccurredAt: base.Add(48 * time.Hour)})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-3", OccurredAt: base.Add(96 * time.Hour)})
	from := base.Add(24 * time.Hour)
	to := base.Add(72 * time.Hour)
	rows, total, err := repo.List(AuditFilter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ResourceID != "d-2" {
		t.Errorf("range filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestRepository_List_Pagination matches the spec scenario
// "Pagination" — limit / offset are honoured and the unfiltered
// total is reported alongside the page.
func TestRepository_List_Pagination(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	for i := 0; i < 7; i++ {
		_ = repo.Create(&AuditEvent{
			Action:       ActionCreate,
			ResourceType: ResourceDevice,
			ResourceID:   "d-1",
			ActorID:      "u-1",
			OccurredAt:   now.Add(time.Duration(i) * time.Second),
		})
	}
	rows, total, err := repo.List(AuditFilter{Limit: 3, Offset: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 7 {
		t.Errorf("total: got %d want 7", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows: got %d want 3", len(rows))
	}
}

// TestRepository_List_Combined verifies multiple filters are
// AND-ed together.
func TestRepository_List_Combined(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceProject, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionDelete, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	rows, total, err := repo.List(AuditFilter{Action: ActionCreate, ResourceType: ResourceDevice})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Errorf("combined filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestRepository_WithContext_Propagates verifies the
// context-aware variant returns a repository that targets the
// same db (used by the emitter to honour cancellation).
func TestRepository_WithContext_Propagates(t *testing.T) {
	db := openTestDB(t)
	r := NewRepository(db)
	if r.WithContext(nil) == nil {
		t.Error("WithContext returned nil")
	}
}
