package k8s

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/k8s/logstream"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// fakeRegistry is the in-test ClientRegistry. It hands out
// the same Client for every clusterID (and returns the
// supplied error for "missing" calls). Tests wire a single
// FakeClient and steer the success / failure branches via
// the FakeClient's own error fields.
type fakeRegistry struct {
	client    Client
	errByID   map[string]error
	allErr    error
}

func (f *fakeRegistry) ClientFor(clusterID string) (Client, error) {
	if f.allErr != nil {
		return nil, f.allErr
	}
	if err, ok := f.errByID[clusterID]; ok {
		return nil, err
	}
	return f.client, nil
}

// logsHandlerFixture builds a Gin engine wired to a real
// Service + Repository + ClientRegistry + FakeClient. The
// returned engine exposes the k8s routes; tests hit the
// /logs endpoint via the standard helper. The cluster is
// pre-created so the happy path can resolve the ID without
// extra setup.
func logsHandlerFixture(t *testing.T, fc *FakeClient, reg ClientRegistry) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{}, testCryptoKey)
	if reg != nil {
		svc.SetRegistry(reg)
	}
	h := NewHandler(svc)

	// Pre-create a cluster so the happy-path test has a
	// real ID to use. The registry's lookup goes through
	// the Service's repository, so this row must exist.
	created := &Cluster{Name: "logs-cl", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("seed cluster: %v", err)
	}

	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r, created.ID
}

func logsHandlerFixtureWithRegistry(t *testing.T, reg ClientRegistry) (*gin.Engine, string) {
	return logsHandlerFixture(t, &FakeClient{}, reg)
}

// TestHandler_GetLogs_HappyPath verifies the full success
// path: query with all params, returns the merged list and
// the echo block. Uses a FakeClient that returns two
// canned LogEntry values.
func TestHandler_GetLogs_HappyPath(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	fc := &FakeClient{
		LogEntries: []LogEntry{
			{Pod: "p1", Container: "app", Timestamp: now, Line: "hello"},
			{Pod: "p1", Container: "app", Timestamp: now.Add(time.Second), Line: "world"},
		},
	}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})

	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web&container=app&tail=200&since="+now.Format(time.RFC3339),
		nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("data len = %d, want 2", len(data))
	}
	q, _ := body["query"].(map[string]any)
	if q["namespace"] != "default" {
		t.Errorf("query.namespace = %v, want default", q["namespace"])
	}
	if q["labelSelector"] != "app=web" {
		t.Errorf("query.labelSelector = %v, want app=web", q["labelSelector"])
	}
	if q["container"] != "app" {
		t.Errorf("query.container = %v, want app", q["container"])
	}
	// tail comes back as a float64 after JSON round-trip.
	if tv, _ := q["tail"].(float64); int(tv) != 200 {
		t.Errorf("query.tail = %v, want 200", q["tail"])
	}
}

// TestHandler_GetLogs_MissingLabelSelector returns 400 when
// the only required query param is absent.
func TestHandler_GetLogs_MissingLabelSelector(t *testing.T) {
	r, id := logsHandlerFixtureWithRegistry(t, &fakeRegistry{client: &FakeClient{}})
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeValidation) {
		t.Errorf("code = %v, want VALIDATION_ERROR", errEnv["code"])
	}
}

// TestHandler_GetLogs_SinceTooOld returns 400 when since
// falls outside the 30-day window. We pass a timestamp from
// 60 days ago so the cap is always exceeded regardless of
// the test wall-clock time.
func TestHandler_GetLogs_SinceTooOld(t *testing.T) {
	fc := &FakeClient{}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})
	tooOld := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web&since="+tooOld,
		nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeValidation) {
		t.Errorf("code = %v, want VALIDATION_ERROR", errEnv["code"])
	}
	if msg, _ := errEnv["message"].(string); msg == "" {
		t.Error("error message is empty")
	}
}

// TestHandler_GetLogs_BadTail covers both the < 1 and > 1000
// branches. Gin's form binder parses "0" as 0, which is
// allowed (means "use apiserver default"); -1 must be
// rejected; 1001 must be rejected.
func TestHandler_GetLogs_BadTail(t *testing.T) {
	fc := &FakeClient{}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})

	cases := []struct {
		name string
		tail string
		want int
	}{
		{"below_floor", "-1", http.StatusBadRequest},
		{"above_ceiling", "1001", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr, _ := doRequest(t, r, "GET",
				"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web&tail="+tc.tail,
				nil)
			if rr.Code != tc.want {
				t.Errorf("tail=%s: status = %d, want %d", tc.tail, rr.Code, tc.want)
			}
		})
	}
}

