package physicalhost

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Type aliases for the cross-module audit types so the
// handler / service signatures read as physicalhost-typed
// without an audit import scattered across the file.
type (
	AuditService    = audit.Service
	AuditRepo       = audit.Repository
	AuditFilter     = audit.AuditFilter
	AuditResourceType = audit.AuditResourceType
)

const (
	AuditResourcePhysicalHost = audit.ResourcePhysicalHost
)

// HandlerConfig bundles the dependencies of Handler. The handler
// is the only place that depends on Gin; the service layer is
// framework-agnostic.
//
// Layering exception (documented per the v0.2.0.0
// architecture audit, item 2): 8 sites in the handler
// reach h.repo / h.auditRepo directly for list-with-join
// reads (List, Get, Create, Update, Delete, the metrics
// path, and the maintenance-history path). A full refactor
// would introduce a physicalhost.Service that hides the
// joins; tracked as a P2 follow-up. The pragmatic decision
// is to keep the reads in the handler: they are pure
// SQL-with-joins and a Service pass-through would be
// empty boilerplate.
type HandlerConfig struct {
	Repo        *Repository
	Monitor     *MonitorService
	Maintenance *MaintenanceService
	Metrics     *MetricsCache
	Audit       *AuditService
	AuditRepo   *AuditRepo
}

// Handler is the HTTP layer for the physical-host monitoring
// subsystem. It is intentionally thin: parse, call service,
// render. All validation, orchestration, and persistence live
// in the service / repository / prober layers.
type Handler struct {
	repo        *Repository
	monitor     *MonitorService
	maintenance *MaintenanceService
	metrics     *MetricsCache
	audit       *AuditService
	auditRepo   *AuditRepo
}

// NewHandler builds a Handler. A nil Metrics cache is tolerated;
// the metrics route returns 503 in that case so misconfigured
// deployments fail loudly. Audit fields are similarly
// optional — only the maintenance-history route needs them.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		repo:        cfg.Repo,
		monitor:     cfg.Monitor,
		maintenance: cfg.Maintenance,
		metrics:     cfg.Metrics,
		audit:       cfg.Audit,
		auditRepo:   cfg.AuditRepo,
	}
}

// Register attaches the physical-host routes to the supplied
// router group. The group is expected to live under /api/v1;
// the handler is agnostic about the surrounding stack.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewPhysicalHosts)
	writeP := perms(rbac.PermissionWritePhysicalHosts)
	probeP := perms(rbac.PermissionProbePhysicalHost)
	maintP := perms(rbac.PermissionMaintenancePhysical)
	auditP := perms(rbac.PermissionViewAuditLog)
	r.GET("/physical-hosts", viewP, h.List)
	r.POST("/physical-hosts", writeP, h.Create)
	r.GET("/physical-hosts/:id", viewP, h.Get)
	r.PUT("/physical-hosts/:id", writeP, h.Replace)
	r.DELETE("/physical-hosts/:id", writeP, h.Delete)
	r.POST("/physical-hosts/:id/probe", probeP, h.Probe)
	r.GET("/physical-hosts/:id/metrics", viewP, h.Metrics)
	r.POST("/physical-hosts/:id/maintenance", maintP, h.EnterMaintenance)
	r.POST("/physical-hosts/:id/maintenance/exit", maintP, h.ExitMaintenance)
	r.GET("/physical-hosts/:id/maintenance-history", auditP, h.MaintenanceHistory)
}

// hostRequest is the wire shape for POST/PUT /physical-hosts.
// It is intentionally flat; the handler maps it onto a
// PhysicalHost so the service stays free of Gin / JSON tags.
type hostRequest struct {
	DeviceID  string `json:"device_id"`
	IPAddress string `json:"ip_address"`
	SSHPort   int    `json:"ssh_port"`
	SSHUser   string `json:"ssh_user"`
	State     string `json:"state"`
}

