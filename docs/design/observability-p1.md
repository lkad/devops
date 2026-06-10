# P1 — Observability and Alert Routing

**Status:** Draft
**Last updated:** 2026-06-10
**Builds on:** [service-layer.md](./service-layer.md) (P0), [observability/metrics.go](../../internal/observability/metrics.go) (P0)
**Spec:** [openspec/specs/observability-p1/spec.md](../../openspec/specs/observability-p1/spec.md)

---

## 1. Why this exists

P0 closed the "is the service healthy right now" gap via a
derived health rollup. P0 deliberately deferred the three
operational questions that come next:

| Question | P0 answer | P1 answer |
|----------|-----------|-----------|
| "What just happened in the request that failed?" | "Look at the logs and search" | **Distributed trace** showing every hop |
| "Why didn't I get paged when the alert fired?" | "alertmanager is not wired" | **Real on-call routing** with receivers, suppression, and dedup |
| "Is the service really running or is it just stale?" | "We look at the last pipeline run" | **K8s pod health** as a live signal (P1.3) |

This document covers P1.1 (tracing) and P1.2 (alertmanager
routing). P1.3 (real K8s pod health) is left for a future
commit because the project does not yet have live K8s
clusters to query against in dev mode.

## 2. P1.1 — Distributed tracing (OpenTelemetry)

### 2.1 Goals

- A single request to the backend produces a **trace** with
  one span per HTTP route and one child span per
  service-catalog operation it triggers.
- The frontend can correlate a failed request to a trace
  via the `X-Trace-Id` response header.
- The same trace can be exported to a local stdout
  exporter (dev) or any OTLP collector (production)
  via a single env var flip (`OTEL_EXPORTER_OTLP_ENDPOINT`).

### 2.2 Non-goals (P1.1)

- Span-level database query tracing (would require
  `otelsql` driver wrappers; deferred to a later iteration).
- Frontend span creation (browser-side tracing; out of
  scope for backend-only P1).
- Tail-based sampling, custom samplers, exemplars.
  Head-based parent-ratio sampling is enough for v1.

### 2.3 Implementation surface

- `internal/observability/tracing.go` — tracer init,
  OTLP exporter wiring, and a middleware that extracts
  the W3C `traceparent` header on incoming requests and
  injects it on outgoing.
- `internal/observability/tracing_test.go` — TDD: the
  middleware extracts an incoming `traceparent`, the
  outgoing response carries a `X-Trace-Id` header, the
  span records the matched route + status + duration.
- `cmd/devops-toolkit/main.go` — `observability.NewTracing`
  is called before any route is registered, so the
  Gin handler chain is already wrapped in the tracing
  middleware by the time the first request arrives.
- A second, narrower middleware in
  `internal/servicecatalog/handler.go` is **not**
  added — the global tracing middleware already creates
  one span per HTTP request, and the catalog business
  methods do not need their own spans in P1.1 (they
  will, in P1.1.1).

### 2.4 Sampling

Default parent-based `TraceIDRatioBased(0.10)` — sample
10% of incoming traces. The denominator is dev traffic;
production should set `OTEL_TRACES_SAMPLER_ARG=1.0` or
tune per workload.

### 2.5 Wire shape

Request:
```
GET /api/v1/services
traceparent: 00-<32 hex trace id>-<16 hex span id>-01
```

Response:
```
HTTP/1.1 200 OK
X-Trace-Id: <32 hex trace id>
```

The frontend can log the `X-Trace-Id` on errors; the
on-call can paste it into Grafana / Tempo to find the
exact trace.

## 3. P1.2 — Alert routing (alertmanager + on-call)

### 3.1 Goals

- Alertmanager is actually wired: Prometheus fires
  alerts → alertmanager receives them → routes to a
  configured receiver (Slack webhook / PagerDuty / log).
- A single config file `deploy/alertmanager/alertmanager.yml`
  declares receivers, routes, and group_by.
- Sample Prometheus rules in
  `deploy/prometheus/rules/` cover the four highest-value
  signals: service down (5xx > threshold), pipeline
  failure rate, physical host offline, alertmanager
  itself down (the "who watches the watchers" rule).

### 3.2 Non-goals (P1.2)

- On-call rotation / PagerDuty integration
  (configuration-only; the receiver YAML supports a
  `webhook_config` PagerDuty stub but it is not
  exercised in dev).
- Slack-specific routing (the YAML supports a
  `slack_configs` block but the URL is left empty in
  dev).
- Per-service suppression windows (deferred to a later
  iteration; the `SuppressedAlert` view in the existing
  spec covers audit, not routing).

### 3.3 Implementation surface

- `deploy/alertmanager/alertmanager.yml` — minimal but
  realistic config: one global route, one catch-all
  receiver ("log"), and a PagerDuty stub.
- `deploy/prometheus/rules/alerts.yml` — four rules
  (high_5xx, pipeline_failures, host_offline,
  alertmanager_down). Each rule carries the spec's
  `service` and `severity` labels so the operator can
  filter on the alertmanager side.
- `deploy/docker-compose.yml` — mount both files
  into the alertmanager and prometheus containers,
  point them at each other.

### 3.4 Backward compatibility

- The existing Alerts.tsx page continues to work
  without change; the route is the same.
- The existing alerts subsystem tests
  (`internal/alerts/suppression_real_test.go`) keep
  passing because the suppression logic is
  in-process and does not touch the alertmanager
  wiring.

## 4. Risks

| Risk | Mitigation |
|------|------------|
| OpenTelemetry SDK adds ~3MB to the binary | Acceptable; the only alternative is a hand-rolled tracer which is worse for the next 3 years |
| `traceparent` header missing on internal calls | Acceptable — we extract when present, generate when not; the OpenTelemetry SDK does the right thing |
| alertmanager.yml breaking the existing test stack | The dev compose file has the alertmanager container; we just add the config-file mount and the prometheus → alertmanager wiring. No test depends on alertmanager's exact behavior |
| The 4 alert rules generating noise in dev | The rules use a 1m rate-of-change threshold so dev-time traffic does not page anyone; production sets the receiver URL |

## 5. Test plan (TDD)

| Layer | Test target | Initial red signal |
|-------|-------------|-------------------|
| `internal/observability/tracing_test.go` | middleware extracts `traceparent`, response carries `X-Trace-Id`, span has route + status attrs | compile error: undefined `Tracing` |
| `internal/observability/tracing_test.go` | nil-OTEL config falls back to noop tracer (no crash when no collector is configured) | panic on missing endpoint |
| `deploy/alertmanager/alertmanager.yml` (static) | the YAML parses (`yq` / `yaml-validator` smoke test) | invalid yaml |
| `deploy/prometheus/rules/alerts.yml` (static) | the YAML parses | invalid yaml |
| Full repo | `go test ./...` and `vitest run` | (eventual green) |

The first failing test for each layer is the entry
point — once it exists and the failure is confirmed,
the production change is written to satisfy *that*
test, not the whole layer in one go.
