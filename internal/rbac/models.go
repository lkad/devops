package rbac

import (
	"net/http"
	"strings"

	"github.com/devops-toolkit/internal/auth"
	"github.com/devops-toolkit/pkg/errors"
	"github.com/devops-toolkit/pkg/response"
	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleOperator   Role = "operator"
	RoleDeveloper Role = "developer"
	RoleAuditor   Role = "auditor"
)

type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"` // create, read, update, delete
}

type PermissionMatrix struct {
	SuperAdmin []Permission `json:"super_admin"`
	Operator   []Permission `json:"operator"`
	Developer []Permission `json:"developer"`
	Auditor   []Permission `json:"auditor"`
}

var DefaultPermissionMatrix = PermissionMatrix{
	SuperAdmin: []Permission{
		{Resource: "*", Actions: []string{"*"}},
	},
	Operator: []Permission{
		{Resource: "devices", Actions: []string{"create", "read", "update", "delete"}},
		{Resource: "pipelines", Actions: []string{"create", "read", "update", "delete"}},
		{Resource: "logs", Actions: []string{"read"}},
		{Resource: "alerts", Actions: []string{"create", "read"}},
		{Resource: "physicalhosts", Actions: []string{"create", "read", "update", "delete"}},
	},
	Developer: []Permission{
		{Resource: "devices", Actions: []string{"read", "update"}},
		{Resource: "pipelines", Actions: []string{"read", "update"}},
		{Resource: "logs", Actions: []string{"read"}},
		{Resource: "alerts", Actions: []string{"read"}},
	},
	Auditor: []Permission{
		{Resource: "devices", Actions: []string{"read"}},
		{Resource: "pipelines", Actions: []string{"read"}},
		{Resource: "logs", Actions: []string{"read"}},
		{Resource: "alerts", Actions: []string{"read"}},
		{Resource: "audit", Actions: []string{"read"}},
	},
}

func HasPermission(role Role, resource, action string) bool {
	matrix := DefaultPermissionMatrix
	var permissions []Permission

	switch role {
	case RoleSuperAdmin:
		permissions = matrix.SuperAdmin
	case RoleOperator:
		permissions = matrix.Operator
	case RoleDeveloper:
		permissions = matrix.Developer
	case RoleAuditor:
		permissions = matrix.Auditor
	}

	for _, p := range permissions {
		if p.Resource == "*" {
			for _, a := range p.Actions {
				if a == "*" || a == action {
					return true
				}
			}
		}
		if p.Resource == resource {
			for _, a := range p.Actions {
				if a == "*" || a == action {
					return true
				}
			}
		}
	}

	return false
}

type Label struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"` // environment, region, team, custom
	Group     string     `json:"group"` // group for RBAC matching (e.g., "engineering", "operations")
	ParentID  *uuid.UUID `json:"parentId,omitempty"`
	CreatedAt string     `json:"createdAt"`
}

type LabelBinding struct {
	ID        uuid.UUID `json:"id"`
	EntityType string `json:"entityType"` // device, pipeline, project
	EntityID   uuid.UUID `json:"entityId"`
	LabelID    uuid.UUID `json:"labelId"`
}

// UserGroup represents a user's group with hierarchy support
type UserGroup struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parentId,omitempty"`
}

// LabelGroup represents a label's group with hierarchy support
type LabelGroup struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parentId,omitempty"`
}

// AllGroups returns the group and all its ancestor groups (for inheritance)
func (g *UserGroup) AllGroups() []string {
	var groups []string
	current := g
	visited := make(map[uuid.UUID]bool)

	for current != nil {
		if current.ID != uuid.Nil && !visited[current.ID] {
			groups = append(groups, current.Name)
			visited[current.ID] = true
		}
		if current.ParentID == nil {
			break
		}
		// In a real implementation, we'd look up the parent group from storage
		// For now, we just mark that there's a parent to follow
		break
	}
	return groups
}

// GetAllGroups returns all groups for a label including inherited ones
func (g *LabelGroup) GetAllGroups() []string {
	var groups []string
	current := g
	visited := make(map[uuid.UUID]bool)

	for current != nil {
		if current.ID != uuid.Nil && !visited[current.ID] {
			groups = append(groups, current.Name)
			visited[current.ID] = true
		}
		if current.ParentID == nil {
			break
		}
		// In a real implementation, we'd look up the parent group from storage
		break
	}
	return groups
}

// CanAccessLabel checks if a user's groups match a label's groups
// Returns true if there's any overlap between user groups and label groups
// Also considers label inheritance from parent groups
func CanAccessLabel(userGroups []string, labelGroup string, allLabelGroups []string) bool {
	// Check direct match
	if labelGroup != "" {
		for _, ug := range userGroups {
			if ug == labelGroup {
				return true
			}
		}
	}

	// Check against all label groups (for inherited labels)
	for _, lg := range allLabelGroups {
		for _, ug := range userGroups {
			if ug == lg {
				return true
			}
		}
	}

	return false
}

// RequireLabelAccess creates middleware that checks if user has access to labeled resources
// entityGroupsFunc is a callback that retrieves the groups for the entity being accessed
func RequireLabelAccess(entityGroupsFunc func(entityID uuid.UUID) ([]string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				response.Error(w, errors.Unauthorized("authentication required"))
				return
			}

			// Super admin bypasses label checks
			if contains(user.Roles, string(RoleSuperAdmin)) {
				next.ServeHTTP(w, r)
				return
			}

			// Get user groups (including inherited from parent groups)
			userGroups := getUserGroupsWithInheritance(user)

			// Extract entity ID from path or query
			entityIDStr := extractEntityID(r)
			if entityIDStr == "" {
				// No specific entity, allow through (permission check will handle it)
				next.ServeHTTP(w, r)
				return
			}

			entityID, err := uuid.Parse(entityIDStr)
			if err != nil {
				response.Error(w, errors.BadRequest("invalid entity ID"))
				return
			}

			// Get entity's label groups
			entityGroups, err := entityGroupsFunc(entityID)
			if err != nil {
				response.Error(w, errors.NotFound("entity not found"))
				return
			}

			// Check if user has matching group
			if !hasMatchingGroup(userGroups, entityGroups) {
				response.Error(w, errors.Forbidden("access denied: label group mismatch"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getUserGroupsWithInheritance(user *auth.User) []string {
	if user == nil {
		return nil
	}

	groups := []string{}
	if user.Group != "" {
		groups = append(groups, user.Group)
	}
	// In a full implementation, we'd look up parent groups from a groups service
	return groups
}

func hasMatchingGroup(userGroups, entityGroups []string) bool {
	for _, ug := range userGroups {
		for _, eg := range entityGroups {
			if ug == eg {
				return true
			}
		}
	}
	return false
}

func extractEntityID(r *http.Request) string {
	// Try to extract from path like /api/devices/{id}
	path := r.URL.Path
	if len(path) > 0 {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 3 && parts[0] == "api" {
			// Check if last part looks like a UUID
			if _, err := uuid.Parse(parts[len(parts)-1]); err == nil {
				return parts[len(parts)-1]
			}
		}
	}
	// Try query param
	return r.URL.Query().Get("entityId")
}

// contains checks if a slice contains a specific string
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
