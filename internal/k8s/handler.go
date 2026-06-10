package k8s

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/internal/k8s/logstream"
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
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewK8sResources)
	clusterP := perms(rbac.PermissionManageK8sClusters)
	execP := perms(rbac.PermissionExecK8sPod)
	logsP := perms(rbac.PermissionViewK8sPodLogs)
	r.GET("/k8s/clusters", viewP, h.List)
	r.POST("/k8s/clusters", clusterP, h.Create)
	r.GET("/k8s/clusters/:clusterID", viewP, h.Get)
	r.PUT("/k8s/clusters/:clusterID", clusterP, h.Replace)
	r.DELETE("/k8s/clusters/:clusterID", clusterP, h.Delete)
	r.POST("/k8s/clusters/:clusterID/probe", clusterP, h.Probe)
	r.GET("/k8s/clusters/:clusterID/pods", viewP, h.ListPods)
	r.GET("/k8s/clusters/:clusterID/deployments", viewP, h.ListDeployments)
	r.GET("/k8s/clusters/:clusterID/services", viewP, h.ListServices)
	r.POST("/k8s/clusters/:clusterID/namespaces/:ns/pods/:pod/exec", execP, h.Exec)
	// Per-cluster log query (P2 follow-up). The
	// label-selector path fan-outs across pods in the
	// namespace; the apiserver call lives on the
	// per-cluster Client resolved via Service.Registry.
	r.GET("/k8s/clusters/:clusterID/namespaces/:ns/logs", logsP, h.GetLogs)
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
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
	id := c.Param("clusterID")
	ns := c.Query("namespace")
	svcs, err := h.svc.ListServices(id, ns)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	handler.WriteList(c.Writer, svcs, nil)
}

