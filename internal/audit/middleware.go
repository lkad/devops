package audit

import (
	"context"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// Header / context-key constants for the audit-context
// middleware. The header names are the canonical names the
// frontend (and the test suite) send. Renaming any of them is a
// breaking change for downstream modules.
const (
	// HeaderUserID identifies the actor. Empty string means
	// "anonymous / system"; the audit row will have a blank
	// actor_id rather than reject the request.
	HeaderUserID = "X-User-Id"
	// HeaderUserName is the human-readable display name. Kept
	// separate from HeaderUserID so the audit row stays
	// readable even if the user id is a UUID.
	HeaderUserName = "X-User-Name"
	// HeaderForwardedFor is the standard proxy chain. The
	// middleware picks the first hop (the original client).
	HeaderForwardedFor = "X-Forwarded-For"
	// HeaderUserAgent is the standard user-agent string.
	HeaderUserAgent = "User-Agent"
)

// actorCtxKey is the unexported context.Context key for the
// RequestActor. Using an unexported empty struct prevents
// accidental key collisions with other packages.
type actorCtxKey struct{}

// actorGinKey is the gin.Context key for the RequestActor.
const actorGinKey = "audit_request_actor"

// RequestActor is the per-request actor + request-scoped data
// the audit middleware extracts from the request headers. It is
// the cross-module DTO; service layers in other modules can
// call AuditFields to get the four strings the audit service
// expects on RecordActionInput.
type RequestActor struct {
	UserID    string
	Username  string
	IPAddress string
	UserAgent string
}

// AuditFields returns the four strings the audit service
// expects. Spread-friendly: the caller can do
// id, name, ip, ua := a.AuditFields() then assign individually,
// or pass them as positional args.
func (a RequestActor) AuditFields() (userID, username, ip, ua string) {
	return a.UserID, a.Username, a.IPAddress, a.UserAgent
}

// Middleware returns the Gin middleware that extracts the
// actor + request-scoped fields from the headers and stashes
// them on the gin.Context. It is intentionally a *helper* for
// other modules: it does NOT itself emit any audit event (that
// would be too noisy — every GET would log a row).
//
// Modules that want to record an audit event call
// service.RecordAction(ctx, RecordActionInput{...}) and read
// the RequestActor via RequestActorFromContext. See
// service.go for the RecordAction signature.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := RequestActor{
			UserID:    c.GetHeader(HeaderUserID),
			Username:  c.GetHeader(HeaderUserName),
			IPAddress: clientIP(c),
			UserAgent: c.GetHeader(HeaderUserAgent),
		}
		c.Set(actorGinKey, actor)
		// Also push to the request's context.Context so
		// downstream code that already holds a context (e.g.
		// repositories called with c.Request.Context()) can
		// pull the actor without going through gin.
		ctx := WithRequestActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequestActorFromContext returns the RequestActor stored on
// the gin.Context by Middleware. A zero value is returned when
// the middleware has not run, so callers can use it
// unconditionally.
func RequestActorFromContext(c *gin.Context) RequestActor {
	if c == nil {
		return RequestActor{}
	}
	if v, ok := c.Get(actorGinKey); ok {
		if a, ok := v.(RequestActor); ok {
			return a
		}
	}
	// Fall back to the request context so the helper works for
	// non-Gin callers too.
	return actorFromContext(c.Request.Context())
}

// WithRequestActor attaches the actor to the supplied
// context. Use it from background jobs or tests that synthesise
// a request without going through Gin.
func WithRequestActor(ctx context.Context, a RequestActor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorCtxKey{}, a)
}

// actorFromContext is the inverse of WithRequestActor. Returns
// the zero value when the actor is not set.
func actorFromContext(ctx context.Context) RequestActor {
	if ctx == nil {
		return RequestActor{}
	}
	if v, ok := ctx.Value(actorCtxKey{}).(RequestActor); ok {
		return v
	}
	return RequestActor{}
}

// clientIP picks the original client IP. We prefer the first
// hop of X-Forwarded-For (the original client) and fall back
// to req.RemoteAddr (host:port). The port is stripped.
//
// Rationale: a production deployment sits behind a proxy that
// appends the client IP to X-Forwarded-For; the first entry is
// always the original client. We do not trust X-Real-IP (it's
// nginx-specific) or RemoteAddr in the proxy case.
func clientIP(c *gin.Context) string {
	if v := c.GetHeader(HeaderForwardedFor); v != "" {
		// X-Forwarded-For: client, proxy1, proxy2
		if i := strings.Index(v, ","); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return c.Request.RemoteAddr
}
