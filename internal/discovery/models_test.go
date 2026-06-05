package discovery

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openDiscoveryDB builds a fresh in-memory sqlite DB and runs
// AutoMigrate for the discovery package's models. The pattern
// mirrors the device package's fixture: a unique DSN per test
// avoids shared-cache surprises, and the single-connection
// limit keeps the schema visible to every query.
func openDiscoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:discovery-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// TestDiscoveryRun_AutoMigrate is the canary test: the model
// must parse cleanly and the schema must not be nil. A failure
// here breaks every downstream test, which is the point.
func TestDiscoveryRun_AutoMigrate(t *testing.T) {
	db := openDiscoveryDB(t)
	for _, m := range AllModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
		if stmt.Schema == nil {
			t.Errorf("%T: schema is nil after parse", m)
		}
	}
}

// TestDiscoveryRun_RoundTrip persists a run, reads it back, and
// checks the fields survive the round trip. This is the minimum
// persistence contract for the run record.
func TestDiscoveryRun_RoundTrip(t *testing.T) {
	db := openDiscoveryDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	completed := now.Add(2 * time.Second)
	run := &DiscoveryRun{
		CIDR:        "10.0.0.0/24",
		StartedAt:   now,
		CompletedAt: &completed,
		HostsFound:  5,
		Status:      RunStatusCompleted,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.ID == "" {
		t.Fatal("run ID was not assigned by BaseModel.BeforeCreate")
	}
	var got DiscoveryRun
	if err := db.First(&got, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.CIDR != "10.0.0.0/24" {
		t.Errorf("CIDR = %q, want 10.0.0.0/24", got.CIDR)
	}
	if got.HostsFound != 5 {
		t.Errorf("HostsFound = %d, want 5", got.HostsFound)
	}
	if got.Status != RunStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, RunStatusCompleted)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completed) {
		t.Errorf("CompletedAt = %v, want %v", got.CompletedAt, completed)
	}
}

// TestDiscoveredHost_RoundTrip persists a host, reads it back,
// and verifies the JSON OpenPorts survives. The IPAddress
// uniqueness constraint must also be enforced (covered by
// TestDiscoveredHost_IPAddressUnique).
func TestDiscoveredHost_RoundTrip(t *testing.T) {
	db := openDiscoveryDB(t)
	run := &DiscoveryRun{
		CIDR:       "10.0.0.0/24",
		StartedAt:  time.Now().UTC(),
		Status:     RunStatusCompleted,
		HostsFound: 1,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	host := &DiscoveredHost{
		RunID:        run.ID,
		IPAddress:    "10.0.0.1",
		Hostname:     "host-1",
		OpenPorts:    JSONMap{"tcp": []int{22, 80}},
		SNMPSysDescr: "Linux router 1.0",
	}
	if err := db.Create(host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	if host.ID == "" {
		t.Fatal("host ID was not assigned")
	}
	var got DiscoveredHost
	if err := db.First(&got, "id = ?", host.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %q, want 10.0.0.1", got.IPAddress)
	}
	if got.SNMPSysDescr != "Linux router 1.0" {
		t.Errorf("SNMPSysDescr = %q", got.SNMPSysDescr)
	}
	if _, ok := got.OpenPorts["tcp"]; !ok {
		t.Errorf("OpenPorts[tcp] missing: %+v", got.OpenPorts)
	}
}

// TestDiscoveredHost_PromotedToDeviceID_Optional pins the
// nullable contract for PromotedToDeviceID. NULL on insert
// must round-trip as nil pointer — a regression would break
// the "promote later" flow.
func TestDiscoveredHost_PromotedToDeviceID_Optional(t *testing.T) {
	db := openDiscoveryDB(t)
	run := &DiscoveryRun{CIDR: "10.0.0.0/24", StartedAt: time.Now().UTC(), Status: RunStatusRunning}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	host := &DiscoveredHost{RunID: run.ID, IPAddress: "10.0.0.5"}
	if err := db.Create(host).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got DiscoveredHost
	if err := db.First(&got, "id = ?", host.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.PromotedToDeviceID != nil {
		t.Errorf("PromotedToDeviceID should be nil when unset, got %v", *got.PromotedToDeviceID)
	}
}
