package discovery

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
)

// openDiscoveryWithDeviceDB returns a *gorm.DB with BOTH the
// discovery and the device models migrated. The service needs
// both because promote writes into the devices table.
func openDiscoveryWithDeviceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:disc-dev-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(append(AllModels(), devicepkg.AllModels()...)...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// discoveryServiceFixture wires the service with fakes for the
// Scanner and Prober. The device repository is real because
// promote must persist into the unified devices table.
func discoveryServiceFixture(t *testing.T, hosts []Host, results map[string]ProbeResult) (*Service, *Repository, *devicepkg.Repository) {
	t.Helper()
	db := openDiscoveryWithDeviceDB(t)
	dRepo := NewRepository(db)
	devRepo := devicepkg.NewRepository(db)
	s := NewService(dRepo, devRepo, NewFakeScanner(hosts, nil), NewFakeProber(results, nil))
	return s, dRepo, devRepo
}

// TestService_StartRun_RejectsEmptyCIDR pins the input
// validation: an empty CIDR is a 400, not a 500. The error
// must be a *contracts.APIError so the handler renders the
// standard envelope.
func TestService_StartRun_RejectsEmptyCIDR(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	_, err := svc.StartRun(context.Background(), StartRunInput{CIDR: ""})
	if err == nil {
		t.Fatal("expected validation error for empty CIDR")
	}
	var apiErr *apiErrorType
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
}

// TestService_StartRun_RejectsInvalidCIDR covers the second
// validation path: a syntactically bogus CIDR.
func TestService_StartRun_RejectsInvalidCIDR(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	_, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "not-a-cidr"})
	if err == nil {
		t.Fatal("expected validation error for invalid CIDR")
	}
}

// TestService_StartRun_HappyPath creates a run, runs the scan,
// probes each host, and persists the results. The hosts come
// from the FakeScanner; the probes come from the FakeProber.
func TestService_StartRun_HappyPath(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "host-1"},
		{IPAddress: "10.0.0.2", Hostname: "host-2"},
	}
	results := map[string]ProbeResult{
		"10.0.0.1": {OpenPorts: []int{22, 80}, SNMPSysDescr: "Linux", Reachable: true},
		"10.0.0.2": {OpenPorts: []int{161}, Reachable: true},
	}
	svc, repo, _ := discoveryServiceFixture(t, hosts, results)

	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Status != RunStatusCompleted {
		t.Errorf("Status = %q, want completed", run.Status)
	}
	if run.HostsFound != 2 {
		t.Errorf("HostsFound = %d, want 2", run.HostsFound)
	}
	if run.CompletedAt == nil {
		t.Error("CompletedAt should be set on completion")
	}

	persisted, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if persisted.Status != RunStatusCompleted {
		t.Errorf("persisted.Status = %q, want completed", persisted.Status)
	}

	dbHosts, err := repo.ListHostsByRun(run.ID)
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}
	if len(dbHosts) != 2 {
		t.Errorf("persisted hosts = %d, want 2", len(dbHosts))
	}
}

// TestService_StartRun_ScannerError is the failure path: when
// the scanner fails, the run is marked failed and no hosts
// are stored.
func TestService_StartRun_ScannerError(t *testing.T) {
	db := openDiscoveryWithDeviceDB(t)
	dRepo := NewRepository(db)
	devRepo := devicepkg.NewRepository(db)
	s := NewService(dRepo, devRepo, NewFakeScanner(nil, errors.New("network down")), NewFakeProber(nil, nil))

	run, err := s.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Status != RunStatusFailed {
		t.Errorf("Status = %q, want failed", run.Status)
	}
	hosts, _ := dRepo.ListHostsByRun(run.ID)
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts on failure, got %d", len(hosts))
	}
}

// TestService_StartRun_EmptyResult pins the empty-state
// transition: a scan that finds nothing is "empty", not
// "failed".
func TestService_StartRun_EmptyResult(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, []Host{}, nil)
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Status != RunStatusEmpty {
		t.Errorf("Status = %q, want empty", run.Status)
	}
	if run.HostsFound != 0 {
		t.Errorf("HostsFound = %d, want 0", run.HostsFound)
	}
}

