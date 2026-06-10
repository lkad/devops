# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

## Open

### P2 — Wire KubeClient.GetLogsBySelector to the HTTP layer

**What:** Add a route that calls `KubeClient.GetLogsBySelector`.
The KubeClient method shipped in commit `f51e9cce` but no
handler exists yet. A natural place is
`GET /api/v1/k8s/clusters/{clusterID}/namespaces/{ns}/logs?labelSelector=...&tail=50`.

**Why:** The dashboard / log-search UI is the user-facing
consumer. Without a route the method is a write-only
in-process capability.

**Estimate:** 30 min CC, ~1-2 hours human (RBAC scope,
container filter, tail-line caps).

---

### P2 — Wire KubeClient.ExecInPod to Handler.Exec (replace 403 stub)

**What:** `Handler.Exec` at `internal/k8s/handler.go:225-247`
still calls `h.svc.Exec(id, ns, pod, req.Command)` (the 403
stub at `service.go:384-395`, gated behind the
`feature_k8s_exec` flag). The KubeClient method shipped in
commit `802e5dbb` but the route is not rewired.

**Scope:**
- Update `execRequest` to include `container` and
  `timeout_seconds` per `openspec/specs/k8s-pod-exec/spec.md`.
- Update `Handler.Exec` to call the registry's KubeClient
  (need to thread the registry into the handler, similar
  to the catalog's pattern).
- Retire the `feature_k8s_exec` flag and the 403 stub once
  the real path is wired.
- Update the existing `TestHandler_ExecStub_DisabledByDefault`
  test to assert the new path; the assertion changes
  from "returns 403 when feature flag off" to "returns 200
  with the spec'd wire shape".

**Why:** Without the route the spec's "operator shells into
a pod from the web UI" workflow is unreachable. The
method exists; only the glue is missing.

**Estimate:** 1 hr CC, ~2-3 hours human (auth/RBAC scoping,
feature-flag retirement, integration test).

---

### P3 — Run promtool check rules pre-merge on alert YAMLs

**What:** `deploy/prometheus/rules/catalog-degraded.yml` was
not run through `promtool check rules` in the audit.
PromQL silently accepts typoes; a CI gate would catch
"reference an undefined recording rule" before the rule
file ships.

**Estimate:** 15 min CC (install promtool in CI, add the
step).

## Completed

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

