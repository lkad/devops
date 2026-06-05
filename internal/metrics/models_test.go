// Package metrics tests live next to the code so the spec
// scenarios map 1:1 to *_test.go functions.
//
// Shared test fixtures (openDB, the in-memory sqlite) live in
// this file so every test file in the package picks them up
// without an import dance.
package metrics

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openDB builds a fresh in-memory sqlite DB, runs AutoMigrate
// for every model this package owns, and registers a Cleanup
// to close the underlying sql.DB. Mirrors the physicalhost
// package's openDB: unique DSN per test, single connection,
// busy_timeout pragma to dodge the CGO deadlock on shared
// in-memory dbs.
func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:metrics-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
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

// TestMetricsModel_AutoMigrate is the canary test for the
// package. If AllModels returns an empty list or the parse
// fails, every other test misbehaves.
func TestMetricsModel_AutoMigrate(t *testing.T) {
	db := openDB(t)
	models := AllModels()
	if len(models) == 0 {
		t.Fatal("AllModels returned no models")
	}
	for _, m := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
		if stmt.Schema == nil {
			t.Fatalf("schema for %T is nil", m)
		}
	}
}

// TestTargetType_Valid pins the enum boundary.
func TestTargetType_Valid(t *testing.T) {
	cases := []struct {
		in   TargetType
		want bool
	}{
		{TargetPhysicalHost, true},
		{TargetK8sPod, true},
		{TargetType(""), false},
		{TargetType("vm"), false},
		{TargetType("PHYSICAL_HOST"), false}, // case-sensitive
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.want {
			t.Errorf("%q.Valid() = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestMaxQueryRange_Documented pins the 90-day choice. The
// spec mandates that list / series calls with a time range
// longer than 90 days are rejected with 422 INVALID_STATE;
// this test guards the constant so a future refactor cannot
// silently change the limit.
func TestMaxQueryRange_Documented(t *testing.T) {
	wantDays := 90.0
	gotDays := MaxQueryRange.Hours() / 24
	if gotDays != wantDays {
		t.Errorf("MaxQueryRange = %v days, want %v days", gotDays, wantDays)
	}
}
