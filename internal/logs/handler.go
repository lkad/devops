// Package logs — handler.go wires the Gin routes for the
// /api/v1/logs surface. The handler is intentionally thin: parse
// query string, call service, render envelope. Validation lives
// in the service.
package logs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the log-aggregation subsystem.
// The Handler is constructed with a Service (the only
// orchestration dependency) and an optional Local backend (used
// for the dev-only echo endpoint). RBAC is the responsibility of
// main.go's middleware chain; this handler is registered onto a
// router group that already has rbac.RequirePermission applied
// in production.
type Handler struct {
	svc   *Service
	local *Local // nil if backend isn't Local; only needed for /_test/echo
}

// NewHandler builds a Handler. The local backend is used by the
// dev-only /_test/echo endpoint to push fake log entries; pass
// nil in production for any other backend. A nil local means the
// /_test/echo route still exists but always returns 400.
func NewHandler(svc *Service, local LogBackend) *Handler {
	var asLocal *Local
	if local != nil {
		if l, ok := local.(*Local); ok {
			asLocal = l
		}
	}
	return &Handler{svc: svc, local: asLocal}
}

// Register attaches the log-aggregation routes to the supplied
// router group. The group is expected to live under /api/v1.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/logs/capabilities", h.Capabilities)
	r.GET("/logs/query", h.Query)
	r.GET("/logs/streams", h.Streams)
	// Dev-only echo endpoint. The leading underscore in the path
	// is the spec's "this is not part of the stable surface"
	// convention.
	r.POST("/logs/_test/echo", h.Echo)
}

// Capabilities is the /capabilities endpoint.
func (h *Handler) Capabilities(c *gin.Context) {
	NewCapabilitiesHandler(h.svc).Get(c)
}

// Query is GET /logs/query. Query string: q, from, to, limit,
// sort, level, source, host, op[level], op[source], op[host].
// The first three (q/from/to/limit/sort) are universal; the
// level/source/host triplet is the universal subset of filters.
func (h *Handler) Query(c *gin.Context) {
	q, apiErr := parseQuery(c)
	if apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	res, err := h.svc.Query(c.Request.Context(), q)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	// Per the spec scenario "Backend unavailable": set Retry-After
	// when the backend is down so a polite client can back off.
	if res.Meta.Backend != "" && res.Meta.Degraded && res.Meta.Reason == "backend unavailable" {
		c.Writer.Header().Set("Retry-After", "30")
	}
	c.JSON(http.StatusOK, res)
}

// Streams is GET /logs/streams.
func (h *Handler) Streams(c *gin.Context) {
	streams, err := h.svc.Streams(c.Request.Context())
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": streams})
}

// Echo is POST /logs/_test/echo. Dev-only — used by the test
// suite to push a fake log entry into the Local backend so
// end-to-end tests can exercise the query path without standing
// up a real storage backend.
func (h *Handler) Echo(c *gin.Context) {
	if h.local == nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "echo endpoint is only available with the Local backend",
		})
		return
	}
	var e LogEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be a valid LogEntry",
		})
		return
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	h.local.Append(e)
	c.JSON(http.StatusCreated, gin.H{"id": e.ID})
}

// parseQuery extracts a Query from the Gin context. Returns
// *contracts.APIError on any validation failure so the handler
// can render the standard envelope.
func parseQuery(c *gin.Context) (Query, *contracts.APIError) {
	q := Query{
		Text: c.Query("q"),
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return q, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "from must be an RFC3339 timestamp",
			}
		}
		q.From = t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return q, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "to must be an RFC3339 timestamp",
			}
		}
		q.To = t
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return q, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "limit must be an integer",
			}
		}
		q.Limit = n
	}
	q.Sort = c.Query("sort")
	// Universal-subset filters: level, source, host.
	for _, k := range []string{"level", "source", "host"} {
		if v := c.Query(k); v != "" {
			q.Filters = append(q.Filters, Filter{Key: k, Op: "eq", Value: v})
		}
	}
	// Repeatable filters: filter[key]=op:value (the wire form
	// the spec implies for arbitrary key/op/value triples).
	for k, vs := range c.Request.URL.Query() {
		if !strings.HasPrefix(k, "filter[") || !strings.HasSuffix(k, "]") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(k, "filter["), "]")
		for _, raw := range vs {
			op, value := splitFilterValue(raw)
			q.Filters = append(q.Filters, Filter{Key: key, Op: op, Value: value})
		}
	}
	return q, nil
}

// splitFilterValue splits "op:value" or just "value" (default op
// "eq") on the first colon.
func splitFilterValue(s string) (string, string) {
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "eq", s
}

// writeAPIError is a small adapter so we can pass an `error`
// returned from the service directly to the handler's WriteError,
// which expects a *contracts.APIError.
func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		handler.WriteError(w, apiErr)
		return
	}
	handler.WriteError(w, &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: fmt.Sprintf("internal error: %s", err.Error()),
	})
}
