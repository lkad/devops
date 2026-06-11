package alerts

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrNotFound is the typed sentinel returned by every Repository
// method when a row is missing. The service layer wraps it in a
// 404 NOT_FOUND APIError; the handler does the rendering.
var ErrNotFound = errors.New("alerts: not found")

// IsNotFound is the package-level convenience used by service /
// handler tests. The function is the same name as the one in
// models.go so callers can use either.
func isNotFoundErr(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// AlertFilter is the query-side DTO for ListAlerts. Pointer
// fields let the handler distinguish "not set" from "zero
// value" when building the SQL WHERE clause.
type AlertFilter struct {
	Name       string
	Severity   Severity
	SourceType SourceType
	SourceID   string
	State      State
	Suppressed *bool
	Limit      int
	Offset     int
}

// Repository is the GORM-only data-access layer for the alert
// subsystem. It knows about rows and indexes but nothing about
// business rules — those live in the service layer.
type Repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the supplied db.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// CreateAlert inserts a new alert row. The ID is filled in by
// BaseModel.BeforeCreate; the caller can read a.ID immediately
// after Create returns.
func (r *Repository) CreateAlert(a *Alert) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("alerts.CreateAlert: %w", err)
	}
	return nil
}

// GetAlert returns the alert with the given ID, or ErrNotFound
// if no such row exists. Soft-deleted rows are hidden.
func (r *Repository) GetAlert(id string) (*Alert, error) {
	var a Alert
	if err := r.db.First(&a, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &a, nil
}

// ListAlerts returns a page of alerts plus the unfiltered total
// count. All filter fields are optional; the zero value returns
// everything.
func (r *Repository) ListAlerts(f AlertFilter) ([]Alert, int64, error) {
	q := r.db.Model(&Alert{})
	if f.Name != "" {
		q = q.Where("name = ?", f.Name)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if f.SourceType != "" {
		q = q.Where("source_type = ?", f.SourceType)
	}
	if f.SourceID != "" {
		q = q.Where("source_id = ?", f.SourceID)
	}
	if f.State != "" {
		q = q.Where("state = ?", f.State)
	}
	if f.Suppressed != nil {
		q = q.Where("suppressed = ?", *f.Suppressed)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("alerts.ListAlerts count: %w", err)
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	rows := []Alert{}
	if err := q.Order("fired_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("alerts.ListAlerts find: %w", err)
	}
	return rows, total, nil
}

// UpdateAlert applies a partial update. The patch is a map so
// callers can ignore zero values. Empty patches are an error so
// the caller gets feedback rather than a silent no-op.
func (r *Repository) UpdateAlert(id string, patch map[string]any) error {
	if len(patch) == 0 {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "update patch is empty",
		}
	}
	tx := r.db.Model(&Alert{}).Where("id = ?", id).Updates(patch)
	if tx.Error != nil {
		return fmt.Errorf("alerts.UpdateAlert: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAlert soft-deletes the alert.
func (r *Repository) DeleteAlert(id string) error {
	tx := r.db.Delete(&Alert{}, "id = ?", id)
	if tx.Error != nil {
		return fmt.Errorf("alerts.DeleteAlert: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateChannel inserts a new channel row.
func (r *Repository) CreateChannel(c *Channel) error {
	if err := r.db.Create(c).Error; err != nil {
		return fmt.Errorf("alerts.CreateChannel: %w", err)
	}
	return nil
}

// GetChannel returns the channel with the given ID.
func (r *Repository) GetChannel(id string) (*Channel, error) {
	var c Channel
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &c, nil
}

// ListChannels returns every channel, sorted by created_at so the
// UI dropdown has a stable order across requests.
func (r *Repository) ListChannels() ([]Channel, error) {
	var out []Channel
	if err := r.db.Order("created_at ASC, id ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("alerts.ListChannels: %w", err)
	}
	return out, nil
}

// UpdateChannel applies a partial update.
func (r *Repository) UpdateChannel(id string, patch map[string]any) error {
	if len(patch) == 0 {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "update patch is empty",
		}
	}
	tx := r.db.Model(&Channel{}).Where("id = ?", id).Updates(patch)
	if tx.Error != nil {
		return fmt.Errorf("alerts.UpdateChannel: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteChannel soft-deletes the channel.
func (r *Repository) DeleteChannel(id string) error {
	tx := r.db.Delete(&Channel{}, "id = ?", id)
	if tx.Error != nil {
		return fmt.Errorf("alerts.DeleteChannel: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateRule inserts a new alert rule row.
func (r *Repository) CreateRule(rl *AlertRule) error {
	if err := r.db.Create(rl).Error; err != nil {
		return fmt.Errorf("alerts.CreateRule: %w", err)
	}
	return nil
}

// GetRule returns the rule with the given ID.
func (r *Repository) GetRule(id string) (*AlertRule, error) {
	var rl AlertRule
	if err := r.db.First(&rl, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
	}
	return &rl, nil
}

// ListRules returns every alert rule.
func (r *Repository) ListRules() ([]AlertRule, error) {
	var out []AlertRule
	if err := r.db.Order("created_at ASC, id ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("alerts.ListRules: %w", err)
	}
	return out, nil
}

// DeleteRule soft-deletes the rule.
func (r *Repository) DeleteRule(id string) error {
	tx := r.db.Delete(&AlertRule{}, "id = ?", id)
	if tx.Error != nil {
		return fmt.Errorf("alerts.DeleteRule: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// WithContext is a thin shim for callers that want to thread a
// context. The repository does not currently use it, but
// exposing it keeps the API future-proof for the audit module.
func (r *Repository) WithContext(_ context.Context) *Repository { return r }
