package rbac

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// AuthUserKey is the Gin context key the auth middleware uses to
// stash the authenticated *contracts.User. Exported so the auth
// package can write to the same key without a shared constant.
const AuthUserKey = "auth.user"

// ProjectAccessFactory is the per-project access middleware
// factory. It mirrors the global Permission-factory shape:
// pass the permission and a project-id extractor, receive a
// Gin middleware pre-bound to both. Handlers register it
// the same way they register the per-route permission factory:
//
//	perms(rbac.PermissionViewProjects)             // global
//	projectAccess(rbac.PermissionViewProjects,
//	    func(c *gin.Context) string { return c.Param("id") })
//
// The factory lives in the rbac package (not caller) so
// the permission argument is typed as Permission rather
// than as the untyped `any` the caller package would
// otherwise need. The actual middleware that runs is
// caller.RequireProjectAccess; the factory wires it up
// here so the modules do not have to import both packages
// and assemble the closure themselves.
type ProjectAccessFactory func(Permission, func(*gin.Context) string) gin.HandlerFunc

// NewProjectAccessFactory returns a ProjectAccessFactory
// pre-bound to a MembershipChecker and the supplied rbac
// service. Modules register the factory like so:
//
//	projectAccess := rbac.NewProjectAccessFactory(
//	    rbacSvc, projectSvc.MembershipChecker())
//	h.Register(group, perms, projectAccess)
//
// The factory is a closure that captures the checker +
// service; one factory per module is the typical shape
// because the membership checker is module-specific.
func NewProjectAccessFactory(svc *Service, m caller.MembershipChecker) ProjectAccessFactory {
	return func(perm Permission, projectIDFn func(*gin.Context) string) gin.HandlerFunc {
		// Capture perm in a closure so the
		// caller-package PermissionChecker signature
		// (user, projectID) -> bool can stay free of
		// the permission argument. The middleware
		// receives the permission once at registration
		// time and re-uses it on every request.
		checker := func(user *contracts.User, projectID string) bool {
			return svc.HasPermissionInProject(user, projectID, perm)
		}
		return caller.RequireProjectAccess(m, checker, projectIDFn)
	}
}

// NoopProjectAccessFactory is the no-op equivalent of
// NoopPermFactory for the per-project access seam. It
// returns a pass-through middleware; useful in unit tests
// that do not exercise the membership / permission check.
func NoopProjectAccessFactory() ProjectAccessFactory {
	return func(Permission, func(*gin.Context) string) gin.HandlerFunc {
		return func(c *gin.Context) { c.Next() }
	}
}

// NoopPermFactory is a no-op Permission factory for tests
// that build a *Handler.Register on a plain *gin.RouterGroup
// without exercising the auth + RBAC chain. Every returned
// middleware is a pass-through; the route handlers run
// untouched. Production wiring MUST NOT use this — pass
// rbac.RequirePermission(svc, perm) instead.
func NoopPermFactory() func(Permission) gin.HandlerFunc {
	return func(Permission) gin.HandlerFunc {
		return func(c *gin.Context) { c.Next() }
	}
}

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
