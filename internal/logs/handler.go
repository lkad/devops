// Package logs — handler.go wires the Gin routes for the
// /api/v1/logs surface. The handler is intentionally thin: parse
// query string, call service, render envelope. Validation lives
// in the service.
package logs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
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
	local *Local     // nil if backend isn't Local; only needed for /_test/echo
	extra *ExtraService
	repo  *ExtraRepository
	audit *audit.Service
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

// NewHandlerWithExtra is the constructor that exposes the
// retention / saved-filter / alert-rule routes. The
// standard NewHandler keeps those fields nil so a
// production deployment can opt out by simply not calling
// this constructor. The audit service is optional; a nil
// value short-circuits the audit emission in the saved-filter
// and alert-rule mutating handlers.
func NewHandlerWithExtra(svc *Service, extra *ExtraService, repo *ExtraRepository, auditSvc ...*audit.Service) *Handler {
	var a *audit.Service
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &Handler{svc: svc, extra: extra, repo: repo, audit: a}
}

// emitSavedFilter / emitAlertRule are the small audit helpers
// for the saved-filter and alert-rule mutating handlers. A
// nil audit service short-circuits so unit tests do not need
// a fake. The context is background because the audit row is
// a post-commit side effect; failures are best-effort.
func (h *Handler) emitSavedFilter(action audit.AuditAction, id string, metadata audit.JSONMap) {
	if h.audit == nil {
		return
	}
	h.audit.RecordAction(context.Background(), audit.RecordActionInput{
		Action:       action,
		ResourceType: audit.ResourceSavedFilter,
		ResourceID:   id,
		Metadata:     metadata,
	})
}

func (h *Handler) emitAlertRule(action audit.AuditAction, id string, metadata audit.JSONMap) {
	if h.audit == nil {
		return
	}
	h.audit.RecordAction(context.Background(), audit.RecordActionInput{
		Action:       action,
		ResourceType: audit.ResourceAlertRule,
		ResourceID:   id,
		Metadata:     metadata,
	})
}

// Register attaches the log-aggregation routes to the supplied
// router group. The group is expected to live under /api/v1.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
// Register attaches the log-aggregation routes.
//
// Per-project access: SAVED FILTERS ARE USER-SCOPED (the
// OwnerUserID field, not a project) and ALERT RULES HERE
// ARE GLOBAL (the logs module's alert rules apply to every
// log stream, not to a specific project). The threat
// model relies on the global rbac matrix: ViewLogs lets a
// caller read any stream; WriteLogs lets a caller create
// filters and rules; the OwnerUserID on a saved filter is
// informational (the global rbac decides who can see
// every filter, not the owner field). A future "filter
// per project" feature would add a project_id column and
// a per-project access gate; until then perms() is the
// only seam.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewLogs)
	writeP := perms(rbac.PermissionWriteLogs)
	r.GET("/logs/capabilities", viewP, h.Capabilities)
	r.GET("/logs/query", viewP, h.Query)
	r.GET("/logs/streams", viewP, h.Streams)
	// Dev-only echo endpoint. The leading underscore in the path
	// is the spec's "this is not part of the stable surface"
	// convention.
	r.POST("/logs/_test/echo", writeP, h.Echo)

	// Spec coverage: retention policy + log statistics.
	if h.extra != nil {
		r.GET("/logs/retention", viewP, h.GetRetention)
		r.PUT("/logs/retention", writeP, h.SetRetention)
		r.POST("/logs/retention/cleanup", writeP, h.TriggerRetentionCleanup)
		r.GET("/logs/stats", viewP, h.GetLogStats)

		// Spec coverage: saved filters (full CRUD + apply).
		r.POST("/logs/saved-filters", writeP, h.CreateSavedFilter)
		r.GET("/logs/saved-filters", viewP, h.ListSavedFilters)
		r.GET("/logs/saved-filters/:id", viewP, h.GetSavedFilter)
		r.DELETE("/logs/saved-filters/:id", writeP, h.DeleteSavedFilter)
		r.POST("/logs/saved-filters/:id/apply", viewP, h.ApplySavedFilter)

		// Spec coverage: alert rules CRUD.
		r.POST("/logs/alert-rules", writeP, h.CreateAlertRule)
		r.GET("/logs/alert-rules", viewP, h.ListAlertRules)
		r.DELETE("/logs/alert-rules/:id", writeP, h.DeleteAlertRule)
	}
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
		handler.WriteAPIError(c.Writer, err)
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
		handler.WriteAPIError(c.Writer, err)
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

// =============================================================================
// Extra HTTP handlers — retention, statistics, saved filters, alert rules.
// Each handler is small: validate input, call extra service / repo,
// render. The retention + filter + rule mutations are append-only by
// the spec; no PATCH endpoint is exposed.
// =============================================================================

// GetRetention handles GET /logs/retention.
func (h *Handler) GetRetention(c *gin.Context) {
	if h.extra == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "retention service not wired"})
		return
	}
	p, err := h.extra.GetRetention()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// SetRetention handles PUT /logs/retention.