// TestService_StartRun_DedupAcrossScans is the spec's
// "Result deduplication across scans" requirement. A host
// seen in an earlier scan is NOT re-inserted when a later
// scan reaches the same IP.
func TestService_StartRun_DedupAcrossScans(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "host-1"},
	}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	svc, repo, _ := discoveryServiceFixture(t, hosts, results)

	first, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.HostsFound != 1 {
		t.Errorf("first.HostsFound = %d, want 1", first.HostsFound)
	}

	// Second scan with the same IP — it must be deduped.
	second, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.HostsFound != 0 {
		t.Errorf("second.HostsFound = %d, want 0 (deduped)", second.HostsFound)
	}
	// And the first run's host is still the only one in the
	// table — we did not insert a duplicate.
	dbHosts, _ := repo.ListHostsByRun(first.ID)
	if len(dbHosts) != 1 {
		t.Errorf("first run hosts = %d, want 1", len(dbHosts))
	}
	dbHosts2, _ := repo.ListHostsByRun(second.ID)
	if len(dbHosts2) != 0 {
		t.Errorf("second run hosts = %d, want 0 (deduped)", len(dbHosts2))
	}
}

// TestService_GetRun covers the simple read path.
func TestService_GetRun(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "192.168.1.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	got, err := svc.GetRun(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("ID = %q, want %q", got.ID, run.ID)
	}
}

// TestService_GetRun_NotFound covers the 404 path.
func TestService_GetRun_NotFound(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	_, err := svc.GetRun("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing run")
	}
	var apiErr *apiErrorType
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
}

