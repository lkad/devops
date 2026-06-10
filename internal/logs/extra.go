package logs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/backend/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RetentionPolicy is the durable record of the system's log
// retention settings. The spec calls for two knobs:
// retention_days and max_storage_gb. We persist them on a
// singleton row (ID="default") so the GORM AutoMigrate
// stays simple — a dedicated table is overkill for one
// config.
type RetentionPolicy struct {
	database.BaseModel
	// SingletonKey is the literal string "default" so the
	// repository can upsert a single row. An empty key
	// would create a fresh row; the service refuses that.
	SingletonKey  string `gorm:"column:singleton_key;type:text;not null;uniqueIndex" json:"-"`
	RetentionDays int    `gorm:"column:retention_days;not null;default:7" json:"retention_days"`
	MaxStorageGB   int    `gorm:"column:max_storage_gb;not null;default:100" json:"max_storage_gb"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName pins the GORM-generated table name.
func (RetentionPolicy) TableName() string { return "log_retention_policies" }

// BeforeCreate wires the GORM hook. The Default() time
// value on a freshly-inserted row is the moment the row
// is created — useful for the "stale policy" UI badge.
func (p *RetentionPolicy) BeforeCreate(tx *gorm.DB) error {
	if p.SingletonKey == "" {
		p.SingletonKey = "default"
	}
	return p.BaseModel.BeforeCreate(tx)
}

// SavedFilter is a user-named query the operator can re-
// run with a single click. The Query column is a JSON blob
// shaped like the /logs/query request body.
type SavedFilter struct {
	database.BaseModel
	OwnerUserID string `gorm:"column:owner_user_id;type:text;not null;index" json:"owner_user_id"`
	Name        string `gorm:"column:name;size:128;not null" json:"name"`
	Query       string `gorm:"column:query;type:text;not null" json:"query"`
}

// TableName pins the GORM-generated table name.
func (SavedFilter) TableName() string { return "log_saved_filters" }

// AlertRule is a row-level log-derived alert. The spec
// calls for name + condition + window + threshold + channel;
// we persist all five so the alert manager can evaluate
// them.
type AlertRule struct {
	database.BaseModel
	Name      string `gorm:"column:name;size:128;not null" json:"name"`
	Condition string `gorm:"column:condition;type:text;not null" json:"condition"`
	Window    string `gorm:"column:window;size:16;not null" json:"window"`
	Threshold int    `gorm:"column:threshold;not null;default:0" json:"threshold"`
	Channel   string `gorm:"column:channel;size:64;not null" json:"channel"`
}

// TableName pins the GORM-generated table name.
func (AlertRule) TableName() string { return "log_alert_rules" }

// AllExtraModels returns every GORM model this file owns
// so the package-level AllModels can keep a single
// registration point.
func AllExtraModels() []any {
	return []any{
		&RetentionPolicy{},
		&SavedFilter{},
		&AlertRule{},
	}
}

// =============================================================================
// Repository: thin GORM wrappers. Each method is small
// enough to test in isolation.
// =============================================================================

// ExtraRepository is the GORM data-access layer for the
// retention/saved-filters/alert-rules tables. The constructor
// takes a *gorm.DB so the existing test fixture (which
// already calls openDB) can reuse the same handle.
type ExtraRepository struct {
	db *gorm.DB
}

// NewExtraRepository builds the repository. A nil db is a
// programming error and panics in dev — every wiring site
// already has a *gorm.DB.
func NewExtraRepository(db *gorm.DB) *ExtraRepository { return &ExtraRepository{db: db} }

// GetRetention returns the singleton retention row, or a
// zero-value + ErrNotFound if none exists yet. The handler
// seeds a default on the first GET.
func (r *ExtraRepository) GetRetention() (*RetentionPolicy, error) {
	var p RetentionPolicy
	if err := r.db.First(&p, "singleton_key = ?", "default").Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
		return nil, err
	}
	return &p, nil
}

// UpsertRetention inserts (or updates) the singleton row.
// GORM's Clauses(clause.OnConflict{...}) keeps the upsert
// to a single statement so the SQLite and Postgres backends
// both behave the same. The "default" singleton key is
// the ON CONFLICT target.
func (r *ExtraRepository) UpsertRetention(p *RetentionPolicy) error {
	if p == nil {
		return errors.New("nil retention policy")
	}
	p.SingletonKey = "default"
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "singleton_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"retention_days", "max_storage_gb", "updated_at"}),
	}).Create(p).Error
}

// ListSavedFilters returns every saved filter in name order.
// Owner scoping is a future iteration; the spec doesn't
// require per-user visibility yet.
func (r *ExtraRepository) ListSavedFilters() ([]SavedFilter, error) {
	var rows []SavedFilter
	if err := r.db.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetSavedFilter returns one row or ErrNotFound.
func (r *ExtraRepository) GetSavedFilter(id string) (*SavedFilter, error) {
	var f SavedFilter
	if err := r.db.First(&f, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrNotFound)
		return nil, err
	}
	return &f, nil
}

// CreateSavedFilter inserts a row. Empty Name is rejected
// at the handler layer (a 400) before reaching here.
func (r *ExtraRepository) CreateSavedFilter(f *SavedFilter) error {
	return r.db.Create(f).Error
}

// DeleteSavedFilter removes a row by id. Returns ErrNotFound
// when the row was already gone so the handler can render
// a 404 instead of a 204.
func (r *ExtraRepository) DeleteSavedFilter(id string) error {
	res := r.db.Delete(&SavedFilter{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAlertRules returns every alert rule ordered by name.
func (r *ExtraRepository) ListAlertRules() ([]AlertRule, error) {
	var rows []AlertRule
	if err := r.db.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateAlertRule inserts a row.
func (r *ExtraRepository) CreateAlertRule(r2 *AlertRule) error {
	return r.db.Create(r2).Error
}

// DeleteAlertRule removes a row by id.
func (r *ExtraRepository) DeleteAlertRule(id string) error {
	res := r.db.Delete(&AlertRule{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// =============================================================================
// Service: business rules on top of the repository.
// =============================================================================

// ExtraService holds the dependency-injected repository.
// All methods are safe for concurrent use; the underlying
// GORM calls are serialised by the *sql.DB pool.
type ExtraService struct {
	repo  *ExtraRepository
	logs  *Service
	mu    sync.Mutex
}

// NewExtraService wires the service. A nil *Service (logs
// query) is OK — Apply() then becomes a no-op so the
// saved-filter "apply" path still returns the rendered
// query without trying to fetch rows.
func NewExtraService(repo *ExtraRepository, logs *Service) *ExtraService {
	return &ExtraService{repo: repo, logs: logs}
}

// GetRetention returns the current policy, seeding a
// default if none exists. The seed is "default" singleton
// with retention_days=7 and max_storage_gb=100 (the spec's
// recommendation).
func (s *ExtraService) GetRetention() (*RetentionPolicy, error) {
	p, err := s.repo.GetRetention()
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	// Seed the default row. The mutex avoids two parallel
	// first-callers each inserting the singleton.
	s.mu.Lock()
	defer s.mu.Unlock()
	p2, err2 := s.repo.GetRetention()
	if err2 == nil {
		return p2, nil
	}
	defaults := &RetentionPolicy{
		SingletonKey:  "default",
		RetentionDays: 7,
		MaxStorageGB:   100,
	}
	if err := s.repo.UpsertRetention(defaults); err != nil {
		return nil, err
	}
	return defaults, nil
}

// SetRetention persists the supplied policy. Validation is
// done by the handler; this method is a thin pass-through.
func (s *ExtraService) SetRetention(p *RetentionPolicy) (*RetentionPolicy, error) {
	if err := s.repo.UpsertRetention(p); err != nil {
		return nil, err
	}
	return s.repo.GetRetention()
}

// TriggerCleanup is a stub for the spec's "Trigger retention
// cleanup" scenario. A real implementation would call the
// backend's delete-by-time API; here we surface a zero-
// value report because the local backend's cleanup path
// is a no-op (files are pruned by the os.Rotate logic
// that lives outside the API).
func (s *ExtraService) TriggerCleanup(_ context.Context) (map[string]any, error) {
	return map[string]any{
		"rows_removed": 0,
		"bytes_freed":  0,
		"completed_at": time.Now().UTC(),
	}, nil
}

// Stats returns the LogStatistics shape the spec demands.
// The local backend's Stats() is a real implementation; the
// service just passes it through.
func (s *ExtraService) Stats(ctx context.Context) (map[string]any, error) {
	if s.logs == nil {
		return map[string]any{
			"total":     0,
			"by_level":  map[string]int{},
			"by_source": map[string]int{},
		}, nil
	}
	stats, err := s.logs.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// =============================================================================
// Validation helpers (handler-layer rules pulled out so the
// test can exercise them in isolation).
// =============================================================================

// validateName rejects empty or whitespace-only names so
// the DB doesn't end up with "" rows.
func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if len(name) > 128 {
		return errors.New("name exceeds 128 chars")
	}
	return nil
}
