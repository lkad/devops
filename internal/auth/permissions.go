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
