package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/k8s"
)

// ---------- test fakes ----------

// fakeLDAP is a controllable LDAPHealthChecker for the unit
// tests. err is returned by HealthCheck; delay is the sleep
// before the return so the concurrent-execution test can
// observe whether other checks are gated by LDAP.
type fakeLDAP struct {
	err   error
	delay time.Duration
	calls int32
}

func (f *fakeLDAP) HealthCheck(ctx context.Context) error {
	atomic.AddInt32(&f.calls, 1)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

// fakeLister is a ClusterLister that returns a fixed slice
// of IDs (or an error) without touching the DB.
type fakeLister struct {
	ids []string
	err error
}

func (f fakeLister) ListClusterIDs(_ context.Context) ([]string, error) {
	return f.ids, f.err
}

// fakeRegistry implements k8s.ClientRegistry. For every
// cluster ID, it returns a FakeClient with the given ping
// err. The fake honours the sticky-cache contract by
// returning the same client for repeat calls.
type fakeRegistry struct {
	mu      chan struct{}
	clients map[string]k8s.Client
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{mu: make(chan struct{}, 1), clients: map[string]k8s.Client{}}
}

func (r *fakeRegistry) ClientFor(id string) (k8s.Client, error) {
	r.mu <- struct{}{}
	defer func() { <-r.mu }()
	if c, ok := r.clients[id]; ok {
		return c, nil
	}
	c := &k8s.FakeClient{}
	r.clients[id] = c
	return c, nil
}

func (r *fakeRegistry) setPingErr(id string, err error) {
	r.mu <- struct{}{}
	defer func() { <-r.mu }()
	c, ok := r.clients[id]
	if !ok {
		c = &k8s.FakeClient{}
		r.clients[id] = c
	}
	fc, ok := c.(*k8s.FakeClient)
	if !ok {
		return
	}
	fc.PingErr = err
}

// ---------- helpers ----------

func newSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "h.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

// closeSQLite shuts the underlying *sql.DB pool so any
// subsequent query returns an error. Used by the
// "DB unreachable" tests to simulate a real connection
// failure without standing up a separate process.
func closeSQLite(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}

func newRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/live", h.Live)
	r.GET("/ready", h.Ready)
	r.GET("/health", h.Health)
	return r
}

func doGET(t *testing.T, r *gin.Engine, path string) (*httptest.ResponseRecorder, Report) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(rr, req)
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("missing Content-Type on %s", path)
	}
	var body Report
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return rr, body
}

// ---------- /live ----------

// TestLive_AlwaysReturns200 pins the liveness contract: the
// endpoint reports process-up regardless of dep state. A
// closed DB or a panicking LDAP must NOT 500 /live.
func TestLive_AlwaysReturns200(t *testing.T) {
	h := NewHandler(&Checker{DB: nil, LDAP: nil, K8sAPIURL: "http://127.0.0.1:1/"})
	r := newRouter(h)
	rr, body := doGET(t, r, "/live")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body.Status != StatusOK {
		t.Errorf("body status = %q, want %q", body.Status, StatusOK)
	}
}

