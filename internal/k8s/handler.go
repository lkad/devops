package k8s

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the k8s cluster management
// subsystem. It is intentionally thin: parse, call service,
// render. All validation, encryption, and orchestration live
// in the service / repository.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The Service is the only
// dependency; future middleware (RBAC, audit logging) can be
// injected here without changing the service contract.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches the k8s routes to the supplied router
// group. The group is expected to live under /api/v1; the
// handler is agnostic about the surrounding stack.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/k8s/clusters", h.List)
	r.POST("/k8s/clusters", h.Create)
	r.GET("/k8s/clusters/:id", h.Get)
	r.PUT("/k8s/clusters/:id", h.Replace)
	r.DELETE("/k8s/clusters/:id", h.Delete)
	r.POST("/k8s/clusters/:id/probe", h.Probe)
	r.GET("/k8s/clusters/:id/pods", h.ListPods)
	r.GET("/k8s/clusters/:id/deployments", h.ListDeployments)
	r.GET("/k8s/clusters/:id/services", h.ListServices)
	r.POST("/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec", h.Exec)
}

// clusterRequest is the wire shape for POST/PUT /k8s/clusters.
// We keep it as a flat struct and map it onto the service's
// input types inside the handler so the service stays free of
// Gin / JSON tags.
type clusterRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	APIServer  string `json:"api_server"`
	Kubeconfig string `json:"kubeconfig"`
	InCluster  bool   `json:"in_cluster"`
}

func (r clusterRequest) toCreate() CreateClusterInput {
	return CreateClusterInput{
		Name:       r.Name,
		Type:       ClusterType(r.Type),
		APIServer:  r.APIServer,
		Kubeconfig: r.Kubeconfig,
		InCluster:  r.InCluster,
	}
}

func (r clusterRequest) toUpdate() UpdateClusterInput {
	in := UpdateClusterInput{}
	if r.Name != "" {
		n := r.Name
		in.Name = &n
	}
	if r.Type != "" {
		ct := ClusterType(r.Type)
		in.Type = &ct
	}
	if r.APIServer != "" {
		a := r.APIServer
		in.APIServer = &a
	}
	if r.Kubeconfig != "" {
		k := r.Kubeconfig
		in.Kubeconfig = &k
	}
	in.InCluster = &r.InCluster
	return in
}

// List handles GET /k8s/clusters.
func (h *Handler) List(c *gin.Context) {
	filter := ListFilter{}
	if v := c.Query("type"); v != "" {
		filter.Type = ClusterType(v)
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
		HasMore: false,
	}
	handler.WriteList(c.Writer, rows, &page)
}

// Get handles GET /k8s/clusters/:id.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.svc.Get(id)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, cl)
}

// Create handles POST /k8s/clusters.
func (h *Handler) Create(c *gin.Context) {
	var req clusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	cl, err := h.svc.Create(req.toCreate())
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, cl)
}

// Replace handles PUT /k8s/clusters/:id. Treated as a partial
// update so the client can patch a subset of fields.
func (h *Handler) Replace(c *gin.Context) {
	id := c.Param("id")
	var req clusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	cl, err := h.svc.Update(id, req.toUpdate())
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, cl)
}

// Delete handles DELETE /k8s/clusters/:id.
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}

// Probe handles POST /k8s/clusters/:id/probe. Returns 200
// with the ProbeResult on success; on connectivity failure,
// the body still includes a ProbeResult with the
// disconnected status and a non-2xx HTTP code is set.
func (h *Handler) Probe(c *gin.Context) {
	id := c.Param("id")
	res, err := h.svc.Probe(id)
	if err != nil {
		// Render both the error envelope AND the ProbeResult
		// body so the client can see the connectivity details
		// even when the API reports a failure.
		if res != nil {
			handler.WriteJSON(c.Writer, http.StatusBadGateway, res)
			return
		}
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, res)
}

// ListPods handles GET /k8s/clusters/:id/pods?namespace=...
func (h *Handler) ListPods(c *gin.Context) {
	id := c.Param("id")
	ns := c.Query("namespace")
	pods, err := h.svc.ListPods(id, ns)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, pods, nil)
}

// ListDeployments handles GET /k8s/clusters/:id/deployments?namespace=...
func (h *Handler) ListDeployments(c *gin.Context) {
	id := c.Param("id")
	ns := c.Query("namespace")
	deps, err := h.svc.ListDeployments(id, ns)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, deps, nil)
}

// ListServices handles GET /k8s/clusters/:id/services?namespace=...
func (h *Handler) ListServices(c *gin.Context) {
	id := c.Param("id")
	ns := c.Query("namespace")
	svcs, err := h.svc.ListServices(id, ns)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, svcs, nil)
}

// execRequest is the wire shape of POST .../pods/:pod/exec.
type execRequest struct {
	Command []string `json:"command"`
}

// Exec handles POST /k8s/clusters/:id/namespaces/:ns/pods/:pod/exec.
// Gated behind the feature_k8s_exec flag.
func (h *Handler) Exec(c *gin.Context) {
	id := c.Param("id")
	ns := c.Param("ns")
	pod := c.Param("pod")
	var req execRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON with a 'command' array",
		})
		return
	}
	res, err := h.svc.Exec(id, ns, pod, req.Command)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, res)
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
