package project

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the project-hierarchy module. It
// is intentionally thin: it parses requests, calls the service,
// and renders the response. Every business rule lives in
// service.go; the handler never queries the repository directly.
//
// Routes are registered via Register on a parent *gin.RouterGroup
// so the main entry point can mount them under /api/v1 alongside
// other modules.
type Handler struct {
	svc   *Service
	repo  *Repository
	audit *audit.Service
}

// NewHandler returns a Handler bound to the supplied service and
// repository. Both are required — the repository is consulted for
// read-only paths (children, ancestors) where the service is a
// pass-through. The audit service is optional (nil means
// "no audit emission"); production wires a real service so
// v0.2.0.0 P0 #3 audit-trail coverage holds.
func NewHandler(svc *Service, repo *Repository, auditSvc ...*audit.Service) *Handler {
	var a *audit.Service
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &Handler{svc: svc, repo: repo, audit: a}
}

// Register wires the project hierarchy routes onto the supplied
// router group. The function is idempotent in the sense that
// registering twice on the same group yields a Gin panic at
// startup, which is the desired fail-fast behaviour.
//
// perms is the per-route permission factory: it returns the
// RBAC middleware (auth is applied at the parent group) for
// the supplied permission key. Pass a no-op factory in unit
// tests that don't exercise auth.
//
// projectAccess is the per-project access factory: it returns
// the caller.RequireProjectAccess middleware pre-configured
// with the membership + permission checkers. Pass a no-op
// factory in unit tests; production wires the real checkers
// from the rbac service and the project repository.
//
//	group := r.Group("/api/v1")
//	project.NewHandler(svc, repo).Register(group, perms, projectAccess)
func (h *Handler) Register(group *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc, projectAccess ...ProjectAccessFactory) {
	viewP := perms(rbac.PermissionViewProjects)
	writeP := perms(rbac.PermissionWriteProjects)
	// projectAccess is variadic so the existing single-arg
	// signature (perms only) still compiles for any caller
	// that does not want the per-project check (e.g. early
	// unit tests). When the factory is supplied every
	// project-scoped route is gated by it.
	var projectView, projectWrite, projectMemberView, projectMemberWrite gin.HandlerFunc
	if len(projectAccess) > 0 && projectAccess[0] != nil {
		// Reuse the same factory for the URL :id routes; the
		// per-handler middleware below reads project_id from
		// either the URL param or the request body.
		view := projectAccess[0](rbac.PermissionViewProjects, projectIDFromURL)
		write := projectAccess[0](rbac.PermissionWriteProjects, projectIDFromURL)
		projectView = view
		projectWrite = write
		projectMemberView = view
		projectMemberWrite = write
	}
	pt := group.Group("/project-types")
	{
		pt.GET("", h.listTypes)
		pt.POST("", h.createType)
	}
	p := group.Group("/projects")
	{
		p.GET("", viewP, h.list)
		p.POST("", writeP, h.create)
		p.GET("/:id", viewP, projectView, h.get)
		p.PUT("/:id", writeP, projectWrite, h.update)
		p.DELETE("/:id", writeP, projectWrite, h.delete)
		p.GET("/:id/children", viewP, projectView, h.children)
		p.GET("/:id/ancestors", viewP, projectView, h.ancestors)
		p.GET("/:id/members", viewP, projectMemberView, h.listMembers)
		p.POST("/:id/members", writeP, projectMemberWrite, h.addMember)
		p.DELETE("/:id/members/:user_id", writeP, projectMemberWrite, h.removeMember)
	}
}

// ProjectAccessFactory is re-exported as a type alias to
// rbac.ProjectAccessFactory so the per-route wiring inside
// Register reads the same as the production code that calls
// Register. Production wires a real factory from
// rbac.NewProjectAccessFactory; tests pass
// rbac.NoopProjectAccessFactory or omit the variadic arg
// entirely.
type ProjectAccessFactory = rbac.ProjectAccessFactory

// projectIDFromURL is the standard project-id extractor for
// the project-hierarchy routes. Every route on /projects/:id
// uses it; the per-handler middleware reads the value via
// c.Param("id"). The signature is a func(*gin.Context) string
// so a future handler that needs the project id from the
// body can supply a different closure.
func projectIDFromURL(c *gin.Context) string {
	return c.Param("id")
}

