# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

## Open

### P3 — Run promtool check rules pre-merge on alert YAMLs

**What:** `deploy/prometheus/rules/catalog-degraded.yml` was
not run through `promtool check rules` in the audit.
PromQL silently accepts typoes; a CI gate would catch
"reference an undefined recording rule" before the rule
file ships.

**Estimate:** 15 min CC (install promtool in CI, add the
step).

## Completed

### P2 — Wire KubeClient.GetLogsBySelector + ExecInPod to HTTP layer

**Completed:** 2026-06-10 (commit `9382c56d`)

**Shipped (single atomic commit, two parallel subagents):**
- `GET /api/v1/k8s/clusters/:clusterID/namespaces/:ns/logs`
  route that fans out via `KubeClient.GetLogsBySelector`.
  Required `labelSelector`, optional `container` / `tail`
  (1-1000, default 100) / `since` (RFC3339, capped at
  30d via `logstream.MaxSinceWindow`). 10 new tests in
  `internal/k8s/logs_handler_test.go`.
- `POST /api/v1/k8s/clusters/:clusterID/namespaces/:ns/pods/:pod/exec`
  route that calls `KubeClient.ExecInPod`. Spec wire shape
  `{command, container, timeout_seconds?}` ->
  `{exit_code, stdout_lines, stderr_lines, duration_ms}`.
  Replaces the 403 stub. The `feature_k8s_exec` flag,
  `Service.Exec` stub, and `ExecResult` legacy type are
  retired. 9 new handler tests + 3 new service tests.
- 6 new `contracts.ErrorCode` constants for the spec's
  error code surface.
- `:id` -> `:clusterID` URL param rename across all k8s
  routes (non-API-breaking; URL param names are not part
  of the HTTP contract; required by Gin's same-name-
  wildcard conflict check).

**Coordination note:** the two subagents ran in parallel
and their changes interlocked — Agent 2 (ExecInPod) had
to rename the existing `:id` to `:clusterID` to satisfy
Gin so Agent 1 (GetLogsBySelector) could add its new
route under the same prefix. The rename is in
`internal/k8s/handler.go` (param + 4 handler bodies).

---

### P3 — Run the load test against a live binary, capture baseline numbers

**Completed:** 2026-06-10 (short `2d650276`)

**Shipped:** `tests/load/baseline.md` with three run levels
(50/100/200 VUs) against a fresh sqlite binary. **p95 list
never exceeds 11ms** (SLO is 500ms — 45-80x headroom), zero
errors across 20,000 requests. Reproducer command in the
baseline file's "How to reproduce" section.

**Side correction:** the earlier P3-ExecInPod item was
miswritten — the `k8s-pod-log-streaming` spec does NOT
actually mention exec. ExecInPod is a separate operator
capability (kubectl exec) that would belong in its own spec;
not v0.2 work. Removed from the TODO list rather than
silently dropped.

---

### P2 — Service health gauge metric + dashboard panel

**Completed:** 2026-06-10 (commit `2d650276`)

**Shipped:** `internal/servicecatalog/metrics.go` (gauge +
counter + Record + Reset), wired into `Handler.Health()` after
every rollup and `Handler.Delete()` to drop stale series.
`main.go`: `buildRouter` now returns the observability Metrics
so `registerServiceCatalogRoutes` can register the catalog
gauges on the same /metrics endpoint. 10 unit tests covering
the enum mapping, nil-safety, gauge overwrite, counter
accumulation, partial-delete, and gather-payload health. 3
new Grafana panels: per-service status (color-coded
gray/green/red), rollup rate by status, rollup rate by signal
source.

