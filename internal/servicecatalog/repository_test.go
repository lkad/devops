// servicecatalog is the Service entity, repo, service, and
// handler for the microservice-on-call workflow. See
// openspec/specs/service-catalog/spec.md and
// docs/design/service-layer.md for the design.
//
// Tests in this package follow the project's standard
// pattern: each test opens a fresh in-memory sqlite,
// runs AutoMigrate, exercises the layer under test, and
// closes the DB on Cleanup.
package servicecatalog

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB builds a fresh in-memory sqlite, migrates the
// Service schema, and registers a Cleanup. Shared with the
// other test files in the package so each test gets a clean
// schema.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:sc-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Service{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// TestRepository_CreateAndGet is the first failing test. A
// Service row is inserted via Repository.Create, then read
// back via Repository.Get. The two rows must match.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := NewRepository(openTestDB(t))

	in := &Service{
		Name:        "payments-api",
		Description: "Payments HTTP API",
		Owner:       "team-payments@example.com",
		RepositoryURL: "https://git.example.com/payments-api",
		Tier:        TierCritical,
	}
	if err := repo.Create(in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if in.ID == "" {
		t.Fatalf("Create did not assign ID")
	}
	if in.CreatedAt.IsZero() {
		t.Fatalf("Create did not set CreatedAt")
	}

	out, err := repo.Get(in.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Name != in.Name || out.Tier != in.Tier || out.Owner != in.Owner {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

// TestRepository_Get_NotFound pins the typed error so the
// service layer can wrap it in a 404 APIError.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	_, err := repo.Get("does-not-exist")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("err = %v, want IsNotFound", err)
	}
}

// TestRepository_Create_DuplicateName pins the unique-name
// constraint; the spec requires a 409 on conflict.
func TestRepository_Create_DuplicateName(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	a := &Service{Name: "dupe", Tier: TierStandard}
	b := &Service{Name: "dupe", Tier: TierStandard}
	if err := repo.Create(a); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := repo.Create(b)
	if err == nil {
		t.Fatalf("expected duplicate-name error, got nil")
	}
}

// TestRepository_Update covers the partial-update path: only
// the fields set on the input change, others are preserved.
func TestRepository_Update(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	in := &Service{Name: "orig", Description: "before", Tier: TierStandard}
	if err := repo.Create(in); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Detach to avoid GORM "primary key zero" surprise on
	// re-Update with the same pointer.
	stored, _ := repo.Get(in.ID)
	stored.Description = "after"
	// Tiny sleep so UpdatedAt > CreatedAt in the assertion.
	time.Sleep(2 * time.Millisecond)
	if err := repo.Update(stored); err != nil {
		t.Fatalf("update: %v", err)
	}
	out, _ := repo.Get(in.ID)
	if out.Description != "after" {
		t.Errorf("description = %q, want %q", out.Description, "after")
	}
	if out.Tier != TierStandard {
		t.Errorf("tier changed unexpectedly: %q", out.Tier)
	}
	if !out.UpdatedAt.After(out.CreatedAt) {
		t.Errorf("UpdatedAt (%v) should be > CreatedAt (%v)", out.UpdatedAt, out.CreatedAt)
	}
}

// TestRepository_SoftDelete pins that DELETE soft-deletes
// (deleted_at is set) and that subsequent Get / List do not
// return the row.
func TestRepository_SoftDelete(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	in := &Service{Name: "to-delete", Tier: TierStandard}
	if err := repo.Create(in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.SoftDelete(in.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.Get(in.ID); !IsNotFound(err) {
		t.Errorf("after soft delete, Get should be NotFound; got %v", err)
	}
	rows, _ := repo.List(ServiceFilter{})
	for _, r := range rows {
		if r.ID == in.ID {
			t.Errorf("soft-deleted row still in List: %+v", r)
		}
	}
}

// TestRepository_List_FilterTier pins the tier filter used
// by the on-call dashboard.
func TestRepository_List_FilterTier(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	for _, s := range []*Service{
		{Name: "a", Tier: TierCritical},
		{Name: "b", Tier: TierImportant},
		{Name: "c", Tier: TierCritical},
	} {
		if err := repo.Create(s); err != nil {
			t.Fatalf("create %s: %v", s.Name, err)
		}
	}
	rows, err := repo.List(ServiceFilter{Tier: TierCritical})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("got %d rows, want 2 (both critical)", len(rows))
	}
	for _, r := range rows {
		if r.Tier != TierCritical {
			t.Errorf("non-critical row in tier=critical result: %+v", r)
		}
	}
}
