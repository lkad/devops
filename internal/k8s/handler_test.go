package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// handlerFixture builds a Gin engine wired to a real Service +
// Repository + FakeClient. Auth / RBAC are not in scope for this
// package; the engine exposes the k8s routes directly.
//
// The per-cluster ClientRegistry is wired against a FakeClient
// that can be overridden by execFixture to return canned
// PodExecResult values; the rest of the k8s read paths (List,
// Get, ListPods) all share the same FakeClient via the
// Service's s.client.
func handlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	return execFixture(t, &FakeClient{})
}

// execFixture is the exec-aware variant: the FakeClient is
// passed in so the caller can pre-set PodExecResult /
// ExecInPodErr. The registry resolves clusterID → this same
// FakeClient (so the per-cluster code path is exercised, not
// the fallback shared s.client).
func execFixture(t *testing.T, fc *FakeClient) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, fc, testCryptoKey)
	// Production wires a real ClientRegistry that
	// decrypts each cluster's kubeconfig. For unit
	// tests the cluster row's kubeconfig is a one-byte
	// stub ("k") that client-go's parser rejects, so
	// the real registry would surface a parse error
	// before our handler test ever gets to the exec
	// path. The fakeExecRegistry short-circuits that
	// and always returns the supplied FakeClient —
	// the per-cluster code path is still exercised
	// (Service.Exec calls reg.ClientFor) and the test
	// drives the FakeClient's PodExecResult hook for
	// the response shape.
	svc.SetRegistry(&fakeExecRegistry{repo: repo, client: fc})
	h := NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api, rbac.NoopPermFactory())
	return r
}

// fakeExecRegistry is a ClientRegistry that consults the
// supplied Repository to determine "is this a known
// cluster" (so the cluster-not-found branch is exercisable
// from the handler tests) and otherwise returns the
// supplied FakeClient. Production wires a real
// ClientRegistry that decrypts each cluster's kubeconfig;
// for unit tests the cluster row's kubeconfig is a one-byte
// stub ("k") that client-go's parser rejects, so the real
// registry would surface a parse error before our handler
// test ever gets to the exec path. This fake short-circuits
// the parse step while keeping the per-cluster code path
// exercised (Service.Exec calls reg.ClientFor).
type fakeExecRegistry struct {
	repo   *Repository
	client Client
}

