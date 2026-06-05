package k8s

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB builds a fresh in-memory sqlite DB, runs AutoMigrate
// for every model this package owns, and registers a Cleanup to
// close the underlying sql.DB. Tests share the pattern so a
// regression in AutoMigrate is caught at the model level rather
// than at the service or handler level.
//
// We use a per-test DSN so each test gets a private database;
// shared in-memory dbs would not work for parallel tests
// because every connection would see the same blank schema.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:k8s-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
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

// TestClusterModel_AutoMigrate verifies the schema lands without
// error and the expected columns exist. This is the canary test —
// if it fails, the model declarations do not match the migration
// contract, and every downstream test will misbehave.
func TestClusterModel_AutoMigrate(t *testing.T) {
	db := openTestDB(t)
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&Cluster{}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stmt.Schema == nil {
		t.Fatal("schema is nil after parse")
	}
}

// TestCluster_BaseModelFieldsPresent verifies that embedding
// database.BaseModel supplies the UUID PK and the soft-delete
// timestamp column. A regression here would silently break every
// Get/Update/Delete call.
func TestCluster_BaseModelFieldsPresent(t *testing.T) {
	c := Cluster{
		Name: "test",
		Type: ClusterTypeK3d,
	}
	if c.ID != "" {
		t.Errorf("ID should be zero-value before Create, got %q", c.ID)
	}
	if !c.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be zero-value before Create, got %v", c.CreatedAt)
	}
}

// TestCluster_RoundTrip verifies the full create -> read -> update
// -> soft-delete cycle on a real sqlite DB.
func TestCluster_RoundTrip(t *testing.T) {
	db := openTestDB(t)
	c := &Cluster{
		Name:                  "c1",
		Type:                  ClusterTypeK3d,
		APIServerURL:          "https://k3d.local:6443",
		KubeconfigEncrypted:   "ciphertext-blob",
		InCluster:             false,
		Status:                ClusterStatusUnknown,
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == "" {
		t.Fatal("ID was not assigned by BaseModel.BeforeCreate")
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set on create: created=%v updated=%v", c.CreatedAt, c.UpdatedAt)
	}

	var got Cluster
	if err := db.First(&got, "id = ?", c.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Name != "c1" {
		t.Errorf("Name = %q, want c1", got.Name)
	}
	if got.Type != ClusterTypeK3d {
		t.Errorf("Type = %q, want %q", got.Type, ClusterTypeK3d)
	}

	// Update path
	got.Status = ClusterStatusConnected
	if err := db.Save(&got).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Status != ClusterStatusConnected {
		t.Errorf("post-update Status = %q", got.Status)
	}

	// Soft delete
	if err := db.Delete(&got).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.First(&Cluster{}, "id = ?", c.ID).Error; err == nil {
		t.Error("soft-deleted cluster should be hidden from default query")
	}
	if err := db.Unscoped().First(&Cluster{}, "id = ?", c.ID).Error; err != nil {
		t.Errorf("soft-deleted cluster should be visible to Unscoped: %v", err)
	}
}

// TestCluster_TableName pins the GORM-generated table name. Hard
// coding it here keeps the SQL predictable for migrations and
// ad-hoc queries.
func TestCluster_TableName(t *testing.T) {
	c := Cluster{}
	if c.TableName() != "k8s_clusters" {
		t.Errorf("TableName() = %q, want k8s_clusters", c.TableName())
	}
}
