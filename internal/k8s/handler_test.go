package k8s

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// handlerFixture builds a Gin engine wired to a real Service +
// Repository + FakeClient. Auth / RBAC are not in scope for this
// package; the engine exposes the k8s routes directly.
func handlerFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{}, testCryptoKey)
	h := NewHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r
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
	h.Register(api)

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
	h.Register(api)

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
	h.Register(api)

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

// TestHandler_ExecStub_DisabledByDefault returns 403 because
// the feature flag is off.
func TestHandler_ExecStub_DisabledByDefault(t *testing.T) {
	r := handlerFixture(t)
	_, created := doRequest(t, r, "POST", "/api/v1/k8s/clusters", map[string]any{
		"name": "p", "type": "k3d", "kubeconfig": "k",
	})
	id, _ := created["id"].(string)
	rr, _ := doRequest(t, r, "POST",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/pods/p1/exec",
		map[string]any{"command": []string{"ls"}})
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
}
