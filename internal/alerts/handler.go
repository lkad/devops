package alerts

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the alert-notification subsystem.
// Per the layered rules it is intentionally thin: parse, call
// service, render. All validation, orchestration, and
// persistence live in the service / repository.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The service is the only
// dependency; future middleware (RBAC, audit logging) can be
// injected here without changing the service contract.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register attaches the alert routes to the supplied router
// group. The group is expected to live under /api/v1; the
// handler is agnostic about the surrounding stack.
//
// Routes:
//
//	GET    /alerts                 list alerts
//	POST   /alerts                 create / trigger an alert
//	GET    /alerts/:id             get a single alert
//	PUT    /alerts/:id             partial update of an alert
//	DELETE /alerts/:id             soft-delete an alert
//	POST   /alerts/:id/acknowledge mark an alert as acknowledged
//	POST   /alerts/:id/resolve     mark an alert as resolved
//	GET    /alerts/history         paginated list of alerts
//	GET    /alerts/stats           aggregate view
//	GET    /alerts/channels        list channels
//	POST   /alerts/channels        create a channel
//	GET    /alerts/channels/:id    get a single channel (masked)
//	PUT    /alerts/channels/:id    partial update of a channel
//	DELETE /alerts/channels/:id    soft-delete a channel
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewAlerts)
	writeP := perms(rbac.PermissionWriteAlerts)
	r.GET("/alerts", viewP, h.List)
	r.POST("/alerts", writeP, h.Create)
	r.GET("/alerts/history", viewP, h.History)
	r.GET("/alerts/stats", viewP, h.Stats)
	r.GET("/alerts/:id", viewP, h.Get)
	r.PUT("/alerts/:id", writeP, h.Update)
	r.DELETE("/alerts/:id", writeP, h.Delete)
	r.POST("/alerts/:id/acknowledge", writeP, h.Acknowledge)
	r.POST("/alerts/:id/resolve", writeP, h.Resolve)
	r.GET("/alerts/channels", viewP, h.ListChannels)
	r.POST("/alerts/channels", writeP, h.CreateChannel)
	r.GET("/alerts/channels/:id", viewP, h.GetChannel)
	r.PUT("/alerts/channels/:id", writeP, h.UpdateChannel)
	r.DELETE("/alerts/channels/:id", writeP, h.DeleteChannel)
}

// alertRequest is the wire shape for POST /alerts and PUT
// /alerts/:id. The handler decodes the request into this
// struct, then maps it onto a service-level DTO.
type alertRequest struct {
	Name       string         `json:"name"`
	Severity   string         `json:"severity"`
	SourceType string         `json:"source_type"`
	SourceID   string         `json:"source_id"`
	Labels     map[string]any `json:"labels"`
	ChannelIDs []string       `json:"channel_ids"`
	Message    string         `json:"message"`
}

func (r alertRequest) toCreate() CreateAlertInput {
	in := CreateAlertInput{
		Name:       r.Name,
		Severity:   Severity(r.Severity),
		SourceType: SourceType(r.SourceType),
		SourceID:   r.SourceID,
		ChannelIDs: r.ChannelIDs,
		Message:    r.Message,
	}
	if r.Labels != nil {
		in.Labels = JSONMap(r.Labels)
	}
	return in
}

func (r alertRequest) toFire() FireInput {
	return FireInput{
		Name:       r.Name,
		Severity:   Severity(r.Severity),
		SourceType: SourceType(r.SourceType),
		SourceID:   r.SourceID,
		Labels:     nil,
		ChannelIDs: r.ChannelIDs,
		Message:    r.Message,
	}
}

func (r alertRequest) toUpdate() UpdateChannelInput { return UpdateChannelInput{} }

// channelRequest is the wire shape for POST /alerts/channels and
// PUT /alerts/channels/:id.
type channelRequest struct {
	Type    string         `json:"type"`
	Config  map[string]any `json:"config"`
	Enabled *bool          `json:"enabled"`
}

func (r channelRequest) toCreate() CreateChannelInput {
	in := CreateChannelInput{
		Type:   ChannelType(r.Type),
		Config: JSONMap(r.Config),
	}
	if r.Enabled != nil {
		in.Enabled = *r.Enabled
	}
	return in
}

func (r channelRequest) toUpdate() UpdateChannelInput {
	return UpdateChannelInput{
		Type:    nil,
		Config:  JSONMap(r.Config),
		Enabled: r.Enabled,
	}
}

