// Package physicalhost implements the physical-host monitoring subsystem.
// Tests live next to the code so the spec scenarios map 1:1 to *_test.go
// functions.
//
// The package owns:
//
//   - PhysicalHost GORM model (separate table from devices, FK to devices.id)
//   - 4-state model: online / monitoring_issue / offline / maintenance
//   - Maintenance-mode semantics (orthogonal flag + state)
//   - SSH reachability probing via a Prober interface seam
//   - Audit-event emission on maintenance transitions
//
// Layered rules: handler -> service -> repository -> model, with the
// Prober injected as its own type. No real SSH client, no real network
// calls in tests.
package physicalhost

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openDB builds a fresh in-memory sqlite DB, runs AutoMigrate for
// every model this package owns, and registers a Cleanup to close
// the underlying sql.DB. Mirrors the device package's openDeviceDB
// pattern: unique DSN per test, single connection, busy_timeout
// pragma to dodge the CGO deadlock on shared in-memory dbs.
func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:physicalhost-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
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
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

// TestPhysicalHostModel_AutoMigrate is the canary test for the
// package. If AllModels returns an empty list or the parse fails,
// every other test misbehaves.
func TestPhysicalHostModel_AutoMigrate(t *testing.T) {
	db := openDB(t)
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

// TestPhysicalHost_TableName pins the table name so the FK from
// other modules (Phase 7 alert suppression, for instance) can rely
// on a stable identifier.
func TestPhysicalHost_TableName(t *testing.T) {
	p := PhysicalHost{}
	if got := p.TableName(); got != "physical_hosts" {
		t.Errorf("TableName = %q, want physical_hosts", got)
	}
}

// TestPhysicalHost_BaseModelFieldsPresent verifies the embedded
// BaseModel provides the UUID PK and timestamps. A regression
// would silently break every Get/Update/Delete call.
func TestPhysicalHost_BaseModelFieldsPresent(t *testing.T) {
	p := PhysicalHost{DeviceID: "dev-1", IPAddress: "10.0.0.1", SSHUser: "root"}
	if p.ID != "" {
		t.Errorf("ID should be zero-value before Create, got %q", p.ID)
	}
	if !p.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be zero-value before Create, got %v", p.CreatedAt)
	}
}

// TestPhysicalHost_RoundTrip exercises the full create->read->update
// ->soft-delete cycle. The minimum persistence contract.
func TestPhysicalHost_RoundTrip(t *testing.T) {
	db := openDB(t)

	p := &PhysicalHost{
		DeviceID:  "device-1",
		IPAddress: "10.0.0.5",
		SSHPort:   2222,
		SSHUser:   "ops",
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID was not assigned by BaseModel.BeforeCreate")
	}
	if p.State != StateOnline {
		t.Errorf("State default = %q, want online", p.State)
	}

	var got PhysicalHost
	if err := db.First(&got, "id = ?", p.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.DeviceID != "device-1" {
		t.Errorf("DeviceID = %q, want device-1", got.DeviceID)
	}
	if got.IPAddress != "10.0.0.5" {
		t.Errorf("IPAddress = %q, want 10.0.0.5", got.IPAddress)
	}
	if got.SSHPort != 2222 {
		t.Errorf("SSHPort = %d, want 2222", got.SSHPort)
	}
	if got.SSHUser != "ops" {
		t.Errorf("SSHUser = %q, want ops", got.SSHUser)
	}

	// Update path: change state and set maintenance fields.
	got.State = StateMaintenance
	now := time.Now().UTC()
	got.MaintenanceStartedAt = &now
	got.MaintenanceReason = "patching"
	got.MaintenanceSetBy = "alice"
	if err := db.Save(&got).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.MaintenanceReason != "patching" {
		t.Errorf("MaintenanceReason = %q, want patching", got.MaintenanceReason)
	}

	// Soft delete — row stays in the table, hidden from default queries.
	if err := db.Delete(&got).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.First(&PhysicalHost{}, "id = ?", p.ID).Error; err == nil {
		t.Error("soft-deleted physical host should be hidden from default query")
	}
	if err := db.Unscoped().First(&PhysicalHost{}, "id = ?", p.ID).Error; err != nil {
		t.Errorf("soft-deleted physical host should be visible to Unscoped: %v", err)
	}
}

// TestPhysicalHost_DeviceIDIsUnique pins the unique constraint on
// the device FK — a device row cannot be linked to more than one
// physical_hosts row. The spec requires a 1:1 view of physical
// hosts even though the underlying devices table is generic.
func TestPhysicalHost_DeviceIDIsUnique(t *testing.T) {
	db := openDB(t)
	first := &PhysicalHost{DeviceID: "dup-1", IPAddress: "10.0.0.1", SSHUser: "root"}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := &PhysicalHost{DeviceID: "dup-1", IPAddress: "10.0.0.2", SSHUser: "root"}
	if err := db.Create(second).Error; err == nil {
		t.Fatal("expected unique-constraint violation on duplicate device_id")
	}
}

// TestPhysicalHost_OptionalTimestampsAreNullable verifies the
// optional timestamp columns (LastCheckAt, NextCheckAt,
// MaintenanceStartedAt) are NULL when unset, not zero-time
// sentinels. A non-NULL zero-time would break IS NULL queries in
// the alert suppression logic.
func TestPhysicalHost_OptionalTimestampsAreNullable(t *testing.T) {
	db := openDB(t)
	p := &PhysicalHost{DeviceID: "dev-1", IPAddress: "10.0.0.1", SSHUser: "root"}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got PhysicalHost
	if err := db.First(&got, "id = ?", p.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.LastCheckAt != nil {
		t.Errorf("LastCheckAt should be nil when unset, got %v", *got.LastCheckAt)
	}
	if got.NextCheckAt != nil {
		t.Errorf("NextCheckAt should be nil when unset, got %v", *got.NextCheckAt)
	}
	if got.MaintenanceStartedAt != nil {
		t.Errorf("MaintenanceStartedAt should be nil when unset, got %v", *got.MaintenanceStartedAt)
	}
	if got.InMaintenance() {
		t.Error("InMaintenance() should be false when MaintenanceStartedAt is nil")
	}
}

// TestPhysicalHost_InMaintenance_TrueWhenStarted pins the InMaintenance
// helper that the alert manager and the monitor use to gate
// suppressions.
func TestPhysicalHost_InMaintenance_TrueWhenStarted(t *testing.T) {
	now := time.Now().UTC()
	p := PhysicalHost{MaintenanceStartedAt: &now}
	if !p.InMaintenance() {
		t.Error("InMaintenance() should be true when MaintenanceStartedAt is set")
	}
}

// TestPhysicalHostState_Valid covers the four legal states. Any
// future state added to the spec is a breaking change for clients
// that branch on the string values.
func TestPhysicalHostState_Valid(t *testing.T) {
	cases := []struct {
		state PhysicalHostState
		want  bool
	}{
		{StateOnline, true},
		{StateMonitoringIssue, true},
		{StateOffline, true},
		{StateMaintenance, true},
		{PhysicalHostState("unknown"), false},
		{PhysicalHostState(""), false},
	}
	for _, c := range cases {
		if got := c.state.Valid(); got != c.want {
			t.Errorf("Valid(%q) = %v, want %v", c.state, got, c.want)
		}
	}
}
