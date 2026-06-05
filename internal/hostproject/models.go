// Package hostproject implements the many-to-many link between
// physical hosts (Device rows) and projects. A host can serve
// multiple projects and a project can span multiple hosts. Links
// are stored in a dedicated host_project_links table with a
// composite unique index on (device_id, project_id) so the same
// pair cannot be linked twice.
//
// The package follows the project's layered rules:
//
//	handler -> service -> repository -> model
//
// Handler is thin: parse, call, render. Service owns the
// business rules (existence checks, bulk semantics, hierarchy
// walk, orphaning). Repository is GORM-only.
package hostproject

import (
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// HostProjectLink binds a physical host (device) to a project.
// Soft-deletes are not used on this table — orphaning is
// represented by setting OrphanedAt to a non-nil timestamp,
// which keeps the link row discoverable for audit / restore
// while removing it from the effective-link set.
//
// The (DeviceID, ProjectID) pair is unique when OrphanedAt IS
// NULL: the partial unique index is expressed via the
// `class=...` option in some GORM dialects, but to stay
// driver-agnostic we use a composite unique index and let the
// repository filter on OrphanedAt explicitly. This is the
// driver-portable equivalent of the partial unique index.
type HostProjectLink struct {
	database.BaseModel

	// DeviceID is the FK to the devices table. It is a plain
	// string (UUID) rather than a GORM relation because the
	// link is a many-to-many — there is no owned association
	// to navigate to from this struct.
	DeviceID string `gorm:"column:device_id;type:text;uniqueIndex:idx_hpl_device_project_active;not null;index" json:"device_id"`

	// ProjectID is the FK to the projects table.
	ProjectID string `gorm:"column:project_id;type:text;uniqueIndex:idx_hpl_device_project_active;not null;index" json:"project_id"`

	// LinkedBy is the user ID of the person who created the
	// link. Stored on every link for audit attribution.
	LinkedBy string `gorm:"column:linked_by;type:text;not null" json:"linked_by"`

	// LinkedAt is the wall-clock time the link was created.
	LinkedAt time.Time `gorm:"column:linked_at;not null" json:"linked_at"`

	// OrphanedAt is set when the device backing the link has
	// been soft-deleted. A non-nil value means the link is no
	// longer "effective" but is kept for audit. The composite
	// unique index (device_id, project_id) treats NULLs as
	// distinct, so a re-link after restore needs the orphaned
	// row cleared first.
	OrphanedAt *time.Time `gorm:"column:orphaned_at;index" json:"orphaned_at,omitempty"`
}

// TableName pins the table name. Keeping it stable shields
// callers from future renames of the struct.
func (HostProjectLink) TableName() string { return "host_project_links" }

// BeforeCreate is a GORM hook. It calls the embedded
// BaseModel.BeforeCreate so the UUID PK is filled in. The
// method must exist as a top-level function (not promoted
// from the embedded struct) so GORM's hook reflection picks
// it up reliably.
func (l *HostProjectLink) BeforeCreate(tx *gorm.DB) error {
	return l.BaseModel.BeforeCreate(tx)
}

// AllModels returns every GORM model this package owns. The
// AutoMigrate entry point uses this so callers do not have to
// repeat the model list when the package grows.
func AllModels() []any {
	return []any{
		&HostProjectLink{},
	}
}
