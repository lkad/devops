package device

import (
	"testing"
)

// deviceRepoFixture is a slim wrapper around *gorm.DB so each test
// can have a fresh repository without re-importing the database
// package's test helpers.
func deviceRepoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openDeviceDB(t))
}

// TestRepository_CreateAndGet verifies the basic create/read path
// — the minimum persistence contract.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := deviceRepoFixture(t)
	d := &Device{
		Name:  "web-01",
		Type:  DeviceTypePhysicalHost,
		State: DeviceStateOnline,
	}
	if err := repo.Create(d); err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.ID == "" {
		t.Fatal("ID was not assigned")
	}

	got, err := repo.Get(d.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "web-01" {
		t.Errorf("Name = %q, want web-01", got.Name)
	}
	if got.Type != DeviceTypePhysicalHost {
		t.Errorf("Type = %q, want %q", got.Type, DeviceTypePhysicalHost)
	}
}

// TestRepository_Get_NotFound verifies the "missing row" case.
// The service layer maps this to a 404 NOT_FOUND, so the error
// must be distinguishable from random DB errors.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := deviceRepoFixture(t)
	_, err := repo.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing device")
	}
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_Update persists a name change. The repository
// must call Save (or equivalent) so UpdatedAt is bumped and the
// row is written.
func TestRepository_Update(t *testing.T) {
	repo := deviceRepoFixture(t)
	d := &Device{Name: "before", Type: DeviceTypeContainer, State: DeviceStateOnline}
	if err := repo.Create(d); err != nil {
		t.Fatalf("create: %v", err)
	}
	d.Name = "after"
	if err := repo.Update(d); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.Get(d.ID)
	if got.Name != "after" {
		t.Errorf("Name = %q, want after", got.Name)
	}
}

// TestRepository_Delete_SoftDelete pins the soft-delete contract:
// the row is hidden from default queries and the call returns
// nil (the service decides which HTTP code).
func TestRepository_Delete_SoftDelete(t *testing.T) {
	repo := deviceRepoFixture(t)
	d := &Device{Name: "x", Type: DeviceTypeContainer, State: DeviceStateOnline}
	if err := repo.Create(d); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(d.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(d.ID); !IsNotFound(err) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestRepository_List_NoFilter returns every device, paged.
func TestRepository_List_NoFilter(t *testing.T) {
	repo := deviceRepoFixture(t)
	for i := 0; i < 5; i++ {
		_ = repo.Create(&Device{Name: "n", Type: DeviceTypeContainer, State: DeviceStateOnline})
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

// TestRepository_List_FilterByType narrows the result by DeviceType.
func TestRepository_List_FilterByType(t *testing.T) {
	repo := deviceRepoFixture(t)
	_ = repo.Create(&Device{Name: "p1", Type: DeviceTypePhysicalHost, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "p2", Type: DeviceTypePhysicalHost, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "c1", Type: DeviceTypeContainer, State: DeviceStateOnline})

	rows, total, err := repo.List(ListFilter{Type: DeviceTypePhysicalHost})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	for _, r := range rows {
		if r.Type != DeviceTypePhysicalHost {
			t.Errorf("got type %q, want physical_host", r.Type)
		}
	}
}

// TestRepository_List_FilterByState narrows the result by DeviceState.
func TestRepository_List_FilterByState(t *testing.T) {
	repo := deviceRepoFixture(t)
	_ = repo.Create(&Device{Name: "a", Type: DeviceTypeContainer, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "b", Type: DeviceTypeContainer, State: DeviceStateOffline})
	_ = repo.Create(&Device{Name: "c", Type: DeviceTypeContainer, State: DeviceStateOffline})

	rows, total, err := repo.List(ListFilter{State: DeviceStateOffline})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	for _, r := range rows {
		if r.State != DeviceStateOffline {
			t.Errorf("got state %q, want offline", r.State)
		}
	}
}

// TestRepository_List_FilterByGroupID narrows the result by group FK.
func TestRepository_List_FilterByGroupID(t *testing.T) {
	repo := deviceRepoFixture(t)
	gid := "grp-1"
	_ = repo.Create(&Device{Name: "a", Type: DeviceTypeContainer, State: DeviceStateOnline, GroupID: &gid})
	_ = repo.Create(&Device{Name: "b", Type: DeviceTypeContainer, State: DeviceStateOnline, GroupID: &gid})
	_ = repo.Create(&Device{Name: "c", Type: DeviceTypeContainer, State: DeviceStateOnline})

	rows, total, err := repo.List(ListFilter{GroupID: "grp-1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

// TestRepository_List_Pagination verifies that limit/offset
// produce a stable page of results and that the count is the
// pre-pagination total.
func TestRepository_List_Pagination(t *testing.T) {
	repo := deviceRepoFixture(t)
	for i := 0; i < 7; i++ {
		_ = repo.Create(&Device{Name: "n", Type: DeviceTypeContainer, State: DeviceStateOnline})
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

// TestRepository_Search runs a case-insensitive substring match
// against Name.
func TestRepository_Search(t *testing.T) {
	repo := deviceRepoFixture(t)
	_ = repo.Create(&Device{Name: "WebServer-01", Type: DeviceTypeContainer, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "Database-01", Type: DeviceTypeContainer, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "Cache-01", Type: DeviceTypeContainer, State: DeviceStateOnline})

	rows, total, err := repo.Search("webserver")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(rows) != 1 || rows[0].Name != "WebServer-01" {
		t.Errorf("rows = %+v", rows)
	}
}

// TestRepository_Update_NotFound ensures the Update path returns
// a typed error when the row no longer exists (it was deleted
// between the read and the write).
func TestRepository_Update_NotFound(t *testing.T) {
	repo := deviceRepoFixture(t)
	d := &Device{Name: "x", Type: DeviceTypeContainer, State: DeviceStateOnline}
	d.ID = "missing"
	err := repo.Update(d)
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_List_SearchIncludesSearchFilter verifies that
// ListFilter.Search translates to a LIKE query, integrated into
// the standard List path so the handler can use one method.
func TestRepository_List_SearchIncludesSearchFilter(t *testing.T) {
	repo := deviceRepoFixture(t)
	_ = repo.Create(&Device{Name: "alpha", Type: DeviceTypeContainer, State: DeviceStateOnline})
	_ = repo.Create(&Device{Name: "beta", Type: DeviceTypeContainer, State: DeviceStateOnline})

	rows, total, err := repo.List(ListFilter{Search: "alp"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Name != "alpha" {
		t.Errorf("rows = %+v total=%d", rows, total)
	}
	_ = rows
	_ = total
}
