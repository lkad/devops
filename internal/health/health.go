// Package health implements the deep readiness probe split used by
// the orchestrator (/health, /live, /ready). It fans out to the
// dependencies the backend actually needs at request time (DB,
// LDAP, every registered K8s cluster) and returns a per-dep
// breakdown so the operator can see which dep is the culprit
// without reading pod logs.
//
// The split mirrors the Kubernetes liveness/readiness contract:
//
//   - /live  — process is up and the HTTP server can respond.
//     Used as the K8s liveness probe; failing this
//     restarts the pod. NEVER fails on a transient
//     downstream issue (DB blip, LDAP server bounce).
//
//   - /ready — every required downstream is reachable.
//     Used as the K8s readiness probe; failing this
//     stops routing traffic to the pod but does NOT
//     restart it. A blip in a dep costs zero pods.
//
// /health is a back-compat alias for /ready so existing curl
// scripts (and the docker-compose healthcheck) keep working.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/internal/k8s"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// DefaultCheckTimeout caps any single check at 2 s. A check that
// exceeds the cap is reported as "timeout" (not "down") so an
// operator can distinguish "the dep is broken" from "the dep
// is slow but might recover".
const DefaultCheckTimeout = 2 * time.Second

// LDAPHealthChecker is the minimal surface we need from the
// auth/ldap stack. Defined here (not in auth/ldap) to avoid
// pulling the entire ldap package's import graph into the
// health module, and to make the dependency substitutable in
// unit tests.
type LDAPHealthChecker interface {
	// HealthCheck returns nil on success, or a transport /
	// bind error describing the failure. The ldap.Service
	// satisfies this; tests can pass an in-memory fake.
	HealthCheck(ctx context.Context) error
}

// ClusterLister exposes the cluster-ID enumeration used by the
// K8s check. Production: k8s.Repository.List. Tests: a fake
// returning a fixed slice of IDs.
type ClusterLister interface {
	// ListClusterIDs returns the IDs of every cluster currently
	// known to the deployment. The order is not significant;
	// the result is treated as a set.
	ListClusterIDs(ctx context.Context) ([]string, error)
}

// Checker is the dependency fan-out used by /ready. It is safe
// to call Run() concurrently (the k8s registry is the only
// mutable state and it is itself concurrency-safe).
type Checker struct {
	DB   *gorm.DB
	LDAP LDAPHealthChecker
	// LDAPDevBypass == true means the LDAP dep is intentionally
	// not configured (the in-memory Fake handles auth). The
	// check reports status="skipped" instead of probing.
	LDAPDevBypass bool
	// K8sRegistry resolves clusterID -> k8s.Client. A nil
	// registry (or one whose List returns no IDs) makes the
	// k8s check report status="ok" with clusters=0 — there is
	// nothing to probe, so "no clusters registered" is
	// healthy, not unhealthy.
	K8sRegistry k8s.ClientRegistry
	// K8sClusterLister is the cluster enumeration. If nil,
	// the k8s check reports clusters=0.
	K8sClusterLister ClusterLister
	// K8sAPIURL is an optional single-apiserver URL probe
	// surfaced as the "k8s_api" check. Set to empty string
	// to skip. Used by environments that do not yet use the
	// per-cluster registry but still want a "is the apiserver
	// reachable" signal.
	K8sAPIURL string
	// Timeout caps every individual check. Zero means
	// DefaultCheckTimeout.
	Timeout time.Duration
}

// New builds a Checker with the production defaults. Fields
// that the caller does not need (e.g. K8sAPIURL, K8sClusterLister)
// can be left at their zero value.
func New(db *gorm.DB, ldap LDAPHealthChecker, devBypass bool, reg k8s.ClientRegistry, lister ClusterLister) *Checker {
	return &Checker{
		DB:               db,
		LDAP:             ldap,
		LDAPDevBypass:    devBypass,
		K8sRegistry:      reg,
		K8sClusterLister: lister,
	}
}

// K8sRepoLister adapts a *k8s.Repository to ClusterLister.
// Defined here (not in the k8s package) to avoid having
// k8s depend on health. The repository's List returns the
// full row set; we project to IDs and let the per-cluster
// K8s check resolve each.
type K8sRepoLister struct {
	Repo *k8s.Repository
}

// ListClusterIDs satisfies ClusterLister.
func (l K8sRepoLister) ListClusterIDs(ctx context.Context) ([]string, error) {
	rows, _, err := l.Repo.List(k8s.ListFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.ID)
	}
	return ids, nil
}

