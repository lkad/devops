package database

import (
	"fmt"
	"time"

	"github.com/devops-toolkit/internal/device"
	"github.com/devops-toolkit/internal/project"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type GORMConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	MaxConnections  int
	SSLMode         string
	ConnMaxLifetime time.Duration
}

func (c *GORMConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func NewGORM(cfg *GORMConfig) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.MaxConnections / 2)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func GetGORM() *gorm.DB {
	return db
}

func SetGORM(d *gorm.DB) {
	db = d
}

// addMissingDeletedAtColumns adds deleted_at columns to existing tables
// that were created before gorm.Model soft delete was enabled
func addMissingDeletedAtColumns() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	tables := []string{"project_types", "systems", "projects", "project_resources", "project_permissions"}
	for _, table := range tables {
		// Check if column exists
		var count int64
		err := db.Raw(fmt.Sprintf(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = '%s' AND column_name = 'deleted_at'
		`, table)).Count(&count).Error
		if err != nil {
			continue
		}
		if count == 0 {
			// Add the column
			if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN deleted_at TIMESTAMP", table)).Error; err != nil {
				return fmt.Errorf("failed to add deleted_at column to %s: %w", table, err)
			}
		}
	}
	return nil
}

// AutoMigrate runs database migrations
func AutoMigrate() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	// First add missing deleted_at columns to existing tables
	// This must happen BEFORE AutoMigrate runs, as some tables were created before gorm.Model soft delete
	if err := addMissingDeletedAtColumns(); err != nil {
		return err
	}
	// Run AutoMigrate
	return db.AutoMigrate(
		&device.GORMDevice{},
		&device.DeviceStateTransition{},
		&device.DeviceGroup{},
		&project.GORMBusinessLine{},
		&project.GORMSystem{},
		&project.GORMProject{},
		&project.GORMProjectType{},
		&project.GORMResource{},
		&project.GORMPermission{},
		&project.AuditLog{},
	)
}
