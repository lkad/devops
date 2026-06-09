package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// AuthMiddlewareConfig bundles the dependencies of NewAuthMiddleware.
// Signer is the production JWT signer; AllowDevBypass, when true,
// accepts "X-User: <id>" without a JWT — kept for the dev
// environment so unit tests and the dev-tier fixture can drive
// the API without standing up LDAP. The bypass MUST be false in
// any non-dev config (config.App.DevBypass is the single switch).
type AuthMiddlewareConfig struct {
	Signer         *Signer
	DevBypass      bool
	RequiredPerm   rbac.Permission
	PermissionSvc  *rbac.Service
}

// NewAuthMiddleware returns a Gin middleware that verifies the
// JWT (or accepts the dev bypass), then enforces the required
// permission. A nil Signer with DevBypass true is the only
// supported dev wiring; production calls panic on the
// combination because it would be a security hole.
func NewAuthMiddleware(cfg AuthMiddlewareConfig) gin.HandlerFunc {
	if cfg.Signer == nil && !cfg.DevBypass {
		panic("auth: Signer is nil but DevBypass is false — would disable auth in production")
	}
	return func(c *gin.Context) {
		user, ok := authenticate(c, cfg)
		if !ok {
			return // authenticate already wrote the response
		}
		c.Set(rbac.AuthUserKey, user)
		if cfg.RequiredPerm != "" && cfg.PermissionSvc != nil {
			if !cfg.PermissionSvc.HasPermission(user, cfg.RequiredPerm) {
				abortForbidden(c, "insufficient permission: "+string(cfg.RequiredPerm))
				return
			}
		}
		c.Next()
	}
}

// authenticate pulls a JWT from the Authorization header (or the
// dev-bypass X-User header), verifies it, and returns the
// *contracts.User. On any failure it writes a 401 and returns
// ok=false so the handler short-circuits.
func authenticate(c *gin.Context, cfg AuthMiddlewareConfig) (*contracts.User, bool) {
	// Dev bypass: X-User header carries the user ID. Only active
	// when the config opts in. Production never sets DevBypass
	// (the config loader rejects it on startup).
	if cfg.DevBypass {
		if uid := c.GetHeader("X-User"); uid != "" {
			return &contracts.User{ID: uid, Username: uid, Role: contracts.RoleOperator}, true
		}
	}
	raw := extractBearer(c.GetHeader("Authorization"))
	if raw == "" {
		abortUnauth(c, "missing bearer token")
		return nil, false
	}
	claims, err := cfg.Signer.Verify(raw)
	if err != nil {
		abortUnauth(c, "invalid token: "+err.Error())
		return nil, false
	}
	if claims == nil || claims.UserID == "" {
		abortUnauth(c, "token has no subject")
		return nil, false
	}
	return &contracts.User{ID: claims.UserID, Username: claims.Username, Role: claims.Role}, true
}

// extractBearer pulls the token from an Authorization header.
// Empty string when the header is missing or the scheme is not
// Bearer. Case-insensitive on the scheme.
func extractBearer(h string) string {
	if h == "" {
		return ""
	}
	idx := strings.Index(h, " ")
	if idx <= 0 {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(h[:idx]))
	if scheme != "bearer" {
		return ""
	}
	return strings.TrimSpace(h[idx+1:])
}

// abortUnauth / abortForbidden keep the response envelope stable.
func abortUnauth(c *gin.Context, msg string) {
	env := contracts.ErrorResponse{Error: contracts.ErrorBody{
		Code:    contracts.CodeUnauthorized,
		Message: msg,
	}}
	c.AbortWithStatusJSON(http.StatusUnauthorized, env)
}

func abortForbidden(c *gin.Context, msg string) {
	env := contracts.ErrorResponse{Error: contracts.ErrorBody{
		Code:    contracts.CodeForbidden,
		Message: msg,
	}}
	c.AbortWithStatusJSON(http.StatusForbidden, env)
}

// Background-friendly context: the *gin.Context has its own
// context, but a few helper callers want a plain context. Exported
// so test helpers can use the same extraction logic.
func ContextFromGin(c *gin.Context) context.Context { return c.Request.Context() }

// (jwt import kept here so future code can use a different
// signing algorithm without re-adding the import.)
var _ = jwt.New
