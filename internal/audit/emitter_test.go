package audit

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB builds a fresh sqlite db, migrates the audit schema,
// and schedules cleanup. Local helper so emitter tests can
// construct a DBEmitter without importing the repository test
// helper.
func emitterDB(t *testing.T) *gorm.DB {
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

// sampleEvent returns a fully-populated event for tests.
func sampleEvent() AuditEvent {
	return AuditEvent{
		Action:        ActionCreate,
		ActorID:       "u-1",
		ActorUsername: "alice",
		ResourceType:  ResourceDevice,
		ResourceID:    "d-1",
		Metadata:      JSONMap{"k": "v"},
		IPAddress:     "10.0.0.1",
		UserAgent:     "ua/1.0",
		OccurredAt:    time.Now().UTC().Truncate(time.Second),
	}
}

// TestEmitter_DBEmitter_Persists verifies the production emitter
// writes the event to the database via the repository. The ID is
// expected to be assigned by BaseModel.BeforeCreate.
func TestEmitter_DBEmitter_Persists(t *testing.T) {
	repo := NewRepository(emitterDB(t))
	e := NewDBEmitter(repo)
	evt := sampleEvent()
	e.Emit(context.Background(), evt)
	// DBEmitter is synchronous so the row is visible immediately
	// after Emit returns.
	rows, total, err := repo.List(AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", total)
	}
	if rows[0].Action != ActionCreate {
		t.Errorf("action: got %q", rows[0].Action)
	}
	if rows[0].ResourceID != "d-1" {
		t.Errorf("resource_id: got %q", rows[0].ResourceID)
	}
}

// TestEmitter_NoopEmitter_Discarded verifies the no-op emitter
// accepts the event without error and does not write any row.
func TestEmitter_NoopEmitter_Discarded(t *testing.T) {
	repo := NewRepository(emitterDB(t))
	e := NoopEmitter{}
	e.Emit(context.Background(), sampleEvent())
	rows, total, err := repo.List(AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Errorf("no-op should not write rows, got %d", total)
	}
}

// TestEmitter_BufferedEmitter_PassesThrough verifies the
// buffered emitter forwards every event to the inner emitter.
// This is the happy path: buffer is large enough.
func TestEmitter_BufferedEmitter_PassesThrough(t *testing.T) {
	var (
		mu    sync.Mutex
		seen  []AuditEvent
		inner = emitterFunc(func(_ context.Context, e AuditEvent) {
			mu.Lock()
			seen = append(seen, e)
			mu.Unlock()
		})
	)
	e := NewBufferedEmitter(inner, BufferedEmitterConfig{BufferSize: 4, Workers: 1})
	defer e.Close()
	for i := 0; i < 3; i++ {
		e.Emit(context.Background(), sampleEvent())
	}
	// Wait for the worker to drain.
	if err := waitFor(2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == 3
	}); err != nil {
		t.Fatalf("worker did not drain: %v (seen=%d)", err, len(seen))
	}
}

// TestEmitter_BufferedEmitter_DropsOnOverflow verifies the
// buffered emitter drops events when the buffer is full. The
// spec says "if buffer full, drop + log a warning".
func TestEmitter_BufferedEmitter_DropsOnOverflow(t *testing.T) {
	var (
		mu       sync.Mutex
		cnt      int
		inner    = emitterFunc(func(_ context.Context, _ AuditEvent) {
			mu.Lock()
			cnt++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		})
		emitted  atomic.Int64
		dropped  atomic.Int64
	)
	e := NewBufferedEmitter(inner, BufferedEmitterConfig{
		BufferSize: 1,
		Workers:    1,
		OnEmitted:  func() { emitted.Add(1) },
		OnDropped:  func() { dropped.Add(1) },
	})
	defer e.Close()
	// Saturate the buffer: the worker is slow, the channel is
	// size 1, so the third Emit must drop.
	for i := 0; i < 20; i++ {
		e.Emit(context.Background(), sampleEvent())
	}
	if dropped.Load() == 0 {
		t.Errorf("expected at least one drop, got 0 (cnt=%d)", cnt)
	}
	// We never assert emitted==20: that's the race-free upper
	// bound. The point of the test is that *some* drops happen
	// and the rest get through.
	if emitted.Load()+dropped.Load() != 20 {
		t.Errorf("emitted+dropped: got %d want 20", emitted.Load()+dropped.Load())
	}
}

// TestEmitter_BufferedEmitter_NonBlocking verifies Emit returns
// even when the buffer is full: it does NOT block on the inner
// emitter.
func TestEmitter_BufferedEmitter_NonBlocking(t *testing.T) {
	inner := emitterFunc(func(_ context.Context, _ AuditEvent) {
		time.Sleep(time.Second)
	})
	e := NewBufferedEmitter(inner, BufferedEmitterConfig{BufferSize: 1, Workers: 1})
	defer e.Close()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			e.Emit(context.Background(), sampleEvent())
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked despite non-blocking contract")
	}
}

// TestEmitter_BufferedEmitter_CloseIdempotent verifies Close is
// safe to call twice. This is the test-suite-friendly behaviour
// main.go relies on for graceful shutdown.
func TestEmitter_BufferedEmitter_CloseIdempotent(t *testing.T) {
	inner := emitterFunc(func(_ context.Context, _ AuditEvent) {})
	e := NewBufferedEmitter(inner, BufferedEmitterConfig{BufferSize: 2, Workers: 1})
	e.Close()
	e.Close() // must not panic
}

// TestEmitter_InterfaceConformance pins the public surface:
// every concrete type implements Emitter.
func TestEmitter_InterfaceConformance(t *testing.T) {
	var _ Emitter = NoopEmitter{}
	var _ Emitter = NewDBEmitter(NewRepository(emitterDB(t)))
	var _ Emitter = NewBufferedEmitter(NoopEmitter{}, BufferedEmitterConfig{BufferSize: 1, Workers: 1})
}

// emitterFunc lets tests build an Emitter from a function value.
// It is the small adaptation of http.HandlerFunc for Emitter.
type emitterFunc func(ctx context.Context, e AuditEvent)

func (f emitterFunc) Emit(ctx context.Context, e AuditEvent) { f(ctx, e) }

// waitFor polls cond every 10ms until it returns true or timeout
// elapses. Returns nil on success, the timeout error otherwise.
func waitFor(timeout time.Duration, cond func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	if cond() {
		return nil
	}
	return context.DeadlineExceeded
}
