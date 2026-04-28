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

// AutoMigrate runs database migrations
func AutoMigrate() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
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
