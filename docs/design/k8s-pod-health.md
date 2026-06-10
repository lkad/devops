# P1.3 — K8s pod health as a Service signal

**Status:** Draft
**Last updated:** 2026-06-10
**Builds on:** [service-layer.md](./service-layer.md) (P0), [observability-p1.md](./observability-p1.md) (P1)
**Spec:** [openspec/specs/k8s-pod-health/spec.md](../../openspec/specs/k8s-pod-health/spec.md)

---

## 1. Why this exists

P0's health rollup answers "is the service healthy?"
with a derived signal: the most recent pipeline run. It
is right most of the time, but a deploy that happened
an hour ago and succeeded tells the operator nothing
about the pods that came out of it. If 3/10 pods are
crashlooping right now, the operator should know — and
should not have to drop into a separate K8s dashboard
to find out.

P1.3 closes that gap by making **live K8s pod health
the primary health signal** for a service. The
pipeline-derived signal remains in the response as
`last_pipeline_run` so the operator still has the
"what was the last deploy" answer; it just no longer
decides the green/yellow/red pill.

## 2. Mapping Service → K8s Deployment

The simplest rule that works for 80% of the world:

> A Service with `name = X` maps to the K8s
> Deployment whose `metadata.name = X` in any namespace.

This is the convention the project already uses for
its existing endpoints (`/k8s/clusters/:id/deployments`
returns deployment objects whose name is shown
verbatim on the page). For the 20% case where the
catalog name and the K8s deployment name differ, an
explicit `k8s_deployment` column will be added in P2;
P1.3 keeps the rule simple.

Search: the K8s client is called for every cluster
that is currently registered. A service is
"associated" with a cluster if at least one deployment
matches the service's name. Multiple matches across
clusters / namespaces are aggregated: a single
crashlooping pod anywhere tips the service to
`degraded`.

## 3. Status rules

| Live signal | Health |
|-------------|--------|
| No K8s deployment found for the service name | `unknown` (with `reason: "no_k8s_deployment"`) — the service may be running outside K8s |
| All deployments have `Available == Replicas` and `Replicas > 0` | `healthy` |
| Any deployment has `Available < Replicas` (e.g. 2/3 ready) | `degraded` (with `reason: "insufficient_replicas"`) |
| All deployments have `Replicas == 0` | `degraded` (with `reason: "scaled_to_zero"`) |
| K8s client error | `unknown` (with `reason: "k8s_query_failed"`) — never report `degraded` from a query failure; that would page on the network |

The K8s signal **overrides** the P0 pipeline-derived
signal. If the last pipeline run succeeded but a pod is
crashlooping right now, the service is `degraded`. If
the last pipeline run failed but every pod is healthy
(the failure was a flaky deploy that was retried), the
service is `healthy`.

## 4. Wire shape change

`GET /api/v1/services/:id/health` extends — does not
replace — the existing P0 response:

```json
{
  "service_id": "svc-abc",
  "status": "degraded",
  "derived_from": "k8s_pod_health",
  "reason": "insufficient_replicas",
  "k8s": {
    "deployments": [
      {
        "cluster_id": "cl-1",
        "namespace": "default",
        "name": "payments-api",
        "ready": "2/3",
        "replicas": 3,
        "available": 2
      }
    ],
    "all_ready": false
  },
  "last_run": { ... },            // unchanged from P0
  "recent_runs": [ ... ]          // unchanged from P0
}
```

The `derived_from` field tells the consumer which
signal won. The `k8s` field is omitted when no K8s
data is available (the cluster is not registered, the
service is not in K8s, etc.) so the frontend can
distinguish "I don't know" from "I checked and it's
fine".

## 5. Non-goals (P1.3)

- **Pod logs** — out of scope. The k8s/logstream
  module already has pod-log streaming for the
  drill-down; this is a separate concern.
- **Per-pod-level alerts** — Prometheus rules already
  cover pod-level signals (the existing
  `devops-toolkit.physicalhost` group; adding
  `k8s_pod_crashlooping` is a one-rule follow-up).
- **Multi-cluster service affinity** — the
  aggregation across clusters is a simple OR
  ("anywhere unhealthy = unhealthy here"). A weighted
  scoring is P2.
- **Explicit `k8s_deployment` column** — the name
  match is enough for P1.3; the column is a P2
  enhancement.

## 6. Test plan (TDD)

| Layer | Test target | Initial red signal |
|-------|-------------|-------------------|
| `internal/servicecatalog/k8s_health_test.go` | Health rules: `Available == Replicas && > 0` → healthy; `Available < Replicas` → degraded | undefined: `k8sHealthSource` |
| `internal/servicecatalog/k8s_health_test.go` | K8s error → `unknown` with `k8s_query_failed` reason (never `degraded` on query failure) | panic on stub |
| `internal/servicecatalog/k8s_health_handler_test.go` | `GET /services/:id/health` includes the `k8s` block when the source returns data | `k8s` field absent from body |
| `internal/servicecatalog/handler_health_test.go` (existing) | when the source is nil, response still has P0 fields (backward compat) | (already green) |
| Full repo | `go test ./...` and `vitest run` | (eventual green) |

The first failing test for each layer is the entry
point — once it exists and the failure is confirmed,
the production change is written to satisfy *that*
test, not the whole layer in one go.
