package discovery

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"
)

// ErrNotFound is the typed sentinel returned by every Repository
// method when a row is missing. The service layer wraps it in a
// 404 NOT_FOUND APIError; the handler does the rendering. We do
// NOT re-export gorm.ErrRecordNotFound so the service can branch
// with errors.Is without taking a transitive dependency on GORM.
var ErrNotFound = errors.New("discovery: not found")

// IsNotFound reports whether err is (or wraps) ErrNotFound.
// The service and handler both call this rather than ==
// comparisons so wrapped errors are still recognised.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// ListFilter narrows the result of a Repository.ListRuns call.
// All fields are optional; the zero value returns every run.
type ListFilter struct {
	// Limit caps the page size; 0 falls back to the default
	// (20, per contracts.defaultPageSize).
	Limit int
	// Offset is the number of rows to skip; 0 means start at
	// the beginning of the result set.
	Offset int
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

// CreateRun inserts a new DiscoveryRun row. The ID is filled
// in by BaseModel.BeforeCreate; the caller can read run.ID
// immediately after CreateRun returns.
func (r *Repository) CreateRun(run *DiscoveryRun) error {
	if err := r.db.Create(run).Error; err != nil {
		return fmt.Errorf("discovery.CreateRun: %w", err)
	}
	return nil
}

// GetRun returns the run with the given ID, or ErrNotFound if
// no such row exists. Soft-deleted rows are hidden.
func (r *Repository) GetRun(id string) (*DiscoveryRun, error) {
	var run DiscoveryRun
	if err := r.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &run, nil
}

// ListRuns returns a page of runs plus the unfiltered total
// count (so the handler can render the Pagination.HasMore
// flag).
func (r *Repository) ListRuns(f ListFilter) ([]DiscoveryRun, int64, error) {
	q := r.db.Model(&DiscoveryRun{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("discovery.ListRuns count: %w", err)
	}
	rows := []DiscoveryRun{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("discovery.ListRuns find: %w", err)
	}
	return rows, total, nil
}

// UpdateRun persists the entire run record. The row must
// already exist; an Update on a missing ID returns ErrNotFound.
func (r *Repository) UpdateRun(run *DiscoveryRun) error {
	var existing DiscoveryRun
	err := r.db.First(&existing, "id = ?", run.ID).Error
	if err != nil {
		return database.MapNotFound(err, ErrNotFound)
	}
	if err := r.db.Save(run).Error; err != nil {
		return fmt.Errorf("discovery.UpdateRun: %w", err)
	}
	return nil
}

// DeleteRun soft-deletes the run and cascades the delete to
// its hosts. The cascade is application-level (GORM's
// soft-delete does not cascade automatically).
func (r *Repository) DeleteRun(id string) error {
	if err := r.db.Delete(&DiscoveryRun{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("discovery.DeleteRun: %w", err)
	}
	if err := r.db.Delete(&DiscoveredHost{}, "run_id = ?", id).Error; err != nil {
		return fmt.Errorf("discovery.DeleteRun cascade: %w", err)
	}
	return nil
}

// CreateHost inserts a new DiscoveredHost row.
func (r *Repository) CreateHost(host *DiscoveredHost) error {
	if err := r.db.Create(host).Error; err != nil {
		return fmt.Errorf("discovery.CreateHost: %w", err)
	}
	return nil
}

// GetHost returns the host with the given ID, or ErrNotFound
// if no such row exists.
func (r *Repository) GetHost(id string) (*DiscoveredHost, error) {
	var host DiscoveredHost
	if err := r.db.First(&host, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &host, nil
}

// UpdateHost persists the entire host record. Used by the
// service to set PromotedToDeviceID after a successful
// promote.
func (r *Repository) UpdateHost(host *DiscoveredHost) error {
	var existing DiscoveredHost
	err := r.db.First(&existing, "id = ?", host.ID).Error
	if err != nil {
		return database.MapNotFound(err, ErrNotFound)
	}
	if err := r.db.Save(host).Error; err != nil {
		return fmt.Errorf("discovery.UpdateHost: %w", err)
	}
	return nil
}

// ListHostsByRun returns every host that belongs to the
// given run, in stable order (IP address ascending).
func (r *Repository) ListHostsByRun(runID string) ([]DiscoveredHost, error) {
	rows := []DiscoveredHost{}
	if err := r.db.Where("run_id = ?", runID).Order("ip_address ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("discovery.ListHostsByRun: %w", err)
	}
	return rows, nil
}

// HostExistsByIP is the dedup key. The service uses it to
// skip hosts that have been seen in any earlier scan, which
// is the spec's "Result deduplication across scans" rule.
func (r *Repository) HostExistsByIP(ip string) (bool, error) {
	var count int64
	if err := r.db.Model(&DiscoveredHost{}).Where("ip_address = ?", ip).Count(&count).Error; err != nil {
		return false, fmt.Errorf("discovery.HostExistsByIP: %w", err)
	}
	return count > 0, nil
}
