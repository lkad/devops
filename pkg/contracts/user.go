package contracts

// Role is a global RBAC role. There are five, per the auth/rbac spec.
// Roles are ordered by privilege; a higher role implicitly has all
// permissions of a lower role.
type Role string

const (
	RoleSuperAdmin   Role = "SuperAdmin"
	RoleOperator     Role = "Operator"
	RoleDeveloper    Role = "Developer"
	RoleAuditor      Role = "Auditor"
	RoleProjectAdmin Role = "ProjectAdmin"
)

// roleRank assigns each role a privilege rank. Higher number = more privilege.
// Unknown roles get rank -1 so they fail every HasPermission check.
func (r Role) roleRank() int {
	switch r {
	case RoleSuperAdmin:
		return 4
	case RoleOperator:
		return 3
	case RoleDeveloper:
		return 2
	case RoleAuditor:
		return 1
	case RoleProjectAdmin:
		return 0
	default:
		return -1
	}
}

// HasPermission reports whether role r satisfies the required role's
// privilege level. A role always has permission for itself; only
// SuperAdmin satisfies any global role check.
func (r Role) HasPermission(required Role) bool {
	return r.roleRank() >= required.roleRank()
}

// ProjectRole is a per-project RBAC role. Three levels per the
// project-hierarchy spec: viewer < editor < admin.
type ProjectRole string

const (
	ProjectRoleViewer ProjectRole = "viewer"
	ProjectRoleEditor ProjectRole = "editor"
	ProjectRoleAdmin  ProjectRole = "admin"
)

func (r ProjectRole) projectRoleRank() int {
	switch r {
	case ProjectRoleAdmin:
		return 2
	case ProjectRoleEditor:
		return 1
	case ProjectRoleViewer:
		return 0
	default:
		return -1
	}
}

// HasPermission mirrors Role.HasPermission for project roles.
func (r ProjectRole) HasPermission(required ProjectRole) bool {
	return r.projectRoleRank() >= required.projectRoleRank()
}

// User is the in-process representation of an authenticated principal.
// The repository layer maps the row into this struct; no DB tags here
// per the architecture-foundation spec (models.go owns persistence).
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     Role   `json:"role"`
}

// JWTClaims is the decoded payload of a session JWT. Kept in contracts
// so handlers and middleware can both reference it without an import
// cycle through the auth package.
type JWTClaims struct {
	UserID    string `json:"uid"`
	Username  string `json:"usr"`
	Role      Role   `json:"rol"`
	ExpiresAt int64  `json:"exp"`
}

// IsExpired reports whether the token's exp claim is in the past.
// Caller is responsible for providing a clock; this stays a pure
// function so it is trivial to test.
func (c *JWTClaims) IsExpired() bool {
	return c.ExpiresAt <= nowFn()
}
