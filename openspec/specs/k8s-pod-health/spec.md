# k8s-pod-health

## Purpose

P0's health rollup is derived from the most recent
pipeline run — right most of the time, but a deploy
that succeeded an hour ago tells the operator nothing
about the pods that came out of it. This spec makes
**live K8s pod health the primary health signal** for
a service. The pipeline-derived signal remains in the
response (so the operator still has the "what was the
last deploy" answer); it just no longer decides the
green / yellow / red pill.

The K8s subsystem already exposes a `Client` interface
(see `internal/k8s/client.go`) with `ListDeployments`
and a `FakeClient` for tests. This spec uses that
seam rather than introducing a parallel one.

## Requirements

### Requirement: Service → Deployment Mapping

The system SHALL map a Service to a K8s Deployment by
exact name match across every registered cluster.

#### Scenario: Service matches a deployment in one cluster

- **WHEN** a Service with `name = "payments-api"`
  exists and at least one registered cluster has a
  Deployment whose `metadata.name = "payments-api"`
- **THEN** the health rollup considers that
  deployment when computing status

#### Scenario: Service matches no deployment anywhere

- **WHEN** no registered cluster has a Deployment
  whose name matches the Service's name
- **THEN** the rollup returns `status = "unknown"`
  with `reason = "no_k8s_deployment"` and the `k8s`
  block is omitted from the response

#### Scenario: Service matches deployments in multiple clusters

- **WHEN** two registered clusters each have a
  Deployment whose name matches
- **THEN** both are aggregated; a single
  crashlooping pod in either cluster tips the
  service to `degraded`

### Requirement: Health Rule — Healthy

The system SHALL report `status = "healthy"` when every
matched deployment is fully ready.

#### Scenario: All deployments fully ready

- **WHEN** every matched Deployment has
  `available == replicas` and `replicas > 0`
- **THEN** `status = "healthy"` and
  `derived_from = "k8s_pod_health"`

### Requirement: Health Rule — Degraded

The system SHALL report `status = "degraded"` when any
matched deployment is below its replica target.

#### Scenario: One deployment below target

- **WHEN** any matched Deployment has
  `available < replicas` (e.g. `2/3`)
- **THEN** `status = "degraded"` with
  `reason = "insufficient_replicas"`

#### Scenario: Deployment scaled to zero

- **WHEN** a matched Deployment has `replicas == 0`
- **THEN** `status = "degraded"` with
  `reason = "scaled_to_zero"`

### Requirement: Health Rule — Query Failure

The system SHALL report `status = "unknown"` when the
K8s client itself fails — a query error must never
be reported as a service-level `degraded`.

#### Scenario: K8s client returns an error

- **WHEN** the K8s client's `ListDeployments` call
  returns a non-nil error
- **THEN** the rollup returns `status = "unknown"`
  with `reason = "k8s_query_failed"` and the `k8s`
  block is omitted

#### Scenario: Cluster not configured

- **WHEN** no K8s clusters are registered
- **THEN** the rollup returns `status = "unknown"`
  with `reason = "no_k8s_clusters"`

### Requirement: K8s Signal Overrides Pipeline Signal

The system SHALL prefer the K8s signal over the P0
pipeline-derived signal when both are present.

#### Scenario: Last deploy succeeded, pods crashlooping

- **WHEN** the most recent pipeline run is
  `succeeded` but a matched Deployment has
  `available < replicas`
- **THEN** the rollup returns
  `status = "degraded"` (the K8s signal wins)
- **AND** `last_run` in the response still reports
  the succeeded pipeline run

#### Scenario: Last deploy failed, pods healthy

- **WHEN** the most recent pipeline run is `failed`
  but every matched Deployment is fully ready
- **THEN** the rollup returns `status = "healthy"`
  (the K8s signal wins)
- **AND** the operator can still see the failed
  `last_run` to investigate

### Requirement: Wire Shape

The `GET /api/v1/services/:id/health` response SHALL
include a `k8s` block when K8s data is available.

#### Scenario: K8s data present

- **WHEN** the K8s source returns at least one
  deployment
- **THEN** the response body has a `k8s` object
  with `deployments: [...]` and `all_ready: <bool>`

#### Scenario: K8s data absent (no clusters / no
matching deployment / query error)

- **WHEN** the K8s source returns no deployments or
  fails
- **THEN** the `k8s` field is omitted (not present)
  from the response body
- **AND** `derived_from` is `"last_pipeline_run"`
  (the P0 signal carries the response)

#### Scenario: Per-deployment record

- **WHEN** the `k8s` block is present
- **THEN** each entry in `deployments` carries
  `cluster_id`, `namespace`, `name`, `ready`
  (e.g. `"2/3"`), `replicas`, `available`

## Notes

- The "K8s signal overrides pipeline signal" rule
  is a deliberate inversion from P0. Operators do not
  page on a stale "last deploy failed" badge when the
  pods are actually fine; they do page on
  crashlooping pods even if the deploy that produced
  them succeeded.
- The name-match rule is a P1.3 simplification. P2
  adds an explicit `k8s_deployment` column on
  `Service` for the rare case where the catalog
  name and the K8s deployment name diverge.
- Multi-cluster aggregation is a simple OR ("any
  unhealthy = unhealthy here"). A weighted score
  across clusters is a P2 follow-up.
