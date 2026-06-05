package hostproject

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
)

// openTestDB creates a fresh in-memory sqlite database with a
// unique DSN per test, runs AutoMigrate for the hostproject
// tables together with the device and project tables, and
// registers a Cleanup that closes the connection. The GORM
// logger is silenced to keep the test output readable.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:hpl-" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		devicepkg.AllModels()...,
	); err != nil {
		t.Fatalf("migrate device: %v", err)
	}
	if err := db.AutoMigrate(
		&projectpkg.ProjectType{},
		&projectpkg.Project{},
		&projectpkg.ProjectMember{},
		&HostProjectLink{},
	); err != nil {
		t.Fatalf("migrate project/link: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}