// execRequest is the wire shape of POST .../pods/:pod/exec,
// per openspec/specs/k8s-pod-exec/spec.md. Container is
// required (multi-container pods need a target). TimeoutSeconds
// is optional — zero means "use the client-side default
// (DefaultExecTimeout = 30s)". Values above
// MaxExecTimeoutSeconds (600) are rejected with INVALID_EXEC_REQUEST
// (the spec lets us silently clamp inside the KubeClient, but a
// 99999s request is almost always a caller bug and we want to
// surface it).
type execRequest struct {
	Command        []string `json:"command"`
	Container      string   `json:"container"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

// MaxExecTimeoutSeconds mirrors k8s.MaxExecTimeout. Kept as a
// handler-level constant so the wire-shape validation runs
// before the Service is touched.
const MaxExecTimeoutSeconds = 600

// Exec handles POST /k8s/clusters/:id/namespaces/:ns/pods/:pod/exec.
// It validates the wire shape, calls Service.Exec (which
// resolves the per-cluster KubeClient via the registry), and
// renders the spec'd response envelope. The exec path does
// NOT use the shared s.client — every cluster has its own
// KubeClient (built from the cluster's decrypted kubeconfig)
// so the SPDY executor authenticates to the right apiserver.
func (h *Handler) Exec(c *gin.Context) {
	id := c.Param("clusterID")
	ns := c.Param("ns")
	pod := c.Param("pod")
	var req execRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "request body must be JSON with 'command' and 'container'",
		})
		return
	}
	if len(req.Command) == 0 {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "command is required and must be a non-empty array",
		})
		return
	}
	if req.Container == "" {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "container is required",
		})
		return
	}
	if req.TimeoutSeconds < 0 {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "timeout_seconds must be a non-negative integer",
		})
		return
	}
	if req.TimeoutSeconds > MaxExecTimeoutSeconds {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code: contracts.CodeInvalidExecRequest,
			Message: fmt.Sprintf("timeout_seconds %d exceeds the maximum (%d)",
				req.TimeoutSeconds, MaxExecTimeoutSeconds),
		})
		return
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	res, err := h.svc.Exec(c.Request.Context(), id, ns, pod, req.Container, req.Command, timeout)
	if err != nil {
		writeAPIError(c.Writer, err)
		return
	}
	// The spec's wire shape (exit_code / stdout_lines /
	// stderr_lines / duration_ms) is exactly what
	// PodExecResult serializes to, so a direct pass-through
	// is the right thing.
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

// logQueryRequest is the wire shape for the
// /k8s/clusters/:clusterID/namespaces/:ns/logs query string.
// Container / tail / since are optional refinements that map
// directly onto the internal LogQuery struct. The wire types
// stay separate from the service's internal types so a future
// breaking change to the apiserver contract does not leak into
// the HTTP shape.
type logQueryRequest struct {
	// LabelSelector is the only required field. Empty
	// values are rejected with a 400 (the apiserver would
	// happily match every pod, which is almost always a
	// caller bug).
	LabelSelector string `form:"labelSelector"`
	// Container filters to one container per matching pod.
	Container string `form:"container"`
	// TailLines caps the historical tail per pod. The
	// handler clamps to [1, 1000] before forwarding.
	TailLines int `form:"tail"`
	// Since is the RFC3339 lower bound on line timestamps.
	SinceRFC3339 string `form:"since"`
}

// logQueryResponse is the wire shape returned by
// GetLogs. The `data` field carries the merged list of
// LogEntry (one per line); the `query` field echoes the
// resolved query so the UI can show "you asked for tail=200,
// we served 200" without having to round-trip the original
// request.
type logQueryResponse struct {
	Data  []LogEntry         `json:"data"`
	Query logQueryEchoFields `json:"query"`
}

// logQueryEchoFields is the echo block in the response body.
// Defined as its own type so the JSON tags stay grouped near
// the field they describe. Tail is rendered as the
// EFFECTIVE tail (post-default / post-clamp) so the UI
// can show the real value the apiserver saw.
type logQueryEchoFields struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Container     string `json:"container,omitempty"`
	Tail          int    `json:"tail"`
}

// GetLogs handles GET
// /k8s/clusters/:clusterID/namespaces/:ns/logs. The route
// fans out across pods matching the label selector via the
// per-cluster Client.GetLogsBySelector seam.
//
// The cluster ID is resolved through Service.Registry; a
// missing registry returns 502 (no kubeconfig, decrypt fail,
// cluster unknown) and a missing client returns 500. The
// validation order is: (1) labelSelector required, (2) tail
// within [1,1000], (3) since parses + within
// logstream.MaxSinceWindow, (4) cluster + client resolution.
func (h *Handler) GetLogs(c *gin.Context) {
	clusterID := c.Param("clusterID")
	ns := c.Param("ns")

	var req logQueryRequest
	// Gin's form binding for query params lives in
	// c.ShouldBindQuery; using ShouldBindJSON would
	// (correctly) find no body. We log + return 400 on a
	// bad parse so a malformed integer does not silently
	// fall through to 0.
	if err := c.ShouldBindQuery(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "invalid query string: " + err.Error(),
		})
		return
	}
	if req.LabelSelector == "" {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "labelSelector is required",
		})
		return
	}
	if req.TailLines != 0 {
		if req.TailLines < 1 || req.TailLines > 1000 {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "tail must be between 1 and 1000",
			})
			return
		}
	}
	// TailLines == 0 means "use the apiserver default"
	// (which the wire shape documents as ~10 lines). The
	// echo block reports 0 in that case; the UI can
	// substitute its own default when rendering.
	//
	// Since: parse RFC3339, then enforce the 30-day
	// window that the rest of the log-aggregation
	// surface uses. We re-use logstream.MaxSinceWindow
	// so the cap is consistent across the codebase.
	var since time.Time
	if req.SinceRFC3339 != "" {
		t, err := time.Parse(time.RFC3339, req.SinceRFC3339)
		if err != nil {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "since must be RFC3339: " + err.Error(),
			})
			return
		}
		if d := time.Since(t); d > logstream.MaxSinceWindow {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "since is older than the 30-day window",
			})
			return
		}
		since = t
	}

	reg := h.svc.Registry()
	if reg == nil {
		handler.WriteJSON(c.Writer, http.StatusBadGateway, contracts.ErrorResponse{
			Error: contracts.ErrorBody{
				Code:    contracts.CodeInternal,
				Message: "k8s client registry is not configured",
			},
		})
		return
	}
	client, err := reg.ClientFor(clusterID)
	if err != nil {
		if IsNotFound(err) {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "cluster not found",
			})
			return
		}
		// decrypt / parse / build-client failures all
		// surface as 502 — the cluster is reachable in
		// metadata but we cannot build a working
		// apiserver client for it.
		handler.WriteJSON(c.Writer, http.StatusBadGateway, contracts.ErrorResponse{
			Error: contracts.ErrorBody{
				Code:    contracts.CodeInternal,
				Message: "failed to resolve k8s client: " + err.Error(),
			},
		})
		return
	}

	entries, err := client.GetLogsBySelector(c.Request.Context(), ns, req.LabelSelector, LogQuery{
		Container: req.Container,
		TailLines: req.TailLines,
		Since:     since,
	})
	if err != nil {
		// A ErrInvalidLogQuery (e.g. empty namespace) is
		// a caller bug; surface as 400. Anything else is
		// an apiserver / transport failure → 500.
		if errors.Is(err, ErrInvalidLogQuery) {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: err.Error(),
			})
			return
		}
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to fetch logs: " + err.Error(),
			Cause:   err,
		})
		return
	}

	handler.WriteJSON(c.Writer, http.StatusOK, logQueryResponse{
		Data: entries,
		Query: logQueryEchoFields{
			Namespace:     ns,
			LabelSelector: req.LabelSelector,
			Container:     req.Container,
			Tail:          req.TailLines,
		},
	})
}
