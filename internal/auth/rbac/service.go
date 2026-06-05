package rbac

import "github.com/devops-toolkit/backend/pkg/contracts"

// Service is the in-process RBAC decision engine. It is stateless
// today — the matrix is a package-level table — but it is exposed
// as a struct so Phase 3+ handlers and the audit-logging layer can
// swap in a persisted implementation without touching callers.
type Service struct{}

// NewService returns a ready-to-use Service. The constructor is
// kept even though Service is empty today; later phases will need
// to thread a clock or audit sink in, and changing the call site
// once is cheaper than a wide refactor.
func NewService() *Service {
	return &Service{}
}

// HasPermission reports whether user has the global permission
// perm. The check is a single matrix lookup; per-project roles
// and runtime context (production environment, device labels)
// are evaluated separately by HasPermissionInProject and by the
// service-layer handlers that wrap this function.
//
// A nil user always returns false — the middleware relies on this
// to fail closed when an upstream auth step forgets to populate
// the principal.
func (s *Service) HasPermission(user *contracts.User, perm Permission) bool {
	if user == nil {
		return false
	}
	return RoleHasPermission(user.Role, perm)
}

// HasPermissionInProject reports whether user has permission perm
// within the project identified by projectID. The evaluation walks
// the project hierarchy implied by the spec:
//
//  1. If the user has a project role on this project, that role is
//     authoritative for the project — admin grants everything,
//     editor grants read/modify/execute, viewer grants read.
//  2. If the user has no project role, the global role decides.
//     This is how Operators keep working in projects they have
//     been added to without an explicit project role.
//
// The projectID argument is reserved for label-scoped checks; it
// is accepted now so handlers do not need a second code path when
// that lands.
func (s *Service) HasPermissionInProject(user *contracts.User, projectID string, perm Permission) bool {
	if user == nil {
		return false
	}
	// 1. Project role is the per-project authority.
	switch user.ProjectRole {
	case contracts.ProjectRoleAdmin:
		return true
	case contracts.ProjectRoleEditor:
		switch perm {
		case PermissionViewDevices,
			PermissionModifyConfig,
			PermissionExecuteCommands:
			return true
		}
		return false
	case contracts.ProjectRoleViewer:
		return perm == PermissionViewDevices
	}
	// 2. No project role — fall back to the global role.
	return RoleHasPermission(user.Role, perm)
}

// CanRestartInEnvironment is the spec scenario "Operator restricted
// on production restart" expressed as a service-layer helper.
// Only SuperAdmin may restart a production device; for non-prod
// devices the matrix decides. The check is conservative: a nil
// user is always denied.
func (s *Service) CanRestartInEnvironment(user *contracts.User, isProduction bool) bool {
	if user == nil {
		return false
	}
	if isProduction {
		// Production restart is the most dangerous operation in
		// the system; restrict to global SuperAdmin only.
		return user.Role == contracts.RoleSuperAdmin
	}
	return s.HasPermission(user, PermissionRemoteRestart)
}

// CanModifyInEnvironment covers the spec scenario "Operator
// restricted on prod devices": Operators can modify non-prod
// devices (matrix grants PermissionModifyConfig) but never
// production ones. SuperAdmin is unrestricted; everyone else
// is denied on prod.
func (s *Service) CanModifyInEnvironment(user *contracts.User, isProduction bool) bool {
	if user == nil {
		return false
	}
	if isProduction {
		return user.Role == contracts.RoleSuperAdmin
	}
	return s.HasPermission(user, PermissionModifyConfig)
}

// HasAccessByLabel is the spec scenario "User accesses device with
// matching label": the user's group set and the device's label
// set must intersect. Empty inputs (no groups, no labels) are
// treated as "no access" — failing closed is the safe default for
// a security check.
func (s *Service) HasAccessByLabel(userGroups, deviceLabels []string) bool {
	if len(userGroups) == 0 || len(deviceLabels) == 0 {
		return false
	}
	devices := make(map[string]struct{}, len(deviceLabels))
	for _, l := range deviceLabels {
		devices[l] = struct{}{}
	}
	for _, g := range userGroups {
		if _, ok := devices[g]; ok {
			return true
		}
	}
	return false
}

// HasAccessByLabelForRole is the label-based access check with the
// SuperAdmin escape hatch. A SuperAdmin can always access any
// device; everyone else is gated by HasAccessByLabel.
func (s *Service) HasAccessByLabelForRole(role contracts.Role, userGroups, deviceLabels []string) bool {
	if role == contracts.RoleSuperAdmin {
		return true
	}
	return s.HasAccessByLabel(userGroups, deviceLabels)
}

// InheritedLabels walks a group's hierarchy and returns every
// label in the chain. The spec scenario "Child group inherits
// parent labels" says a device in a child group inherits all
// labels from the parent — passing the full path in is the
// simplest model and matches the LDAP-style DN hierarchy the
// platform actually uses.
//
// The input is a list of group names ordered from root-most to
// leaf-most (e.g. ["team-a", "team-a/subteam-1"]); each entry is
// added to the result. Duplicate entries are de-duplicated so a
// device belongs to a unique label set.
func InheritedLabels(groupHierarchy []string) []string {
	if len(groupHierarchy) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(groupHierarchy))
	out := make([]string, 0, len(groupHierarchy))
	for _, g := range groupHierarchy {
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	return out
}
