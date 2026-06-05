package middleware

import (
	"time"

	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Logger returns a Gin middleware that emits a single structured log
// line per request via the project's pkg/logger (log/slog under the
// hood). The fields it writes match the spec: method, path, status,
// and duration. client_ip and request_id are added when available so
// log lines are correlatable with the rest of the system.
//
// A nil log is tolerated: the middleware becomes a no-op so tests and
// callers that don't care about output can pass nil.
func Logger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// c.Next() runs the rest of the chain (and the handler) before
		// returning, so by the time we log, the status code is set.
		c.Next()

		if log == nil {
			return
		}

		dur := time.Since(start)

		// Attach the request id (set by RequestID middleware) when
		// present so the line is correlatable with the rest of the
		// request lifecycle.
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", dur.Milliseconds(),
			"client_ip", c.ClientIP(),
			"bytes", c.Writer.Size(),
		}
		if v, ok := c.Get(RequestIDKey); ok {
			if s, ok := v.(string); ok && s != "" {
				attrs = append(attrs, "request_id", s)
			}
		}

		// 5xx is an error from the client's point of view; log at
		// Error. Everything else is Info.
		status := c.Writer.Status()
		if status >= 500 {
			log.Error("http request", attrs...)
		} else {
			log.Info("http request", attrs...)
		}
	}
}
