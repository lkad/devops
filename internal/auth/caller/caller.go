// Package caller carries the authenticated principal across
// the service / repository boundary. The auth middleware
// stamps the principal on every request's *gin.Context (and
// the underlying context.Context); every service-layer
// method that needs the actor (audit emissions, project-
// membership checks) reads it from here.
//
// The type wraps contracts.User and adds the project-
// membership cache that supports IsMemberOf without a
// per-call DB round trip. The cache is built lazily on
// the first membership check and re-used for the lifetime
// of the request.
package caller

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// PermissionChecker is the per-project permission decision
// function the caller package calls when RequireProjectAccess
// runs. It receives the caller's *contracts.User and the
// projectID extracted from the request and returns true when
// the caller is allowed to perform the action in that project.
//
// Production wires this to rbac.HasPermissionInProject;
// tests can supply a stub. The indirection keeps the caller
// package free of any rbac import so the two can be evolved
// independently.
type PermissionChecker func(user *contracts.User, projectID string) bool

// MembershipChecker returns the set of project IDs the
// caller is a member of. Production wires this to the
// project repository; tests can supply a fake.
//
// An empty / nil result means "no memberships"; the
// helper treats that as "no access" so an unattached
// caller cannot read or write any project's state.
type MembershipChecker func(ctx context.Context, userID string) (map[string]struct{}, error)

// Caller is the request-scoped principal. The struct lives
// in this package (not contracts) so the audit / rbac
// helpers can extend it without a package dependency on
// every caller.
type Caller struct {
	User *contracts.User
	// projectIDs is the cached membership set built by
	// EnsureProjectMembership. nil means "not loaded yet".
	projectIDs map[string]struct{}
	// projectIDsErr records the most recent membership-
	// load error so subsequent calls can short-circuit
	// (the check is fail-closed: an error denies access).
	projectIDsErr error
}

// New builds a Caller from a contracts.User. The
// membership cache starts unloaded; the first IsMemberOf
// call populates it via the supplied MembershipChecker.
func New(u *contracts.User) *Caller {
	return &Caller{User: u}
}

// FromGin reads the Caller stashed on the gin.Context by
// the auth middleware. Returns (nil, false) when the
// middleware has not run; callers MUST treat that as an
// unauthenticated request and refuse to act.
//
// The second return value mirrors the gin idiom so the
// caller does not have to nil-check the user pointer.
func FromGin(c *gin.Context) (*Caller, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(ginKey)
	if !ok {
		return nil, false
	}
	cl, ok := v.(*Caller)
	return cl, ok
}

// FromContext reads the Caller off a plain context.Context.
// Background workers and tests that synthesise a context
// (without going through Gin) use this to thread the
// principal through.
func FromContext(ctx context.Context) (*Caller, bool) {
	if ctx == nil {
		return nil, false
	}
	cl, ok := ctx.Value(ctxKey{}).(*Caller)
	return cl, ok
}

// WithGin attaches the Caller to the supplied gin.Context
// under a package-private key. Called by the auth
// middleware on every authenticated request.
func WithGin(c *gin.Context, cl *Caller) {
	if c == nil {
		return
	}
	c.Set(ginKey, cl)
	// Mirror onto the request's context.Context so
	// service / repository code that already holds a
	// context (the GORM calls do) can read the caller
	// without going through gin.
	if cl != nil {
		c.Request = c.Request.WithContext(WithContext(c.Request.Context(), cl))
	}
}

// WithContext attaches the Caller to a plain
// context.Context. Use it from background jobs or tests
// that synthesise a request without going through Gin.
func WithContext(ctx context.Context, cl *Caller) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, cl)
}

// UserID is a convenience that returns the caller's user
// ID (or empty when nil). Several audit / RBAC helpers
// call this frequently; the wrapper keeps the call sites
// short.
func (c *Caller) UserID() string {
	if c == nil || c.User == nil {
		return ""
	}
	return c.User.ID
}

// IsMemberOf reports whether the caller is a member of
// projectID. The check honours three rules:
//
//  1. SuperAdmin is a member of every project (the spec's
//     "SuperAdmin is implicitly a member of every project"
//     rule — it would be a footgun to make the audit
//     admin manually attach SuperAdmin to every row).
//  2. Operators with the global Operator role are not
//     implicit members; they must be added explicitly.
//     The spec treats Operator as a global role, not a
//     project role. (The role check is therefore the
//     "global role grant" path — see rbac.HasPermission.)
//  3. Everyone else must be an explicit member of
//     projectID; the membership set is the project_members
//     rows for the user.
//
// The membership set is built lazily on the first call
// and cached for the lifetime of the Caller. A nil
// MembershipChecker means "no memberships" — a fail-closed
// default that prevents a misconfigured service from
// accidentally granting every caller access to every
// project.
func (c *Caller) IsMemberOf(ctx context.Context, projectID string, m MembershipChecker) bool {
	if c == nil || c.User == nil {
		return false
	}
	// Rule 1: SuperAdmin is implicit.
	if c.User.Role == contracts.RoleSuperAdmin {
		return true
	}
	// Empty projectID: refuse so an uninitialised variable
	// in a service method cannot accidentally pass.
	if projectID == "" {
		return false
	}
	if m == nil {
		return false
	}
	// Lazy load: one DB round trip per request, ever.
	if c.projectIDs == nil && c.projectIDsErr == nil {
		ids, err := m(ctx, c.User.ID)
		c.projectIDs = ids
		c.projectIDsErr = err
		if c.projectIDs == nil {
			c.projectIDs = map[string]struct{}{}
		}
	}
	if c.projectIDsErr != nil {
		return false
	}
	_, ok := c.projectIDs[projectID]
	return ok
}

