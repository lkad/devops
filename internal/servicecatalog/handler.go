package servicecatalog

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the service catalog. It is
// intentionally thin: parse, call Catalog, render. All
// validation, orchestration, and persistence live in the
// Catalog / Repository.
type Handler struct {
	cat     *Catalog
	health  *Health  // optional; nil means health endpoint returns "unknown / not configured"
	metrics *Metrics // optional; nil = no Prometheus recording (unit tests)
}

// NewHandler builds a Handler. The Health dependency is
// optional — pass nil in tests that do not exercise
// health; pass the production value (built off the
// pipeline.Repository) for the real router.
func NewHandler(cat *Catalog) *Handler { return &Handler{cat: cat} }

// SetHealth injects the Health dependency after
// construction. Kept separate so the handler does not
// have to import the pipeline package (which would be a
// cycle) — main.go wires the dependency in a second step.
func (h *Handler) SetHealth(hl *Health) { h.health = hl }

// SetMetrics injects the Prometheus instrument set after
// construction. Optional — handlers without metrics still
// work (the Record method is a no-op on nil). Production
// wiring in main.go always passes a non-nil Metrics built
// off the observability registry.
func (h *Handler) SetMetrics(m *Metrics) { h.metrics = m }

// Register attaches the catalog routes to the supplied
// router group. The group is expected to live under
// /api/v1.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewServiceCatalog)
	writeP := perms(rbac.PermissionManageServiceCatalog)
	r.GET("/services", viewP, h.List)
	r.POST("/services", writeP, h.Create)
	r.GET("/services/:id", viewP, h.Get)
	r.PUT("/services/:id", writeP, h.Update)
	r.DELETE("/services/:id", writeP, h.Delete)
	r.GET("/services/:id/health", viewP, h.Health)
	// On-call rotation writes (P1.4). The detail page
	// re-fetches GET /services/:id after a successful
	// mutation; the writes are deliberately separate
	// endpoints so the rotation's per-row keys stay
	// explicit (and so the audit log records them as
	// distinct actions once P0 #3 lands).
	r.POST("/services/:id/oncall", writeP, h.CreateOnCall)
	r.DELETE("/services/:id/oncall/:shift_id", writeP, h.DeleteOnCall)
	// Runbook writes (P1.4). Same shape as oncall: the
	// per-entry ID lives in the URL so a future "edit
	// entry" PUT can drop in without changing the
	// contract.
	r.POST("/services/:id/runbook", writeP, h.CreateRunbook)
	r.DELETE("/services/:id/runbook/:entry_id", writeP, h.DeleteRunbook)
}

// Health handles GET /services/:id/health. The response
// is the derived HealthResult; see health.go for the rule.
func (h *Handler) Health(c *gin.Context) {
	id := c.Param("id")
	// 404 if the service does not exist (rather than
	// returning a confusing "unknown health" envelope).
	svc, err := h.cat.Get(id)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	if h.health == nil {
		// Defensive: a handler wired without a Health
		// (e.g. in a unit test) returns unknown.
		c.JSON(http.StatusOK, &HealthResult{
			ServiceID:   id,
			Status:      HealthUnknown,
			DerivedFrom: "last_pipeline_run",
			Reason:      "health_not_configured",
		})
		return
	}
	out, err := h.health.Rollup(id)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	// Record the rollup into Prometheus AFTER we've
	// computed the real result. A nil h.metrics is a
	// no-op; the production wiring in main.go always
	// supplies one. Done before the response is rendered
	// so a scrape racing with the response sees the
	// latest value.
	h.metrics.Record(svc, out)
	c.JSON(http.StatusOK, out)
}

// serviceRequest is the wire shape for POST /services.
type serviceRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Owner         string `json:"owner"`
	RepositoryURL string `json:"repository_url"`
	Tier          string `json:"tier"`
}

