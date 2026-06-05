package rbac

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// AuthUserKey is the Gin context key the auth middleware uses to
// stash the authenticated *contracts.User. Exported so the auth
// package can write to the same key without a shared constant.
const AuthUserKey = "auth.user"

// RequirePermission returns a Gin middleware that 403s any request
// whose authenticated user does not hold the required global
// permission. The check is the matrix lookup in Service.HasPermission
// — per-project scoping is the service layer's responsibility, not
// the middleware's, because the project context is not always
// derivable from the URL path alone.
//
// The middleware always returns the standard contracts.ErrorResponse
// envelope; the HTTP status is taken from the ErrorCode so future
// permission categories (e.g. rate-limited reads) can be added
// without touching this code.
func RequirePermission(svc *Service, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(AuthUserKey)
		if !ok {
			abortForbidden(c, "authentication required")
			return
		}
		user, ok := v.(*contracts.User)
		if !ok || user == nil {
			abortForbidden(c, "authentication required")
			return
		}
		if !svc.HasPermission(user, perm) {
			abortForbidden(c, "insufficient permission")
			return
		}
		c.Next()
	}
}

// abortForbidden writes a 403 ErrorResponse with the FORBIDDEN
// code and aborts the request. Centralized so the message format
// stays consistent across the package.
func abortForbidden(c *gin.Context, msg string) {
	env := contracts.ErrorResponse{
		Error: contracts.ErrorBody{
			Code:    contracts.CodeForbidden,
			Message: msg,
		},
	}
	c.AbortWithStatusJSON(http.StatusForbidden, env)
}
