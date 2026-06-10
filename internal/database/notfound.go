// Package database — NotFound translation helper.
package database

import (
	"errors"

	"gorm.io/gorm"
)

// MapNotFound translates GORM's ErrRecordNotFound into the
// caller-supplied package-local sentinel. The pattern
// was duplicated as the same 3-line `if errors.Is(...)`
// block in every repository's Get/List/Delete path (14
// packages, ~31 sites). Centralising here makes the
// translation auditable from one place and lets a future
// swap to a different ORM touch one function instead of
// every repo.
//
// Usage:
//
//	if err := r.db.First(&row, id).Error; err != nil {
//	    return nil, database.MapNotFound(err, ErrNotFound)
//	}
//
// The function preserves non-NotFound errors verbatim
// (so a connection-refused or context-cancelled error
// surfaces to the caller intact).
func MapNotFound(err, sentinel error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sentinel
	}
	return err
}