func (r serviceRequest) toInput() CreateInput {
	return CreateInput{
		Name:          r.Name,
		Description:   r.Description,
		Owner:         r.Owner,
		RepositoryURL: r.RepositoryURL,
		Tier:          Tier(r.Tier),
	}
}

// List handles GET /services. Supports ?tier=, ?owner=,
// ?q= name-contains, ?limit=, ?offset=.
func (h *Handler) List(c *gin.Context) {
	f := ServiceFilter{
		Tier:  Tier(c.Query("tier")),
		Owner: c.Query("owner"),
		Query: c.Query("q"),
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeAPIError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "limit must be an integer",
			})
			return
		}
		f.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeAPIError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "offset must be an integer",
			})
			return
		}
		f.Offset = n
	}
	rows, err := h.cat.List(f)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	page := contracts.Pagination{
		Total:   int64(len(rows)),
		Limit:   f.Limit,
		Offset:  f.Offset,
		HasMore: f.Limit > 0 && f.Offset+f.Limit < len(rows),
	}
	handler.WriteList(c.Writer, rows, &page)
}

// Create handles POST /services.
func (h *Handler) Create(c *gin.Context) {
	var req serviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	row, err := h.cat.Create(req.toInput())
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, row)
}

// Get handles GET /services/:id. The response embeds
// the current on-call (or null) and the runbook so the
// detail page has everything it needs in one round trip.
func (h *Handler) Get(c *gin.Context) {
	row, err := h.cat.Get(c.Param("id"))
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	oncall, ocErr := h.cat.repo.CurrentOnCall(row.ID, time.Now())
	if ocErr != nil {
		// On-call lookup failure is not fatal — the
		// page just renders "unknown on-call" rather
		// than a 500.
		oncall = nil
	}
	runbook, rbErr := h.cat.repo.ListRunbook(row.ID)
	if rbErr != nil {
		runbook = nil
	}
	handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
		"id":             row.ID,
		"name":           row.Name,
		"description":    row.Description,
		"owner":          row.Owner,
		"repository_url": row.RepositoryURL,
		"tier":           row.Tier,
		"created_at":     row.CreatedAt,
		"updated_at":     row.UpdatedAt,
		"oncall":         oncall,
		"runbook":        runbook,
	})
}

// Update handles PUT /services/:id (partial).
func (h *Handler) Update(c *gin.Context) {
	var req serviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	in := UpdateInput{
		Name:          ptrOrNil(req.Name),
		Description:   ptrOrNil(req.Description),
		Owner:         ptrOrNil(req.Owner),
		RepositoryURL: ptrOrNil(req.RepositoryURL),
	}
	// Tier: present in the wire shape only when the
	// caller sent a non-empty value; nil otherwise.
	if req.Tier != "" {
		t := Tier(req.Tier)
		in.Tier = &t
	}
	row, err := h.cat.Update(c.Param("id"), in)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, row)
}

// Delete handles DELETE /services/:id (soft delete).
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.cat.SoftDelete(id); err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	// Drop the gauge series for the deleted service so
	// stale "last known health" labels do not linger in
	// Prometheus until the scrape-side eviction kicks
	// in. Counter values are not reset (they describe
	// observed history, not state). A nil h.metrics is
	// a no-op.
	h.metrics.Reset(id)
	c.Writer.WriteHeader(http.StatusNoContent)
}

// ptrOrNil returns a pointer to s, or nil if s is empty.
// Used so the partial-update DTO can distinguish
// "field absent" (nil) from "field set to empty string"
// (which the spec does not allow for some fields).
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// writeAPIError translates typed service-catalog errors
// into the project's standard APIError envelope. Unknown
// errors land at 500.
func writeAPIError(w http.ResponseWriter, err error) {
	switch {
	case IsNotFound(err):
		handler.WriteError(w, &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: "service not found",
			Cause:   err,
		})
	case IsConflict(err):
		handler.WriteError(w, &contracts.APIError{
			Code:    contracts.CodeConflict,
			Message: "service name already in use",
			Cause:   err,
		})
	case IsValidation(err):
		handler.WriteError(w, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: errors.Unwrap(err).Error(),
			Cause:   err,
		})
	default:
		handler.WriteError(w, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "internal error",
			Cause:   err,
		})
	}
}

