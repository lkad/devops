// Package project implements the 3-level project hierarchy:
// BusinessLine → System → Project. The hierarchy is encoded in a
// single self-referential Project table whose depth is enforced by
// the service layer (MaxDepth). Project types live in a separate
// table to comply with the database-schema spec; embedding an enum
// in Project is explicitly forbidden.
//
// All public HTTP endpoints live in handler.go; service.go owns
// the business rules (uniqueness, cycle detection, depth, weight
// aggregation); repository.go is GORM-only.
package project

import (
	"time"

	"github.com/devops-toolkit/backend/internal/database"
)

// ProjectType classifies a Project for cost allocation and
// reporting. Per the database-schema spec the type is its own
// table with a foreign key, never an embedded enum in Project.
type ProjectType struct {
	database.BaseModel
	Name        string `gorm:"column:name;size:64;uniqueIndex;not null" json:"name"`
	Description string `gorm:"column:description;size:512" json:"description"`
	// Weight is the per-type cost-allocation weight. Combined with
	// the project's own weight it is used to apportion shared costs
	// across the hierarchy.
	Weight int `gorm:"column:weight;default:0" json:"weight"`
}

// TableName pins the table name explicitly so GORM does not have
// to guess from the struct name. The spec is opinionated here —
// "project_types" — and guessing risks breaking migrations if
// the struct is ever renamed.
func (ProjectType) TableName() string { return "project_types" }

// Project is the unified table for the 3-level hierarchy. A
// BusinessLine is a Project with ParentID == nil. A System is a
// Project whose parent's ParentID == nil. A Project is a Project
// whose parent is a System. The depth invariant is enforced by
// the service layer; the schema does not need a CHECK constraint
// to be safe.
//
// The Code field is a human-friendly unique slug (e.g. "payments")
// used in URLs and external systems; it is independent of Name so
// teams can rename the display label without breaking links.
type Project struct {
	database.BaseModel
	Name        string  `gorm:"column:name;size:128;not null;index" json:"name"`
	Code        string  `gorm:"column:code;size:64;uniqueIndex;not null" json:"code"`
	Description string  `gorm:"column:description;size:512" json:"description"`
	ParentID    *string `gorm:"column:parent_id;type:text;index" json:"parent_id,omitempty"`
	TypeID      string  `gorm:"column:type_id;type:text;not null;index" json:"type_id"`
	OwnerUserID *string `gorm:"column:owner_user_id;type:text;index" json:"owner_user_id,omitempty"`
	// Weight is the project-level cost-allocation weight. The
	// service layer multiplies this with the type's weight when
	// computing hierarchical aggregates.
	Weight  int     `gorm:"column:weight;default:0" json:"weight"`
	Labels  JSONMap `gorm:"column:labels;type:text" json:"labels"`
	Metadata JSONMap `gorm:"column:metadata;type:text" json:"metadata"`
}

// TableName pins the project table. Keeping it stable shields
// callers from future renames of the struct.
func (Project) TableName() string { return "projects" }

// ProjectMember binds a user to a project with a per-project role.
// The (ProjectID, UserID) pair is unique so a user cannot be added
// twice to the same project; an upsert pattern in the service
// layer is used to "promote" the role if the user is re-added.
type ProjectMember struct {
	database.BaseModel
	ProjectID string    `gorm:"column:project_id;type:text;uniqueIndex:idx_project_user;not null;index" json:"project_id"`
	UserID    string    `gorm:"column:user_id;type:text;uniqueIndex:idx_project_user;not null;index" json:"user_id"`
	Role      string    `gorm:"column:role;size:16;not null" json:"role"`
	AddedBy   string    `gorm:"column:added_by;type:text;not null" json:"added_by"`
	AddedAt   time.Time `gorm:"column:added_at;not null" json:"added_at"`
}

// TableName pins the project_members table. The unique index name
// is shared between the two columns so GORM generates the
// composite unique constraint we need.
func (ProjectMember) TableName() string { return "project_members" }
