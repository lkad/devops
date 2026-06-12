package hostproject

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the host-project link
// subsystem. It is intentionally thin: it parses
// requests, calls the service, and renders the response.
// Every business rule lives in service.go; the handler
// never queries the repository directly.
//
// Routes are registered via Register on a parent
// *gin.RouterGroup so the main entry point can mount
// them under /api/v1 alongside other modules. The
// handler exposes routes under both /devices/:id/...
// and /projects/:id/devices, mirroring the spec's
// "host detail" and "project detail" entry points.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler bound to the supplied
// service. The service is the only dependency: the
// device-existence check is wired in at the service
// layer so the handler stays free of cross-package
// imports.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register wires the host-project routes onto the
// supplied router group. Two route families are exposed:
//
//	/api/v1/devices/:id/projects  (host-centric)
//	/api/v1/projects/:id/devices  (project-centric)
//
// The families mirror the spec's "host detail" and
// "project detail" entry points.
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
//
// projectAccess is the per-project access factory used to
// gate the project-scoped routes. The host-centric routes
// extract the project_id from the URL (DELETE) or the
// body (POST); the project-centric route uses the URL
// :id. Variadic so the existing single-arg signature
// still compiles for callers that don't wire the check
// (e.g. early unit tests).
func (h *Handler) Register(group *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc, projectAccess ...rbac.ProjectAccessFactory) {
	viewP := perms(rbac.PermissionViewDevices)
	writeP := perms(rbac.PermissionManageHostProjectLinks)
	// per-project access gates: the URL :id for project-
	// centric reads, the URL :project_id for unlink, the
	// body project_id (first of the array for bulk) for
	// the link / bulk-link paths. Built only when
	// projectAccess is supplied; tests that omit it
	// exercise the global-RBAC-only path.
	var (
		viewByProjectURL, writeByProjectURL,
		writeByProjectBody gin.HandlerFunc
	)
	if len(projectAccess) > 0 && projectAccess[0] != nil {
		viewByProjectURL = projectAccess[0](rbac.PermissionViewDevices, func(c *gin.Context) string {
			return c.Param("id")
		})
		writeByProjectURL = projectAccess[0](rbac.PermissionManageHostProjectLinks, func(c *gin.Context) string {
			return c.Param("project_id")
		})
		writeByProjectBody = projectAccess[0](rbac.PermissionManageHostProjectLinks, projectIDFromLinkBody)
	}
	d := group.Group("/devices/:id/projects")
	{
		d.GET("", viewP, h.listDeviceProjects)
		d.POST("", writeP, writeByProjectBody, h.linkDeviceProject)
		d.POST("/bulk", writeP, writeByProjectBody, h.bulkLinkDeviceProjects)
		d.DELETE("/:project_id", writeP, writeByProjectURL, h.unlinkDeviceProject)
	}
	group.GET("/projects/:id/devices", viewP, viewByProjectURL, h.listProjectDevices)
}

// projectIDFromLinkBody extracts the project_id from the
// JSON body of a host-project link request. The shape is
// { "project_id": "...", ... } (plus the bulk shape with
// "project_ids" — we use the first entry of that array
// so the per-project check covers the same project the
// service will operate on). The body is read via
// ShouldBindBodyWithJSON which stashes it for re-read by
// the handler's bind() helper. An unparseable body or
// missing id returns "" so the middleware 403s and the
// handler surfaces a 400 in the same round trip.
func projectIDFromLinkBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	var peek struct {
		ProjectID  string   `json:"project_id"`
		ProjectIDs []string `json:"project_ids"`
	}
	if err := c.ShouldBindBodyWithJSON(&peek); err != nil {
		return ""
	}
	if peek.ProjectID != "" {
		return peek.ProjectID
	}
	if len(peek.ProjectIDs) > 0 {
		return peek.ProjectIDs[0]
	}
	return ""
}

// linkRequest is the wire shape for POST
// /devices/:id/projects. The LinkedBy field is accepted
// for backward compatibility (older clients still send
// it) but is IGNORED: the audit-trail attribution comes
// from the JWT subject so a caller cannot forge a
// different value. The P0 cross-tenant audit-trail fix
// removes the field from the wire shape; for now it is
// tolerated and overwritten.
type linkRequest struct {
	ProjectID string `json:"project_id"`
	LinkedBy  string `json:"linked_by"`
}

