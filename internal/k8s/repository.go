package k8s

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"
)

// ErrNotFound is the typed sentinel returned by every
// Repository method when a row is missing. The service layer
// wraps it in a 404 NOT_FOUND APIError; the handler does the
// rendering. We do NOT re-export gorm.ErrRecordNotFound so the
// service can branch with errors.Is without taking a transitive
// dependency on GORM.
var ErrNotFound = errors.New("k8s: cluster not found")

// IsNotFound reports whether err is (or wraps) ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// Repository is the GORM-only data-access layer. Per the
// layering rules it knows nothing about Gin, contracts, or
// business validation — it just translates method calls into
// queries.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository. The DB is shared with
// the rest of the application; the repository does not own its
// lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new cluster row. The ID is filled in by
// BaseModel.BeforeCreate; the caller can read c.ID immediately
// after Create returns.
func (r *Repository) Create(c *Cluster) error {
	if err := r.db.Create(c).Error; err != nil {
		return fmt.Errorf("k8s.Create: %w", err)
	}
	return nil
}

// Get returns the cluster with the given ID, or ErrNotFound if
// no such row exists. Soft-deleted rows are hidden.
func (r *Repository) Get(id string) (*Cluster, error) {
	var c Cluster
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &c, nil
}

// FindByName returns the cluster with the given Name, or
// ErrNotFound if no such row exists. Used by the service to
// enforce the unique-name constraint.
func (r *Repository) FindByName(name string) (*Cluster, error) {
	var c Cluster
	if err := r.db.First(&c, "name = ?", name).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &c, nil
}

// List returns a page of clusters plus the unfiltered total
// count. All filter fields are optional; the zero value returns
// everything.
func (r *Repository) List(f ListFilter) ([]Cluster, int64, error) {
	q := r.db.Model(&Cluster{})
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("k8s.List count: %w", err)
	}

	rows := []Cluster{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("k8s.List find: %w", err)
	}
	return rows, total, nil
}

// Update persists the entire cluster record. The row must
// already exist; an Update on a missing ID returns ErrNotFound.
func (r *Repository) Update(c *Cluster) error {
	var existing Cluster
	err := r.db.First(&existing, "id = ?", c.ID).Error
	if err != nil {
		return database.MapNotFound(err, ErrNotFound)
	}
	if err := r.db.Save(c).Error; err != nil {
		return fmt.Errorf("k8s.Update: %w", err)
	}
	return nil
}

// UpdateStatus writes only the status + last-checked columns
// of a cluster. Used by the connectivity probe so the read
// path can show "connected / disconnected" without round-tripping
// the entire row.
func (r *Repository) UpdateStatus(id string, status ClusterStatus, checkedAt *time.Time) error {
	res := r.db.Model(&Cluster{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":         status,
			"last_checked_at": checkedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("k8s.UpdateStatus: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete soft-deletes the cluster. The row stays in the table
// with DeletedAt set; subsequent Get/List calls will not see it.
func (r *Repository) Delete(id string) error {
	res := r.db.Delete(&Cluster{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("k8s.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
