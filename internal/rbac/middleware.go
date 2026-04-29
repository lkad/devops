package rbac

import (
	"net/http"
	"strings"

	"github.com/devops-toolkit/internal/auth"
	"github.com/devops-toolkit/pkg/errors"
	"github.com/devops-toolkit/pkg/response"
)

type PermissionCheck struct {
	Resource string
	Action   string
}

func RequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				response.Error(w, errors.Unauthorized("authentication required"))
				return
			}

			// Super admin has all permissions
			if contains(user.Roles, string(RoleSuperAdmin)) {
				next.ServeHTTP(w, r)
				return
			}

			// Check each role the user has
			for _, roleStr := range user.Roles {
				role := Role(roleStr)
				if HasPermission(role, resource, action) {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.Error(w, errors.Forbidden("insufficient permissions"))
		})
	}
}

func RequireAnyPermission(permissions ...PermissionCheck) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				response.Error(w, errors.Unauthorized("authentication required"))
				return
			}

			// Super admin has all permissions
			if contains(user.Roles, string(RoleSuperAdmin)) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has any of the required permissions
			for _, perm := range permissions {
				for _, roleStr := range user.Roles {
					role := Role(roleStr)
					if HasPermission(role, perm.Resource, perm.Action) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			response.Error(w, errors.Forbidden("insufficient permissions"))
		})
	}
}

func RequireAllPermissions(permissions ...PermissionCheck) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				response.Error(w, errors.Unauthorized("authentication required"))
				return
			}

			// Super admin has all permissions
			if contains(user.Roles, string(RoleSuperAdmin)) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has ALL required permissions
			for _, perm := range permissions {
				hasPerm := false
				for _, roleStr := range user.Roles {
					role := Role(roleStr)
					if HasPermission(role, perm.Resource, perm.Action) {
						hasPerm = true
						break
					}
				}
				if !hasPerm {
					response.Error(w, errors.Forbidden("insufficient permissions"))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ExtractResourceFromPath(path string) (resource, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) >= 2 {
		resource = parts[1]
	}

	// Infer action from HTTP method and path
	return resource, ""
}
