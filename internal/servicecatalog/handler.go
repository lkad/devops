package servicecatalog

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the service catalog. It is
// intentionally thin: parse, call Catalog, render. All
// validation, orchestration, and persistence live in the
// Catalog / Repository.
type Handler struct {
	cat    *Catalog
	health *Health // optional; nil means health endpoint returns "unknown / not configured"
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

// Register attaches the catalog routes to the supplied
// router group. The group is expected to live under
// /api/v1.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/services", h.List)
	r.POST("/services", h.Create)
	r.GET("/services/:id", h.Get)
	r.PUT("/services/:id", h.Update)
	r.DELETE("/services/:id", h.Delete)
	r.GET("/services/:id/health", h.Health)
}

// Health handles GET /services/:id/health. The response
// is the derived HealthResult; see health.go for the rule.
func (h *Handler) Health(c *gin.Context) {
	id := c.Param("id")
	// 404 if the service does not exist (rather than
	// returning a confusing "unknown health" envelope).
	if _, err := h.cat.Get(id); err != nil {
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

// Get handles GET /services/:id.
func (h *Handler) Get(c *gin.Context) {
	row, err := h.cat.Get(c.Param("id"))
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, row)
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
	if err := h.cat.SoftDelete(c.Param("id")); err != nil {
		writeAPIError(c.Writer, err)
		return
	}
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
