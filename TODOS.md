# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

## Open

### P2 — Service health gauge metric + dashboard panel

**What:** Add a Prometheus gauge
`devops_toolkit_service_health_status{service_id, service_name, tier, derived_from}`
that records the latest rollup status (0=unknown, 1=healthy, 2=degraded)
each time `Health.Rollup` is called. Plus a counter
`devops_toolkit_service_health_rollup_total{status, derived_from}` for
trend graphs.

**Why:** The current `service-catalog.json` Grafana dashboard
(deploy/grafana/dashboards/service-catalog.json) shows catalog API
traffic and Postgres-derived service snapshots — but it has NO per-
service health status. An operator looking at the dashboard cannot
tell which service is currently red/yellow/green without clicking
into the frontend Services page.

**Where:**
- New: `internal/servicecatalog/metrics.go` (~80 LOC)
- Edit: `internal/servicecatalog/handler.go` — call `metrics.Record(svc, result)`
  after `Health.Rollup` succeeds (~10 LOC)
- Edit: `cmd/devops-toolkit/main.go` — build the metric set from the
  observability registry, wire it into the catalog handler (~5 LOC)
- New tests: ~60 LOC (counter + gauge update assertions)
- Edit: `deploy/grafana/dashboards/service-catalog.json` — add a
  "Per-Service Status" stat row driven by the new gauge.

**Estimate:** 30 min CC, ~1 day human.

**Reference:** the metric naming convention follows
`internal/observability/metrics.go` (`devops_toolkit_<subsystem>_<name>`).
The Handler.Health() method already calls `h.cat.Get(id)` so it has
the Service struct (name + tier) ready to label the gauge with.

---

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

(empty)
