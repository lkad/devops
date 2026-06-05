package hostproject

import (
	"testing"
	"time"

	"gorm.io/gorm"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
)

// newRepoWithDB wires a Repository over a fresh in-memory DB.
func newRepoWithDB(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	return NewRepository(db), db
}

// seedDevice inserts a Device row and returns it.
func seedDevice(t *testing.T, db *gorm.DB, name string) *devicepkg.Device {
	t.Helper()
	d := &devicepkg.Device{
		Name:  name,
		Type:  devicepkg.DeviceTypePhysicalHost,
		State: devicepkg.DeviceStateOnline,
	}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
	return d
}

// seedProject inserts a Project row (with a ProjectType) and
// returns the project.
func seedProject(t *testing.T, db *gorm.DB, code string, parent *projectpkg.Project) *projectpkg.Project {
	t.Helper()
	pt := &projectpkg.ProjectType{Name: code + "-type", Weight: 1}
	if err := db.Create(pt).Error; err != nil {
		t.Fatalf("seed project type: %v", err)
	}
	var parentID *string
	if parent != nil {
		id := parent.ID
		parentID = &id
	}
	p := &projectpkg.Project{
		Name:     code,
		Code:     code,
		TypeID:   pt.ID,
		ParentID: parentID,
		Weight:   1,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

// TestRepository_Create_Persists verifies a Create writes a
// row that can be read back. The (DeviceID, ProjectID) pair
// must be queryable; the Link timestamp and actor must
// survive the round-trip.
func TestRepository_Create_Persists(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev := seedDevice(t, db, "h1")
	prj := seedProject(t, db, "p1", nil)

	link, err := repo.Create(HostProjectLink{
		DeviceID:  dev.ID,
		ProjectID: prj.ID,
		LinkedBy:  "user-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if link.ID == "" {
		t.Fatal("expected non-empty ID after create")
	}
	if link.LinkedAt.IsZero() {
		t.Error("LinkedAt should be populated by the service")
	}
	// Read back via Get
	fetched, err := repo.Get(link.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.DeviceID != dev.ID || fetched.ProjectID != prj.ID {
		t.Errorf("fetched link = %+v, want device=%s project=%s", fetched, dev.ID, prj.ID)
	}
	if fetched.LinkedBy != "user-1" {
		t.Errorf("LinkedBy = %q, want user-1", fetched.LinkedBy)
	}
	if fetched.OrphanedAt != nil {
		t.Errorf("OrphanedAt = %v, want nil for fresh link", fetched.OrphanedAt)
	}
}

// TestRepository_Create_DuplicatePairConflicts verifies that a
// second create with the same (device, project) pair returns
// a conflict error. The unique index on the pair enforces it.
func TestRepository_Create_DuplicatePairConflicts(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev := seedDevice(t, db, "h1")
	prj := seedProject(t, db, "p1", nil)

	if _, err := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: prj.ID, LinkedBy: "u1"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: prj.ID, LinkedBy: "u2"})
	if err == nil {
		t.Fatal("expected conflict on duplicate (device, project) pair")
	}
	if !IsConflict(err) {
		t.Errorf("expected Conflict APIError, got %v", err)
	}
}

// TestRepository_ListByDevice_ExcludesOrphaned verifies that
// ListByDevice does not return links whose OrphanedAt is set.
// An orphaned link is invisible to the effective-link query.
func TestRepository_ListByDevice_ExcludesOrphaned(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev := seedDevice(t, db, "h1")
	p1 := seedProject(t, db, "p1", nil)
	p2 := seedProject(t, db, "p2", nil)

	l1, _ := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: p1.ID, LinkedBy: "u"})
	l2, _ := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: p2.ID, LinkedBy: "u"})

	// Manually orphan l1 only (simulates a row that was
	// already orphaned before the test scenario).
	now := time.Now().UTC()
	if err := db.Model(&HostProjectLink{}).Where("id = ?", l1.ID).Update("orphaned_at", now).Error; err != nil {
		t.Fatalf("set orphan: %v", err)
	}

	links, err := repo.ListByDevice(dev.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1 (non-orphaned)", len(links))
	}
	if links[0].ID != l2.ID {
		t.Errorf("links[0].ID = %s, want %s", links[0].ID, l2.ID)
	}
	if l2.ID == l1.ID {
		t.Fatal("ids collided; test invalid")
	}
}

