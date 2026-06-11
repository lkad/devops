package hostproject

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"github.com/devops-toolkit/backend/internal/database"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Repository is the GORM-only data-access layer for
// host_project_links. It is the only place in the package
// that issues SQL. Business rules (existence checks, bulk
// semantics, hierarchy walk, orphaning policy) live in the
// service layer.
type Repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the supplied
// db. The caller owns the db lifecycle; the repository is
// stateless.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create persists a new link. A duplicate (DeviceID,
// ProjectID) pair yields a Conflict APIError so the service
// layer can render a 409. LinkedAt defaults to now() if the
// caller did not supply one — the service layer is the
// authoritative source of the timestamp, but the repository
// is forgiving so tests can omit it.
func (r *Repository) Create(l HostProjectLink) (HostProjectLink, error) {
	if l.LinkedAt.IsZero() {
		l.LinkedAt = time.Now().UTC()
	}
	if err := r.db.Create(&l).Error; err != nil {
		if isUniqueViolation(err, "host_project_links.device_id") {
			return HostProjectLink{}, &contracts.APIError{
				Code:    contracts.CodeConflict,
				Message: "host is already linked to this project",
			}
		}
		return HostProjectLink{}, err
	}
	return l, nil
}

// Get returns a single link by ID. Missing rows surface as a
// NotFound APIError so handlers do not have to translate
// gorm errors themselves.
func (r *Repository) Get(id string) (HostProjectLink, error) {
	var l HostProjectLink
	if err := r.db.First(&l, "id = ?", id).Error; err != nil {
		return HostProjectLink{}, database.MapNotFound(err, notFound("host project link", id))
	}
	return l, nil
}

// ListByDevice returns every active (non-orphaned) link for
// a given device. Active means OrphanedAt IS NULL.
func (r *Repository) ListByDevice(deviceID string) ([]HostProjectLink, error) {
	var out []HostProjectLink
	if err := r.db.Where("device_id = ? AND orphaned_at IS NULL", deviceID).
		Order("linked_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListByProject returns every active link for a given
// project. Active means OrphanedAt IS NULL.
func (r *Repository) ListByProject(projectID string) ([]HostProjectLink, error) {
	var out []HostProjectLink
	if err := r.db.Where("project_id = ? AND orphaned_at IS NULL", projectID).
		Order("linked_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Delete hard-deletes a link by ID. A missing row returns
// NotFound. This is the user-initiated unlink path; the
// device-cascade path uses OrphanByDevice instead.
func (r *Repository) Delete(id string) error {
	tx := r.db.Where("id = ?", id).Delete(&HostProjectLink{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return notFound("host project link", id)
	}
	return nil
}

// DeleteByDeviceAndProject is the bulk-unlink-friendly
// counterpart to Delete: it removes the active link between
// a (device, project) pair. A missing pair returns NotFound.
func (r *Repository) DeleteByDeviceAndProject(deviceID, projectID string) error {
	tx := r.db.Where("device_id = ? AND project_id = ? AND orphaned_at IS NULL", deviceID, projectID).
		Delete(&HostProjectLink{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return notFound("host project link", deviceID+":"+projectID)
	}
	return nil
}

// OrphanByDevice marks every active link for the supplied
// device as orphaned. The rows stay in the table for audit
// but are excluded from ListByDevice / ListByProject. This
// is the cascade hook called when a Device is soft-deleted.
func (r *Repository) OrphanByDevice(deviceID string) error {
	now := time.Now().UTC()
	return r.db.Model(&HostProjectLink{}).
		Where("device_id = ? AND orphaned_at IS NULL", deviceID).
		Update("orphaned_at", now).Error
}

// WithContext is a thin shim for callers that want to thread
// a context. The repository does not currently use it, but
// exposing it keeps the API future-proof for the audit
// module.
func (r *Repository) WithContext(_ context.Context) *Repository { return r }

// notFound is a tiny constructor for a 404 APIError.
// Centralised so the message format is consistent across
// the package.
func notFound(kind, id string) error {
	return &contracts.APIError{
		Code:    contracts.CodeNotFound,
		Message: kind + " " + id + " not found",
	}
}

// IsNotFound reports whether err is a contracts.APIError
// carrying the NotFound code. Used by tests and the service
// layer to branch on missing rows.
func IsNotFound(err error) bool {
	var ae *contracts.APIError
	if errors.As(err, &ae) {
		return ae.Code == contracts.CodeNotFound
	}
	return false
}

// IsConflict mirrors IsNotFound for 409 envelopes.
func IsConflict(err error) bool {
	var ae *contracts.APIError
	if errors.As(err, &ae) {
		return ae.Code == contracts.CodeConflict
	}
	return false
}

// isUniqueViolation is a best-effort detector for SQLite
// unique index violations. The error message contains the
// index name; we substring match rather than importing the
// driver's typed errors to keep the package driver-agnostic.
func isUniqueViolation(err error, indexSubstring string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "UNIQUE constraint failed") &&
		!strings.Contains(msg, "duplicate key") {
		return false
	}
	if indexSubstring == "" {
		return true
	}
	return strings.Contains(msg, indexSubstring)
}
