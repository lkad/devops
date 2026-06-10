# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

## Open

### P3 — Run the load test against a live binary, capture baseline numbers

**What:** Run `tests/load/loadgen_test.go` against `:3000` with
`LOAD_VUS=50 LOAD_DURATION=20s`, record the p50/p95/p99 numbers
into a baseline file (e.g. `tests/load/baseline.md`).

**Why:** The load test asserts SLOs (p95 < 500ms, err < 1%) but
has never actually run end-to-end. Today we don't know whether the
binary is comfortably under the SLO or hovering at 480ms p95.
Without a baseline number, we can't tell whether tomorrow's change
regressed performance.

**Estimate:** 20 min CC (binary up + run + analyze + write
baseline.md), ~2 hours human.

---

### P3 — Implement KubeClient.ExecInPod for K8s log-streaming spec

**What:** The `k8s-pod-log-streaming` spec exists at
`openspec/specs/k8s-pod-log-streaming/spec.md` and the
`internal/k8s/logstream/` package handles streaming. The
underlying `KubeClient` does NOT yet have an `ExecInPod` method
that the spec mentions for the SSH-into-pod workflow.

**Why:** Adds the only remaining read/exec method missing from
the production KubeClient now that Ping/ListPods/ListDeployments/
ListServices are real (v0.2.0.0).

**Estimate:** 1 hour CC.

## Completed

### P2 — Service health gauge metric + dashboard panel

**Completed:** 2026-06-11 (v0.2.1.0 candidate)

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

