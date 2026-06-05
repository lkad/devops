package physicalhost

import (
	"errors"
	"testing"
)

// repoFixture builds a Repository backed by a fresh in-memory DB.
// The repo is the only seam the service layer talks to, so a
// single fixture covers the full test surface.
func repoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openDB(t))
}

// TestRepository_CreateAndGet is the minimum persistence contract.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := repoFixture(t)
	p := &PhysicalHost{
		DeviceID:  "dev-1",
		IPAddress: "10.0.0.10",
		SSHPort:   22,
		SSHUser:   "root",
	}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID was not assigned")
	}

	got, err := repo.Get(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DeviceID != "dev-1" {
		t.Errorf("DeviceID = %q, want dev-1", got.DeviceID)
	}
	if got.IPAddress != "10.0.0.10" {
		t.Errorf("IPAddress = %q, want 10.0.0.10", got.IPAddress)
	}
}

// TestRepository_Get_NotFound must return the typed sentinel.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := repoFixture(t)
	_, err := repo.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing physical host")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_Update persists field changes.
func TestRepository_Update(t *testing.T) {
	repo := repoFixture(t)
	p := &PhysicalHost{DeviceID: "dev-1", IPAddress: "10.0.0.1", SSHUser: "root"}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	p.IPAddress = "10.0.0.2"
	if err := repo.Update(p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.Get(p.ID)
	if got.IPAddress != "10.0.0.2" {
		t.Errorf("IPAddress = %q, want 10.0.0.2", got.IPAddress)
	}
}

// TestRepository_Delete_SoftDelete pins the soft-delete contract.
func TestRepository_Delete_SoftDelete(t *testing.T) {
	repo := repoFixture(t)
	p := &PhysicalHost{DeviceID: "dev-1", IPAddress: "10.0.0.1", SSHUser: "root"}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestRepository_List_NoFilter returns every host paged.
func TestRepository_List_NoFilter(t *testing.T) {
	repo := repoFixture(t)
	for i := 0; i < 3; i++ {
		_ = repo.Create(&PhysicalHost{
			DeviceID:  uuidStr(t, i),
			IPAddress: "10.0.0.1",
			SSHUser:   "root",
		})
	}
	rows, total, err := repo.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestRepository_List_FilterByState narrows the result to one
// state — used by GET /api/v1/physical-hosts?state=maintenance.
func TestRepository_List_FilterByState(t *testing.T) {
	repo := repoFixture(t)
	_ = repo.Create(&PhysicalHost{DeviceID: "a", IPAddress: "10.0.0.1", SSHUser: "root", State: StateOnline})
	_ = repo.Create(&PhysicalHost{DeviceID: "b", IPAddress: "10.0.0.2", SSHUser: "root", State: StateMaintenance})
	_ = repo.Create(&PhysicalHost{DeviceID: "c", IPAddress: "10.0.0.3", SSHUser: "root", State: StateMaintenance})

	rows, total, err := repo.List(ListFilter{State: StateMaintenance})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	for _, r := range rows {
		if r.State != StateMaintenance {
			t.Errorf("got state %q, want maintenance", r.State)
		}
	}
}

// TestRepository_List_FilterByDeviceID looks up a host by its
// device FK — used by the join from the device-management layer.
func TestRepository_List_FilterByDeviceID(t *testing.T) {
	repo := repoFixture(t)
	_ = repo.Create(&PhysicalHost{DeviceID: "d1", IPAddress: "10.0.0.1", SSHUser: "root"})
	_ = repo.Create(&PhysicalHost{DeviceID: "d2", IPAddress: "10.0.0.2", SSHUser: "root"})

	rows, total, err := repo.List(ListFilter{DeviceID: "d2"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%d, want 1/1", total, len(rows))
	}
	if rows[0].DeviceID != "d2" {
		t.Errorf("DeviceID = %q, want d2", rows[0].DeviceID)
	}
}

// TestRepository_List_Pagination verifies limit/offset yield a
// stable page.
func TestRepository_List_Pagination(t *testing.T) {
	repo := repoFixture(t)
	for i := 0; i < 7; i++ {
		_ = repo.Create(&PhysicalHost{
			DeviceID:  uuidStr(t, i),
			IPAddress: "10.0.0.1",
			SSHUser:   "root",
		})
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

// TestRepository_Update_NotFound must return ErrNotFound when the
// row no longer exists.
func TestRepository_Update_NotFound(t *testing.T) {
	repo := repoFixture(t)
	created := &PhysicalHost{DeviceID: "tmp", IPAddress: "10.0.0.1", SSHUser: "root"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	err := repo.Update(created)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// uuidStr is a tiny helper that builds a unique device-id per test
// iteration so list-pagination and state-filter tests don't trip
// the unique constraint.
func uuidStr(t *testing.T, i int) string {
	t.Helper()
	return "dev-" + t.Name() + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
