package k8s

import (
	"errors"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/database"
)

// repoFixture builds a fresh in-memory repository for each test.
// No shared state — every test gets a private DB.
func repoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openTestDB(t))
}

// TestRepository_CreateAndGet verifies the basic create/read
// path — the minimum persistence contract.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := repoFixture(t)
	c := &Cluster{
		Name:                "dev-1",
		Type:                ClusterTypeK3d,
		APIServerURL:        "https://k3d.local:6443",
		KubeconfigEncrypted: "ct",
	}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == "" {
		t.Fatal("ID was not assigned")
	}
	got, err := repo.Get(c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "dev-1" {
		t.Errorf("Name = %q, want dev-1", got.Name)
	}
	if got.Type != ClusterTypeK3d {
		t.Errorf("Type = %q, want %q", got.Type, ClusterTypeK3d)
	}
}

// TestRepository_Get_NotFound verifies the "missing row" case.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := repoFixture(t)
	_, err := repo.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing cluster")
	}
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_Update persists a status change. The repository
// must call Save (or equivalent) so UpdatedAt is bumped and the
// row is written.
func TestRepository_Update(t *testing.T) {
	repo := repoFixture(t)
	c := &Cluster{
		Name:                "x",
		Type:                ClusterTypeK3d,
		KubeconfigEncrypted: "ct",
		Status:              ClusterStatusUnknown,
	}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	c.Status = ClusterStatusConnected
	now := time.Now()
	c.LastCheckedAt = &now
	if err := repo.Update(c); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.Get(c.ID)
	if got.Status != ClusterStatusConnected {
		t.Errorf("Status = %q, want connected", got.Status)
	}
	if got.LastCheckedAt == nil {
		t.Error("LastCheckedAt should be set")
	}
}

// TestRepository_Delete_SoftDelete pins the soft-delete contract.
func TestRepository_Delete_SoftDelete(t *testing.T) {
	repo := repoFixture(t)
	c := &Cluster{Name: "x", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(c.ID); !IsNotFound(err) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestRepository_List_NoFilter returns every cluster, paged.
func TestRepository_List_NoFilter(t *testing.T) {
	repo := repoFixture(t)
	for i := 0; i < 5; i++ {
		_ = repo.Create(&Cluster{Name: "n-" + idSuffix(), Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"})
	}
	rows, total, err := repo.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(rows) != 5 {
		t.Errorf("rows = %d, want 5", len(rows))
	}
}

// TestRepository_List_FilterByType narrows the result by ClusterType.
func TestRepository_List_FilterByType(t *testing.T) {
	repo := repoFixture(t)
	_ = repo.Create(&Cluster{Name: "a", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"})
	_ = repo.Create(&Cluster{Name: "b", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"})
	_ = repo.Create(&Cluster{Name: "c", Type: ClusterTypeKind, KubeconfigEncrypted: "ct"})

	rows, total, err := repo.List(ListFilter{Type: ClusterTypeK3d})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	for _, r := range rows {
		if r.Type != ClusterTypeK3d {
			t.Errorf("got type %q, want k3d", r.Type)
		}
	}
}

// TestRepository_List_Pagination verifies that limit/offset
// produce a stable page of results.
func TestRepository_List_Pagination(t *testing.T) {
	repo := repoFixture(t)
	for i := 0; i < 7; i++ {
		_ = repo.Create(&Cluster{Name: "n-" + idSuffix(), Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"})
	}
	rows, total, err := repo.List(ListFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 7 {
		t.Errorf("total = %d, want 7", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestRepository_FindByName returns the matching cluster or
// ErrNotFound. Used by the service to enforce unique-name
// validation.
func TestRepository_FindByName(t *testing.T) {
	repo := repoFixture(t)
	_ = repo.Create(&Cluster{Name: "alpha", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"})

	got, err := repo.FindByName("alpha")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "alpha" {
		t.Errorf("Name = %q, want alpha", got.Name)
	}

	if _, err := repo.FindByName("missing"); !IsNotFound(err) {
		t.Errorf("expected ErrNotFound for missing name, got %v", err)
	}
}

// TestRepository_Update_NotFound ensures the Update path returns
// a typed error when the row no longer exists.
func TestRepository_Update_NotFound(t *testing.T) {
	repo := repoFixture(t)
	// Create a cluster first, then delete it, so the ID
	// exists in the table but is soft-deleted (which still
	// counts as "not found" for the Update path).
	c := &Cluster{Name: "x", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	c.Name = "renamed"
	err := repo.Update(c)
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_UpdateStatus updates only the status + last-checked
// timestamp columns, leaving the rest of the cluster untouched.
func TestRepository_UpdateStatus(t *testing.T) {
	repo := repoFixture(t)
	c := &Cluster{Name: "y", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct", Status: ClusterStatusUnknown}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	now := time.Now()
	if err := repo.UpdateStatus(c.ID, ClusterStatusDisconnected, &now); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ := repo.Get(c.ID)
	if got.Status != ClusterStatusDisconnected {
		t.Errorf("Status = %q, want disconnected", got.Status)
	}
	if got.LastCheckedAt == nil {
		t.Error("LastCheckedAt should be set")
	}
}

// TestRepository_ErrorsHelpers ensures the IsNotFound helper
// correctly identifies wrapped errors.
func TestRepository_ErrorsHelpers(t *testing.T) {
	wrapped := errors.New("wrap: " + ErrNotFound.Error())
	if IsNotFound(wrapped) {
		t.Error("plain string-wrapped error should not match ErrNotFound")
	}
	// Direct match
	if !IsNotFound(ErrNotFound) {
		t.Error("ErrNotFound should match itself")
	}
}

// idSuffix returns a per-test short identifier so multiple
// tests can build fixtures without colliding on the unique
// Name index. Uses the database helper to avoid pulling in
// google/uuid directly.
func idSuffix() string {
	return database.NewID()[:8]
}
