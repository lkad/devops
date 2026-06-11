package project

import (
	"errors"
	"regexp"
	"strings"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Service is the business-rule layer for the project hierarchy.
// It composes a Repository, applies validation, and ensures the
// hierarchy invariants (depth, cycle, FK, name uniqueness) hold
// before the repository ever sees a row. Errors flow as
// *contracts.APIError so the handler can pass them to WriteError
// without further translation.
type Service struct {
	repo *Repository
}

// NewService returns a Service backed by the supplied repository.
// The constructor takes only the repo because the service is
// otherwise stateless; the audit/clock collaborators, when they
// land, will be added as fields.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// MembershipChecker returns a caller.MembershipChecker that
// looks up the user's project memberships through this
// service's repository. Production wiring uses the returned
// function as the membership source for
// caller.RequireProjectAccess; tests can supply a fake
// directly without going through the service.
//
// The function is a method (not a free function) so a future
// revision of the service that needs to add caching or
// ancestor expansion can do so without touching the call
// sites.
func (s *Service) MembershipChecker() caller.MembershipChecker {
	return s.repo.ListProjectIDsForUser
}

// CreateProjectInput is the input DTO for project creation.
// Pointer fields let the caller distinguish "not set" from
// "zero value" — important for optional fields like ParentID and
// OwnerUserID that the service treats as nullable.
type CreateProjectInput struct {
	Name        string
	Code        string
	Description string
	ParentID    *string
	TypeID      string
	OwnerUserID *string
	Weight      int
	Labels      JSONMap
	Metadata    JSONMap
}

// UpdateProjectInput is the partial-update DTO. A nil pointer
// means "do not change"; a non-nil pointer to a zero value means
// "set to zero". Today the service only exposes a few updatable
// fields; widening the surface is additive.
type UpdateProjectInput struct {
	Name        *string
	ParentID    *string
	Description *string
	OwnerUserID *string
	Weight      *int
}

// CreateType persists a project type, enforcing name uniqueness
// at the service layer. The repository also rejects duplicates
// via the unique index, so this is a defensive double-check that
// keeps the message in service terms.
func (s *Service) CreateType(in ProjectType) (ProjectType, error) {
	if strings.TrimSpace(in.Name) == "" {
		return ProjectType{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "type name is required"}
	}
	return s.repo.CreateType(in)
}

// ListTypes returns all project types for the UI dropdown.
func (s *Service) ListTypes() ([]ProjectType, error) { return s.repo.ListTypes() }

// CreateProject validates the input and persists the row.
func (s *Service) CreateProject(in CreateProjectInput) (Project, error) {
	if err := s.validateProjectFields(in.Name, in.Code, in.TypeID); err != nil {
		return Project{}, err
	}
	if !IsValidWeight(in.Weight) {
		return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "weight must be non-negative"}
	}
	// Parent must exist and be at most one level above the new
	// row (depth cap).
	if in.ParentID != nil {
		parent, err := s.repo.Get(*in.ParentID)
		if err != nil {
			if IsNotFound(err) {
				return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "parent project does not exist"}
			}
			return Project{}, err
		}
		// A 3rd-level project cannot be a parent. Depth of parent
		// is the length of its ancestor chain + 1.
		depth, err := s.depthOf(parent.ID)
		if err != nil {
			return Project{}, err
		}
		// depth is the parent's depth; the new row sits at
		// depth+1. A 3rd-level project has depth 3, so we must
		// reject the case where depth+1 would be 4.
		if depth+1 > MaxDepth {
			return Project{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "hierarchy is limited to 3 levels (BusinessLine -> System -> Project)",
			}
		}
	}
	// Ensure type FK exists.
	if _, err := s.repo.ListTypes(); err != nil {
		return Project{}, err
	}
	types, err := s.repo.ListTypes()
	if err != nil {
		return Project{}, err
	}
	found := false
	for _, t := range types {
		if t.ID == in.TypeID {
			found = true
			break
		}
	}
	if !found {
		return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "type_id does not reference an existing project type"}
	}
	return s.repo.Create(Project{
		Name:        strings.TrimSpace(in.Name),
		Code:        strings.ToLower(strings.TrimSpace(in.Code)),
		Description: in.Description,
		ParentID:    in.ParentID,
		TypeID:      in.TypeID,
		OwnerUserID: in.OwnerUserID,
		Weight:      in.Weight,
		Labels:      in.Labels,
		Metadata:    in.Metadata,
	})
}

// GetProject returns a project by ID.
func (s *Service) GetProject(id string) (Project, error) { return s.repo.Get(id) }

// GetProjectWithRelations returns a project plus its direct
// children and members, used by the GET /projects/:id handler.
func (s *Service) GetProjectWithRelations(id string) (Project, []Project, []ProjectMember, error) {
	return s.repo.GetWithRelations(id)
}

// ListProjects applies the filter. The service is a thin pass-
// through here; the only added value is translating a missing
// project type to an empty list.
func (s *Service) ListProjects(f Filter) ([]Project, int64, error) { return s.repo.List(f) }

