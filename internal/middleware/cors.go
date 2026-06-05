package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that sets the standard CORS response
// headers and short-circuits OPTIONS preflight requests with 204.
//
// allowedOrigins is the configured allowlist. The special value "*"
// disables allowlist checking and echoes the request Origin back, which
// is the correct behaviour for fully-public APIs. For any other value,
// the request Origin must match one of the entries exactly or the
// allow header is omitted (the browser will then block the response).
//
// An empty / nil list behaves the same as "*" — no restrictions
// configured, so we still set the headers but echo the request Origin.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	// Build a lookup set for O(1) matching. The special token "*"
	// (either passed alone or in a list) means "any origin is fine" —
	// we echo the request Origin back so credentials still work.
	// nil/empty list is treated the same as ["*"] for backwards
	// compatibility with callers that haven't configured origins yet.
	allowAll := len(allowedOrigins) == 0
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
			continue
		}
		allowed[o] = struct{}{}
	}

	methods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	headers := "Content-Type, Authorization, X-Request-ID"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Decide what to put in Access-Control-Allow-Origin. If the
		// server is fully open (no allowlist or "*" passed) we echo
		// the request's Origin back. If a specific allowlist is set,
		// only echo it when the Origin matches.
		switch {
		case allowAll:
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		default:
			if _, ok := allowed[origin]; ok && origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
		}

		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		c.Header("Access-Control-Max-Age", "600")

		// Preflight: respond 204 and stop the chain so no handler runs.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
