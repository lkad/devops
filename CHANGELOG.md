# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0.0] - 2026-06-10

A big week. The service catalog, distributed tracing, and real client-go all landed end-to-end. Operators can now see live K8s pod health rolled up per service, click a trace id in an error toast to land on a deep-link page, and the multi-cluster walker actually talks to apiservers instead of returning empty.

### Added

- **Microservice Catalog** — New `internal/servicecatalog` module. A Service entity, health rollup across pipeline-derived and K8s-derived signals, Service ↔ Pipeline association, on-call rotation lookup (per-service then global), and runbook entries. Frontend Services page with list, detail, and health pill. Single round trip via `GET /services/:id` embeds `oncall` + `runbook`.
- **K8s Pod Health as Primary Service Signal** — Live deployment status (`available/replicas`) is now the green/yellow/red pill driver for a service. Pipeline signal stays in the response as "last deploy" context. Spec: `openspec/specs/k8s-pod-health/`.
- **Multi-cluster K8s Walker** — `internal/servicecatalog.MultiClusterK8sSource` aggregates deployment status across every registered cluster, filtered by deployment name == service name. Interface seam keeps the package free of cross-package coupling.
- **Per-cluster K8s ClientRegistry (P1.5)** — `internal/k8s.ClientRegistry` resolves per-cluster `kubernetes.Interface`, caches per process, sticky-errors a cluster whose kubeconfig fails to decrypt/parse so we don't hammer the decrypt path on every health rollup.
- **Real client-go in KubeClient (P1.6)** — `KubeClient.Ping / ListPods / ListDeployments / ListServices` now wrap `k8s.io/client-go` v0.36.1 with typed clientset and in-memory kubeconfig parsing (`clientcmd.RESTConfigFromKubeConfig`). 8 new fake-clientset tests cover happy path, ctx cancel, namespace filter, nil-Spec.Replicas defaults, and apiserver error propagation.
- **OpenTelemetry Tracing** — `internal/observability` middleware accepts inbound `traceparent`, generates one when missing, and writes `X-Trace-Id` to every response header (set before `c.Next` so it survives streaming writes). Production wiring uses stdouttrace by default; OTLP optional.
- **Trace Detail Page** — `/trace/:id` shows trace id, Opened timestamp, Source explanation, "Open in Tempo ↗" deep link (template `<tempo>/api/traces/<traceId>` or override via `VITE_TEMPO_URL` with `{traceId}` placeholder), "Search Logs" deep link, and copy-to-clipboard. Error toasts render trace id as a clickable shortened `8-char…4-char (trace)` link.
- **Prometheus Alert Rules + Alertmanager Wiring** — `deploy/prometheus/rules/` + `deploy/alertmanager/alertmanager.yml`. Per-service alert routing.
- **On-Call Rotation** — `on_calls` table (`service_id` nullable for global shifts) + `CurrentOnCall(svcID, at)` lookup: per-service first, global fallback.
- **Runbook Entries** — `runbook_entries` table + `ListRunbook` (newest first). Service detail page shows entries or an empty state.
- **Pipeline Strategies** — `internal/pipeline/strategy.go`: blue-green / canary / rolling planners. Phases endpoint exposed for the frontend pipeline flow visualization.
- **Physical Host Prober + Metrics Pipeline** — SSH-based prober abstraction (`internal/physicalhost/prober/`), metrics cache with stale-on-failure, InfluxDB async writer, monitor loop, audit adapter.
- **TLS + mTLS** — `internal/server/tls.go` + `cmd/gen-mtls-cert/` certificate generator CLI.
- **Auth Stack** — JWT middleware, LDAP login flow (with connection pooling + retry + group → role mapping), RBAC permission matrix.
- **Dev CLIs** — `cmd/mint-dev-token` (JWT minter), `cmd/ws-smoke` (WebSocket smoke tester), `cmd/gen-mtls-cert` (mTLS cert generator).
- **Grafana Dashboards** — 4 dashboards under `deploy/grafana/dashboards/`: overview, logs, physical-host metrics, pipeline execution. Provisioned via `deploy/grafana/provisioning/dashboards/dashboards.yml`.
- **Load Test** — Go-native `tests/load/loadgen_test.go` (build tag `load`) with SLO enforcement (p95 < 500ms, error rate < 1%). Runs via `go test -tags=load -run TestLoadPhysicalHosts`.
- **Containerlab Topology** — `deploy/containerlab/topology.yml` for real network device testing.
- **CI** — Split into backend + frontend jobs in `.github/workflows/ci.yml`. Backend runs `go vet` + race tests + load smoke. Frontend runs `tsc` + vitest + build.
- **Frontend MetricsPanel** — Reusable component with vitest setup. Physical Hosts page now displays IP and device name (was undefined).
- **Logs Retention + Stats** — Retention policy + stats endpoints in `internal/logs`.
- **Pipeline service_id tests** — Test coverage for service ↔ pipeline association.

