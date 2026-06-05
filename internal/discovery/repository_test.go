package discovery

import (
	"testing"
	"time"
)

// discoveryRepoFixture builds a Repository backed by a fresh
// in-memory sqlite DB. Mirrors the device package fixture.
func discoveryRepoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openDiscoveryDB(t))
}

// TestRepository_CreateRun verifies the basic create / read
// path on DiscoveryRun. This is the minimum persistence
// contract.
func TestRepository_CreateRun(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{
		CIDR:       "10.0.0.0/24",
		StartedAt:  time.Now().UTC(),
		Status:     RunStatusRunning,
		HostsFound: 0,
	}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create: %v", err)
	}
	if run.ID == "" {
		t.Fatal("ID was not assigned")
	}
	got, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CIDR != "10.0.0.0/24" {
		t.Errorf("CIDR = %q, want 10.0.0.0/24", got.CIDR)
	}
}

// TestRepository_GetRun_NotFound pins the "missing row" case.
func TestRepository_GetRun_NotFound(t *testing.T) {
	repo := discoveryRepoFixture(t)
	_, err := repo.GetRun("missing")
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_UpdateRun covers the partial-update path.
// HostsFound and Status are the two fields the service layer
// sets when the scan completes.
func TestRepository_UpdateRun(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusRunning}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create: %v", err)
	}
	run.HostsFound = 7
	run.Status = RunStatusCompleted
	now := time.Now().UTC()
	run.CompletedAt = &now
	if err := repo.UpdateRun(run); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetRun(run.ID)
	if got.HostsFound != 7 {
		t.Errorf("HostsFound = %d, want 7", got.HostsFound)
	}
	if got.Status != RunStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt is nil after update")
	}
}

// TestRepository_ListRuns returns every run, paged. The
// pagination block is what the handler renders into the
// envelope.
func TestRepository_ListRuns(t *testing.T) {
	repo := discoveryRepoFixture(t)
	for i := 0; i < 5; i++ {
		_ = repo.CreateRun(&DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusRunning})
	}
	runs, total, err := repo.ListRuns(ListFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(runs) != 3 {
		t.Errorf("runs = %d, want 3", len(runs))
	}
}

// TestRepository_HostCRUD covers the per-host create / read /
// update / delete cycle. The promote flow updates
// PromotedToDeviceID on an existing host row.
func TestRepository_HostCRUD(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusCompleted}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	host := &DiscoveredHost{
		RunID:        run.ID,
		IPAddress:    "10.0.0.1",
		Hostname:     "host-1",
		OpenPorts:    JSONMap{"tcp": []int{22, 80}},
		SNMPSysDescr: "Linux",
	}
	if err := repo.CreateHost(host); err != nil {
		t.Fatalf("create host: %v", err)
	}
	if host.ID == "" {
		t.Fatal("ID was not assigned")
	}

	// Read back
	got, err := repo.GetHost(host.ID)
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	if got.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %q", got.IPAddress)
	}
	if got.SNMPSysDescr != "Linux" {
		t.Errorf("SNMPSysDescr = %q", got.SNMPSysDescr)
	}

	// Update PromotedToDeviceID
	devID := "device-uuid"
	got.PromotedToDeviceID = &devID
	if err := repo.UpdateHost(got); err != nil {
		t.Fatalf("update host: %v", err)
	}
	got2, _ := repo.GetHost(host.ID)
	if got2.PromotedToDeviceID == nil || *got2.PromotedToDeviceID != devID {
		t.Errorf("PromotedToDeviceID = %v, want %s", got2.PromotedToDeviceID, devID)
	}
}

// TestRepository_ListHostsByRun returns every host that
// belongs to a run, in the order they were created.
func TestRepository_ListHostsByRun(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusCompleted}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	for _, ip := range ips {
		_ = repo.CreateHost(&DiscoveredHost{RunID: run.ID, IPAddress: ip})
	}
	// Add a host from a different run — must not show up.
	other := &DiscoveryRun{CIDR: "192.168.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusCompleted}
	if err := repo.CreateRun(other); err != nil {
		t.Fatalf("create other run: %v", err)
	}
	_ = repo.CreateHost(&DiscoveredHost{RunID: other.ID, IPAddress: "192.168.0.1"})

	hosts, err := repo.ListHostsByRun(run.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(hosts) != 3 {
		t.Errorf("hosts = %d, want 3", len(hosts))
	}
}

// TestRepository_HostExistsByIP is the dedup key. The service
// layer uses it to skip hosts it has already seen across
// scans.
func TestRepository_HostExistsByIP(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusCompleted}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	_ = repo.CreateHost(&DiscoveredHost{RunID: run.ID, IPAddress: "10.0.0.1"})

	exists, err := repo.HostExistsByIP("10.0.0.1")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Error("expected 10.0.0.1 to exist")
	}
	exists, err = repo.HostExistsByIP("10.0.0.99")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists {
		t.Error("expected 10.0.0.99 to not exist")
	}
}

// TestRepository_DeleteRun_SoftDeleteAndCascades ensures that
// deleting a run also hides its hosts. The cascade is
// application-level because GORM's soft-delete does not
// automatically cascade.
func TestRepository_DeleteRun_SoftDeleteAndCascades(t *testing.T) {
	repo := discoveryRepoFixture(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusCompleted}
	if err := repo.CreateRun(run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	_ = repo.CreateHost(&DiscoveredHost{RunID: run.ID, IPAddress: "10.0.0.1"})
	_ = repo.CreateHost(&DiscoveredHost{RunID: run.ID, IPAddress: "10.0.0.2"})

	if err := repo.DeleteRun(run.ID); err != nil {
		t.Fatalf("delete run: %v", err)
	}
	if _, err := repo.GetRun(run.ID); !IsNotFound(err) {
		t.Errorf("expected ErrNotFound for run, got %v", err)
	}
	hosts, err := repo.ListHostsByRun(run.ID)
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected hosts to be soft-deleted, got %d", len(hosts))
	}
}
