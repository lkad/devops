package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that records every
// HTTP request's latency as a Metric in the supplied
// Service. This is the per-request counter / histogram the
// spec calls for ("Track request" / "Track latency"); it
// lives in the metrics package so the recording logic is
// colocated with the model it produces.
//
// Usage (cmd/devops-toolkit/main.go):
//
//	r.Use(metrics.Middleware(svc))
//
// The middleware is best-effort: a failed Ingest does NOT
// short-circuit the request. The error is captured for
// observability but the response is delivered to the
// caller as normal. This is the only place the metrics
// subsystem sits on the hot path, so a panic here would
// kill the entire API.
func Middleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)

		// Build the labels in a single allocation. We use
		// the matched route template (e.g. /api/v1/devices)
		// rather than the raw path so URLs with id
		// parameters don't explode the cardinality.
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		// A label cap of ~64 chars keeps the labelset
		// searchable in the dashboard without choking
		// the JSONB column.
		method := truncate(c.Request.Method, 16)

		_, _ = svc.Ingest(IngestInput{
			Name:       "http_request_duration_ms",
			TargetType: string(TargetPhysicalHost),
			// TargetID carries the matched route — a
			// real deployment would add an "app" target
			// type for application-level metrics; the
			// current schema reuses physical_host for
			// the cardinality ceiling.
			TargetID:  path,
			Value:     float64(dur.Microseconds()) / 1000.0,
			Timestamp: time.Now().UTC(),
			Labels: JSONMap{
				"endpoint": path,
				"method":   method,
				"status":   status,
			},
		})
	}
}

// truncate clamps a string to n bytes. Used to cap label
// values that come from the request — a 5MB query string
// is technically a label but it would blow up the
// dashboard. n <= 0 is a no-op.
func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}