// TestService_ListRuns covers the list path.
func TestService_ListRuns(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	for i := 0; i < 3; i++ {
		_, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
		if err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
	}
	rows, total, err := svc.ListRuns(ListFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

// TestService_Promote_CreatesDevicesForAllHosts covers the
// spec's "Approve pending devices" scenario: a user promotes
// all hosts in a run, and the system creates a Device row for
// each.
func TestService_Promote_CreatesDevicesForAllHosts(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "host-1"},
		{IPAddress: "10.0.0.2", Hostname: "host-2"},
	}
	results := map[string]ProbeResult{
		"10.0.0.1": {OpenPorts: []int{22}, Reachable: true},
		"10.0.0.2": {OpenPorts: []int{161}, SNMPSysDescr: "Switch v1", Reachable: true},
	}
	svc, dRepo, devRepo := discoveryServiceFixture(t, hosts, results)

	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	promoted, err := svc.PromoteHosts(context.Background(), PromoteInput{RunID: run.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if len(promoted) != 2 {
		t.Fatalf("promoted = %d, want 2", len(promoted))
	}

	// Devices must be real rows in the devices table.
	for _, d := range promoted {
		if _, err := devRepo.Get(d.ID); err != nil {
			t.Errorf("device %s missing: %v", d.ID, err)
		}
	}

	// And every host row must now carry PromotedToDeviceID.
	dbHosts, _ := dRepo.ListHostsByRun(run.ID)
	if len(dbHosts) != 2 {
		t.Fatalf("persisted hosts = %d, want 2", len(dbHosts))
	}
	for _, h := range dbHosts {
		if h.PromotedToDeviceID == nil {
			t.Errorf("host %s has nil PromotedToDeviceID after promote", h.IPAddress)
		}
	}
}

// TestService_Promote_SelectedHostsOnly covers the body
// shape `{host_ids: [...]}` — a partial promote of a subset.
func TestService_Promote_SelectedHostsOnly(t *testing.T) {
	hosts := []Host{
		{IPAddress: "10.0.0.1", Hostname: "host-1"},
		{IPAddress: "10.0.0.2", Hostname: "host-2"},
	}
	results := map[string]ProbeResult{
		"10.0.0.1": {Reachable: true},
		"10.0.0.2": {Reachable: true},
	}
	svc, dRepo, _ := discoveryServiceFixture(t, hosts, results)
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	dbHosts, _ := dRepo.ListHostsByRun(run.ID)
	var firstID string
	for _, h := range dbHosts {
		if h.IPAddress == "10.0.0.1" {
			firstID = h.ID
			break
		}
	}
	if firstID == "" {
		t.Fatal("could not find host 10.0.0.1")
	}

	promoted, err := svc.PromoteHosts(context.Background(), PromoteInput{RunID: run.ID, HostIDs: []string{firstID}})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if len(promoted) != 1 {
		t.Errorf("promoted = %d, want 1", len(promoted))
	}

	// The second host must still be nil-promoted.
	dbHosts2, _ := dRepo.ListHostsByRun(run.ID)
	for _, h := range dbHosts2 {
		if h.IPAddress == "10.0.0.2" {
			if h.PromotedToDeviceID != nil {
				t.Errorf("10.0.0.2 was promoted, want nil")
			}
		}
	}
}

// TestService_Promote_SkipsAlreadyPromoted verifies the
// idempotency contract: re-promoting a host is a no-op.
func TestService_Promote_SkipsAlreadyPromoted(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "host-1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	svc, _, devRepo := discoveryServiceFixture(t, hosts, results)
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	first, err := svc.PromoteHosts(context.Background(), PromoteInput{RunID: run.ID})
	if err != nil {
		t.Fatalf("first promote: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first = %d, want 1", len(first))
	}

	second, err := svc.PromoteHosts(context.Background(), PromoteInput{RunID: run.ID})
	if err != nil {
		t.Fatalf("second promote: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second = %d, want 0 (idempotent)", len(second))
	}

	// The device count must still be 1.
	all, _, _ := devRepo.List(devicepkg.ListFilter{})
	if len(all) != 1 {
		t.Errorf("device count = %d, want 1", len(all))
	}
}

// TestService_Promote_RunNotFound covers the 404 path.
func TestService_Promote_RunNotFound(t *testing.T) {
	svc, _, _ := discoveryServiceFixture(t, nil, nil)
	_, err := svc.PromoteHosts(context.Background(), PromoteInput{RunID: "missing"})
	if err == nil {
		t.Fatal("expected error for missing run")
	}
}

// TestService_GetRunWithHosts verifies that GetRunWithHosts
// returns the run plus its hosts — what the GET /runs/:id
// handler renders.
func TestService_GetRunWithHosts(t *testing.T) {
	hosts := []Host{{IPAddress: "10.0.0.1", Hostname: "h1"}}
	results := map[string]ProbeResult{"10.0.0.1": {Reachable: true}}
	svc, _, _ := discoveryServiceFixture(t, hosts, results)
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	detail, err := svc.GetRunWithHosts(run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if detail.Run.ID != run.ID {
		t.Errorf("Run.ID = %q", detail.Run.ID)
	}
	if len(detail.Hosts) != 1 {
		t.Errorf("Hosts = %d, want 1", len(detail.Hosts))
	}
}

// TestCIDRRange is a small unit test for the CIDR helper. The
// service uses it to validate the CIDR string at the start of
// StartRun, before invoking the scanner.
func TestCIDRRange(t *testing.T) {
	ips, err := CIDRRange("10.0.0.0/30")
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	// /30 has 4 addresses; we skip network + broadcast, so
	// the helper returns 2 (10.0.0.1, 10.0.0.2).
	if len(ips) != 2 {
		t.Errorf("ips = %d (%v), want 2", len(ips), ips)
	}
	// Make sure the network address is skipped
	for _, ip := range ips {
		if net.ParseIP(ip).Equal(net.ParseIP("10.0.0.0")) {
			t.Errorf("network address leaked into range: %s", ip)
		}
	}
	// And invalid input is rejected.
	if _, err := CIDRRange("not-a-cidr"); err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

// _ = time.Second keeps the import live if the file's test
// bodies trim their time references; future tests may add
// timing assertions.
var _ = time.Second
