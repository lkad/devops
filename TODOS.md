# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

## Open

(none right now)

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