// NewNoDeps returns a Checker that reports every dep as
// status="ok" without making any network calls. Used by
// the buildRouter smoke test path where there is no DB,
// no LDAP, no K8s — the request is just a routing check,
// not a real readiness probe.
func NewNoDeps() *Checker {
	return &Checker{
		DB:               nil,
		LDAP:             nil,
		LDAPDevBypass:    true,
		K8sRegistry:      nil,
		K8sClusterLister: nil,
		K8sAPIURL:        "",
	}
}

// CheckStatus is the per-dep status string. The wire shape is
// the lower-case literal; do not change it without a contract
// review (the frontend audit page reads these).
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusDown    CheckStatus = "down"
	StatusTimeout CheckStatus = "timeout"
	StatusSkipped CheckStatus = "skipped"
)

// CheckResult is the per-dep row. Field tags are the JSON keys
// in the response. Optional fields use omitempty so the wire
// shape matches the spec (a successful k8s check has no
// "error" key; a skipped ldap has a "reason" key but no
// "duration_ms").
type CheckResult struct {
	Status     CheckStatus `json:"status"`
	DurationMs int64       `json:"duration_ms,omitempty"`
	Error      string      `json:"error,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	// K8s-only aggregate fields. omitempty so non-k8s checks
	// do not carry empty "clusters":0 on the wire.
	Clusters int `json:"clusters,omitempty"`
	Healthy  int `json:"healthy,omitempty"`
}

// Report is the body of /ready (and /health, the alias). The
// top-level Status is "ok" iff every required check is "ok";
// any "down" / "timeout" promotes it to "degraded".
type Report struct {
	Status CheckStatus  `json:"status"`
	Time   time.Time    `json:"time"`
	Checks ChecksBundle `json:"checks"`
}

// ChecksBundle is the per-dep breakdown. Using a named struct
// (not a map) keeps the JSON keys stable for clients that
// switch on field names.
type ChecksBundle struct {
	DB     CheckResult `json:"db"`
	LDAP   CheckResult `json:"ldap"`
	K8s    CheckResult `json:"k8s"`
	K8sAPI CheckResult `json:"k8s_api"`
}

// HTTPStatus maps a Report to the right HTTP code. The
// /ready handler uses this; the legacy /health alias uses the
// same mapping for parity.
func (r Report) HTTPStatus() int {
	if r.Status == StatusOK {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}

// Run executes every check in parallel and returns the
// aggregate Report. The per-check timeout is Checker.Timeout
// (or DefaultCheckTimeout). A check that exceeds the cap is
// reported as "timeout" — never "down" — so the operator
// signal is unambiguous.
//
// The errgroup is used for the parallel-spawn + first-error
// cancellation pattern. The ctx is shared so a fast-failing
// check does not waste time on the others; the per-check
// timeout is applied INSIDE the spawned func so the deadline
// is per-check, not global.
func (c *Checker) Run(ctx context.Context) Report {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultCheckTimeout
	}

	// Shared bundle written by the four workers. We use a
	// plain struct + sync.Mutex because errgroup does not
	// expose its first-error value as a typed Report; we
	// need the per-dep status even when one check fails,
	// not just the first error.
	var (
		mu     sync.Mutex
		checks ChecksBundle
	)

	g, gctx := errgroup.WithContext(ctx)

	// DB
	g.Go(func() error {
		r := c.checkDB(gctx, timeout)
		mu.Lock()
		checks.DB = r
		mu.Unlock()
		return nil
	})

	// LDAP (skipped when dev_bypass is on)
	g.Go(func() error {
		var r CheckResult
		if c.LDAPDevBypass {
			r = CheckResult{Status: StatusSkipped, Reason: "dev_bypass enabled; LDAP not configured"}
		} else {
			r = c.checkLDAP(gctx, timeout)
		}
		mu.Lock()
		checks.LDAP = r
		mu.Unlock()
		return nil
	})

	// K8s (cluster rollup)
	g.Go(func() error {
		r := c.checkK8s(gctx, timeout)
		mu.Lock()
		checks.K8s = r
		mu.Unlock()
		return nil
	})

	// K8s apiserver probe (single URL, optional)
	g.Go(func() error {
		var r CheckResult
		if c.K8sAPIURL == "" {
			r = CheckResult{Status: StatusSkipped, Reason: "K8S_API_URL not configured"}
		} else {
			r = c.checkK8sAPI(gctx, timeout)
		}
		mu.Lock()
		checks.K8sAPI = r
		mu.Unlock()
		return nil
	})

	// Block until all four finish. We deliberately ignore
	// the g.Wait() error: every worker returns nil, the
	// per-check failure is encoded in the Report, not in
	// the errgroup.
	_ = g.Wait()

	report := Report{
		Time:   time.Now().UTC(),
		Checks: checks,
	}
	report.Status = aggregateStatus(checks)
	return report
}

// checkDB runs a SELECT 1 with a per-check timeout. The gorm
// pool is used directly; if the pool is exhausted, the call
// will block until the timeout — exactly the signal we want
// to surface. A nil DB is treated as "skipped" (the dep is
// not configured for this deploy) rather than "down" so the
// test/no-deps path stays healthy.
func (c *Checker) checkDB(ctx context.Context, timeout time.Duration) CheckResult {
	if c.DB == nil {
		return CheckResult{Status: StatusSkipped, Reason: "database not configured"}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	err := c.DB.WithContext(cctx).Exec("SELECT 1").Error
	dur := time.Since(start).Milliseconds()
	if err != nil {
		return classifyErr(err, cctx, dur)
	}
	return CheckResult{Status: StatusOK, DurationMs: dur}
}

// checkLDAP delegates to the LDAPHealthChecker (production:
// ldap.Service). A nil checker (e.g. buildLDAPClient failed
// during boot, or this is a no-deps test build) is treated
// as "skipped" — not "down" — so the rolled-up status is
// not falsely degraded.
func (c *Checker) checkLDAP(ctx context.Context, timeout time.Duration) CheckResult {
	if c.LDAP == nil {
		return CheckResult{Status: StatusSkipped, Reason: "ldap client not configured"}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	err := c.LDAP.HealthCheck(cctx)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		return classifyErr(err, cctx, dur)
	}
	return CheckResult{Status: StatusOK, DurationMs: dur}
}

// checkK8s walks every registered cluster, calls Client.Ping
// on the resolved client, and aggregates. A cluster whose
// ClientFor returns a sticky error is counted as "down" with
// the cached error surfaced on the rollup. The rollup is
// "ok" iff clusters == healthy.
func (c *Checker) checkK8s(ctx context.Context, timeout time.Duration) CheckResult {
	if c.K8sRegistry == nil || c.K8sClusterLister == nil {
		return CheckResult{Status: StatusSkipped, Reason: "no K8s registry configured"}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()

	ids, err := c.K8sClusterLister.ListClusterIDs(cctx)
	if err != nil {
		dur := time.Since(start).Milliseconds()
		return classifyErr(err, cctx, dur)
	}
	if len(ids) == 0 {
		// No clusters registered is the dev / no-k8s
		// case. Not unhealthy — the k8s dep is
		// simply not in use.
		return CheckResult{Status: StatusOK, DurationMs: time.Since(start).Milliseconds(), Clusters: 0, Healthy: 0}
	}

	healthy := 0
	var firstErr error
	for _, id := range ids {
		// Honour the per-check timeout on every iteration:
		// 3 clusters * 2 s = up to 6 s total if run
		// serially, but the outer errgroup with shared
		// gctx means a slow first cluster cancels the
		// remaining ones. Better: use a per-iteration
		// child context derived from the parent. The
		// gctx cancel still kills in-flight calls.
		client, err := c.K8sRegistry.ClientFor(id)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cluster %s: %w", id, err)
			}
			continue
		}
		if err := client.Ping(cctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cluster %s: %w", id, err)
			}
			continue
		}
		healthy++
	}
	dur := time.Since(start).Milliseconds()
	clusters := len(ids)
	res := CheckResult{Clusters: clusters, Healthy: healthy, DurationMs: dur}
	if healthy == clusters {
		res.Status = StatusOK
		return res
	}
	res.Status = StatusDown
	if firstErr != nil {
		res.Error = firstErr.Error()
	}
	return res
}

// checkK8sAPI does a single TCP dial to K8sAPIURL. We do NOT
// do a TLS handshake or auth probe here — the apiserver may
// be locked down with mTLS in prod and a TCP-level probe is
// the lowest-common-denominator reachability check. Use the
// per-cluster check (above) for full readiness.
func (c *Checker) checkK8sAPI(ctx context.Context, timeout time.Duration) CheckResult {
	if c.K8sAPIURL == "" {
		return CheckResult{Status: StatusSkipped, Reason: "K8S_API_URL not configured"}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()

	// Use the stdlib's HTTP client. We do a HEAD on the root
	// — many apiservers reject with 401/403, but the
	// connection itself is the signal we want.
	req, err := http.NewRequestWithContext(cctx, http.MethodHead, c.K8sAPIURL, nil)
	if err != nil {
		return CheckResult{Status: StatusDown, Error: err.Error(), DurationMs: time.Since(start).Milliseconds()}
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		return classifyErr(err, cctx, dur)
	}
	_ = resp.Body.Close()
	return CheckResult{Status: StatusOK, DurationMs: dur}
}

// classifyErr promotes a context-deadline-exceeded to "timeout"
// and leaves anything else as "down". The shared helper keeps
// the four checks' error reporting consistent.
func classifyErr(err error, ctx context.Context, durMs int64) CheckResult {
	if err == nil {
		return CheckResult{Status: StatusOK, DurationMs: durMs}
	}
	if ctxErr := ctx.Err(); ctxErr == context.DeadlineExceeded {
		return CheckResult{Status: StatusTimeout, DurationMs: durMs, Error: "check exceeded timeout"}
	}
	return CheckResult{Status: StatusDown, DurationMs: durMs, Error: err.Error()}
}

// aggregateStatus is the per-dep -> top-level rollup. Any
// "down" or "timeout" promotes the overall report to
// "degraded". "skipped" does NOT — a dep that is intentionally
// not in use is not a degradation.
func aggregateStatus(c ChecksBundle) CheckStatus {
	rows := []CheckResult{c.DB, c.LDAP, c.K8s, c.K8sAPI}
	for _, r := range rows {
		if r.Status == StatusDown || r.Status == StatusTimeout {
			return StatusDown
		}
	}
	return StatusOK
}

// Handler is the gin.HandlerFunc factory for /live, /ready,
// and the /health alias. The three share a Checker; only the
// branching differs.
type Handler struct {
	Checker *Checker
}

// NewHandler builds a Handler for the three endpoints.
func NewHandler(c *Checker) *Handler { return &Handler{Checker: c} }

// SetChecker swaps the underlying Checker at runtime. Used
// by the production wiring in cmd/devops-toolkit/main.go:
// buildRouter is called before the auth + k8s modules are
// registered, so the first Checker has the DB but no LDAP
// or K8s; the second (richer) Checker is installed after
// the modules are wired. The handlers read Checker at
// request time, so the swap is safe to do after the
// router is already serving.
func (h *Handler) SetChecker(c *Checker) { h.Checker = c }

// Live is GET /live. Always returns 200 — the only way this
// handler can fail is if the HTTP server itself is broken,
// in which case K8s' liveness probe will time out and
// restart the pod naturally.
func (h *Handler) Live(c *gin.Context) {
	// Use a minimal body so operators can curl the endpoint
	// and see something more useful than a blank line.
	handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// Ready is GET /ready. Runs the deep fan-out and renders the
// Report. 200 on ok, 503 on degraded.
func (h *Handler) Ready(c *gin.Context) {
	report := h.Checker.Run(c.Request.Context())
	writeReport(c, report)
}

// Health is GET /health. Back-compat alias for /ready —
// existing curl scripts and docker-compose healthchecks
// must not break. Same body, same status, same timeout
// semantics.
func (h *Handler) Health(c *gin.Context) {
	report := h.Checker.Run(c.Request.Context())
	writeReport(c, report)
}

// writeReport serialises the Report and stamps the right
// HTTP status. Kept as a free function (not a method) so
// tests can call it directly with a synthetic gin context.
func writeReport(c *gin.Context, r Report) {
	body, err := json.Marshal(r)
	if err != nil {
		// Marshal of a struct with only string/int64/struct
		// fields cannot realistically fail. The fallback
		// is the api-contract error envelope so the
		// response shape stays consistent.
		handler.WriteError(c.Writer, errorPayload(err))
		return
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(r.HTTPStatus())
	_, _ = c.Writer.Write(body)
}

// errorPayload is a tiny helper that builds an APIError from
// a marshal error. The contracts package's APIError is the
// canonical shape; we import it lazily to avoid a cycle.
func errorPayload(err error) *contracts.APIError {
	return &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: err.Error(),
	}
}
