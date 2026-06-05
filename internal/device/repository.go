package device

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
var ErrNotFound = errors.New("device: not found")

// IsNotFound reports whether err is (or wraps) ErrNotFound.
// The service and handler both call this rather than ==
// comparisons so wrapped errors are still recognised.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
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

// Create inserts a new device row. The ID is filled in by
// BaseModel.BeforeCreate; the caller can read d.ID immediately
// after Create returns.
func (r *Repository) Create(d *Device) error {
	if err := r.db.Create(d).Error; err != nil {
		return fmt.Errorf("device.Create: %w", err)
	}
	return nil
}

// Get returns the device with the given ID, or ErrNotFound if
// no such row exists. Soft-deleted rows are hidden.
func (r *Repository) Get(id string) (*Device, error) {
	var d Device
	if err := r.db.First(&d, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("device.Get: %w", err)
	}
	return &d, nil
}

// List returns a page of devices plus the unfiltered total count
// (so the handler can render the Pagination.HasMore flag). All
// filter fields are optional; the zero value returns everything.
func (r *Repository) List(f ListFilter) ([]Device, int64, error) {
	q := r.db.Model(&Device{})
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	if f.State != "" {
		q = q.Where("state = ?", f.State)
	}
	if f.GroupID != "" {
		q = q.Where("group_id = ?", f.GroupID)
	}
	if f.Search != "" {
		// SQLite's LIKE is case-insensitive for ASCII by default;
		// GORM translates this to a portable LOWER(...) LIKE LOWER(...)
		// when the driver supports it. We use the simpler form
		// here and rely on the database collation.
		like := "%" + f.Search + "%"
		q = q.Where("name LIKE ?", like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("device.List count: %w", err)
	}

	rows := []Device{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("device.List find: %w", err)
	}
	return rows, total, nil
}

// Update persists the entire device record. The row must already
// exist; an Update on a missing ID returns ErrNotFound. We
// verify existence explicitly rather than relying on Save's
// upsert behaviour so the contract is "update only".
func (r *Repository) Update(d *Device) error {
	var existing Device
	err := r.db.First(&existing, "id = ?", d.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("device.Update lookup: %w", err)
	}
	if err := r.db.Save(d).Error; err != nil {
		return fmt.Errorf("device.Update: %w", err)
	}
	return nil
}

// Delete soft-deletes the device. The row stays in the table
// with DeletedAt set; subsequent Get/List calls will not see it.
func (r *Repository) Delete(id string) error {
	res := r.db.Delete(&Device{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("device.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Search is a thin wrapper around List with a single Search
// filter. It exists so the handler has a single method to call
// for /api/v1/devices/search without re-implementing the
// LIKE-clause construction.
func (r *Repository) Search(query string) ([]Device, int64, error) {
	return r.List(ListFilter{Search: query, Limit: 0, Offset: 0})
}