func (f *fakeExecRegistry) ClientFor(clusterID string) (Client, error) {
	if clusterID == "" {
		return nil, ErrNoSuchCluster
	}
	if _, err := f.repo.Get(clusterID); err != nil {
		if IsNotFound(err) {
			// Return the repo's ErrNotFound sentinel so the
			// service's IsNotFound branch (which maps to 404)
			// fires. The registry sentinel ErrNoSuchCluster
			// is the production contract for the servicecatalog
			// walker; the exec path uses the repo's sentinel
			// because that's what the real defaultRegistry
			// bubbles up via repo.Get on a cache miss.
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f.client, nil
}

// ListerFor mirrors ClientFor but typed as the narrower
// Lister interface. The fake's stored client (FakeClient)
// satisfies Lister, so the type-assertion through c, nil
// is the entire body.
func (f *fakeExecRegistry) ListerFor(clusterID string) (Lister, error) {
	c, err := f.ClientFor(clusterID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// createClusterID is a small helper that POSTs a new cluster
// and returns its generated ID. Used by the exec tests to
// produce a real (decryptable) cluster row that the registry
// can resolve.
func createClusterID(t *testing.T, r *gin.Engine) string {
	t.Helper()
	_, body := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("create cluster: id missing in response: %+v", body)
	}
	return id
}

// doRequest is a thin helper around httptest.NewRecorder that
// also decodes the body into a generic JSON value.
func doRequest(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewBuffer(raw)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var decoded map[string]any
	if rr.Body.Len() > 0 {
		_ = json.Unmarshal(rr.Body.Bytes(), &decoded)
	}
	return rr, decoded
}

// TestHandler_List_EmptyReturnsEnvelope covers the spec's "list
// returns the standard envelope" requirement.
func TestHandler_List_EmptyReturnsEnvelope(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doRequest(t, r, "GET", "/api/v1/k8s/clusters", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(data) != 0 {
		t.Errorf("data len = %d, want 0", len(data))
	}
	page, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination missing: %T", body["pagination"])
	}
	if total, _ := page["total"].(float64); int(total) != 0 {
		t.Errorf("pagination.total = %v, want 0", page["total"])
	}
}

// TestHandler_Create_OK returns 201 with the created cluster.
func TestHandler_Create_OK(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name":        "dev-1",
		"type":        "k3d",
		"api_server":  "https://k3d.local:6443",
		"kubeconfig":  "apiVersion: v1\n",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("id is empty in response")
	}
	if body["name"] != "dev-1" {
		t.Errorf("name = %v, want dev-1", body["name"])
	}
	if body["type"] != "k3d" {
		t.Errorf("type = %v, want k3d", body["type"])
	}
	// KubeconfigEncrypted MUST NOT be the plaintext
	if body["kubeconfig_encrypted"] == "apiVersion: v1\n" {
		t.Error("kubeconfig_encrypted should be ciphertext, not plaintext")
	}
}

// TestHandler_Create_DuplicateName returns 409.
func TestHandler_Create_DuplicateName(t *testing.T) {
	r := handlerFixture(t)
	body := map[string]any{
		"name": "dup", "type": "k3d", "kubeconfig": "k",
	}
	_, _ = doRequest(t, r, "POST", "/api/v1/k8s/clusters", body)
	rr, dec := doRequest(t, r, "POST", "/api/v1/k8s/clusters", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	errEnv, _ := dec["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeConflict) {
		t.Errorf("code = %v, want CONFLICT", errEnv["code"])
	}
}

// TestHandler_Create_ValidationError returns 400 for an empty
// name.
func TestHandler_Create_ValidationError(t *testing.T) {
	r := handlerFixture(t)
	rr, _ := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "", "type": "k3d", "kubeconfig": "k",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestHandler_Get_OK returns 200 with the cluster details.
func TestHandler_Get_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "h", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r, "GET", "/api/v1/k8s/clusters/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body["name"] != "h" {
		t.Errorf("name = %v, want h", body["name"])
	}
}

// TestHandler_Get_NotFound renders a 404.
func TestHandler_Get_NotFound(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doRequest(t, r, "GET", "/api/v1/k8s/clusters/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Put_OK updates the cluster.
func TestHandler_Put_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "before", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r, "PUT", "/api/v1/k8s/clusters/"+id, map[string]any{
		"name": "after", "type": "k3d",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["name"] != "after" {
		t.Errorf("name = %v, want after", body["name"])
	}
}

// TestHandler_Delete_OK returns 204.
func TestHandler_Delete_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "x", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, _ := doRequest(t, r, "DELETE", "/api/v1/k8s/clusters/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	rr2, _ := doRequest(t, r, "GET", "/api/v1/k8s/clusters/"+id, nil)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("post-delete status = %d, want 404", rr2.Code)
	}
}

// TestHandler_Probe_OK returns connectivity info.
func TestHandler_Probe_OK(t *testing.T) {
	r := handlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r, "POST", "/api/v1/k8s/clusters/"+id+"/probe", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if body["status"] != string(ClusterStatusConnected) {
		t.Errorf("status = %v, want connected", body["status"])
	}
}

// TestHandler_ListPods_OK returns a list of pods.
func TestHandler_ListPods_OK(t *testing.T) {
	// Replace the client to return canned pods. We need to reach
	// the service through a custom fixture.
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{
		Pods: []Pod{{Name: "p1", Namespace: "default", Phase: "Running"}},
	}, testCryptoKey)
	h := NewHandler(svc)
	r2 := gin.New()
	api := r2.Group("/api/v1")
	h.Register(api, rbac.NoopPermFactory())

	_, created := doRequest(t, r2, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r2, "GET", "/api/v1/k8s/clusters/"+id+"/pods?namespace=default", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("pods len = %d, want 1", len(data))
	}
}

// TestHandler_ListDeployments_OK returns a list of deployments.
func TestHandler_ListDeployments_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{
		Deployments: []Deployment{{Name: "web", Namespace: "default", Ready: "3/3"}},
	}, testCryptoKey)
	h := NewHandler(svc)
	r2 := gin.New()
	api := r2.Group("/api/v1")
	h.Register(api, rbac.NoopPermFactory())

	_, created := doRequest(t, r2, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r2, "GET", "/api/v1/k8s/clusters/"+id+"/deployments?namespace=default", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("deployments len = %d, want 1", len(data))
	}
}

// TestHandler_ListServices_OK returns a list of services.
func TestHandler_ListServices_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{
		Services: []ServiceEntry{{Name: "svc", Namespace: "default", Type: "ClusterIP"}},
	}, testCryptoKey)
	h := NewHandler(svc)
	r2 := gin.New()
	api := r2.Group("/api/v1")
	h.Register(api, rbac.NoopPermFactory())

	_, created := doRequest(t, r2, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, body := doRequest(t, r2, "GET", "/api/v1/k8s/clusters/"+id+"/services?namespace=default", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("services len = %d, want 1", len(data))
	}
}

// TestHandler_Exec_HappyPath asserts the spec'd wire shape:
// a command that exits 0 returns 200 with exit_code /
// stdout_lines / stderr_lines / duration_ms. The FakeClient
// is wired with a canned PodExecResult so the test does not
// need a real apiserver.
func TestHandler_Exec_HappyPath(t *testing.T) {
	fc := &FakeClient{PodExecResult: &PodExecResult{
		Stdout:     []string{"line1", "line2"},
		Stderr:     []string{},
		ExitCode:   0,
		DurationMs: 42,
	}}
	r := execFixture(t, fc)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"ls", "-la"},
			"container": "app",
		})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ec, _ := body["exit_code"].(float64); int(ec) != 0 {
		t.Errorf("exit_code = %v, want 0", ec)
	}
	if dur, _ := body["duration_ms"].(float64); int(dur) != 42 {
		t.Errorf("duration_ms = %v, want 42", dur)
	}
	stdout, _ := body["stdout_lines"].([]any)
	if len(stdout) != 2 || stdout[0] != "line1" {
		t.Errorf("stdout_lines = %v, want [line1, line2]", stdout)
	}
}

