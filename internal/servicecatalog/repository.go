package servicecatalog

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrNotFound is the typed sentinel returned when a row
// is missing. The service layer wraps it in a 404 APIError;
// the handler does the rendering. We use a single sentinel
// for the whole package rather than one-per-method so
// wrapped errors still satisfy errors.Is.
var ErrNotFound = errors.New("servicecatalog: not found")

// IsNotFound reports whether err is (or wraps) the
// package's not-found sentinel. Callers should prefer
// this over == comparisons.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// Repository is the GORM-only data-access layer for the
// Service entity. Per the project's layering rules it
// knows nothing about Gin, contracts, or business
// validation — it just translates method calls into
// queries.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository. The DB is shared
// with the rest of the application; the repository does not
// own its lifecycle.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ServiceFilter narrows the result of List. All fields
// are optional; the zero value returns every non-deleted
// service.
type ServiceFilter struct {
	// Tier restricts to services with this exact tier.
	Tier Tier
	// Owner restricts to services with this exact owner.
	Owner string
	// Query restricts to services whose Name contains
	// the substring (case-insensitive).
	Query string
	// Limit caps the page size; 0 means "no limit".
	Limit  int
	Offset int
}

// Create inserts a new Service row. The ID and timestamps
// are populated by GORM hooks; the caller can read
// s.ID / s.CreatedAt / s.UpdatedAt immediately after
// Create returns.
func (r *Repository) Create(s *Service) error {
	if err := r.db.Create(s).Error; err != nil {
		return fmt.Errorf("servicecatalog.Create: %w", err)
	}
	return nil
}

// Get returns a single Service by ID. Returns ErrNotFound
// (possibly wrapped) if the row is missing or soft-deleted.
func (r *Repository) Get(id string) (*Service, error) {
	var s Service
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("servicecatalog.Get: %w", err)
	}
	return &s, nil
}

// Update persists the full Service row identified by
// s.ID. Only the fields set on s are written; the rest
// are preserved by the row's existing values.
func (r *Repository) Update(s *Service) error {
	res := r.db.Model(&Service{}).Where("id = ?", s.ID).Updates(s)
	if res.Error != nil {
		return fmt.Errorf("servicecatalog.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete marks the row as deleted (deleted_at = now).
// The row is preserved so pipelines with this service_id
// keep their history; the FK is ON DELETE SET NULL at the
// schema level so this method does not need to NULL out
// any children.
func (r *Repository) SoftDelete(id string) error {
	res := r.db.Delete(&Service{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("servicecatalog.SoftDelete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns services matching f, in name-ascending order.
func (r *Repository) List(f ServiceFilter) ([]Service, error) {
	q := r.db.Model(&Service{}).Order("name ASC")
	if f.Tier != "" {
		q = q.Where("tier = ?", f.Tier)
	}
	if f.Owner != "" {
		q = q.Where("owner = ?", f.Owner)
	}
	if f.Query != "" {
		// LIKE is case-insensitive in SQLite by default for
		// ASCII; in Postgres it is case-sensitive so we
		// LOWER both sides. This costs an index-skip on
		// large datasets, but the operator-facing filter
		// set is small (per page, not full table).
		q = q.Where("LOWER(name) LIKE ?", "%"+lowerASCII(f.Query)+"%")
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	var rows []Service
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("servicecatalog.List: %w", err)
	}
	return rows, nil
}

// lowerASCII is a tiny case-folder used only by List.Query.
// Full unicode is intentionally avoided; the operator-facing
// search box expects ASCII names.
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
