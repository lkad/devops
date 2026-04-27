package auth

import (
	"context"
	"net/http"

	"github.com/devops-toolkit/internal/apierror"
)

// Permission constants
const (
	PermAll          = "all"
	PermRead         = "read"
	PermDeploy       = "deploy"
	PermConfigManage = "config-manage"
	PermExecute      = "execute"
	PermTestDeploy   = "test-deploy"
	PermAuditRead    = "audit-read"
)

// Environment constants
const (
	EnvProd = "prod"
	EnvDev  = "dev"
	EnvTest = "test"
)

// RequirePermission returns middleware that checks if user has required permission
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				apierror.Unauthorized(w, "not authenticated")
				return
			}

			if !userHasPermission(user, permission) {
				apierror.Forbidden(w, "permission denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func userHasPermission(user *User, required string) bool {
	for _, perm := range user.Permissions {
		if perm == PermAll || perm == required {
			return true
		}
	}
	return false
}

// CanAccessProd checks if user has permission to access production resources
func CanAccessProd(ctx context.Context) bool {
	user := GetUserFromContext(ctx)
	if user == nil {
		return false
	}
	// Only SuperAdmin and Operator can access prod
	for _, role := range user.Roles {
		if role == "SuperAdmin" || role == "Operator" {
			return true
		}
	}
	return false
}

// CanModifyProdDevice checks if user can modify a production device
// Operators cannot modify production devices - only SuperAdmin can
func CanModifyProdDevice(ctx context.Context) bool {
	user := GetUserFromContext(ctx)
	if user == nil {
		return false
	}
	// Only SuperAdmin can modify production devices
	for _, role := range user.Roles {
		if role == "SuperAdmin" {
			return true
		}
	}
	return false
}

// CanRestartDevice checks if user can restart a device in the given environment
// Operators can only restart non-production (dev/test) devices
func CanRestartDevice(ctx context.Context, environment string) bool {
	user := GetUserFromContext(ctx)
	if user == nil {
		return false
	}

	// Production restart is only allowed for SuperAdmin
	if environment == EnvProd {
		for _, role := range user.Roles {
			if role == "SuperAdmin" {
				return true
			}
		}
		return false
	}

	// Non-production (dev/test) restart is allowed for SuperAdmin and Operator
	for _, role := range user.Roles {
		if role == "SuperAdmin" || role == "Operator" {
			return true
		}
	}
	return false
}

// RequireProdAccess returns middleware that denies access to production resources for Operators
func RequireProdAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				apierror.Unauthorized(w, "not authenticated")
				return
			}

			// Check if this is a production request based on query param or header
			env := r.URL.Query().Get("environment")
			if env == "" {
				env = r.Header.Get("X-Environment")
			}

			if env == EnvProd {
				// Only SuperAdmin can access production resources directly
				hasAccess := false
				for _, role := range user.Roles {
					if role == "SuperAdmin" {
						hasAccess = true
						break
					}
				}
				if !hasAccess {
					apierror.Forbidden(w, "production environment access denied")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
