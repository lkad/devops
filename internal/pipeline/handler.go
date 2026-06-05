package pipeline

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the pipeline subsystem. It
// is intentionally thin: parse, call service, render. All
// validation, orchestration, and persistence live in the
// service / repository.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The Service is the only
// dependency; future middleware (RBAC, audit logging) can
// be injected here without changing the service contract.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register attaches the pipeline routes to the supplied
// router group. The group is expected to live under
// /api/v1; the handler is agnostic about the surrounding
// stack.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/pipelines", h.List)
	r.POST("/pipelines", h.Create)
	r.GET("/pipelines/:id", h.Get)
	r.PUT("/pipelines/:id", h.Update)
	r.DELETE("/pipelines/:id", h.Delete)
	r.POST("/pipelines/:id/trigger", h.Trigger)
	r.GET("/pipelines/:id/runs", h.ListRuns)
	r.GET("/runs/:run_id", h.GetRun)
	r.POST("/runs/:run_id/cancel", h.CancelRun)
}

// pipelineRequest is the wire shape for POST /pipelines. We
// keep it as a flat struct and map it onto
// CreatePipelineInput / UpdatePipelineInput inside the
// handler so the service stays free of Gin / JSON tags.
type pipelineRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	ProjectID   string                   `json:"project_id"`
	TargetType  string                   `json:"target_type"`
	TargetID    string                   `json:"target_id"`
	Trigger     string                   `json:"trigger"`
	Enabled     *bool                    `json:"enabled"`
	Steps       []pipelineStepRequest    `json:"steps"`
}

type pipelineStepRequest struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Config map[string]any         `json:"config"`
}

// toCreateInput maps the wire shape onto a service-layer
// CreatePipelineInput.
func (r pipelineRequest) toCreateInput() CreatePipelineInput {
	steps := make([]PipelineStep, 0, len(r.Steps))
	for _, s := range r.Steps {
		steps = append(steps, PipelineStep{
			Name:   s.Name,
			Type:   StepType(s.Type),
			Config: StepConfig(s.Config),
		})
	}
	return CreatePipelineInput{
		Name:        r.Name,
		Description: r.Description,
		ProjectID:   r.ProjectID,
		TargetType:  TargetType(r.TargetType),
		TargetID:    r.TargetID,
		Trigger:     r.Trigger,
		Enabled:     r.Enabled,
		Steps:       steps,
	}
}

// toUpdateInput maps the wire shape onto a service-layer
// UpdatePipelineInput. Empty fields are treated as "leave
// unchanged" (pointer is left nil) so PUT can be used for
// partial updates.
func (r pipelineRequest) toUpdateInput() UpdatePipelineInput {
	in := UpdatePipelineInput{}
	if r.Name != "" {
		n := r.Name
		in.Name = &n
	}
	if r.Description != "" {
		d := r.Description
		in.Description = &d
	}
	if r.TargetType != "" {
		tt := TargetType(r.TargetType)
		in.TargetType = &tt
	}
	if r.TargetID != "" {
		tid := r.TargetID
		in.TargetID = &tid
	}
	if r.Enabled != nil {
		en := *r.Enabled
		in.Enabled = &en
	}
	return in
}

// List handles GET /pipelines. It honours the
// project_id query param plus limit/offset for paging. The
// response is the standard ListResponse envelope.
func (h *Handler) List(c *gin.Context) {
	filter, err := parsePipelineFilter(c)
	if err != nil {
		handler.WriteError(c.Writer, err)
		return
	}
	rows, total, svcErr := h.svc.List(filter)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	page := contracts.Pagination{
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: filter.Limit > 0 && filter.Offset+filter.Limit < int(total),
	}
	handler.WriteList(c.Writer, rows, &page)
}

// Get handles GET /pipelines/:id. A single resource is
// rendered as the bare object (no envelope), matching the
// rest of the API.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	p, svcErr := h.svc.Get(id)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// Create handles POST /pipelines. A 201 is returned with the
// freshly-stored pipeline.
func (h *Handler) Create(c *gin.Context) {
	var req pipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	p, svcErr := h.svc.Create(req.toCreateInput())
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteCreated(c.Writer, p)
}

// Update handles PUT /pipelines/:id. The spec calls it
// "update"; we treat it as a partial update so the client
// can patch a subset of fields.
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req pipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	p, svcErr := h.svc.Update(id, req.toUpdateInput())
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// Delete handles DELETE /pipelines/:id. A 204 is returned on
// success; a 404 on missing rows.
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if svcErr := h.svc.Delete(id); svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// Trigger handles POST /pipelines/:id/trigger. The
// handler returns the new run object. The executor is
// dispatched in a goroutine by the service layer.
func (h *Handler) Trigger(c *gin.Context) {
	id := c.Param("id")
	triggeredBy := c.GetHeader("X-User-Id")
	run, svcErr := h.svc.Trigger(id, triggeredBy)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteCreated(c.Writer, run)
}

// ListRuns handles GET /pipelines/:id/runs. The response
// is the standard ListResponse envelope.
func (h *Handler) ListRuns(c *gin.Context) {
	id := c.Param("id")
	limit, offset := parsePaging(c)
	runs, total, svcErr := h.svc.ListRuns(id, limit, offset)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	page := contracts.Pagination{
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: limit > 0 && offset+limit < int(total),
	}
	handler.WriteList(c.Writer, runs, &page)
}

// GetRun handles GET /runs/:run_id. The response is the run
// object plus its per-step records.
type runDetailResponse struct {
	*PipelineRun
	Steps []PipelineStepRun `json:"steps"`
}

func (h *Handler) GetRun(c *gin.Context) {
	id := c.Param("run_id")
	run, steps, svcErr := h.svc.GetRunWithSteps(id)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, runDetailResponse{PipelineRun: run, Steps: steps})
}

// CancelRun handles POST /runs/:run_id/cancel. The handler
// is idempotent: cancelling an already-finished run is a
// 200 with the run's terminal status.
func (h *Handler) CancelRun(c *gin.Context) {
	id := c.Param("run_id")
	if svcErr := h.svc.Cancel(id); svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	run, _, svcErr := h.svc.GetRunWithSteps(id)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, run)
}

// parsePipelineFilter turns the query string into a
// PipelineFilter. Any error here is a 400.
func parsePipelineFilter(c *gin.Context) (PipelineFilter, *contracts.APIError) {
	f := PipelineFilter{ProjectID: c.Query("project_id"), Search: c.Query("search")}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "limit must be an integer",
			}
		}
		f.Limit = n
	} else {
		f.Limit = 20
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "offset must be an integer",
			}
		}
		f.Offset = n
	}
	return f, nil
}

// parsePaging extracts limit/offset from the query string.
// Returns (0, 0) when not supplied — the repository treats
// 0 as "no pagination".
func parsePaging(c *gin.Context) (int, int) {
	limit := 0
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	return limit, offset
}

// writeAPIError is a small adapter so we can pass an
// `error` returned from the service directly to the
// handler's WriteError, which expects a *contracts.APIError.
func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		handler.WriteError(w, apiErr)
		return
	}
	handler.WriteError(w, &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: err.Error(),
	})
}
