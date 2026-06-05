package physicalhost

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrNotFound is the typed sentinel returned by every Repository
// method when a row is missing. The service layer wraps it in a
// 404 NOT_FOUND APIError; the handler does the rendering. We do
// NOT re-export gorm.ErrRecordNotFound so the service can branch
// with errors.Is without taking a transitive dependency on GORM.
var ErrNotFound = errors.New("physicalhost: not found")

// IsNotFound reports whether err is (or wraps) ErrNotFound. The
// service and handler both call this rather than == comparisons
// so wrapped errors are still recognised.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// ListFilter narrows the result of a Repository.List call. All
// fields are optional; the zero value returns every row.
type ListFilter struct {
	// State filters by PhysicalHostState (exact match).
	State PhysicalHostState
	// DeviceID filters by the device FK (exact match).
	DeviceID string
	// Limit caps the page size; 0 falls back to the default
	// (20, per contracts.defaultPageSize).
	Limit int
	// Offset is the number of rows to skip.
	Offset int
}

// Repository is the GORM-only data-access layer. Per the layering
// rules it knows nothing about Gin, contracts, or business
// validation — it just translates method calls into queries.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository. The DB is shared with
// the rest of the application; the repository does not own its
// lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new physical_hosts row. The ID is filled in
// by BaseModel.BeforeCreate; the caller can read p.ID immediately
// after Create returns.
func (r *Repository) Create(p *PhysicalHost) error {
	if err := r.db.Create(p).Error; err != nil {
		return fmt.Errorf("physicalhost.Create: %w", err)
	}
	return nil
}

// Get returns the host with the given ID, or ErrNotFound if no
// such row exists. Soft-deleted rows are hidden.
func (r *Repository) Get(id string) (*PhysicalHost, error) {
	var p PhysicalHost
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("physicalhost.Get: %w", err)
	}
	return &p, nil
}

// List returns a page of physical hosts plus the unfiltered
// total count (so the handler can render the Pagination.HasMore
// flag). All filter fields are optional; the zero value returns
// everything.
func (r *Repository) List(f ListFilter) ([]PhysicalHost, int64, error) {
	q := r.db.Model(&PhysicalHost{})
	if f.State != "" {
		q = q.Where("state = ?", f.State)
	}
	if f.DeviceID != "" {
		q = q.Where("device_id = ?", f.DeviceID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("physicalhost.List count: %w", err)
	}

	rows := []PhysicalHost{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("physicalhost.List find: %w", err)
	}
	return rows, total, nil
}

// Update persists the entire record. The row must already exist;
// an Update on a missing ID returns ErrNotFound. We verify
// existence explicitly rather than relying on Save's upsert
// behaviour so the contract is "update only".
func (r *Repository) Update(p *PhysicalHost) error {
	var existing PhysicalHost
	err := r.db.First(&existing, "id = ?", p.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("physicalhost.Update lookup: %w", err)
	}
	if err := r.db.Save(p).Error; err != nil {
		return fmt.Errorf("physicalhost.Update: %w", err)
	}
	return nil
}

// Delete soft-deletes the host. The row stays in the table with
// DeletedAt set; subsequent Get/List calls will not see it.
func (r *Repository) Delete(id string) error {
	res := r.db.Delete(&PhysicalHost{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("physicalhost.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
