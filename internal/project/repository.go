package project

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Repository is the GORM-only data access layer for the project
// hierarchy. It knows about rows and indexes but nothing about
// business rules — those live in the service layer. The struct
// holds a *gorm.DB for testability: a fresh in-memory SQLite is
// enough to exercise every code path.
type Repository struct {
	db *gorm.DB
}

// NewRepository returns a Repository backed by the supplied db. The
// caller owns the db lifecycle; the repository is stateless.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Filter is the query-side DTO for List. Pointer fields let the
// handler distinguish "not set" from "zero value" when building
// the SQL WHERE clause.
type Filter struct {
	ParentID *string
	TypeID   string
	OwnerID  *string
	Search   string
	Depth    int
	// Pagination is optional; when zero, the repo returns the full
	// result set. The handler is responsible for clamping values.
	Limit  int
	Offset int
}

// CreateType persists a new project type and returns the row with
// the assigned ID. Duplicate name violations surface as a Conflict
// APIError so the service layer can render a 409.
func (r *Repository) CreateType(t ProjectType) (ProjectType, error) {
	if err := r.db.Create(&t).Error; err != nil {
		if isUniqueViolation(err, "project_types.name") {
			return ProjectType{}, &contracts.APIError{
				Code:    contracts.CodeConflict,
				Message: "project type name already exists",
			}
		}
		return ProjectType{}, err
	}
	return t, nil
}

// ListTypes returns every project type, sorted by name so the UI
// dropdown has a stable order across requests.
func (r *Repository) ListTypes() ([]ProjectType, error) {
	var out []ProjectType
	if err := r.db.Order("name ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Create persists a new project row. Unique-index violations on
// code are mapped to a Conflict error for the same reason as
// CreateType.
func (r *Repository) Create(p Project) (Project, error) {
	if err := r.db.Create(&p).Error; err != nil {
		if isUniqueViolation(err, "projects.code") {
			return Project{}, &contracts.APIError{
				Code:    contracts.CodeConflict,
				Message: "project code already exists",
			}
		}
		return Project{}, err
	}
	return p, nil
}

// Get returns the project by ID. Missing rows surface as a
// NotFound APIError so handlers do not have to translate gorm
// errors themselves.
func (r *Repository) Get(id string) (Project, error) {
	var p Project
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Project{}, notFound("project", id)
		}
		return Project{}, err
	}
	return p, nil
}

// GetWithRelations fetches a project and eagerly loads its parent,
// direct children, and members. Used by the GET /projects/:id
// handler to return a single rich payload.
func (r *Repository) GetWithRelations(id string) (Project, []Project, []ProjectMember, error) {
	p, err := r.Get(id)
	if err != nil {
		return Project{}, nil, nil, err
	}
	var children []Project
	if err := r.db.Where("parent_id = ?", id).Find(&children).Error; err != nil {
		return Project{}, nil, nil, err
	}
	members, err := r.ListMembers(id)
	if err != nil {
		return Project{}, nil, nil, err
	}
	return p, children, members, nil
}