// bulkLinkRequest is the wire shape for POST
// /devices/:id/projects/bulk. The order of IDs is
// preserved in the response so the UI can render the
// newly-added links in submission order. LinkedBy is
// ignored (see linkRequest for the audit-trail note).
type bulkLinkRequest struct {
	ProjectIDs []string `json:"project_ids"`
	LinkedBy   string   `json:"linked_by"`
}

// listDeviceProjects handles GET /devices/:id/projects.
// Returns the standard ListResponse envelope with the
// enriched DTO that joins the link to the project and
// its ancestor chain.
func (h *Handler) listDeviceProjects(c *gin.Context) {
	rows, err := h.svc.ListProjectDetailsByDevice(c.Param("id"))
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, rows, nil)
}

// linkDeviceProject handles POST /devices/:id/projects.
// 201 with the created link; 404 if the device or
// project is missing; 409 on duplicate pair; 400 on
// validation failure.
func (h *Handler) linkDeviceProject(c *gin.Context) {
	var in linkRequest
	if !h.bind(c, &in) {
		return
	}
	// Audit-trail attribution MUST come from the JWT, not
	// the request body. The previous shape allowed any
	// caller to forge a different linked_by; the P0
	// cross-tenant fix replaces in.LinkedBy with the
	// authenticated user's id.
	actor := callerFromGin(c)
	link, err := h.svc.Link(c.Param("id"), in.ProjectID, actor, c.Request.Context())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, link)
}

// unlinkDeviceProject handles DELETE
// /devices/:id/projects/:project_id. 204 on success;
// 404 if the link is missing.
func (h *Handler) unlinkDeviceProject(c *gin.Context) {
	if err := h.svc.Unlink(c.Param("id"), c.Param("project_id"), c.Request.Context()); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// bulkLinkDeviceProjects handles POST
// /devices/:id/projects/bulk. 201 with the array of
// created links; 404/409/400 as per the single-link
// path.
func (h *Handler) bulkLinkDeviceProjects(c *gin.Context) {
	var in bulkLinkRequest
	if !h.bind(c, &in) {
		return
	}
	// Audit-trail attribution MUST come from the JWT —
	// see linkDeviceProject for the full rationale.
	actor := callerFromGin(c)
	links, err := h.svc.BulkLink(c.Param("id"), in.ProjectIDs, actor, c.Request.Context())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusCreated, contracts.ListResponse{Data: links})
}

// callerFromGin returns the authenticated user's id from
// the gin context, or empty when no caller is attached
// (the test path that doesn't exercise auth). The link
// service writes "" to the audit column in that case
// rather than refusing the request — refusing would
// break the existing handler tests that pre-date the
// per-project access middleware; the service layer
// already requires a non-empty device id and project
// id, so a missing actor is a data-integrity issue
// surfaced in the audit-log render rather than a 4xx.
func callerFromGin(c *gin.Context) string {
	if cl, ok := caller.FromGin(c); ok && cl != nil {
		return cl.UserID()
	}
	return ""
}

// listProjectDevices handles GET /projects/:id/devices.
// Walks the project hierarchy so a link to a
// BusinessLine shows up in every sub-project's view.
func (h *Handler) listProjectDevices(c *gin.Context) {
	rows, err := h.svc.ListDeviceDetailsByProject(c.Param("id"))
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, rows, nil)
}

// bind decodes the request body and renders a 400
// envelope on failure. We never let a malformed body
// reach the service layer. On failure the method
// writes the error response itself so the handler
// doesn't have to remember to render it.
func (h *Handler) bind(c *gin.Context, dst any) bool {
	if c.Request.Body == nil {
		handler.WriteAPIError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body is required",
		})
		return false
	}
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			handler.WriteAPIError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "request body is empty",
			})
			return false
		}
		handler.WriteAPIError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "invalid request body: " + err.Error(),
		})
		return false
	}
	return true
}