### Fixed

- **X-Trace-Id missing from streaming responses** — Middleware was setting the header after `c.Next()`. Gin's response writer commits headers on the first body write; the JSON encoder wrote the body first, so the header never made it onto the response. Unit test passed because `c.String` is non-streaming. Fix: set the header before `c.Next()`. Verified end-to-end on a fresh binary on :18080.
- **Physical host page showed undefined** — Page now displays IP and device name correctly.
- **TraceDetail typo** — `TemPO_URL` search-and-replace artifact corrected to `TEMPO_URL` (compiled because both declaration and use were the same typo).
- **Auth state lost on page reload** — JWT now properly restored from localStorage.

### Changed

- **Pipeline StrategyRunner stub removed** — Unused, deleted in favor of the new strategy planner architecture.
- **Backend router** — All new modules wired in `cmd/devops-toolkit/main.go`. `registerK8sClusterRoutes` now returns the `*k8s.Service` so `registerServiceCatalogRoutes` can hand it to the K8s ClientRegistry constructor.

### For contributors

- New spec docs: `openspec/specs/{service-catalog,observability-p1,k8s-pod-health}/`
- New deps: `k8s.io/client-go` v0.36.1 + `k8s.io/api` + `k8s.io/apimachinery`; OpenTelemetry packages.
- Test count: 28 backend packages green; 14 new/updated tests in `internal/k8s` for the registry + KubeClient rewrite.

---

## [0.1.0.0] - 2026-04-26

### Added

- **LDAP Authentication** — Full LDAP auth with connection pooling, retry logic, group resolution, and role mapping
- **JWT Middleware** — Bearer token validation on all protected routes with user context propagation
- **RBAC Permissions** — Role-based access control (Auditor, Developer, Operator, SuperAdmin) with permission enforcement middleware
- **Auth Handler** — Login, logout, and /me endpoints for session management
- **Config Loader** — YAML config loading with environment variable overrides
- **Project Management** — Full CRUD for BusinessLine → System → Project hierarchy with PostgreSQL persistence
- **Project Audit Logging** — All project management changes are now recorded in the audit log
- **K8s Multi-cluster Filtering** — Multi-cluster support with environment color coding
- **Physical Host Bug Fix** — Fixed mux.Vars path parameter extraction in GetHostHTTP and DeleteHostHTTP
- **Integration Tests** — Comprehensive HTTP integration tests for device, physicalhost, logs, and project modules

### Fixed

- Auth middleware blocking static files — now excludes /static and assets paths
- Auth store rehydration — JWT now properly restored on page reload
- Repository list result slices — initialized to empty arrays instead of nil
- Project integration tests — cleanup before tests to remove stale test data
- K8s container bounds check — panic prevention in GetPods
- Physical host path parameters — uses mux.Vars instead of query params

### Changed

- DevOps Toolkit frontend now served from /devops/ sub-path
- Vite base configuration set to './' for relative asset loading
- ARCHITECTURE.md updated with nginx reverse proxy sub-path configuration