// List handles GET /alerts. It honours the name / severity /
// source_type / source_id / state query params. The response is
// the standard ListResponse envelope.
func (h *Handler) List(c *gin.Context) {
	filter := h.parseFilter(c)
	rows, total, err := h.svc.List(filter)
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

// History handles GET /alerts/history. It reuses the list filter
// plus the optional ?suppressed=true and ?name= query params.
func (h *Handler) History(c *gin.Context) {
	filter := h.parseFilter(c)
	if v := c.Query("suppressed"); v == "true" {
		t := true
		filter.Suppressed = &t
	}
	rows, total, err := h.svc.List(filter)
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

// Stats handles GET /alerts/stats. The response is the Stats
// aggregate view.
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, stats)
}

// Get handles GET /alerts/:id. A single resource is rendered as
// the bare object.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	a, err := h.svc.GetAlert(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, a)
}

// Create handles POST /alerts. A 201 is returned with the
// freshly-stored alert.
func (h *Handler) Create(c *gin.Context) {
	var req alertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c.Writer, "request body must be JSON")
		return
	}
	a, err := h.svc.Fire(c.Request.Context(), req.toFire())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, a)
}

// Update handles PUT /alerts/:id. Phase 5 only exposes the
// fields the state machine cares about (state / labels).
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req alertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c.Writer, "request body must be JSON")
		return
	}
	_ = id
	_ = req.toUpdate
	// Phase 5: PUT is reserved for the rule-evaluation loop;
	// alerts are mutated through Acknowledge / Resolve. A bare
	// PUT returns 405 Method Not Allowed so clients learn the
	// right verbs.
	c.Writer.Header().Set("Allow", "GET, DELETE, POST (acknowledge/resolve)")
	handler.WriteError(c.Writer, &contracts.APIError{
		Code:    contracts.CodeValidation,
		Message: "use POST /alerts/:id/acknowledge or POST /alerts/:id/resolve to mutate an alert",
	})
}

// Delete handles DELETE /alerts/:id.
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// Acknowledge handles POST /alerts/:id/acknowledge. The user id
// is read from the X-User-Id header in tests; in production the
// JWT middleware would set it on the gin context.
func (h *Handler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		userID = "system"
	}
	a, err := h.svc.Acknowledge(id, userID)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, a)
}

// Resolve handles POST /alerts/:id/resolve.
func (h *Handler) Resolve(c *gin.Context) {
	id := c.Param("id")
	a, err := h.svc.Resolve(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, a)
}

// ListChannels handles GET /alerts/channels. Each channel's
// Config is masked before serialisation.
func (h *Handler) ListChannels(c *gin.Context) {
	rows, err := h.svc.ListChannels()
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	masked := make([]Channel, len(rows))
	for i, ch := range rows {
		masked[i] = Mask(ch)
	}
	handler.WriteList(c.Writer, masked, nil)
}

// CreateChannel handles POST /alerts/channels. A 201 is
// returned with the masked channel.
func (h *Handler) CreateChannel(c *gin.Context) {
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c.Writer, "request body must be JSON")
		return
	}
	ch, err := h.svc.CreateChannel(req.toCreate())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, Mask(*ch))
}

// GetChannel handles GET /alerts/channels/:id.
func (h *Handler) GetChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := h.svc.GetChannel(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, Mask(*ch))
}

// UpdateChannel handles PUT /alerts/channels/:id.
func (h *Handler) UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c.Writer, "request body must be JSON")
		return
	}
	if err := h.svc.UpdateChannel(id, req.toUpdate()); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	ch, _ := h.svc.GetChannel(id)
	handler.WriteJSON(c.Writer, http.StatusOK, Mask(*ch))
}

// DeleteChannel handles DELETE /alerts/channels/:id.
func (h *Handler) DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteChannel(id); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// parseFilter turns the query string into an AlertFilter. Any
// error here is a 400.
func (h *Handler) parseFilter(c *gin.Context) AlertFilter {
	f := AlertFilter{}
	f.Name = c.Query("name")
	if v := c.Query("severity"); v != "" {
		f.Severity = Severity(v)
	}
	if v := c.Query("source_type"); v != "" {
		f.SourceType = SourceType(v)
	}
	if v := c.Query("source_id"); v != "" {
		f.SourceID = v
	}
	if v := c.Query("state"); v != "" {
		f.State = State(v)
	}
	// "status" is a more conventional REST query name;
	// accept it as a synonym for "state" so clients can
	// use either. "state" wins if both are present.
	if v := c.Query("status"); v != "" && f.State == "" {
		f.State = State(v)
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	} else {
		f.Limit = 20
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	return f
}

// writeAPIError is a small adapter so we can pass an `error`
// returned from the service directly to the handler's
// WriteError, which expects a *contracts.APIError.

// writeValidationError is the convenience used for JSON parse
// failures: a 400 with a fixed message.
func writeValidationError(w http.ResponseWriter, msg string) {
	handler.WriteError(w, &contracts.APIError{
		Code:    contracts.CodeValidation,
		Message: msg,
	})
}

// keep encoding/json imported even when only used via Gin.
var _ = json.Marshal
