package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"
)

// ErrNotFound is the typed sentinel returned by every Repository
// method when a row is missing. The service layer wraps it in a
// 404 NOT_FOUND APIError; the handler does the rendering.
var ErrNotFound = errors.New("audit: not found")

// IsNotFound reports whether err is (or wraps) the repository
// ErrNotFound sentinel. Mirrors the alerts.IsNotFound helper so
// callers in this package can use a single function.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// AuditFilter is the query-side DTO for List. Pointer fields
// (From / To) let the handler distinguish "not set" from "zero
// value" when building the SQL WHERE clause. The remaining
// fields use the empty string convention: a zero value means
// "no filter on this dimension".
type AuditFilter struct {
	Action       AuditAction
	ActorID      string
	ActorUsername string
	ResourceType AuditResourceType
	ResourceID   string
	// From and To are inclusive lower / upper bounds on
	// occurred_at. Nil means "unbounded on this side".
	From *time.Time
	To   *time.Time
	// Limit / Offset are the standard pagination fields. A
	// zero Limit means "no limit" (the repository falls back to
	// the default page size for total-counting purposes only).
	Limit  int
	Offset int
	// ProjectIDsIn is the per-tenant scope filter used by
	// the scoped-Auditor role. When non-empty, the
	// repository restricts results to events whose
	// metadata's `project_id` is in this set. Empty
	// means "no tenant filter" (a SuperAdmin caller);
	// a non-empty set of size >0 is the contract for
	// scoped access.
	//
	// The filter is applied via json_extract on the
	// Metadata column, which works on both SQLite
	// (in-memory test DB) and Postgres (production).
	ProjectIDsIn []string
}

// Repository is the GORM-only data-access layer for the audit
// subsystem. It knows about rows, indexes, and the SQL WHERE
// clause for each filter dimension. The service layer
// (service.go) builds the AuditFilter and wraps the result in
// the HTTP envelope; the repository never imports contracts.
type Repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the supplied db.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Create inserts a new event row. The ID is filled in by
// BaseModel.BeforeCreate; the caller can read e.ID immediately
// after Create returns.
func (r *Repository) Create(e *AuditEvent) error {
	if err := r.db.Create(e).Error; err != nil {
		return fmt.Errorf("audit.Repository.Create: %w", err)
	}
	return nil
}

// Get returns the event with the given ID, or ErrNotFound if no
// such row exists. Soft-deleted rows are hidden — the audit log
// is append-only at the API surface so this never returns a
// tombstoned row in practice.
func (r *Repository) Get(id string) (*AuditEvent, error) {
	var e AuditEvent
	if err := r.db.First(&e, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
		return nil, fmt.Errorf("audit.Repository.Get: %w", err)
	}
	return &e, nil
}

// List returns a page of events plus the unfiltered total count
// for the supplied filter (limit / offset excluded from the
// count). All filter fields are optional; the zero value returns
// every event in DESC occurred_at order, which is what the
// default audit-log view needs.
//
// Ordering: DESC occurred_at, then ASC id as a tie-breaker. The
// id tie-breaker keeps the page stable across rows that share
// the same timestamp (typical for bursts of activity).
func (r *Repository) List(f AuditFilter) ([]AuditEvent, int64, error) {
	q := r.db.Model(&AuditEvent{})
	if f.Action != "" {
		q = q.Where("action = ?", string(f.Action))
	}
	if f.ActorID != "" {
		q = q.Where("actor_id = ?", f.ActorID)
	}
	if f.ActorUsername != "" {
		q = q.Where("actor_username = ?", f.ActorUsername)
	}
	if f.ResourceType != "" {
		q = q.Where("resource_type = ?", string(f.ResourceType))
	}
	if f.ResourceID != "" {
		q = q.Where("resource_id = ?", f.ResourceID)
	}
	if f.From != nil {
		q = q.Where("occurred_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("occurred_at <= ?", *f.To)
	}
	// Per-tenant scope filter for the scoped-Auditor
	// role. The filter restricts results to events
	// whose Metadata->'project_id' is in the supplied
	// set. json_extract is the cross-driver expression
	// (works on SQLite + Postgres + MySQL 5.7+).
	//
	// Distinguish two cases:
	//   - ProjectIDsIn == nil: not set (default
	//     behaviour, no filter)
	//   - ProjectIDsIn == []string{} (non-nil but
	//     empty): explicit "scoped caller with no
	//     memberships" — the deny-by-default case
	//     (WHERE 1=0). A non-SuperAdmin caller with
	//     zero memberships is the contract for "deny
	//     by default".
	if f.ProjectIDsIn != nil {
		if len(f.ProjectIDsIn) == 0 {
			q = q.Where("1 = 0")
		} else {
			q = q.Where(
				"json_extract(metadata, '$.project_id') IN ?",
				f.ProjectIDsIn,
			)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("audit.Repository.List count: %w", err)
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	rows := []AuditEvent{}
	if err := q.Order("occurred_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("audit.Repository.List find: %w", err)
	}
	return rows, total, nil
}

// WithContext is a thin shim for callers that want to thread a
// context. The repository does not currently use it, but
// exposing it keeps the API future-proof for the emitter (which
// needs context-aware inserts when the buffer overflows).
func (r *Repository) WithContext(_ context.Context) *Repository { return r }