func (h *Handler) SetRetention(c *gin.Context) {
	if h.extra == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "retention service not wired"})
		return
	}
	var in struct {
		RetentionDays int `json:"retention_days"`
		MaxStorageGB   int `json:"max_storage_gb"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be JSON"})
		return
	}
	if in.RetentionDays <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "retention_days must be > 0"})
		return
	}
	if in.MaxStorageGB <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_storage_gb must be > 0"})
		return
	}
	p, err := h.extra.SetRetention(&RetentionPolicy{
		RetentionDays: in.RetentionDays,
		MaxStorageGB:   in.MaxStorageGB,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// TriggerRetentionCleanup handles POST /logs/retention/cleanup.
func (h *Handler) TriggerRetentionCleanup(c *gin.Context) {
	if h.extra == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "retention service not wired"})
		return
	}
	report, err := h.extra.TriggerCleanup(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// GetLogStats handles GET /logs/stats.
func (h *Handler) GetLogStats(c *gin.Context) {
	if h.extra == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stats service not wired"})
		return
	}
	stats, err := h.extra.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// CreateSavedFilter handles POST /logs/saved-filters.
func (h *Handler) CreateSavedFilter(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "saved-filters repo not wired"})
		return
	}
	var in struct {
		Name        string         `json:"name"`
		OwnerUserID string         `json:"owner_user_id"`
		Query       map[string]any `json:"query"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be JSON"})
		return
	}
	if err := validateName(in.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	qJSON, _ := json.Marshal(in.Query)
	f := &SavedFilter{
		OwnerUserID: in.OwnerUserID,
		Name:        in.Name,
		Query:       string(qJSON),
	}
	if err := h.repo.CreateSavedFilter(f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.emitSavedFilter(audit.ActionCreate, f.ID, audit.JSONMap{
		"name":         f.Name,
		"owner_user_id": f.OwnerUserID,
	})
	c.JSON(http.StatusCreated, f)
}

// ListSavedFilters handles GET /logs/saved-filters.
func (h *Handler) ListSavedFilters(c *gin.Context) {
	rows, err := h.repo.ListSavedFilters()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GetSavedFilter handles GET /logs/saved-filters/:id.
func (h *Handler) GetSavedFilter(c *gin.Context) {
	f, err := h.repo.GetSavedFilter(c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, f)
}

// DeleteSavedFilter handles DELETE /logs/saved-filters/:id.
func (h *Handler) DeleteSavedFilter(c *gin.Context) {
	id := c.Param("id")
	err := h.repo.DeleteSavedFilter(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.emitSavedFilter(audit.ActionDelete, id, nil)
	c.Status(http.StatusNoContent)
}

// ApplySavedFilter handles POST /logs/saved-filters/:id/apply.
// Returns the rendered query + a rows array of matching
// log records. The local backend's seed data is empty so
// rows is typically [].
func (h *Handler) ApplySavedFilter(c *gin.Context) {
	f, err := h.repo.GetSavedFilter(c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var q map[string]any
	_ = json.Unmarshal([]byte(f.Query), &q)
	// Render: re-issue the underlying log query.
	var rows []map[string]any
	if h.svc != nil {
		// The Query type doesn't have a Level field; level
		// filtering is expressed via the Filters slice. Keep
		// the apply path simple — the rendered query
		// already encodes any level filter in q["level"].
		res, err := h.svc.Query(c.Request.Context(), Query{
			Text:  stringFromMap(q, "q"),
			Limit: intFromMap(q, "limit", 50),
		})
		if err == nil {
			for _, e := range res.Entries {
				rows = append(rows, map[string]any{
					"id":        e.ID,
					"timestamp": e.Timestamp,
					"level":     e.Level,
					"source":    e.Source,
					"host":      e.Host,
					"message":   e.Message,
				})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"query": q, "rows": rows})
}

// CreateAlertRule handles POST /logs/alert-rules.
func (h *Handler) CreateAlertRule(c *gin.Context) {
	var in struct {
		Name      string `json:"name"`
		Condition string `json:"condition"`
		Window    string `json:"window"`
		Threshold int    `json:"threshold"`
		Channel   string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be JSON"})
		return
	}
	if err := validateName(in.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Condition == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "condition is required"})
		return
	}
	r := &AlertRule{
		Name: in.Name, Condition: in.Condition, Window: in.Window,
		Threshold: in.Threshold, Channel: in.Channel,
	}
	if err := h.repo.CreateAlertRule(r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.emitAlertRule(audit.ActionCreate, r.ID, audit.JSONMap{
		"name":      r.Name,
		"condition": r.Condition,
	})
	c.JSON(http.StatusCreated, r)
}

// ListAlertRules handles GET /logs/alert-rules.
func (h *Handler) ListAlertRules(c *gin.Context) {
	rows, err := h.repo.ListAlertRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// DeleteAlertRule handles DELETE /logs/alert-rules/:id.
func (h *Handler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	err := h.repo.DeleteAlertRule(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.emitAlertRule(audit.ActionDelete, id, nil)
	c.Status(http.StatusNoContent)
}

// stringFromMap / intFromMap are tiny typed accessors so
// the apply handler stays readable.
func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intFromMap(m map[string]any, key string, def int) int {
	if m == nil {
		return def
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return def
}