// TestLive_IgnoresCheckerDeps asserts the /live handler does
// not even consult the Checker — a nil Checker must not
// nil-deref.
func TestLive_IgnoresCheckerDeps(t *testing.T) {
	h := &Handler{Checker: nil}
	r := newRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/live", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

// ---------- /ready: response shape ----------

// TestReady_ResponseShape pins the wire contract: every
// spec'd field is present, even when zero, and the JSON
// keys match the spec.
func TestReady_ResponseShape(t *testing.T) {
	c := New(newSQLiteDB(t), &fakeLDAP{}, true, newFakeRegistry(), fakeLister{})
	h := NewHandler(c)
	r := newRouter(h)
	_, body := doGET(t, r, "/ready")

	if body.Time.IsZero() {
		t.Error("time is zero")
	}
	if body.Status != StatusOK {
		t.Errorf("status = %q, want ok", body.Status)
	}
	// Every spec'd check must be present (even if skipped).
	if body.Checks.DB.Status == "" {
		t.Error("db.status missing")
	}
	if body.Checks.LDAP.Status != StatusSkipped {
		t.Errorf("ldap.status = %q, want skipped (dev_bypass)", body.Checks.LDAP.Status)
	}
	if body.Checks.K8s.Status != StatusOK {
		t.Errorf("k8s.status = %q, want ok (no clusters)", body.Checks.K8s.Status)
	}
	if body.Checks.K8sAPI.Status != StatusSkipped {
		t.Errorf("k8s_api.status = %q, want skipped (no URL)", body.Checks.K8sAPI.Status)
	}
}

// TestReady_AllHealthyReturns200 pins the happy path.
func TestReady_AllHealthyReturns200(t *testing.T) {
	reg := newFakeRegistry()
	reg.clients["c1"] = &k8s.FakeClient{}
	reg.clients["c2"] = &k8s.FakeClient{}
	c := New(
		newSQLiteDB(t),
		&fakeLDAP{},
		true, // dev_bypass -> ldap skipped
		reg,
		fakeLister{ids: []string{"c1", "c2"}},
	)
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body.Status != StatusOK {
		t.Errorf("body status = %q, want ok", body.Status)
	}
	if body.Checks.K8s.Healthy != 2 {
		t.Errorf("k8s.healthy = %d, want 2", body.Checks.K8s.Healthy)
	}
	if body.Checks.K8s.Clusters != 2 {
		t.Errorf("k8s.clusters = %d, want 2", body.Checks.K8s.Clusters)
	}
}

// ---------- /ready: failure modes ----------

// TestReady_DBFailureReturns503 pins the spec'd "DB
// unreachable -> 503" path. We close the underlying
// *sql.DB pool so any subsequent query returns an error.
// (A 1ms timeout alone is not enough — sqlite is
// in-process and SELECT 1 returns faster than the
// deadline can be checked.)
func TestReady_DBFailureReturns503(t *testing.T) {
	db := newSQLiteDB(t)
	closeSQLite(db) // make every subsequent query fail
	c := New(db, &fakeLDAP{}, true, newFakeRegistry(), fakeLister{})
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if body.Status != StatusDown {
		t.Errorf("body status = %q, want down", body.Status)
	}
	if body.Checks.DB.Status != StatusDown {
		t.Errorf("db.status = %q, want down", body.Checks.DB.Status)
	}
	if body.Checks.DB.Error == "" {
		t.Error("db.error empty")
	}
}

// TestReady_LDAPDownReturns503 pins the LDAP branch: a
// configured (non-bypass) LDAP that returns an error
// must surface as down and roll up the top-level status
// to "degraded" -> 503.
func TestReady_LDAPDownReturns503(t *testing.T) {
	c := New(
		newSQLiteDB(t),
		&fakeLDAP{err: errors.New("dial tcp: connection refused")},
		false, // dev_bypass off -> LDAP actually probed
		newFakeRegistry(),
		fakeLister{},
	)
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if body.Checks.LDAP.Status != StatusDown {
		t.Errorf("ldap.status = %q, want down", body.Checks.LDAP.Status)
	}
	if body.Checks.LDAP.Error == "" {
		t.Error("ldap.error empty")
	}
}

// TestReady_K8sClusterDownReturns503 pins the per-cluster
// rollup: one bad cluster in a set of 2 produces
// healthy=1, clusters=2, status=down.
func TestReady_K8sClusterDownReturns503(t *testing.T) {
	reg := newFakeRegistry()
	reg.clients["c1"] = &k8s.FakeClient{}
	reg.setPingErr("c2", errors.New("apiserver 503"))
	c := New(
		newSQLiteDB(t),
		&fakeLDAP{},
		true,
		reg,
		fakeLister{ids: []string{"c1", "c2"}},
	)
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if body.Checks.K8s.Healthy != 1 || body.Checks.K8s.Clusters != 2 {
		t.Errorf("k8s.healthy/clusters = %d/%d, want 1/2",
			body.Checks.K8s.Healthy, body.Checks.K8s.Clusters)
	}
	if body.Checks.K8s.Error == "" {
		t.Error("k8s.error empty")
	}
}

// TestReady_NilDBIsSkipped pins the no-deps / test-mode
// contract: a Checker with no DB or LDAP must not be
// reported as degraded just because the field is nil.
func TestReady_NilDBIsSkipped(t *testing.T) {
	c := New(nil, nil, true, newFakeRegistry(), fakeLister{})
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body.Checks.DB.Status != StatusSkipped {
		t.Errorf("db.status = %q, want skipped", body.Checks.DB.Status)
	}
	if body.Checks.LDAP.Status != StatusSkipped {
		t.Errorf("ldap.status = %q, want skipped", body.Checks.LDAP.Status)
	}
}

// ---------- /ready: concurrency ----------

// TestReady_ChecksRunConcurrently pins the spec's
// "concurrent fan-out" requirement. The strategy: LDAP
// blocks for 300ms; the K8s check also blocks for the
// same window. If they ran serially, total = ~600ms. If
// they ran in parallel, total = ~300ms. We assert the
// elapsed time is < 500ms (well under the serial sum).
// The two-delay design avoids the ambiguity of
// "fast-check + slow-check" which would also be ~300ms
// either way.
func TestReady_ChecksRunConcurrently(t *testing.T) {
	ldap := &fakeLDAP{delay: 300 * time.Millisecond}
	reg := newFakeRegistry()
	// Use a slow k8s client. We can't make the
	// FakeClient sleep directly, but we can wrap it
	// with a delay. fakeRegistry returns the
	// FakeClient as-is; we need a real Client that
	// sleeps. Use a tiny adapter.
	reg.clients["c1"] = &slowClient{delay: 300 * time.Millisecond}
	c := New(
		newSQLiteDB(t),
		ldap,
		false, // probe LDAP for real
		reg,
		fakeLister{ids: []string{"c1"}},
	)
	c.Timeout = 1 * time.Second
	h := NewHandler(c)
	r := newRouter(h)

	start := time.Now()
	rr, body := doGET(t, r, "/ready")
	elapsed := time.Since(start)

	// Serial would be ~600ms, parallel ~300ms. The 500ms
	// threshold catches a serial regression (prints ~600ms)
	// without flaking on a slow CI runner doing the parallel
	// path (which is ~300ms).
	if elapsed >= 500*time.Millisecond {
		t.Errorf("elapsed = %v; expected < 500ms (parallel) but saw serial-ish behaviour (LDAP 300ms + K8s 300ms = 600ms serial)", elapsed)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body=%+v", rr.Code, body)
	}
	if atomic.LoadInt32(&ldap.calls) == 0 {
		t.Error("LDAP was not probed")
	}
}

// slowClient wraps k8s.FakeClient to add a sleep on Ping.
// Used only by the concurrency test — production never
// sees this.
type slowClient struct {
	*k8s.FakeClient
	delay time.Duration
}

func (s *slowClient) Ping(ctx context.Context) error {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// TestReady_TimeoutDistinctFromDown pins the spec's
// "timeout != down" rule: a check that exceeds the
// per-check timeout is reported as "timeout", not "down",
// so the operator signal is unambiguous.
func TestReady_TimeoutDistinctFromDown(t *testing.T) {
	ldap := &fakeLDAP{delay: 500 * time.Millisecond}
	c := &Checker{
		DB:            newSQLiteDB(t),
		LDAP:          ldap,
		LDAPDevBypass: false,
		Timeout:       10 * time.Millisecond, // LDAP will hit this
	}
	h := NewHandler(c)
	r := newRouter(h)
	_, body := doGET(t, r, "/ready")
	if body.Checks.LDAP.Status != StatusTimeout {
		t.Errorf("ldap.status = %q, want timeout", body.Checks.LDAP.Status)
	}
}

// ---------- /ready: k8s_api optional ----------

// TestReady_K8sAPISkippedWhenURLEmpty pins the optional
// k8s_api behaviour: with no K8S_API_URL, the check is
// reported as "skipped" (not "down") and the top-level
// status stays at "ok" — the apiserver probe is
// opt-in.
func TestReady_K8sAPISkippedWhenURLEmpty(t *testing.T) {
	c := New(newSQLiteDB(t), &fakeLDAP{}, true, newFakeRegistry(), fakeLister{})
	c.K8sAPIURL = ""
	h := NewHandler(c)
	r := newRouter(h)
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body.Checks.K8sAPI.Status != StatusSkipped {
		t.Errorf("k8s_api.status = %q, want skipped", body.Checks.K8sAPI.Status)
	}
}

// ---------- /health alias ----------

// TestHealth_AliasForReady pins the back-compat contract:
// /health returns the same shape and the same status as
// /ready. Existing curl scripts (and the docker-compose
// healthcheck) must keep working unchanged.
func TestHealth_AliasForReady(t *testing.T) {
	c := New(newSQLiteDB(t), &fakeLDAP{}, true, newFakeRegistry(), fakeLister{})
	h := NewHandler(c)
	r := newRouter(h)
	rrH, bodyH := doGET(t, r, "/health")
	rrR, bodyR := doGET(t, r, "/ready")
	if rrH.Code != rrR.Code {
		t.Errorf("/health=%d /ready=%d status mismatch", rrH.Code, rrR.Code)
	}
	if bodyH.Status != bodyR.Status {
		t.Errorf("/health=%q /ready=%q status mismatch", bodyH.Status, bodyR.Status)
	}
}

// TestHealth_DegradedReturns503 pins the back-compat
// failure case: a failing dep on /health still returns
// 503 (not 200), so a docker-compose healthcheck using
// curl -f gets a non-zero exit.
func TestHealth_DegradedReturns503(t *testing.T) {
	db := newSQLiteDB(t)
	closeSQLite(db) // make every query fail
	c := New(db, &fakeLDAP{}, true, newFakeRegistry(), fakeLister{})
	h := NewHandler(c)
	r := newRouter(h)
	rr, _ := doGET(t, r, "/health")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/health status = %d, want 503", rr.Code)
	}
}

// ---------- SetChecker swap ----------

// TestSetChecker_AllowsRuntimeSwap pins the "build router
// with placeholder, swap in real Checker after wiring"
// pattern used by cmd/devops-toolkit/main.go.
func TestSetChecker_AllowsRuntimeSwap(t *testing.T) {
	h := NewHandler(NewNoDeps())
	r := newRouter(h)
	// First request: no-deps checker, all ok.
	rr, body := doGET(t, r, "/ready")
	if rr.Code != http.StatusOK {
		t.Fatalf("initial /ready = %d, want 200", rr.Code)
	}
	// Swap in a Checker that will report down. The
	// closed DB makes the DB ping fail.
	db := newSQLiteDB(t)
	closeSQLite(db)
	h.SetChecker(New(db, &fakeLDAP{}, true, newFakeRegistry(), fakeLister{}))
	rr, body = doGET(t, r, "/ready")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("post-swap /ready = %d, want 503", rr.Code)
	}
	_ = body
}
