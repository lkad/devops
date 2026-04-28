package auth

import (
	"context"
	"net/http"

	"github.com/devops-toolkit/internal/apierror"
)

// DeviceOperation represents types of device operations
type DeviceOperation string

const (
	OpDeviceRead    DeviceOperation = "read"
	OpDeviceModify  DeviceOperation = "modify"
	OpDeviceRestart DeviceOperation = "restart"
	OpDeviceDelete  DeviceOperation = "delete"
)

// CheckDevicePermission checks if user can perform operation on device in given environment
// Returns true if allowed, false otherwise
func CheckDevicePermission(ctx context.Context, op DeviceOperation, environment string) bool {
	user := GetUserFromContext(ctx)
	if user == nil {
		return false
	}

	// SuperAdmin can do everything
	for _, role := range user.Roles {
		if role == string(RoleSuperAdmin) {
			return true
		}
	}

	// Operator restrictions
	for _, role := range user.Roles {
		if role == string(RoleOperator) {
			switch op {
			case OpDeviceRead:
				return true
			case OpDeviceModify, OpDeviceDelete:
				// Operator cannot modify or delete production devices
				if environment == EnvProd {
					return false
				}
				return true
			case OpDeviceRestart:
				// Operator can only restart non-production devices
				if environment == EnvProd {
					return false
				}
				return true
			}
		}
	}

	// Developer can read and test-deploy only
	for _, role := range user.Roles {
		if role == string(RoleDeveloper) {
			switch op {
			case OpDeviceRead:
				return true
			default:
				return false
			}
		}
	}

	// Auditor can read only
	for _, role := range user.Roles {
		if role == string(RoleAuditor) {
			if op == OpDeviceRead {
				return true
			}
			return false
		}
	}

	return false
}

// RequireDeviceOperation returns middleware that checks device operation permissions
// This middleware expects the device ID in the URL path and fetches the device to check environment
func RequireDeviceOperation(op DeviceOperation, getDeviceFunc func(r *http.Request) (environment string, ok bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				apierror.Unauthorized(w, "not authenticated")
				return
			}

			// Get device environment using the provided function
			env, ok := getDeviceFunc(r)
			if !ok {
				// If we can't get the device, let the handler deal with it
				next.ServeHTTP(w, r)
				return
			}

			// Check permission based on environment
			if !CheckDevicePermission(r.Context(), op, env) {
				apierror.Forbidden(w, "operation denied for environment: "+env)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns middleware that checks if user has one of the required roles
// SuperAdmin passes any role check
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				apierror.Unauthorized(w, "not authenticated")
				return
			}

			// SuperAdmin passes any role check
			for _, userRole := range user.Roles {
				if userRole == string(RoleSuperAdmin) {
					next.ServeHTTP(w, r)
					return
				}
			}

			for _, userRole := range user.Roles {
				for _, required := range roles {
					if userRole == required {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			apierror.Forbidden(w, "insufficient role")
			return
		})
	}
}