package audit

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

// Handler is the HTTP layer for the audit-logging subsystem.
// Per the layered rules it is intentionally thin: parse query
// params, call service, render. All validation, persistence,
// and emission live in the service / repository / emitter.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler bound to the supplied service.
// The service is the only dependency; future RBAC middleware
// can be added at the route level without changing this type.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register wires the audit routes onto the supplied router
// group. The group is expected to live under /api/v1; the
// handler is agnostic about the surrounding stack.
//
//	GET /api/v1/audit          list events (filterable)
//	GET /api/v1/audit/:id      get a single event
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewAuditLog)
	r.GET("/audit", viewP, h.List)
	r.GET("/audit/:id", viewP, h.Get)
}

// List handles GET /api/v1/audit. The supported query
// parameters are: action, actor_id, actor_username,
// resource_type, resource_id, from, to, limit, offset. The
// response is the standard ListResponse envelope.
func (h *Handler) List(c *gin.Context) {
	filter, err := h.parseFilter(c)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	rows, total, err := h.svc.List(filter)
	if err != nil {
		writeAPIError(c.Writer, err)
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

// Get handles GET /api/v1/audit/:id. A single resource is
// rendered as the bare object.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	e, err := h.svc.Get(id)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, e)
}

// parseFilter turns the query string into an AuditFilter. A
// malformed time-range parameter is a 400 VALIDATION_ERROR.
func (h *Handler) parseFilter(c *gin.Context) (AuditFilter, error) {
	f := AuditFilter{}
	if v := c.Query("action"); v != "" {
		f.Action = AuditAction(v)
	}
	if v := c.Query("actor_id"); v != "" {
		f.ActorID = v
	}
	if v := c.Query("actor_username"); v != "" {
		f.ActorUsername = v
	}
	if v := c.Query("resource_type"); v != "" {
		f.ResourceType = AuditResourceType(v)
	}
	if v := c.Query("resource_id"); v != "" {
		f.ResourceID = v
	}
	if v := c.Query("from"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "from must be an RFC3339 timestamp",
			}
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "to must be an RFC3339 timestamp",
			}
		}
		f.To = &t
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	} else {
		// default page size mirrors the rest of the codebase
		// (see alerts.handler.parseFilter).
		f.Limit = 20
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	return f, nil
}

// parseTime accepts the RFC3339 wire format used by the rest of
// the project. Other layouts are rejected with an explicit
// 400 so the client gets a clear "what format do you want"
// answer.
func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("invalid time format")
}

// writeAPIError is a small adapter so we can pass an `error`
// returned from the service directly to the handler's
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