// TestHandler_GetLogs_ClusterNotFound returns 404 when the
// registry reports ErrNotFound for the requested cluster.
// The fixture's pre-seeded cluster ID is NOT the one the
// test requests, so the registry's lookup misses.
func TestHandler_GetLogs_ClusterNotFound(t *testing.T) {
	fc := &FakeClient{}
	reg := &fakeRegistry{errByID: map[string]error{
		"cl-missing": ErrNotFound,
	}}
	r := handlerForLogsTest(t, fc, reg)
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/cl-missing/namespaces/default/logs?labelSelector=app=web", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeNotFound) {
		t.Errorf("code = %v, want NOT_FOUND", errEnv["code"])
	}
}

// TestHandler_GetLogs_RegistryError returns 502 when the
// registry returns a non-ErrNotFound error (e.g. decrypt
// failure, kubeconfig parse). 502 distinguishes "cluster
// is reachable in metadata but the apiserver client could
// not be built" from 500 "apiserver call itself failed".
func TestHandler_GetLogs_RegistryError(t *testing.T) {
	fc := &FakeClient{}
	reg := &fakeRegistry{allErr: errors.New("decrypt kubeconfig: bad key")}
	r := handlerForLogsTest(t, fc, reg)
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/any/namespaces/default/logs?labelSelector=app=web", nil)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil {
		t.Fatalf("missing error envelope; body=%s", rr.Body.String())
	}
}

// TestHandler_GetLogs_ApiserverError returns 500 when the
// apiserver call itself fails. The FakeClient's
// GetLogsBySelectorErr field drives the failure.
func TestHandler_GetLogs_ApiserverError(t *testing.T) {
	fc := &FakeClient{
		GetLogsBySelectorErr: fmt.Errorf("apiserver timeout"),
	}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rr.Code, rr.Body.String())
	}
	errEnv, _ := body["error"].(map[string]any)
	if errEnv["code"] != string(contracts.CodeInternal) {
		t.Errorf("code = %v, want INTERNAL_ERROR", errEnv["code"])
	}
}

// TestHandler_GetLogs_NoRegistryConfigured returns 502 when
// the service has no ClientRegistry wired. This is the
// dev-mode failure path; production main.go wires a real
// registry. Document the behaviour so a future refactor
// does not silently swap it for a 500.
func TestHandler_GetLogs_NoRegistryConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	created := &Cluster{Name: "no-reg", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc := NewService(repo, &FakeClient{}, testCryptoKey)
	// No SetRegistry call.
	h := NewHandler(svc)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)

	rr, _ := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+created.ID+"/namespaces/default/logs?labelSelector=app=web", nil)
	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
}

// TestHandler_GetLogs_RejectsZeroTail_AllowsZero is a guard:
// the wire spec says "tail is optional, default 100, max 1000".
// 0 means "use the apiserver default"; the handler does not
// reject 0. This test pins that behaviour so a future
// refactor does not accidentally 400 on the default case.
func TestHandler_GetLogs_TailZeroIsAllowed(t *testing.T) {
	fc := &FakeClient{LogEntries: []LogEntry{{Pod: "p1", Line: "x"}}}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})
	rr, _ := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web&tail=0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

// TestHandler_GetLogs_DefaultTailEchoed is a companion to
// the spec: when tail is omitted, the response echoes
// tail=0 (the "use apiserver default" sentinel). The UI
// is expected to render its own default in that case.
func TestHandler_GetLogs_DefaultTailEchoed(t *testing.T) {
	fc := &FakeClient{LogEntries: []LogEntry{}}
	r, id := logsHandlerFixture(t, fc, &fakeRegistry{client: fc})
	rr, body := doRequest(t, r, "GET",
		"/api/v1/k8s/clusters/"+id+"/namespaces/default/logs?labelSelector=app=web", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	q, _ := body["query"].(map[string]any)
	if tv, _ := q["tail"].(float64); int(tv) != 0 {
		t.Errorf("query.tail = %v, want 0 (no tail sent)", q["tail"])
	}
}

// _ = logstream.MaxSinceWindow is referenced via the
// handler; this dummy assignment keeps the constant on the
// radar if a future refactor removes the import path
// through the handler.
var _ = logstream.MaxSinceWindow

// _ = httptest.NewRecorder keeps the net/http/httptest
// import on the radar for future tests in this file.
var _ = httptest.NewRecorder

// handlerForLogsTest is a smaller fixture used by the
// negative-path tests. Unlike logsHandlerFixture it does
// NOT pre-seed a cluster row in the repo — those tests
// supply a registry that errors before the repo lookup
// matters. The route is registered on a fresh engine.
func handlerForLogsTest(t *testing.T, _ *FakeClient, reg ClientRegistry) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openTestDB(t))
	svc := NewService(repo, &FakeClient{}, testCryptoKey)
	if reg != nil {
		svc.SetRegistry(reg)
	}
	h := NewHandler(svc)
	r := gin.New()
	api := r.Group("/api/v1")
	h.Register(api)
	return r
}
