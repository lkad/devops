// Package middleware contains the HTTP middleware chain for the devops
// toolkit backend. Each middleware in this package has a single
// responsibility and no business logic; the chain in chain.go is the
// only sanctioned composition point.
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/devops-toolkit/backend/pkg/contracts"
	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Recovery returns a Gin middleware that catches panics in downstream
// handlers, logs the panic value and stack trace, and renders a 500
// Internal Server Error envelope so the client never sees a half-written
// response or a connection drop.
//
// Per the middleware-stack spec, recovery is the outermost middleware:
// it MUST run before request_id, CORS, logger, auth, and rbac so a panic
// inside any of them still produces a sane response.
//
// The log is a no-op if log is nil, so tests and callers can pass nil
// when they don't care about the log output.
func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if log != nil {
					// Include the stack trace as a field so the log
					// consumer can render it however it prefers.
					log.Error("panic recovered",
						"panic", fmt.Sprintf("%v", rec),
						"path", c.Request.URL.Path,
						"method", c.Request.Method,
						"stack", string(debug.Stack()),
					)
				}
				// Abort prevents any later middleware from also writing
				// to the response. We render the standard error envelope
				// via internal/handler.WriteError so the wire format
				// matches the rest of the API.
				c.AbortWithStatusJSON(http.StatusInternalServerError, contracts.ErrorResponse{
					Error: contracts.ErrorBody{
						Code:    contracts.CodeInternal,
						Message: "internal server error",
					},
				})
			}
		}()
		c.Next()
	}
}