// createTypeInput is the wire shape for POST /project-types. The
// caller supplies a Name and an optional Description/Weight.
type createTypeInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
}

// createType handles POST /project-types. Returns 201 with the
// created type, or 4xx with a contracts.ErrorResponse envelope.
func (h *Handler) createType(c *gin.Context) {
	var in createTypeInput
	if !h.bind(c, &in) {
		return
	}
	pt, err := h.svc.CreateType(ProjectType{Name: in.Name, Description: in.Description, Weight: in.Weight}, c.Request.Context())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, pt)
}

// listTypes handles GET /project-types. Returns the full list —
// the types are a small, slowly-changing vocabulary that the UI
// caches.
func (h *Handler) listTypes(c *gin.Context) {
	pts, err := h.svc.ListTypes(c.Request.Context())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, pts)
}

// createInput is the wire shape for POST /projects. Pointer
// fields preserve "not set" semantics across the JSON boundary.
type createInput struct {
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Description string  `json:"description"`
	ParentID    *string `json:"parent_id"`
	TypeID      string  `json:"type_id"`
	OwnerUserID *string `json:"owner_user_id"`
	Weight      int     `json:"weight"`
	Labels      JSONMap `json:"labels"`
	Metadata    JSONMap `json:"metadata"`
}

// create handles POST /projects.
func (h *Handler) create(c *gin.Context) {
	var in createInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc.CreateProject(CreateProjectInput{
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		ParentID:    in.ParentID,
		TypeID:      in.TypeID,
		OwnerUserID: in.OwnerUserID,
		Weight:      in.Weight,
		Labels:      in.Labels,
		Metadata:    in.Metadata,
	}, c.Request.Context())
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, p)
}

// updateInput mirrors the service's UpdateProjectInput.
type updateInput struct {
	Name        *string `json:"name"`
	ParentID    *string `json:"parent_id"`
	Description *string `json:"description"`
	OwnerUserID *string `json:"owner_user_id"`
	Weight      *int    `json:"weight"`
}

// update handles PUT /projects/:id.
func (h *Handler) update(c *gin.Context) {
	var in updateInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc.UpdateProject(c.Request.Context(), c.Param("id"), UpdateProjectInput{
		Name:        in.Name,
		ParentID:    in.ParentID,
		Description: in.Description,
		OwnerUserID: in.OwnerUserID,
		Weight:      in.Weight,
	})
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, p)
}

// delete handles DELETE /projects/:id. A non-leaf yields 422;
// the rest are 204.
func (h *Handler) delete(c *gin.Context) {
	if err := h.svc.DeleteProject(c.Request.Context(), c.Param("id")); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// get handles GET /projects/:id and returns the project plus its
// direct children and members. The shape is { project, children,
// members }; the spec's "Get Project with resource links" maps
// to this payload.
func (h *Handler) get(c *gin.Context) {
	p, kids, members, err := h.svc.GetProjectWithRelations(c.Request.Context(), c.Param("id"))
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
		"project":  p,
		"children": kids,
		"members":  members,
	})
}

// children handles GET /projects/:id/children. Returns a
// paginated list per the api-contract spec.
func (h *Handler) children(c *gin.Context) {
	page, pageSize := readPagination(c)
	kids, total, err := h.repo.List(Filter{ParentID: ptr(c.Param("id")), Limit: pageSize, Offset: (page - 1) * pageSize})
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	p := contracts.NewPagination(page, pageSize)
	p.Total = total
	p.HasMore = p.MoreAvailable()
	handler.WriteList(c.Writer, kids, &p)
}

// ancestors handles GET /projects/:id/ancestors. The shape is a
// plain array (no pagination) because the chain is bounded by
// MaxDepth.
func (h *Handler) ancestors(c *gin.Context) {
	chain, err := h.svc.ListAncestors(c.Request.Context(), c.Param("id"))
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, chain)
}

// listMembers handles GET /projects/:id/members. Returns the raw
// array — projects typically have a small member set.
func (h *Handler) listMembers(c *gin.Context) {
	members, err := h.svc.ListMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, members)
}

