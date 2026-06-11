package discovery

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the network-discovery
// subsystem. It is intentionally thin: parse, call service,
// render. All validation, orchestration, and persistence
// live in the service / repository / scanner / prober.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The Service is the only
// dependency; future middleware (RBAC, audit logging) can be
// injected here without changing the service contract.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches the discovery routes to the supplied
// router group. The group is expected to live under /api/v1;
// the handler is agnostic about the surrounding stack.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
// Register attaches the network-discovery routes.
//
// Per-project access: DISCOVERY RUNS ARE GLOBAL — a
// DiscoveryRun is a platform-level scan, not a per-tenant
// resource. The PromoteHosts endpoint is the boundary:
// the resulting Device rows are project-scoped via the
// hostproject links table, and PromoteHosts must enforce
// that the caller can write to the target project. Today
// PromoteHosts reuses the global PermissionWriteDevices
// only; a future iteration can add a project_id from the
// request body and gate Promote on
// rbac.HasPermissionInProject. Until then the global
// rbac matrix is the only seam and the threat model is
// "any Operator can promote any discovered host".
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewDiscovery)
	runP := perms(rbac.PermissionRunDiscovery)
	r.POST("/discovery/runs", runP, h.CreateRun)
	r.GET("/discovery/runs", viewP, h.ListRuns)
	r.GET("/discovery/runs/:id", viewP, h.GetRun)
	r.POST("/discovery/runs/:id/promote", runP, h.Promote)
}

// createRunRequest is the wire shape of POST /discovery/runs.
// The handler decodes it into a StartRunInput for the service.
type createRunRequest struct {
	CIDR  string `json:"cidr"`
	Ports []int  `json:"ports"`
	SNMP  bool   `json:"snmp"`
}

// CreateRun handles POST /discovery/runs. A 201 is returned
// with the freshly-stored run. The spec's "Trigger network
// scan" scenario.
func (h *Handler) CreateRun(c *gin.Context) {
	var req createRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	run, svcErr := h.svc.StartRun(c.Request.Context(), StartRunInput{
		CIDR:  req.CIDR,
		Ports: req.Ports,
		SNMP:  req.SNMP,
	})
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteCreated(c.Writer, run)
}

// ListRuns handles GET /discovery/runs. The response is the
// standard ListResponse envelope; limit/offset come from the
// query string.
func (h *Handler) ListRuns(c *gin.Context) {
	filter := ListFilter{Limit: 20}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "limit must be an integer",
			})
			return
		}
		filter.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "offset must be an integer",
			})
			return
		}
		filter.Offset = n
	}
	rows, total, svcErr := h.svc.ListRuns(filter)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
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

// GetRun handles GET /discovery/runs/:id. The response is a
// RunDetail (run + hosts) per the spec's "Get scan status"
// requirement.
func (h *Handler) GetRun(c *gin.Context) {
	id := c.Param("id")
	detail, svcErr := h.svc.GetRunWithHosts(id)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, detail)
}

// promoteRequest is the wire shape of POST /runs/:id/promote.
// HostIDs is optional; an empty list promotes every host.
type promoteRequest struct {
	HostIDs []string `json:"host_ids"`
}

// promoteResponse is the wire shape of the promote response.
// It is a thin envelope so we can return the list of devices
// in a single field.
type promoteResponse struct {
	Devices []any `json:"devices"`
}

// Promote handles POST /discovery/runs/:id/promote. Empty
// body = promote all hosts; {host_ids: [...]} = promote
// only the listed hosts (idempotent).
func (h *Handler) Promote(c *gin.Context) {
	id := c.Param("id")
	var req promoteRequest
	// A missing body is allowed — promotes every host.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "request body must be JSON",
			})
			return
		}
	}
	devices, svcErr := h.svc.PromoteHosts(c.Request.Context(), PromoteInput{
		RunID:   id,
		HostIDs: req.HostIDs,
	})
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	// Marshal the device slice through a generic slice so
	// the JSON output does not carry the device pkg's GORM
	// tags directly (which would expose internal fields).
	out := make([]any, len(devices))
	for i, d := range devices {
		out[i] = d
	}
	handler.WriteJSON(c.Writer, http.StatusOK, promoteResponse{Devices: out})
}

// writeAPIError is a small adapter so we can pass an
// `error` returned from the service directly to the handler's
// WriteError, which expects a *contracts.APIError.
