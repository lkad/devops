package metrics

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"
)

// ErrNotFound is the typed sentinel returned by every
// Repository method when a row is missing. The service layer
// wraps it in a 404 NOT_FOUND APIError; the handler does the
// rendering. We do NOT re-export gorm.ErrRecordNotFound so the
// service can branch with errors.Is without taking a
// transitive dependency on GORM.
var ErrNotFound = errors.New("metrics: not found")

// IsNotFound reports whether err is (or wraps) ErrNotFound.
// The service and handler both call this rather than ==
// comparisons so wrapped errors are still recognised.
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

// NewRepository constructs a Repository. The DB is shared
// with the rest of the application; the repository does not
// own its lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new metric row. The ID is filled in by
// BaseModel.BeforeCreate; the caller can read m.ID
// immediately after Create returns.
func (r *Repository) Create(m *Metric) error {
	if err := r.db.Create(m).Error; err != nil {
		return fmt.Errorf("metrics.Create: %w", err)
	}
	return nil
}

// Get returns the metric with the given ID, or ErrNotFound
// if no such row exists. Soft-deleted rows are hidden.
func (r *Repository) Get(id string) (*Metric, error) {
	var m Metric
	if err := r.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
		return nil, fmt.Errorf("metrics.Get: %w", err)
	}
	return &m, nil
}

// List returns a page of metrics plus the unfiltered total
// count (so the handler can render the Pagination.HasMore
// flag). All filter fields are optional; the zero value
// returns everything.
//
// Time bounds: From is inclusive (>=), To is exclusive (<).
// The service layer is responsible for validating that the
// range does not exceed MaxQueryRange.
func (r *Repository) List(f ListFilter) ([]Metric, int64, error) {
	q := r.db.Model(&Metric{})
	if f.Name != "" {
		q = q.Where("name = ?", f.Name)
	}
	if f.TargetType != "" {
		q = q.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	if !f.From.IsZero() {
		q = q.Where("timestamp >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("timestamp < ?", f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("metrics.List count: %w", err)
	}

	rows := []Metric{}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	// Order by timestamp descending for list endpoints — the
	// newest observation first matches the dashboard's
	// expectation.
	if err := q.Order("timestamp DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("metrics.List find: %w", err)
	}
	return rows, total, nil
}

// ListSeries returns the unique (name, target_type, target_id,
// labels) tuples present in the metrics table, optionally
// filtered by name / target_type / target_id / time-range.
// The result is the catalogue of time-series the dashboard
// can plot; data points are not included.
func (r *Repository) ListSeries(f ListFilter) ([]MetricSeries, error) {
	q := r.db.Model(&Metric{})
	if f.Name != "" {
		q = q.Where("name = ?", f.Name)
	}
	if f.TargetType != "" {
		q = q.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	if !f.From.IsZero() {
		q = q.Where("timestamp >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("timestamp < ?", f.To)
	}

	type row struct {
		Name       string
		TargetType string
		TargetID   string
		Labels     JSONMap
	}
	var rows []row
	if err := q.Select("name, target_type, target_id, labels").Group("name, target_type, target_id, labels").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("metrics.ListSeries: %w", err)
	}
	out := make([]MetricSeries, 0, len(rows))
	for _, r := range rows {
		out = append(out, MetricSeries{
			Name:       r.Name,
			TargetType: r.TargetType,
			TargetID:   r.TargetID,
			Labels:     r.Labels,
			Points:     []DataPoint{},
		})
	}
	return out, nil
}

// GetSeriesByName returns the data points of a single series
// identified by (name, target_type, target_id), ordered by
// timestamp ascending. A missing series returns an empty
// slice and no error — the dashboard should treat that as
// "no data" rather than a 404.
//
// Time bounds: From is inclusive (>=), To is exclusive (<).
// The service layer is responsible for validating that the
// range does not exceed MaxQueryRange.
func (r *Repository) GetSeriesByName(name, targetType, targetID string, f ListFilter) ([]DataPoint, error) {
	q := r.db.Model(&Metric{}).
		Where("name = ?", name).
		Where("target_type = ?", targetType).
		Where("target_id = ?", targetID)
	if !f.From.IsZero() {
		q = q.Where("timestamp >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("timestamp < ?", f.To)
	}
	var rows []DataPoint
	if err := q.Order("timestamp ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("metrics.GetSeriesByName: %w", err)
	}
	return rows, nil
}