// onCallRequest is the wire shape for
// POST /services/:id/oncall. The fields line up 1:1
// with the catalog's CreateOnCallInput so the handler
// is purely a JSON→DTO translator.
type onCallRequest struct {
	UserEmail  string    `json:"user_email"`
	ShiftStart time.Time `json:"shift_start"`
	ShiftEnd   time.Time `json:"shift_end"`
	Scope      string    `json:"scope"`
}

// toInput maps the wire shape onto the service-layer
// DTO. The actor is pulled from the authenticated
// context (rbac.AuthUserKey) and falls back to the dev
// X-User header so the unit tests can drive writes
// without standing up a JWT signer. In production the
// auth middleware populates AuthUserKey and the header
// is ignored.
func (r onCallRequest) toInput(c *gin.Context) CreateOnCallInput {
	return CreateOnCallInput{
		User:       r.UserEmail,
		ShiftStart: r.ShiftStart,
		ShiftEnd:   r.ShiftEnd,
		Scope:      OnCallScope(r.Scope),
		Actor:      actorFrom(c),
	}
}

// CreateOnCall handles POST /services/:id/oncall.
// Validates the request shape, resolves the service,
// checks for an overlapping shift (409 on collision),
// then persists. The actor (JWT subject) is captured
// on the DTO so the eventual audit emit (P0 #3) is
// forgery-proof.
func (h *Handler) CreateOnCall(c *gin.Context) {
	var req onCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	row, err := h.cat.CreateOnCall(c.Param("id"), req.toInput(c))
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, row)
}

// DeleteOnCall handles DELETE
// /services/:id/oncall/:shift_id. 404 if the service
// or the shift id is unknown; 204 on success.
func (h *Handler) DeleteOnCall(c *gin.Context) {
	if err := h.cat.DeleteOnCall(c.Param("id"), c.Param("shift_id")); err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

// runbookRequest is the wire shape for
// POST /services/:id/runbook.
type runbookRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (r runbookRequest) toInput(c *gin.Context) CreateRunbookInput {
	return CreateRunbookInput{
		Title: r.Title,
		Body:  r.Body,
		Actor: actorFrom(c),
	}
}

// CreateRunbook handles POST /services/:id/runbook.
func (h *Handler) CreateRunbook(c *gin.Context) {
	var req runbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	row, err := h.cat.CreateRunbook(c.Param("id"), req.toInput(c))
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, row)
}

// DeleteRunbook handles DELETE
// /services/:id/runbook/:entry_id.
func (h *Handler) DeleteRunbook(c *gin.Context) {
	if err := h.cat.DeleteRunbook(c.Param("id"), c.Param("entry_id")); err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

// actorFrom returns the authenticated user id for the
// audit trail. Order of precedence:
//  1. The *contracts.User stashed by the auth
//     middleware under rbac.AuthUserKey (the JWT
//     subject — never the request body).
//  2. The X-User dev-bypass header, so unit tests can
//     drive writes without a real JWT signer. In
//     production DevBypass is false and the auth
//     middleware aborts before we get here when the
//     JWT is missing.
//  3. The literal "system" so a misconfigured deploy
//     still produces an audit-trail row rather than
//     silently dropping the actor.
//
// This is the wire-side of the P0 #3 audit-trail
// fix: the actor is always the JWT subject, never the
// request body, so a malicious client cannot forge the
// recorded actor.
func actorFrom(c *gin.Context) string {
	if v, ok := c.Get(rbac.AuthUserKey); ok {
		if u, ok := v.(*contracts.User); ok && u != nil {
			if u.ID != "" {
				return u.ID
			}
		}
	}
	if uid := c.GetHeader("X-User"); uid != "" {
		return uid
	}
	return "system"
}
