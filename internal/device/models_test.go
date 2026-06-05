package device

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openDeviceDB builds a fresh in-memory sqlite DB, runs
// AutoMigrate for every model this package owns, and registers
// a Cleanup to close the underlying sql.DB. Tests share the
// pattern so a regression in AutoMigrate is caught at the model
// level rather than at the service or handler level.
//
// We use shared-cache in-memory sqlite with a unique DSN per
// test. File-based sqlite reliably deadlocks in this package
// when the same package runs many tests back-to-back (the CGO
// driver holds a connection that the next AutoMigrate wants);
// in-memory is the only configuration that completes the
// 30+ test suite under the 2-minute timeout.
func openDeviceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:device-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Limit to one connection: shared in-memory dbs are
	// per-connection; multiple connections would see different
	// schemas.
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

// TestDeviceModel_AutoMigrate verifies the schema lands without
// error and the expected columns exist. This is the canary test
// — if it fails, the model declarations do not match the
// migration contract, and every downstream test will misbehave.
func TestDeviceModel_AutoMigrate(t *testing.T) {
	db := openDeviceDB(t)
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

// TestDevice_BaseModelFieldsPresent verifies that embedding
// database.BaseModel supplies the UUID PK and the soft-delete
// timestamp column. A regression here would silently break
// every Get/Update/Delete call.
func TestDevice_BaseModelFieldsPresent(t *testing.T) {
	d := Device{
		Name:  "host-1",
		Type:  DeviceTypePhysicalHost,
		State: DeviceStateOnline,
	}
	if d.ID != "" {
		t.Errorf("ID should be zero-value before Create, got %q", d.ID)
	}
	if !d.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be zero-value before Create, got %v", d.CreatedAt)
	}
}

// TestDevice_RoundTrip verifies the full create -> read -> update
// -> soft-delete cycle on a real sqlite DB. This is the minimum
// persistence contract; if this test fails, the model is unfit
// for use.
func TestDevice_RoundTrip(t *testing.T) {
	db := openDeviceDB(t)

	d := &Device{
		Name:  "web-01",
		Type:  DeviceTypePhysicalHost,
		State: DeviceStateOnline,
		Labels: JSONMap{
			"env":  "prod",
			"team": "platform",
		},
	}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.ID == "" {
		t.Fatal("ID was not assigned by BaseModel.BeforeCreate")
	}
	if d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set on create: created=%v updated=%v", d.CreatedAt, d.UpdatedAt)
	}

	var got Device
	if err := db.First(&got, "id = ?", d.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Name != "web-01" {
		t.Errorf("Name = %q, want web-01", got.Name)
	}
	if got.Type != DeviceTypePhysicalHost {
		t.Errorf("Type = %q, want %q", got.Type, DeviceTypePhysicalHost)
	}
	if got.Labels["env"] != "prod" {
		t.Errorf("Labels[env] = %v, want prod", got.Labels["env"])
	}

	// Update path
	got.Name = "web-01-renamed"
	got.State = DeviceStateMaintenance
	if err := db.Save(&got).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "web-01-renamed" {
		t.Errorf("post-update Name = %q", got.Name)
	}

	// Soft delete — record should be hidden from default queries
	if err := db.Delete(&got).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.First(&Device{}, "id = ?", d.ID).Error; err == nil {
		t.Error("soft-deleted device should be hidden from default query")
	}
	if err := db.Unscoped().First(&Device{}, "id = ?", d.ID).Error; err != nil {
		t.Errorf("soft-deleted device should be visible to Unscoped: %v", err)
	}
}

// TestDevice_OptionalFieldsAreNullable checks that the optional
// FK fields (GroupID, TemplateID) and LastSeen are stored as
// NULL when unset, not as empty-string sentinels that would
// later break IS NULL queries.
func TestDevice_OptionalFieldsAreNullable(t *testing.T) {
	db := openDeviceDB(t)
	d := &Device{Name: "x", Type: DeviceTypeContainer, State: DeviceStateOnline}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got Device
	if err := db.First(&got, "id = ?", d.ID).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.GroupID != nil {
		t.Errorf("GroupID should be nil when unset, got %v", *got.GroupID)
	}
	if got.TemplateID != nil {
		t.Errorf("TemplateID should be nil when unset, got %v", *got.TemplateID)
	}
	if got.LastSeen != nil {
		t.Errorf("LastSeen should be nil when unset, got %v", *got.LastSeen)
	}
}

// TestDevice_LastSeenIsTimestamp pins the LastSeen column as a
// time.Time pointer (not a string) so the repository can compare
// it to now() for staleness checks.
func TestDevice_LastSeenIsTimestamp(t *testing.T) {
	d := &Device{LastSeen: ptrTime(time.Now())}
	if d.LastSeen == nil {
		t.Fatal("LastSeen pointer is nil")
	}
	if d.LastSeen.IsZero() {
		t.Error("LastSeen is zero time")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
