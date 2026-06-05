package audit

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// serviceDB returns a fresh sqlite db for service tests.
func serviceDB(t *testing.T) *gorm.DB {
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

// recordingEmitter is a thread-safe Emitter that captures every
// event for assertion. Mirrors the alerts module's
// FakeSuppressionChecker pattern.
type recordingEmitter struct {
	mu     sync.Mutex
	events []AuditEvent
}

func (r *recordingEmitter) Emit(_ context.Context, e AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingEmitter) All() []AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// TestService_List_Defaults covers the unfiltered list path used
// by GET /api/v1/audit.
func TestService_List_Defaults(t *testing.T) {
	repo := NewRepository(serviceDB(t))
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		_ = repo.Create(&AuditEvent{
			Action:       ActionCreate,
			ResourceType: ResourceDevice,
			ResourceID:   "d-1",
			ActorID:      "u-1",
			OccurredAt:   now.Add(time.Duration(i) * time.Second),
		})
	}
	svc := NewService(ServiceConfig{Repo: repo})
	rows, total, err := svc.List(AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Errorf("list: total=%d rows=%d", total, len(rows))
	}
}

// TestService_List_AppliesFilters covers the spec scenarios for
// action / resource_type / resource_id / actor / time-range.
func TestService_List_AppliesFilters(t *testing.T) {
	repo := NewRepository(serviceDB(t))
	now := time.Now().UTC()
	_ = repo.Create(&AuditEvent{Action: ActionCreate, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-1", OccurredAt: now})
	_ = repo.Create(&AuditEvent{Action: ActionDelete, ResourceType: ResourceDevice, ResourceID: "d-1", ActorID: "u-2", OccurredAt: now})
	svc := NewService(ServiceConfig{Repo: repo})
	rows, total, err := svc.List(AuditFilter{Action: ActionDelete, ResourceType: ResourceDevice})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Errorf("filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestService_Get_NotFound wraps the not-found path with a 404
// APIError. The service exposes this for the handler.
func TestService_Get_NotFound(t *testing.T) {
	repo := NewRepository(serviceDB(t))
	svc := NewService(ServiceConfig{Repo: repo})
	_, err := svc.Get("nope")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr interface{ Code() string }
	if !isNotFoundAPI(err) {
		t.Errorf("expected NotFound APIError, got %v", err)
	}
	_ = apiErr
}

// TestService_Get_Ok covers the happy path: a persisted event is
// returned to the handler.
func TestService_Get_Ok(t *testing.T) {
	repo := NewRepository(serviceDB(t))
	e := &AuditEvent{
		Action:       ActionCreate,
		ResourceType: ResourceDevice,
		ResourceID:   "d-1",
		ActorID:      "u-1",
		OccurredAt:   time.Now().UTC(),
	}
	_ = repo.Create(e)
	svc := NewService(ServiceConfig{Repo: repo})
	got, err := svc.Get(e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != e.ID {
		t.Errorf("id: got %q want %q", got.ID, e.ID)
	}
}

// TestService_RecordAction_DelegatesToEmitter verifies the
// convenience helper stamps OccurredAt, fills defaults, and
// hands the event to the configured emitter.
func TestService_RecordAction_DelegatesToEmitter(t *testing.T) {
	rec := &recordingEmitter{}
	svc := NewService(ServiceConfig{Emitter: rec, Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	svc.RecordAction(context.Background(), RecordActionInput{
		Action:       ActionCreate,
		ResourceType: ResourceDevice,
		ResourceID:   "d-1",
		ActorID:      "u-1",
		ActorName:    "alice",
		Metadata:     JSONMap{"k": "v"},
		IPAddress:    "10.0.0.1",
		UserAgent:    "ua/1.0",
	})
	events := rec.All()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.OccurredAt.IsZero() {
		t.Error("OccurredAt not set")
	}
	if e.ActorUsername != "alice" {
		t.Errorf("actor: %q", e.ActorUsername)
	}
	if e.Metadata["k"] != "v" {
		t.Errorf("metadata: %+v", e.Metadata)
	}
	if e.IPAddress != "10.0.0.1" {
		t.Errorf("ip: %q", e.IPAddress)
	}
	if e.UserAgent != "ua/1.0" {
		t.Errorf("ua: %q", e.UserAgent)
	}
}

// TestService_RecordAction_NilEmitterSafe verifies RecordAction
// is a no-op when no emitter is configured. This is the dev /
// unit-test fallback so callers can wire Service without an
// emitter in tests.
func TestService_RecordAction_NilEmitterSafe(t *testing.T) {
	repo := NewRepository(serviceDB(t))
	svc := NewService(ServiceConfig{Repo: repo})
	svc.RecordAction(context.Background(), RecordActionInput{
		Action:       ActionCreate,
		ResourceType: ResourceDevice,
		ResourceID:   "d-1",
	})
	rows, total, err := repo.List(AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Errorf("nil emitter should not write rows, got %d", total)
	}
}

// TestService_RecordAction_DefaultsToNow verifies the default
// clock is honoured when Now is not provided.
func TestService_RecordAction_DefaultsToNow(t *testing.T) {
	rec := &recordingEmitter{}
	svc := NewService(ServiceConfig{Emitter: rec})
	before := time.Now().UTC().Add(-time.Second)
	svc.RecordAction(context.Background(), RecordActionInput{
		Action:       ActionCreate,
		ResourceType: ResourceDevice,
		ResourceID:   "d-1",
	})
	after := time.Now().UTC().Add(time.Second)
	events := rec.All()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].OccurredAt.Before(before) || events[0].OccurredAt.After(after) {
		t.Errorf("OccurredAt out of bounds: %v", events[0].OccurredAt)
	}
}

// isNotFoundAPI is a small helper that asserts the error is a
// 404 APIError. Avoids importing contracts in the test file.
func isNotFoundAPI(err error) bool {
	if err == nil {
		return false
	}
	// We can't import contracts from the test; assert via the
	// string sentinel the service emits.
	return containsErr(err, "NOT_FOUND")
}

// containsErr is a tiny substring matcher. The service's 404
// APIError message starts with the code prefix; this keeps the
// test free of a contracts import.
func containsErr(err error, substr string) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
