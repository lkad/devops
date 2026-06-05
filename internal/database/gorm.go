package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectionConfig is the minimum information needed to open a DB.
// Pulled from the database-schema spec — we only support sqlite and
// postgres; other drivers must be added explicitly.
type ConnectionConfig struct {
	Driver         string
	DSN            string
	Host           string
	Port           int
	User           string
	Password       string
	DBName         string
	SSLMode        string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime int
}

// Open returns a configured *gorm.DB. It applies pool limits and a
// quiet GORM logger so SQL spam does not drown the application logs.
func Open(cfg ConnectionConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	var dialector gorm.Dialector
	switch cfg.Driver {
	case "sqlite", "":
		dsn := cfg.DSN
		if dsn == "" {
			dsn = "file::memory:?cache=shared"
		}
		dialector = sqlite.Open(dsn)
	case "postgres":
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 5432
		}
		sslmode := cfg.SSLMode
		if sslmode == "" {
			sslmode = "disable"
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			host, port, cfg.User, cfg.Password, cfg.DBName, sslmode)
		dialector = postgres.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver %q (supported: sqlite, postgres)", cfg.Driver)
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
	return db, nil
}

// AutoMigrate runs GORM's schema sync for the given models. The
// argument list is open so any module can register its own tables
// from cmd/devops-toolkit/main.go.
func AutoMigrate(db *gorm.DB, models ...any) error {
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("automigrate: %w", err)
	}
	return nil
}