// TestHandler_Exec_NonZeroExit pins the spec's "non-zero
// exit is not a request error" rule: a command that exits
// 7 still returns 200 with exit_code=7 in the body.
func TestHandler_Exec_NonZeroExit(t *testing.T) {
	fc := &FakeClient{PodExecResult: &PodExecResult{
		Stdout:   []string{"matched", "these"},
		Stderr:   []string{},
		ExitCode: 7,
	}}
	r := execFixture(t, fc)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"grep", "foo", "/var/log/app.log"},
			"container": "app",
		})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ec, _ := body["exit_code"].(float64); int(ec) != 7 {
		t.Errorf("exit_code = %v, want 7", ec)
	}
}

// TestHandler_Exec_EmptyCommandRejected returns 400
// INVALID_EXEC_REQUEST when the command array is missing.
func TestHandler_Exec_EmptyCommandRejected(t *testing.T) {
	r := handlerFixture(t)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{},
			"container": "app",
		})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInvalidExecRequest) {
		t.Errorf("code = %v, want INVALID_EXEC_REQUEST", errEnv["code"])
	}
}

// TestHandler_Exec_EmptyContainerRejected returns 400
// INVALID_EXEC_REQUEST when the container field is empty.
func TestHandler_Exec_EmptyContainerRejected(t *testing.T) {
	r := handlerFixture(t)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"ls"},
			"container": "",
		})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInvalidExecRequest) {
		t.Errorf("code = %v, want INVALID_EXEC_REQUEST", errEnv["code"])
	}
}

// TestHandler_Exec_TimeoutOverMaxRejected pins the
// "reject with 400" choice: a timeout_seconds > 600 is almost
// certainly a client bug, so we surface it as INVALID_EXEC_REQUEST
// rather than silently clamping (the KubeClient clamps to
// MaxExecTimeout as a defence-in-depth, but the route layer
// should not pretend a 99999s request is sane).
func TestHandler_Exec_TimeoutOverMaxRejected(t *testing.T) {
	r := handlerFixture(t)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":         []string{"sleep", "60"},
			"container":       "app",
			"timeout_seconds": 99999,
		})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInvalidExecRequest) {
		t.Errorf("code = %v, want INVALID_EXEC_REQUEST", errEnv["code"])
	}
}

// TestHandler_Exec_NegativeTimeoutRejected covers the
// adjacent "negative timeout" branch — the spec is silent
// but a negative duration would silently flip into a
// huge timeout under time.Duration arithmetic, so the
// handler rejects it explicitly.
func TestHandler_Exec_NegativeTimeoutRejected(t *testing.T) {
	r := handlerFixture(t)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":         []string{"ls"},
			"container":       "app",
			"timeout_seconds": -1,
		})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInvalidExecRequest) {
		t.Errorf("code = %v, want INVALID_EXEC_REQUEST", errEnv["code"])
	}
}

// TestHandler_Exec_ClusterNotFound returns 404 when the
// clusterID in the URL is unknown to the registry. The
// handler's Service.Exec surfaces the cluster-not-found
// branch (mapped from registry.ErrNotFound) as a 404
// envelope so the operator UI can show "cluster missing".
func TestHandler_Exec_ClusterNotFound(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/does-not-exist/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"ls"},
			"container": "app",
		})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_Exec_ClientError returns 502 APISERVER_UNREACHABLE
// when the FakeClient's ExecInPod returns a non-typed error
// (the FakeClient's ExecInPodErr hook). The handler
// distinguishes this from INVALID_EXEC_REQUEST and TIMEOUT
// by walking errors.Is on the typed sentinels.
func TestHandler_Exec_ClientError(t *testing.T) {
	fc := &FakeClient{
		ExecInPodErr: errors.New("apiserver connection refused"),
	}
	r := execFixture(t, fc)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"ls"},
			"container": "app",
		})
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeAPIServerUnreachable) {
		t.Errorf("code = %v, want APISERVER_UNREACHABLE", errEnv["code"])
	}
}

// TestHandler_Exec_TimeoutReturnsTIMEOUT pins the
// context-deadline branch: a FakeClient that returns
// context.DeadlineExceeded gets mapped to 504 TIMEOUT.
func TestHandler_Exec_TimeoutReturnsTIMEOUT(t *testing.T) {
	fc := &FakeClient{ExecInPodErr: context.DeadlineExceeded}
	r := execFixture(t, fc)
	id := createClusterID(t, r)
	rr, body := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{
			"command":   []string{"sleep", "60"},
			"container": "app",
		})
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeTimeout) {
		t.Errorf("code = %v, want TIMEOUT", errEnv["code"])
	}
}
