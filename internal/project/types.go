package project

import "github.com/devops-toolkit/backend/pkg/contracts"

// ProjectRole is a type alias around the contract role so service
// code can speak in domain terms without leaking the wire shape.
// We deliberately re-export rather than redeclare: any change to
// the role vocabulary belongs in pkg/contracts.
type ProjectRole = contracts.ProjectRole

// Re-exported role constants for callers that want package-local
// import paths. They must stay equal to the contract values; the
// tests in service_test.go pin the relationship.
const (
	RoleViewer = contracts.ProjectRoleViewer
	RoleEditor = contracts.ProjectRoleEditor
	RoleAdmin  = contracts.ProjectRoleAdmin
)

// MaxDepth caps the hierarchy at 3 levels: BusinessLine (1) →
// System (2) → Project (3). The constant is the count of levels,
// not the depth index. A 4th level would be a project of a
// project, which the spec forbids.
const MaxDepth = 3

// IsValidProjectRole reports whether role matches one of the three
// canonical per-project roles. Empty and unknown values return
// false; the service layer relies on this to fail closed before
// the role string is written to the database.
func IsValidProjectRole(role string) bool {
	switch contracts.ProjectRole(role) {
	case contracts.ProjectRoleViewer,
		contracts.ProjectRoleEditor,
		contracts.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

// IsValidWeight reports whether w is a meaningful weight. Negative
// values are rejected because cost allocation weights must be
// non-negative; zero is allowed (the row simply does not contribute).
func IsValidWeight(w int) bool {
	return w >= 0
}