// List applies the filter and returns the matching page plus the
// total count. The total is computed before pagination so the
// handler can populate contracts.Pagination.Total.
func (r *Repository) List(f Filter) ([]Project, int64, error) {
	q := r.db.Model(&Project{})
	if f.ParentID != nil {
		q = q.Where("parent_id = ?", *f.ParentID)
	}
	if f.TypeID != "" {
		q = q.Where("type_id = ?", f.TypeID)
	}
	if f.OwnerID != nil {
		q = q.Where("owner_user_id = ?", *f.OwnerID)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	if f.Depth == 1 {
		q = q.Where("parent_id IS NULL")
	} else if f.Depth == 2 {
		q = q.Where("parent_id IS NOT NULL AND parent_id IN (SELECT id FROM projects WHERE parent_id IS NULL)")
	} else if f.Depth == 3 {
		q = q.Where("parent_id IN (SELECT id FROM projects WHERE parent_id IS NOT NULL AND parent_id IN (SELECT id FROM projects WHERE parent_id IS NULL))")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if f.Limit > 0 {
		q = q.Limit(f.Limit).Offset(f.Offset)
	}
	var out []Project
	if err := q.Order("name ASC").Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Update applies a partial update. The patch is a map so callers
// can ignore zero values. Empty patches are an error so the caller
// gets feedback rather than a silent no-op.
func (r *Repository) Update(id string, patch map[string]any) (Project, error) {
	if len(patch) == 0 {
		return Project{}, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "update patch is empty",
		}
	}
	tx := r.db.Model(&Project{}).Where("id = ?", id).Updates(patch)
	if tx.Error != nil {
		if isUniqueViolation(tx.Error, "projects.code") {
			return Project{}, &contracts.APIError{
				Code:    contracts.CodeConflict,
				Message: "project code already exists",
			}
		}
		return Project{}, tx.Error
	}
	if tx.RowsAffected == 0 {
		return Project{}, notFound("project", id)
	}
	return r.Get(id)
}

// Delete soft-deletes the project. The service layer is
// responsible for the "has children" check; the repository just
// performs the delete.
func (r *Repository) Delete(id string) error {
	tx := r.db.Delete(&Project{}, "id = ?", id)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return notFound("project", id)
	}
	return nil
}

// CountChildren returns the number of direct children of a
// project. Soft-deleted children are excluded.
func (r *Repository) CountChildren(id string) (int64, error) {
	var n int64
	if err := r.db.Model(&Project{}).Where("parent_id = ?", id).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// GetTree returns every descendant of rootID, including the root
// itself. The implementation is iterative to avoid GORM preloading
// limits and to keep the call graph simple to test.
func (r *Repository) GetTree(rootID string) ([]Project, error) {
	root, err := r.Get(rootID)
	if err != nil {
		return nil, err
	}
	out := []Project{root}
	frontier := []string{rootID}
	for len(frontier) > 0 {
		var children []Project
		if err := r.db.Where("parent_id IN ?", frontier).Find(&children).Error; err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		frontier = frontier[:0]
		for _, c := range children {
			out = append(out, c)
			frontier = append(frontier, c.ID)
		}
	}
	return out, nil
}

// GetAncestors returns the parent chain in root → leaf order. The
// subject itself is excluded; if the project is a BusinessLine
// the result is empty. We collect leaf-first and reverse at the
// end to avoid a quadratic prepend.
func (r *Repository) GetAncestors(id string) ([]Project, error) {
	cur, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	var chain []Project
	for cur.ParentID != nil {
		parent, err := r.Get(*cur.ParentID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, parent)
		cur = parent
	}
	// reverse in place
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// AddMember inserts a (project, user, role) row. Duplicate pairs
// surface as a Conflict so the service layer can decide whether
// to upsert or error.
func (r *Repository) AddMember(projectID, userID, role, addedBy string) error {
	m := ProjectMember{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
		AddedBy:   addedBy,
	}
	if err := r.db.Create(&m).Error; err != nil {
		if isUniqueViolation(err, "project_members") {
			return &contracts.APIError{
				Code:    contracts.CodeConflict,
				Message: "user is already a member of this project",
			}
		}
		return err
	}
	m.AddedAt = m.CreatedAt
	return nil
}

// RemoveMember deletes the (project, user) row. A missing pair is
// a NotFound error so the handler can render a 404.
func (r *Repository) RemoveMember(projectID, userID string) error {
	tx := r.db.Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&ProjectMember{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return notFound("project member", projectID+":"+userID)
	}
	return nil
}

// ListMembers returns every member of a project, sorted by user_id
// for a stable UI ordering.
func (r *Repository) ListMembers(projectID string) ([]ProjectMember, error) {
	var out []ProjectMember
	if err := r.db.Where("project_id = ?", projectID).Order("user_id ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// WithContext is a thin shim for callers that want to thread a
// context. The repository does not currently use it, but exposing
// it keeps the API future-proof for the audit module.
func (r *Repository) WithContext(_ context.Context) *Repository { return r }

// notFound is a tiny constructor for a 404 APIError. Centralised
// so the message format is consistent across the package.
func notFound(kind, id string) error {
	return &contracts.APIError{
		Code:    contracts.CodeNotFound,
		Message: kind + " " + id + " not found",
	}
}

// IsNotFound reports whether err is a contracts.APIError carrying
// the NotFound code. Used by tests and by the service layer to
// branch on missing rows.
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

// isUniqueViolation is a best-effort detector for SQLite unique
// index violations. The error message contains the index name; we
// substring match rather than importing the driver's typed errors
// to keep the package driver-agnostic.
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
	return strings.Contains(msg, indexSubstring) || strings.Contains(msg, strings.SplitN(indexSubstring, ".", 2)[1])
}
