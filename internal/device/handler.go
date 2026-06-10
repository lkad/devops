package device

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the device-management subsystem.
// It is intentionally thin: parse, call service, render. All
// validation, orchestration, and persistence live in the
// service / repository.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The Service is the only
// dependency; future middleware (RBAC, audit logging) can be
// injected here without changing the service contract.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches the device routes to the supplied router
// group. The group is expected to live under /api/v1; the
// handler is agnostic about the surrounding stack.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewDevices)
	modP := perms(rbac.PermissionModifyConfig)
	writeP := perms(rbac.PermissionWriteDevices)
	r.GET("/devices", viewP, h.List)
	r.GET("/devices/search", viewP, h.Search)
	r.POST("/devices", writeP, h.Create)
	r.GET("/devices/:id", viewP, h.Get)
	r.PUT("/devices/:id", writeP, h.Replace)
	r.DELETE("/devices/:id", writeP, h.Delete)
	r.POST("/devices/:id/actions", modP, h.Action)
}

// deviceRequest is the wire shape for POST /devices. We keep
// it as a flat struct and map it onto CreateDeviceInput /
// UpdateDeviceInput inside the handler so the service stays
// free of Gin / JSON tags.
type deviceRequest struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	State      string         `json:"state"`
	GroupID    *string        `json:"group_id"`
	TemplateID *string        `json:"template_id"`
	Labels     map[string]any `json:"labels"`
	Metadata   map[string]any `json:"metadata"`
}

func (r deviceRequest) toCreate() CreateDeviceInput {
	in := CreateDeviceInput{
		Name:       r.Name,
		Type:       DeviceType(r.Type),
		GroupID:    r.GroupID,
		TemplateID: r.TemplateID,
	}
	if r.State != "" {
		in.State = DeviceState(r.State)
	}
	if r.Labels != nil {
		in.Labels = JSONMap(r.Labels)
	}
	if r.Metadata != nil {
		in.Metadata = JSONMap(r.Metadata)
	}
	return in
}

func (r deviceRequest) toUpdate() UpdateDeviceInput {
	in := UpdateDeviceInput{
		GroupID:    r.GroupID,
		TemplateID: r.TemplateID,
	}
	if r.Name != "" {
		name := r.Name
		in.Name = &name
	}
	if r.Type != "" {
		dt := DeviceType(r.Type)
		in.Type = &dt
	}
	if r.State != "" {
		ds := DeviceState(r.State)
		in.State = &ds
	}
	if r.Labels != nil {
		in.Labels = JSONMap(r.Labels)
	}
	if r.Metadata != nil {
		in.Metadata = JSONMap(r.Metadata)
	}
	return in
}

// List handles GET /devices. It honours the type/state/group_id
// query params, plus search/limit/offset for paging. The
// response is the standard ListResponse envelope.
func (h *Handler) List(c *gin.Context) {
	filter, err := parseListFilter(c)
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

// Search handles GET /devices/search. We accept a single
// "q" parameter (the spec mentions "tag" but the user-facing
// surface is a free-text search) and run it through the
// repository's Search method.
func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	rows, total, svcErr := h.svc.Search(q)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	page := contracts.Pagination{
		Total:   total,
		Limit:   len(rows),
		Offset:  0,
		HasMore: false,
	}
	handler.WriteList(c.Writer, rows, &page)
}

// Get handles GET /devices/:id. A single resource is rendered
// as the bare object (no envelope), matching the rest of the
// API.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	d, svcErr := h.svc.Get(id)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, d)
}

// Create handles POST /devices. A 201 is returned with the
// freshly-stored device.
func (h *Handler) Create(c *gin.Context) {
	var req deviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	d, svcErr := h.svc.Create(req.toCreate())
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteCreated(c.Writer, d)
}

// Replace handles PUT /devices/:id. The spec calls it "update";
// we treat it as a partial update so the client can patch a
// subset of fields.
func (h *Handler) Replace(c *gin.Context) {
	id := c.Param("id")
	var req deviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	d, svcErr := h.svc.Update(id, req.toUpdate())
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, d)
}

// Delete handles DELETE /devices/:id. A 204 is returned on
// success; a 404 on missing rows.
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if svcErr := h.svc.Delete(id); svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// actionRequest is the wire shape of POST /devices/:id/actions.
type actionRequest struct {
	Action string `json:"action"`
}

// Action handles POST /devices/:id/actions. The handler does
// not know what actions exist; the service does. This keeps
// the state machine in one place.
func (h *Handler) Action(c *gin.Context) {
	id := c.Param("id")
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON with an 'action' field",
		})
		return
	}
	d, svcErr := h.svc.ApplyAction(id, req.Action)
	if svcErr != nil {
		writeAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, d)
}

// parseListFilter turns the query string into a ListFilter.
// Any error here is a 400 — bad limit/offset, unknown enum, etc.
func parseListFilter(c *gin.Context) (ListFilter, *contracts.APIError) {
	f := ListFilter{}
	if v := c.Query("type"); v != "" {
		dt := DeviceType(v)
		if !dt.Valid() {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "type filter is not a recognised device type",
			}
		}
		f.Type = dt
	}
	if v := c.Query("state"); v != "" {
		ds := DeviceState(v)
		if !ds.Valid() {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "state filter is not a recognised device state",
			}
		}
		f.State = ds
	}
	f.GroupID = c.Query("group_id")
	f.Search = c.Query("search")
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

// writeAPIError is a small adapter so we can pass an
// `error` returned from the service directly to the handler's
// WriteError, which expects a *contracts.APIError.
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