// addMemberInput is the wire shape for POST /projects/:id/members.
// The AddedBy field is intentionally absent from the wire
// shape: the audit-trail attribution comes from the JWT
// (the authenticated user) rather than the request body,
// which would otherwise allow any caller to forge a
// different "added_by" value. See the project audit-trail
// fix in the P0 cross-tenant work.
type addMemberInput struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// addMember handles POST /projects/:id/members. The spec's
// "Grant viewer/editor permission" maps here. A re-grant is an
// upsert — the service promotes the role in place.
func (h *Handler) addMember(c *gin.Context) {
	var in addMemberInput
	if !h.bind(c, &in) {
		return
	}
	// The AddedBy value MUST come from the JWT, never from
	// the request body — see the comment on addMemberInput.
	cl, ok := caller.FromGin(c)
	if !ok || cl == nil || cl.User == nil {
		handler.WriteAPIError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeUnauthorized,
			Message: "authentication required",
		})
		return
	}
	if err := h.svc.AssignMember(c.Request.Context(), c.Param("id"), in.UserID, in.Role, cl.User.ID); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, gin.H{
		"project_id": c.Param("id"),
		"user_id":    in.UserID,
		"role":       in.Role,
	})
}

// removeMember handles DELETE /projects/:id/members/:user_id.
func (h *Handler) removeMember(c *gin.Context) {
	if err := h.svc.RevokeMember(c.Request.Context(), c.Param("id"), c.Param("user_id")); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// list handles GET /projects with the full filter set.
func (h *Handler) list(c *gin.Context) {
	page, pageSize := readPagination(c)
	f := Filter{
		TypeID:  c.Query("type_id"),
		Search:  c.Query("search"),
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	}
	if v := c.Query("parent_id"); v != "" {
		f.ParentID = &v
	}
	if v := c.Query("owner_id"); v != "" {
		f.OwnerID = &v
	}
	if v := c.Query("depth"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			f.Depth = d
		}
	}
	items, total, err := h.svc.ListProjects(c.Request.Context(), f)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	p := contracts.NewPagination(page, pageSize)
	p.Total = total
	p.HasMore = p.MoreAvailable()
	handler.WriteList(c.Writer, items, &p)
}

// bind decodes the request body and renders a 400 envelope on
// failure. We never let a malformed body reach the service layer.
// On failure the method writes the error response itself so the
// handler doesn't have to remember to render it.
func (h *Handler) bind(c *gin.Context, dst any) bool {
	if c.Request.Body == nil {
		handler.WriteAPIError(c.Writer, &contracts.APIError{Code: contracts.CodeValidation, Message: "request body is required"})
		return false
	}
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			handler.WriteAPIError(c.Writer, &contracts.APIError{Code: contracts.CodeValidation, Message: "request body is empty"})
			return false
		}
		handler.WriteAPIError(c.Writer, &contracts.APIError{Code: contracts.CodeValidation, Message: "invalid request body: " + err.Error()})
		return false
	}
	return true
}

// readPagination parses the standard page/page_size query params
// with safe defaults. The page_size cap is enforced by
// contracts.NewPagination.
func readPagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	return page, pageSize
}

// ptr is a tiny helper that returns the address of a string. It
// exists so the handler can build the Filter struct without
// declaring a temporary variable on every call.
func ptr(s string) *string { return &s }

// emitAudit is the single seam between the project HTTP layer
// and the cross-module audit subsystem. The actor is read from
// the gin context (stamped by the auth middleware) so the audit
// trail is forgery-proof. A nil h.audit is a no-op so unit
// tests can wire the handler without an audit sink. The
// emission is best-effort: a failure to record does not roll
// back the mutation.
func (h *Handler) emitAudit(c *gin.Context, in audit.RecordActionInput) {
	if h.audit == nil {
		return
	}
	cl, _ := caller.FromGin(c)
	if in.ActorID == "" {
		in.ActorID = cl.UserID()
	}
	if in.ActorName == "" {
		in.ActorName = cl.UserID()
	}
	h.audit.RecordAction(c.Request.Context(), in)
}
