// Package servicecatalog implements the Service entity, the
// GORM repository, the business-logic service, the HTTP
// handler, and the health-rollup view. See
// openspec/specs/service-catalog/spec.md and
// docs/design/service-layer.md for the design.
//
// Service is the unit the on-call operator triages against.
// A pipeline declares which Service it deploys via the
// service_id foreign key on the pipelines table; that FK is
// the join that makes "what deploy last touched this
// service" answerable in one query.
package servicecatalog

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tier is the service-tiers enum. It is the single field
// the on-call operator sorts on first when triaging a list
// of 30+ services.
type Tier string

const (
	TierCritical Tier = "critical" // page on any health degradation
	TierImportant Tier = "important" // page if degraded for > 30m
	TierStandard  Tier = "standard"  // best-effort, business-hours response
)

// Service is a microservice under management. It is the
// first-class entity the spec introduces; everything else
// (pipelines, hosts, alerts) hangs off the Service via
// foreign keys or filters.
type Service struct {
	ID            string         `gorm:"primaryKey;column:id;type:text;size:64" json:"id"`
	Name          string         `gorm:"column:name;type:text;size:64;uniqueIndex;not null" json:"name"`
	Description   string         `gorm:"column:description;type:text" json:"description,omitempty"`
	Owner         string         `gorm:"column:owner;type:text;size:256" json:"owner,omitempty"`
	RepositoryURL string         `gorm:"column:repository_url;type:text;size:512" json:"repository_url,omitempty"`
	Tier          Tier           `gorm:"column:tier;type:text;size:16;not null;default:standard" json:"tier"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// BeforeCreate is the GORM hook that fills the primary key
// before INSERT. We use uuid.NewString() so the IDs are
// independent of the database (no auto-increment, no
// UUID-from-time so no information leak).
func (s *Service) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