func (r hostRequest) validate() *contracts.APIError {
	if strings.TrimSpace(r.DeviceID) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "device_id is required"}
	}
	if strings.TrimSpace(r.IPAddress) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "ip_address is required"}
	}
	if strings.TrimSpace(r.SSHUser) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "ssh_user is required"}
	}
	if r.SSHPort < 0 || r.SSHPort > 65535 {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "ssh_port must be 0-65535"}
	}
	if r.State != "" {
		if !PhysicalHostState(r.State).Valid() {
			return &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "state is not a recognised physical-host state",
			}
		}
	}
	return nil
}

func (r hostRequest) toModel() *PhysicalHost {
	p := &PhysicalHost{
		DeviceID:  strings.TrimSpace(r.DeviceID),
		IPAddress: strings.TrimSpace(r.IPAddress),
		SSHPort:   r.SSHPort,
		SSHUser:   strings.TrimSpace(r.SSHUser),
	}
	if p.SSHPort == 0 {
		p.SSHPort = 22
	}
	if r.State != "" {
		p.State = PhysicalHostState(r.State)
	}
	return p
}

// maintenanceRequest is the wire shape for POST
// /physical-hosts/:id/maintenance. Reason is required; the
// caller is the LDAP user, captured by the handler from the
// JWT (wired in a later phase — see the auth/rbac spec).
type maintenanceRequest struct {
	Reason string `json:"reason"`
}

// List handles GET /physical-hosts. State and device_id query
// params narrow the result; limit/offset provide pagination.
func (h *Handler) List(c *gin.Context) {
	filter := ListFilter{
		DeviceID: c.Query("device_id"),
	}
	if v := c.Query("state"); v != "" {
		st := PhysicalHostState(v)
		if !st.Valid() {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "state filter is not a recognised physical-host state",
			})
			return
		}
		filter.State = st
	}
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
	} else {
		filter.Limit = 20
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

	rows, total, err := h.repo.ListWithDevice(filter)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
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

// Get handles GET /physical-hosts/:id.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	p, err := h.repo.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// Create handles POST /physical-hosts. 201 is returned with
// the freshly-stored host.
func (h *Handler) Create(c *gin.Context) {
	var req hostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	if apiErr := req.validate(); apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	p := req.toModel()
	if err := h.repo.Create(p); err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, p.DeviceID))
		return
	}
	handler.WriteCreated(c.Writer, p)
}

// Replace handles PUT /physical-hosts/:id. Treated as a partial
// update so the client can patch a subset of fields.
func (h *Handler) Replace(c *gin.Context) {
	id := c.Param("id")
	var req hostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	if apiErr := req.validate(); apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}

	p, err := h.repo.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	if req.DeviceID != "" {
		p.DeviceID = strings.TrimSpace(req.DeviceID)
	}
	if req.IPAddress != "" {
		p.IPAddress = strings.TrimSpace(req.IPAddress)
	}
	if req.SSHPort > 0 {
		p.SSHPort = req.SSHPort
	}
	if req.SSHUser != "" {
		p.SSHUser = strings.TrimSpace(req.SSHUser)
	}
	if req.State != "" {
		p.State = PhysicalHostState(req.State)
	}
	if err := h.repo.Update(p); err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// Delete handles DELETE /physical-hosts/:id. 204 on success.
// Per the spec, deleting a host also stops monitoring; the
// monitor loop queries by ID and a missing row is treated as
// "not present, not probed", so the loop naturally drops the
// host without an explicit stop call.
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.repo.Delete(id); err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	handler.WriteNoContent(c.Writer)
}

