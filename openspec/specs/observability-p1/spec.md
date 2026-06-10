# observability-p1

## Purpose

P0 closed the "is the service healthy right now" gap via a
derived health rollup. P1 closes the two operational
questions that come next:

1. **Distributed tracing** — when a request fails, an
   operator can follow the trace through every backend
   hop and see which step is slow or broken.
2. **Alert routing** — Prometheus alerts actually reach
   the on-call: alertmanager is wired, has a real config,
   and a baseline set of alert rules fires on the four
   highest-value signals (5xx storm, pipeline failure
   rate, host offline, alertmanager down).

The P1 deliverables are bounded so P2 (real K8s pod
health, runbook pages, on-call rotation) can land as a
separate commit.

## Requirements

### Requirement: Trace Context Propagation

The system SHALL participate in the W3C Trace Context
specification on every HTTP request it handles.

#### Scenario: Incoming request carries traceparent

- **WHEN** an HTTP request arrives at the backend with
  the `traceparent: 00-<trace-id>-<parent-id>-01` header
- **THEN** the system creates a span whose parent_id is
  the value of `<parent-id>` from the header
- **AND** the trace_id matches the header's `<trace-id>`

#### Scenario: Incoming request has no traceparent

- **WHEN** an HTTP request arrives at the backend
  without a `traceparent` header
- **THEN** the system creates a new trace (random
  128-bit trace_id) and a new root span for the request
- **AND** the trace_id appears in the `X-Trace-Id`
  response header

#### Scenario: Response carries X-Trace-Id

- **WHEN** the system returns a response for any HTTP
  request handled by the backend
- **THEN** the response includes the header
  `X-Trace-Id: <32 hex trace id>`
- **AND** the value matches the trace_id of the request's
  root span

#### Scenario: Matched route is recorded on the span

- **WHEN** the system records the root span for a
  request that matched a Gin route template (e.g.
  `/api/v1/services/:id`)
- **THEN** the span carries attributes
  `http.route = "/api/v1/services/:id"` and
  `http.request.method = "<method>"`
- **AND** after the handler runs, the span also has
  `http.response.status_code = <code>` and
  `http.server.duration = <seconds>`

### Requirement: OpenTelemetry Exporter

The system SHALL support a stdout exporter and an OTLP
exporter, switched by env.

#### Scenario: No OTLP endpoint configured

- **WHEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is unset or
  empty
- **THEN** the system uses a noop tracer (no exporters
  attached)
- **AND** every request still produces a span in
  memory and a `X-Trace-Id` response header

#### Scenario: OTLP endpoint configured

- **WHEN** `OTEL_EXPORTER_OTLP_ENDPOINT` is set to a
  reachable collector URL
- **THEN** the system uses the OTLP HTTP exporter and
  ships spans to that endpoint

#### Scenario: Stdout exporter in dev

- **WHEN** `OTEL_EXPORTER=stdout`
- **THEN** every span is written to stdout in JSON
  format, suitable for `docker compose logs -f backend`

### Requirement: Sampling

The system SHALL use parent-based ratio sampling.

#### Scenario: Default sampling rate

- **WHEN** no sampling config is provided
- **THEN** 10% of root spans are sampled
- **AND** a child of a sampled parent is always sampled

#### Scenario: 100% sampling for debug

- **WHEN** `OTEL_TRACES_SAMPLER_ARG=1.0`
- **THEN** every root span is sampled

### Requirement: alertmanager Configuration

The system SHALL have a working alertmanager config
that can route Prometheus alerts to a configured
receiver.

#### Scenario: alertmanager boots with config

- **WHEN** the alertmanager container starts with
  `deploy/alertmanager/alertmanager.yml` mounted
- **THEN** the container reports `level=info msg="Loading
  configuration file"` and binds the configured receivers

#### Scenario: Catch-all receiver exists

- **WHEN** the alertmanager config is loaded
- **THEN** a default route matches every alert and
  routes it to a receiver named `log`
- **AND** the `log` receiver writes the alert payload
  to the alertmanager's stdout (so dev can see alerts
  in `docker compose logs alertmanager`)

#### Scenario: PagerDuty stub is configurable

- **WHEN** the operator sets `PAGERDUTY_SERVICE_KEY` in
  the compose environment
- **THEN** the PagerDuty receiver in the YAML is
  enabled
- **AND** alerts with `severity: page` route to
  PagerDuty, while `severity: log` alerts still go to
  the catch-all

### Requirement: Prometheus Alert Rules

The system SHALL ship a baseline set of alert rules in
`deploy/prometheus/rules/`.

#### Scenario: High 5xx rate fires

- **WHEN** `http_requests_total{status=~"5.."}` grows
  at > 1 req/sec for 2 minutes on any backend instance
- **THEN** Prometheus fires a `high_5xx_rate` alert
  with labels `service=devops-toolkit`,
  `severity=page`

#### Scenario: Pipeline failure rate

- **WHEN** the ratio of failed pipeline runs to total
  pipeline runs in the last 15 minutes is > 0.5
- **THEN** Prometheus fires a `pipeline_failure_rate`
  alert with `severity=page`

#### Scenario: Host offline

- **WHEN** the on-call phone receives an event
  indicating a physical host probe has been failing
  for 5 minutes (recorded in the existing alerts
  subsystem)
- **THEN** Prometheus fires a `host_offline` alert
  with `severity=page`

#### Scenario: alertmanager itself is down

- **WHEN** Prometheus cannot reach alertmanager on the
  configured endpoint for 2 minutes
- **THEN** Prometheus fires an `alertmanager_down`
  alert with `severity=log` (visible in Prometheus UI;
  does not page, since paging alertmanager from itself
  is a cycle)

## Notes

- The OpenTelemetry SDK adds ~3MB to the binary. This is
  accepted; a hand-rolled tracer would cost more in
  maintenance than the binary size.
- The 4 alert rules in `deploy/prometheus/rules/` are
  baseline. Operators add more per-deployment; the
  `service` and `severity` labels are the contract
  between the rules and the alertmanager route tree.
- P1.3 (real K8s pod health as a Service health
  signal) is deliberately out of scope here. The
  derived `last_pipeline_run` health rollup from P0
  remains the source of truth until P1.3 lands.
