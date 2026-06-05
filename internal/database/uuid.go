// Package database owns the GORM connection, AutoMigrate registration,
// and the base model every other module embeds for consistent UUID
// primary keys, timestamps, and soft deletes (per openspec/database-schema).
package database

import "github.com/google/uuid"

// NewID returns a freshly generated UUIDv4 string. Exported so module
// tests can build a fixture without importing google/uuid directly.
func NewID() string {
	return uuid.NewString()
}
