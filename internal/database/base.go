package database

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel is the foundation for every persisted struct in the project.
// It supplies a UUID primary key plus the standard GORM timestamp/soft-delete
// columns. Embed it into a module's model:
//
//	type Device struct {
//		BaseModel
//		Name string
//	}
//
// All migrations pick up these columns automatically because they are
// plain GORM tags, not overridden in the child struct. DeletedAt is the
// gorm.DeletedAt type — using *time.Time would not trigger soft-delete
// semantics automatically.
type BaseModel struct {
	ID        string           `gorm:"primaryKey;type:text;column:id" json:"id"`
	CreatedAt time.Time        `gorm:"column:created_at;index" json:"created_at"`
	UpdatedAt time.Time        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt   `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

// BeforeCreate ensures every new row gets a UUID before insert. GORM
// invokes this hook (the *gorm.DB parameter is the documented signature)
// when a model with BaseModel is created.
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = NewID()
	}
	return nil
}
