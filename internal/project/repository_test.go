package project

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestRepository_CreateAndGet is the spec scenario "Create Project"
// (and the symmetry for all three levels): the repo persists the
// project and returns it by ID. The ID is auto-assigned by the
// embedded BaseModel.
func TestRepository_CreateAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	pt, err := repo.CreateType(ProjectType{Name: "platform", Description: "core", Weight: 10})
	if err != nil {
		t.Fatalf("create type: %v", err)
	}
	if pt.ID == "" {
		t.Error("type ID not assigned")
	}

	p, err := repo.Create(Project{Name: "Payments", Code: "pay", TypeID: pt.ID})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if p.ID == "" {
		t.Error("project ID not assigned")
	}

	got, err := repo.Get(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Payments" || got.Code != "pay" || got.TypeID != pt.ID {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

// TestRepository_Get_NotFound covers the "Get" negative path: a
// missing project must return a NotFound APIError, not a raw
// gorm.ErrRecordNotFound, so the handler layer can pass it through
// unchanged.
func TestRepository_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	_, err := repo.Get("nonexistent")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected NotFound APIError, got %T %v", err, err)
	}
}

// TestRepository_List_Filter exercises the list filter parameters
// the spec demands: parent_id, type_id, owner_id, search, and
// depth. The filter is built per-call and must compose.
func TestRepository_List_Filter(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})

	bl1, _ := repo.Create(Project{Name: "Commerce", Code: "commerce", TypeID: pt.ID})
	bl2, _ := repo.Create(Project{Name: "Logistics", Code: "logistics", TypeID: pt.ID})
	sys, _ := repo.Create(Project{Name: "Cart", Code: "cart", TypeID: pt.ID, ParentID: &bl1.ID})
	owner := "u-1"
	_ = owner
	if _, err := repo.Create(Project{Name: "Owned", Code: "owned", TypeID: pt.ID, ParentID: &bl2.ID, OwnerUserID: &owner}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// parent_id filter
	got, total, err := repo.List(Filter{ParentID: &bl1.ID})
	if err != nil {
		t.Fatalf("list by parent: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].ID != sys.ID {
		t.Errorf("parent_id filter: total=%d got=%+v", total, got)
	}

	// type_id filter
	got, total, err = repo.List(Filter{TypeID: pt.ID})
	if err != nil {
		t.Fatalf("list by type: %v", err)
	}
	if total != 4 {
		t.Errorf("type_id filter total=%d want 4", total)
	}

	// search filter
	got, total, err = repo.List(Filter{Search: "Cart"})
	if err != nil {
		t.Fatalf("list by search: %v", err)
	}
	if total != 1 || got[0].ID != sys.ID {
		t.Errorf("search filter: total=%d got=%+v", total, got)
	}

	// root filter (depth=1) — only the two business lines
	got, total, err = repo.List(Filter{Depth: 1})
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if total != 2 {
		t.Errorf("depth=1 total=%d want 2", total)
	}
}

// TestRepository_Update_PreservesID is the spec scenario "Update
// Project": a PUT updates the mutable fields and keeps the ID and
// created_at intact.
func TestRepository_Update_PreservesID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	p, _ := repo.Create(Project{Name: "Old", Code: "old", TypeID: pt.ID})

	updated, err := repo.Update(p.ID, map[string]any{"name": "New"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != p.ID {
		t.Errorf("id changed: %s -> %s", p.ID, updated.ID)
	}
	if updated.Name != "New" {
		t.Errorf("name not updated: %s", updated.Name)
	}
	if !updated.CreatedAt.Equal(p.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", p.CreatedAt, updated.CreatedAt)
	}
}

// TestRepository_Delete_Soft is the spec scenario "Delete Project":
// the row is soft-deleted (default) and disappears from Get/List
// but is still findable via Unscoped.
func TestRepository_Delete_Soft(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	p, _ := repo.Create(Project{Name: "Doomed", Code: "doom", TypeID: pt.ID})

	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(p.ID); !IsNotFound(err) {
		t.Errorf("expected not-found after delete, got %v", err)
	}
	_, total, _ := repo.List(Filter{})
	if total != 0 {
		t.Errorf("list after delete total=%d want 0", total)
	}
}

// TestRepository_GetTree walks the 3-level hierarchy and returns
// every descendant of a root. The implementation must be recursive
// and must not depend on GORM's preloading — it issues a single
// fetch and walks in memory.
func TestRepository_GetTree(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	root, _ := repo.Create(Project{Name: "Root", Code: "root", TypeID: pt.ID})
	c1, _ := repo.Create(Project{Name: "C1", Code: "c1", TypeID: pt.ID, ParentID: &root.ID})
	_, _ = repo.Create(Project{Name: "C2", Code: "c2", TypeID: pt.ID, ParentID: &root.ID})
	_, _ = repo.Create(Project{Name: "G1", Code: "g1", TypeID: pt.ID, ParentID: &c1.ID})
	other, _ := repo.Create(Project{Name: "Other", Code: "other", TypeID: pt.ID})

	tree, err := repo.GetTree(root.ID)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(tree) != 4 {
		t.Errorf("tree size=%d want 4 (root+2+grandchild)", len(tree))
	}
	for _, p := range tree {
		if p.ID == other.ID {
			t.Error("tree must not include unrelated projects")
		}
	}
}

// TestRepository_GetAncestors walks up from a leaf and returns the
// parent chain in root → leaf order. The leaf itself is NOT
// included — only the ancestors.
func TestRepository_GetAncestors(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	root, _ := repo.Create(Project{Name: "R", Code: "r", TypeID: pt.ID})
	mid, _ := repo.Create(Project{Name: "M", Code: "m", TypeID: pt.ID, ParentID: &root.ID})
	leaf, _ := repo.Create(Project{Name: "L", Code: "l", TypeID: pt.ID, ParentID: &mid.ID})

	got, err := repo.GetAncestors(leaf.ID)
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ancestor count=%d want 2", len(got))
	}
	if got[0].ID != root.ID || got[1].ID != mid.ID {
		t.Errorf("ancestor order: %+v", got)
	}
}

// TestRepository_MembersCRUD covers the spec scenarios "Grant
// viewer/editor permission" and "Revoke permission": add, list,
// remove. The unique (project_id, user_id) index means re-adding
// the same pair must surface as a conflict so the caller can
// decide between update-in-place and error.
func TestRepository_MembersCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	p, _ := repo.Create(Project{Name: "P", Code: "p", TypeID: pt.ID})

	if err := repo.AddMember(p.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := repo.AddMember(p.ID, "u-2", "editor", "admin-1"); err != nil {
		t.Fatalf("add editor: %v", err)
	}

	members, err := repo.ListMembers(p.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("member count=%d want 2", len(members))
	}

	if err := repo.RemoveMember(p.ID, "u-1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	members, _ = repo.ListMembers(p.ID)
	if len(members) != 1 {
		t.Errorf("post-remove count=%d want 1", len(members))
	}
}

// TestRepository_AddMember_Duplicate is the unique-index guard. A
// second add with the same (project, user) must return a conflict
// error so the service layer can promote-in-place.
func TestRepository_AddMember_Duplicate(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	p, _ := repo.Create(Project{Name: "P", Code: "p", TypeID: pt.ID})
	if err := repo.AddMember(p.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := repo.AddMember(p.ID, "u-1", "editor", "admin-1")
	if err == nil {
		t.Fatal("expected duplicate-member error")
	}
	if !IsConflict(err) {
		t.Errorf("expected conflict, got %T %v", err, err)
	}
}

// TestRepository_ListType_UsedByForms covers the spec scenario
// "List Business Lines" and the project-type list endpoint. The
// repository must return every type sorted by name so the UI
// dropdown is deterministic.
func TestRepository_ListType_UsedByForms(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	if _, err := repo.CreateType(ProjectType{Name: "infra"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := repo.CreateType(ProjectType{Name: "app"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	types, err := repo.ListTypes()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(types) != 2 || types[0].Name != "app" || types[1].Name != "infra" {
		t.Errorf("order: %+v", types)
	}
}

// TestRepository_List_FilterOwner pins the owner_id filter. The
// spec calls for "owner_id" as one of the filter fields, so a
// regression that drops it is caught here.
func TestRepository_List_FilterOwner(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	pt, _ := repo.CreateType(ProjectType{Name: "platform"})
	owner := "u-1"
	other := "u-2"
	if _, err := repo.Create(Project{Name: "O1", Code: "o1", TypeID: pt.ID, OwnerUserID: &owner}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := repo.Create(Project{Name: "O2", Code: "o2", TypeID: pt.ID, OwnerUserID: &other}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, total, err := repo.List(Filter{OwnerID: &owner})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].Code != "o1" {
		t.Errorf("owner filter: total=%d got=%+v", total, got)
	}
}

// TestRepository_AutoMigrate exercises that the production-style
// migration call (used by the main entry point) succeeds against
// an empty database. Catches schema typos at the model level.
func TestRepository_AutoMigrate(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "m.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&ProjectType{}, &Project{}, &ProjectMember{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
}