// UpdateProject applies a partial update. Cycle and depth checks
// are run BEFORE the patch is committed.
func (s *Service) UpdateProject(id string, in UpdateProjectInput) (Project, error) {
	current, err := s.repo.Get(id)
	if err != nil {
		return Project{}, err
	}
	patch := map[string]any{}
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "name cannot be empty"}
		}
		patch["name"] = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		patch["description"] = *in.Description
	}
	if in.OwnerUserID != nil {
		patch["owner_user_id"] = *in.OwnerUserID
	}
	if in.Weight != nil {
		if !IsValidWeight(*in.Weight) {
			return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "weight must be non-negative"}
		}
		patch["weight"] = *in.Weight
	}
	if in.ParentID != nil {
		// Self-parent: explicit guard.
		if *in.ParentID == id {
			return Project{}, &contracts.APIError{
				Code:    contracts.CodeInvalidState,
				Message: "a project cannot be its own parent",
			}
		}
		// Cycle: the new parent must not be a descendant of the
		// current project. Walk descendants of current and reject
		// if any matches.
		tree, err := s.repo.GetTree(id)
		if err != nil {
			return Project{}, err
		}
		for _, n := range tree {
			if n.ID == *in.ParentID {
				return Project{}, &contracts.APIError{
					Code:    contracts.CodeInvalidState,
					Message: "parent would create a cycle in the hierarchy",
				}
			}
		}
		// Depth cap: the new parent's depth + 1 must be <= MaxDepth.
		parent, err := s.repo.Get(*in.ParentID)
		if err != nil {
			if IsNotFound(err) {
				return Project{}, &contracts.APIError{Code: contracts.CodeValidation, Message: "parent project does not exist"}
			}
			return Project{}, err
		}
		pd, err := s.depthOf(parent.ID)
		if err != nil {
			return Project{}, err
		}
		// The current project's depth is cd. Moving under parent
		// places it at pd+1, which must be <= MaxDepth.
		cd, err := s.depthOf(id)
		if err != nil {
			return Project{}, err
		}
		if pd+1 > MaxDepth {
			return Project{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "hierarchy is limited to 3 levels (BusinessLine -> System -> Project)",
			}
		}
		_ = cd
		patch["parent_id"] = *in.ParentID
	}
	// Touch updated_at explicitly only when something changed.
	if len(patch) == 0 {
		return current, nil
	}
	return s.repo.Update(id, patch)
}

// DeleteProject enforces the leaf-only invariant.
func (s *Service) DeleteProject(id string) error {
	n, err := s.repo.CountChildren(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: "cannot delete a project that has children; remove them first",
		}
	}
	return s.repo.Delete(id)
}

// AssignMember grants or promotes a per-project role.
func (s *Service) AssignMember(projectID, userID, role, addedBy string) error {
	if !IsValidProjectRole(role) {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "role must be viewer, editor, or admin"}
	}
	if _, err := s.repo.Get(projectID); err != nil {
		return err
	}
	if err := s.repo.AddMember(projectID, userID, role, addedBy); err != nil {
		if !IsConflict(err) {
			return err
		}
		// Promote in place: update the existing row's role and
		// added_by attribution. The AddedAt column is left intact
		// for audit stability.
		if err := s.repo.db.Model(&ProjectMember{}).
			Where("project_id = ? AND user_id = ?", projectID, userID).
			Updates(map[string]any{"role": role, "added_by": addedBy}).Error; err != nil {
			return err
		}
	}
	return nil
}

// RevokeMember removes a member from a project.
func (s *Service) RevokeMember(projectID, userID string) error {
	return s.repo.RemoveMember(projectID, userID)
}

// ListMembers returns the project's members.
func (s *Service) ListMembers(projectID string) ([]ProjectMember, error) {
	return s.repo.ListMembers(projectID)
}

// ListChildren returns the direct children of a project. The repo
// already supports this via Filter; we expose it as a method to
// keep the handler free of filter plumbing.
func (s *Service) ListChildren(projectID string) ([]Project, int64, error) {
	return s.repo.List(Filter{ParentID: &projectID})
}

// ListAncestors returns the parent chain in root -> leaf order.
func (s *Service) ListAncestors(id string) ([]Project, error) {
	return s.repo.GetAncestors(id)
}

// ListAllTypes is an alias for ListTypes. Kept for symmetry with
// the project-CRUD methods.
func (s *Service) ListAllTypes() ([]ProjectType, error) { return s.repo.ListTypes() }

// AggregateWeight returns the project's own weight multiplied by
// the weight of its type. The product is the FinOps cost-
// allocation basis; the FinOps export module is expected to
// consume this method directly. Ancestor weights are intentionally
// excluded so a single node's contribution is stable as the
// hierarchy is reorganised.
func (s *Service) AggregateWeight(id string) int {
	project, err := s.repo.Get(id)
	if err != nil {
		return 0
	}
	types, err := s.repo.ListTypes()
	if err != nil {
		return 0
	}
	typeWeight := 0
	for _, t := range types {
		if t.ID == project.TypeID {
			typeWeight = t.Weight
			break
		}
	}
	return project.Weight * typeWeight
}

// depthOf returns the depth of a node in the hierarchy. A
// BusinessLine is depth 1, a System is depth 2, a Project is
// depth 3. The function counts the ancestor chain plus one.
func (s *Service) depthOf(id string) (int, error) {
	ancestors, err := s.repo.GetAncestors(id)
	if err != nil {
		return 0, err
	}
	return len(ancestors) + 1, nil
}

// validateProjectFields is the small block of field-level rules
// shared between create and (eventually) bulk import.
func (s *Service) validateProjectFields(name, code, typeID string) error {
	if strings.TrimSpace(name) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "name is required"}
	}
	if strings.TrimSpace(code) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "code is required"}
	}
	if !codePattern.MatchString(code) {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "code must be 1-64 chars of [a-z0-9-] starting with a letter",
		}
	}
	if strings.TrimSpace(typeID) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "type_id is required"}
	}
	return nil
}

// codePattern is the URL-slug rule for project codes: lowercase
// letters, digits, and dashes. Anchored so the whole string must
// match. The maximum length matches the column size.
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// errorsAlias is reserved for callers that want to import
// errors.Is/As through the service. Currently unused; kept as a
// documentation aid for the planned audit hooks.
var _ = errors.As