// TestRepository_ListByDevice_AllOrphaned_ReturnsEmpty covers
// the boundary case where every link is orphaned.
func TestRepository_ListByDevice_AllOrphaned_ReturnsEmpty(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev := seedDevice(t, db, "h1")
	p1 := seedProject(t, db, "p1", nil)

	if _, err := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: p1.ID, LinkedBy: "u"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.OrphanByDevice(dev.ID); err != nil {
		t.Fatalf("orphan: %v", err)
	}
	links, err := repo.ListByDevice(dev.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

// TestRepository_ListByProject_ExcludesOrphaned verifies the
// project-side counterpart: orphaning on the device side
// hides the link from ListByProject.
func TestRepository_ListByProject_ExcludesOrphaned(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev1 := seedDevice(t, db, "h1")
	dev2 := seedDevice(t, db, "h2")
	prj := seedProject(t, db, "p1", nil)

	_, _ = repo.Create(HostProjectLink{DeviceID: dev1.ID, ProjectID: prj.ID, LinkedBy: "u"})
	_, _ = repo.Create(HostProjectLink{DeviceID: dev2.ID, ProjectID: prj.ID, LinkedBy: "u"})

	if err := repo.OrphanByDevice(dev1.ID); err != nil {
		t.Fatalf("orphan: %v", err)
	}
	links, err := repo.ListByProject(prj.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].DeviceID != dev2.ID {
		t.Errorf("links[0].DeviceID = %s, want %s", links[0].DeviceID, dev2.ID)
	}
}

// TestRepository_Delete_RemovesRow verifies that Delete
// actually removes the row, not just orphans it. A delete
// must be a hard delete because orphaning is the
// device-cascade semantic.
func TestRepository_Delete_RemovesRow(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev := seedDevice(t, db, "h1")
	prj := seedProject(t, db, "p1", nil)
	link, err := repo.Create(HostProjectLink{DeviceID: dev.ID, ProjectID: prj.ID, LinkedBy: "u"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(link.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(link.ID); err == nil {
		t.Fatal("expected NotFound after delete")
	} else if !IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// TestRepository_Delete_MissingReturnsNotFound covers the
// missing-row boundary on Delete.
func TestRepository_Delete_MissingReturnsNotFound(t *testing.T) {
	repo, _ := newRepoWithDB(t)
	err := repo.Delete("ghost")
	if err == nil {
		t.Fatal("expected NotFound on missing delete")
	}
	if !IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// TestRepository_OrphanByDevice_OnlyAffectsThatDevice ensures
// orphaning is scoped to a single device; other devices'
// links remain intact.
func TestRepository_OrphanByDevice_OnlyAffectsThatDevice(t *testing.T) {
	repo, db := newRepoWithDB(t)
	dev1 := seedDevice(t, db, "h1")
	dev2 := seedDevice(t, db, "h2")
	prj := seedProject(t, db, "p1", nil)

	_, _ = repo.Create(HostProjectLink{DeviceID: dev1.ID, ProjectID: prj.ID, LinkedBy: "u"})
	_, _ = repo.Create(HostProjectLink{DeviceID: dev2.ID, ProjectID: prj.ID, LinkedBy: "u"})

	if err := repo.OrphanByDevice(dev1.ID); err != nil {
		t.Fatalf("orphan: %v", err)
	}
	links, err := repo.ListByProject(prj.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1 (dev2's link)", len(links))
	}
	if links[0].DeviceID != dev2.ID {
		t.Errorf("links[0].DeviceID = %s, want %s", links[0].DeviceID, dev2.ID)
	}
}

// TestRepository_RelinkAfterOrphan is intentionally absent:
// the spec's unique index is composite on (device_id,
// project_id) and re-linking a previously-orphaned pair is
// not in scope. The orphaning semantic is "keep the row for
// audit, exclude from effective links", not "permit a fresh
// active row in parallel".