// IsSuperAdmin is the common-case shortcut for handlers
// that need to know "is this the platform admin". It is a
// pure function of the role — no DB lookup.
func (c *Caller) IsSuperAdmin() bool {
	return c != nil && c.User != nil && c.User.Role == contracts.RoleSuperAdmin
}

// ginKey is the gin.Context key the middleware uses to
// stash the Caller. Unexported so other packages cannot
// race with the middleware by writing to the same key.
const ginKey = "auth.caller"

// ctxKey is the context.Context key. The empty-struct
// type prevents key collisions with other packages.
type ctxKey struct{}

// CheckProjectAccess is the plain boolean flavour of the
// per-project access check. It returns true when the
// caller is allowed to perform an action that requires
// `perm` in `projectID`.
//
// The check honours three rules:
//
//  1. SuperAdmin is implicitly a member of every project
//     and is granted access without consulting the
//     permission checker.
//  2. The caller must be an explicit member of projectID
//     (per IsMemberOf) AND the permission checker must
//     return true for (user, projectID). Membership is
//     the prerequisite; permission is the cap on top.
//  3. A missing caller (no auth context) is denied. A
//     missing projectID is denied. A missing membership
//     checker is denied (fail-closed).
//
// The function is the building block for
// RequireProjectAccess (the Gin middleware) and is also
// safe to call directly from the service layer for cases
// where a single handler needs to do a one-off check.
func CheckProjectAccess(ctx context.Context, projectID string, m MembershipChecker, p PermissionChecker) bool {
	cl, ok := FromContext(ctx)
	if !ok {
		cl2, ok2 := fromContextGin(ctx)
		if !ok2 {
			return false
		}
		cl = cl2
	}
	if cl == nil {
		return false
	}
	if cl.IsSuperAdmin() {
		return true
	}
	if projectID == "" {
		return false
	}
	if !cl.IsMemberOf(ctx, projectID, m) {
		return false
	}
	if p == nil {
		return false
	}
	return p(cl.User, projectID)
}

// RequireProjectAccess returns a Gin middleware that 403s
// any request whose authenticated caller is not a member
// of the project identified by projectIDFn(c) AND does
// not hold the supplied permission in that project.
//
// projectIDFn extracts the project ID from the request —
// usually `c.Param("id")` or a closure that reads a
// body field. The function is called only when the
// caller is present (an absent caller is denied before
// the lookup).
//
// The middleware short-circuits on its own via
// c.AbortWithStatusJSON so handlers can rely on `if
// c.IsAborted() { return }` after calling it. The 403
// envelope is the standard contracts.ErrorResponse.
//
// Wiring pattern (production):
//
//	group.GET("/projects/:id",
//	    a.RequireAuth(),
//	    rbac.RequirePermission(svc, rbac.PermissionViewProjects),
//	    caller.RequireProjectAccess(svc.MembershipChecker, rbacSvc.HasPermissionInProject,
//	        func(c *gin.Context) string { return c.Param("id") }),
//	    h.Get)
//
// The per-project check runs AFTER the global RBAC
// middleware so a caller without the right global
// permission is rejected with the standard "insufficient
// permission" message, not the more specific "not a
// member of project X" message.
func RequireProjectAccess(m MembershipChecker, p PermissionChecker, projectIDFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl, ok := FromGin(c)
		if !ok || cl == nil {
			abortForbidden(c, "authentication required")
			return
		}
		if cl.IsSuperAdmin() {
			c.Next()
			return
		}
		projectID := ""
		if projectIDFn != nil {
			projectID = projectIDFn(c)
		}
		if projectID == "" {
			abortForbidden(c, "project id is required")
			return
		}
		if !cl.IsMemberOf(c.Request.Context(), projectID, m) {
			abortForbidden(c, "not a member of project "+projectID)
			return
		}
		if p == nil || !p(cl.User, projectID) {
			abortForbidden(c, "insufficient permission in project "+projectID)
			return
		}
		c.Next()
	}
}

// fromContextGin is a small adapter so CheckProjectAccess
// can be called with a *gin.Context's underlying
// context.Context (which the middleware does NOT expose
// directly) and still get the right answer when the
// Caller was attached via WithGin. We mirror the
// gin.Context lookup here; the public FromContext path
// remains the context-typed entry point.
func fromContextGin(ctx context.Context) (*Caller, bool) {
	if ctx == nil {
		return nil, false
	}
	cl, ok := ctx.Value(ctxKey{}).(*Caller)
	return cl, ok
}

// abortForbidden writes a 403 ErrorResponse and aborts
// the request. Mirrors the helper in rbac/middleware.go
// so the envelope stays consistent across packages; the
// caller package's import graph is small enough that
// lifting the helper up to a shared package is not
// worth the churn today.
func abortForbidden(c *gin.Context, msg string) {
	env := contracts.ErrorResponse{Error: contracts.ErrorBody{
		Code:    contracts.CodeForbidden,
		Message: msg,
	}}
	c.AbortWithStatusJSON(http.StatusForbidden, env)
}