// Probe handles POST /physical-hosts/:id/probe. The handler
// kicks the monitor's Check() and returns the post-probe
// record. The "register host returns state=online" scenario
// is exercised through the Create path; the manual probe is
// the operator-initiated health check.
func (h *Handler) Probe(c *gin.Context) {
	id := c.Param("id")
	if err := h.monitor.Check(c.Request.Context(), id); err != nil {
		handler.WriteAPIError(c.Writer, mapMonitorError(err, id))
		return
	}
	p, err := h.repo.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// EnterMaintenance handles POST /physical-hosts/:id/maintenance.
// "user_id" is the LDAP user (wired through the auth middleware
// in a later phase); for now the test surface passes it
// directly via the X-User header.
func (h *Handler) EnterMaintenance(c *gin.Context) {
	id := c.Param("id")
	var req maintenanceRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "request body must be JSON",
			})
			return
		}
	}
	if strings.TrimSpace(req.Reason) == "" {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "reason is required",
		})
		return
	}
	userID := c.GetHeader("X-User")
	if userID == "" {
		userID = "system"
	}
	host, _, err := h.maintenance.EnterMaintenance(contextFor(c), id, req.Reason, userID)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, host)
}

// ExitMaintenance handles POST /physical-hosts/:id/maintenance/exit.
func (h *Handler) ExitMaintenance(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User")
	if userID == "" {
		userID = "system"
	}
	host, _, err := h.maintenance.ExitMaintenance(contextFor(c), id, userID)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, host)
}

// Metrics handles GET /physical-hosts/:id/metrics. The cache
// provides the stale / unavailable semantics; the handler
// just resolves the host row, calls the cache, and renders.
func (h *Handler) Metrics(c *gin.Context) {
	id := c.Param("id")
	host, err := h.repo.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, mapRepoError(err, id))
		return
	}
	if h.metrics == nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "metrics cache not configured",
		})
		return
	}
	m, _ := h.metrics.GetOrCollect(c.Request.Context(), host.ID, Host{
		IPAddress: host.IPAddress,
		SSHPort:   host.SSHPort,
		SSHUser:   host.SSHUser,
	})
	handler.WriteJSON(c.Writer, http.StatusOK, m)
}

// MaintenanceHistory handles GET
// /physical-hosts/:id/maintenance-history. It returns every
// maintenance_enter / maintenance_exit audit row for the host
// in descending occurred_at order, so the UI can render the
// "last 5 maintenance windows" timeline.
func (h *Handler) MaintenanceHistory(c *gin.Context) {
	id := c.Param("id")
	if h.auditRepo == nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "audit repository not configured",
		})
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, _, err := h.auditRepo.List(AuditFilter{
		ResourceType: AuditResourcePhysicalHost,
		ResourceID:   id,
		Limit:        limit,
	})
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	page := contracts.Pagination{
		Total:   int64(len(rows)),
		Limit:   limit,
		Offset:  0,
		HasMore: false,
	}
	handler.WriteList(c.Writer, rows, &page)
}

// contextFor returns the request's context. Wrapped in a
// helper so the test surface can swap it out in a later phase
// (tracing / deadlines from upstream middleware).
func contextFor(c *gin.Context) context.Context {
	return c.Request.Context()
}

// writeAPIError is the same adapter the device package uses: it
// turns an `error` returned from the service into a
// *contracts.APIError, then delegates to handler.WriteError.

// mapRepoError converts a repository-layer error into a
// *contracts.APIError so the handler can render the right HTTP
// status. The repository only knows two things: "row missing"
// and "everything else". The mapping belongs here, at the
// boundary, so the repository stays free of contracts.
func mapRepoError(err error, id string) *contracts.APIError {
	if IsNotFound(err) {
		return &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: "physical host " + id + " not found",
		}
	}
	return &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: "physical host repository error",
		Cause:   err,
	}
}

// mapMonitorError converts a monitor-layer error into a
// *contracts.APIError. The monitor returns either a typed
// ErrNotFound (host deleted) or an APIError already; this
// helper keeps the handler symmetric with mapRepoError.
func mapMonitorError(err error, id string) *contracts.APIError {
	if IsNotFound(err) {
		return &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: "physical host " + id + " not found",
		}
	}
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: "physical host monitor error",
		Cause:   err,
	}
}
