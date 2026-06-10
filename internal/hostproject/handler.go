package hostproject

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

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
func (h *Handler) Register(group *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewDevices)
	writeP := perms(rbac.PermissionManageHostProjectLinks)
	d := group.Group("/devices/:id/projects")
	{
		d.GET("", viewP, h.listDeviceProjects)
		d.POST("", writeP, h.linkDeviceProject)
		d.POST("/bulk", writeP, h.bulkLinkDeviceProjects)
		d.DELETE("/:project_id", writeP, h.unlinkDeviceProject)
	}
	group.GET("/projects/:id/devices", viewP, h.listProjectDevices)
}

// linkRequest is the wire shape for POST
// /devices/:id/projects. LinkedBy is required so audit
// attribution is complete.
type linkRequest struct {
	ProjectID string `json:"project_id"`
	LinkedBy  string `json:"linked_by"`
}

// bulkLinkRequest is the wire shape for POST
// /devices/:id/projects/bulk. The order of IDs is
// preserved in the response so the UI can render the
// newly-added links in submission order.
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
		h.writeAPIError(c, err)
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
	link, err := h.svc.Link(c.Param("id"), in.ProjectID, in.LinkedBy)
	if err != nil {
		h.writeAPIError(c, err)
		return
	}
	handler.WriteCreated(c.Writer, link)
}

// unlinkDeviceProject handles DELETE
// /devices/:id/projects/:project_id. 204 on success;
// 404 if the link is missing.
func (h *Handler) unlinkDeviceProject(c *gin.Context) {
	if err := h.svc.Unlink(c.Param("id"), c.Param("project_id")); err != nil {
		h.writeAPIError(c, err)
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
	links, err := h.svc.BulkLink(c.Param("id"), in.ProjectIDs, in.LinkedBy)
	if err != nil {
		h.writeAPIError(c, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusCreated, contracts.ListResponse{Data: links})
}

// listProjectDevices handles GET /projects/:id/devices.
// Walks the project hierarchy so a link to a
// BusinessLine shows up in every sub-project's view.
func (h *Handler) listProjectDevices(c *gin.Context) {
	rows, err := h.svc.ListDeviceDetailsByProject(c.Param("id"))
	if err != nil {
		h.writeAPIError(c, err)
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
		h.writeAPIError(c, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body is required",
		})
		return false
	}
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			h.writeAPIError(c, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "request body is empty",
			})
			return false
		}
		h.writeAPIError(c, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "invalid request body: " + err.Error(),
		})
		return false
	}
	return true
}

// writeAPIError centralises the error render. It also
// unwraps generic errors to a 500 envelope so the
// handler never panics.
func (h *Handler) writeAPIError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var ae *contracts.APIError
	if !errors.As(err, &ae) {
		ae = &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: err.Error(),
			Cause:   err,
		}
	}
	handler.WriteError(c.Writer, ae)
}
