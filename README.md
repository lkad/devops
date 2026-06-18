<div align="center">

# DevOps Toolkit

**Internal platform that runs 100+ nodes across two data centers — CI/CD pipelines, multi-cluster Kubernetes, physical hosts, a service catalog with on-call and runbooks, and a Prometheus-grade observability stack.**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](#)
[![React 18](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](#)
[![PostgreSQL 14+](https://img.shields.io/badge/PostgreSQL-14+-336791?logo=postgresql&logoColor=white)](#)
[![License: MIT](https://img.shields.io/badge/License-MIT-22c55e)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.5.1.0-22d3ee)](VERSION)

[Why this exists](#why-this-exists) · [Who is this for](#who-is-this-for) · [Features](#features) · [Screenshots](#screenshots) · [Quick start](#quick-start) · [Architecture](#architecture) · [Contributing](#contributing)

</div>

> 📖 **Languages:** [English](README.md) · [简体中文](README.zh.md) — see [docs/i18n/SPEC.md](docs/i18n/SPEC.md) for how the project handles translations of both docs and the UI.

---

## Why this exists

If you have ever stitched together **Jenkins + Argo + Prometheus + Loki + a hand-rolled audit log + a hand-rolled device state machine**, you have also kept 14 tabs open and context-switched your way through a single deploy. This project is what an SRE team builds after living that pain. One Go binary, one React SPA:

- A **device lifecycle** with explicit state machine — `online` → `monitoring_issue` → `offline`, with `maintenance` as a parallel hold
- **Pipelines that run for real** with blue-green / canary / rolling strategies and phase-level tracking
- **Real `client-go`** talking to registered K8s clusters (k3d, kind, standard, EKS, TKE) — not a custom abstraction
- **Pod logs streamed over WebSocket** and **typed exec** into a terminal widget
- An **audit trail** for every write across 8 modules, with `actor_id` taken from the JWT subject — never from the request body
- **7 production-shaped Grafana dashboards** out of the box, including two role-based entry points

The code is the implementation, not a spec: ~25,700 lines of Go, 33 packages, 31 test packages all green, and an 8-node containerlab topology that runs locally so you can poke at it before you trust it.

---

## Who is this for

| You are… | Open this dashboard first |
|---|---|
| The on-call operator woken at 03:00 | [Ops view](docs/screenshots/ops-view.png) — SLO row, top error routes, flapping hosts, alert state |
| The engineer shipping a pipeline or endpoint | [Developer view](docs/screenshots/developer-view.png) — *your* route traffic / 5xx / p95, *your* pipeline success rate |
| The SRE / platform lead checking the whole fleet | [Overview](docs/screenshots/overview.png) — system-wide health at a glance |
| The hardware / DC operator tracking one host | [Physical host metrics](docs/screenshots/physical-host-metrics.png) — per-host CPU / memory / disk |

The two role-based dashboards (`developer-view`, `ops-view`) landed in v0.5.1.0 — they are the entry points. The other five are per-domain drill-downs, linked from the bottom row of each entry.

---

## Features

### 🛠  Infrastructure that gets out of the way

- **Single Go binary** — `go build -o devops-toolkit ./cmd/devops-toolkit` → ~95 MB static binary, distroless-compatible
- **Single-page React frontend** — TypeScript + Vite + Zustand, dark ops theme, 14 pages
- **One-process deployment** — listens on `:3000`, serves the SPA, the JSON API, and WebSocket on the same port
- **PostgreSQL or SQLite** — same GORM models; SQLite by default for dev, Postgres for prod
- **TLS / mTLS ready** — `internal/server/tls.go` plus `cmd/gen-mtls-cert/` for cert generation
- **Bilingual UI** — i18next + react-i18next, en / zh-CN, with a `LanguageSwitcher` in the header

### 🔄  Device and physical-host lifecycle

- **State machine** (`internal/physicalhost/monitor.go:32`): `online` → `monitoring_issue` (1 failure) → `offline` (3 in a row) → `online` (1 success); `maintenance` is a parallel hold with its own audit trail
- **Real SSH prober** (`internal/physicalhost/prober/sshprober.go`): ed25519 key auth, optional connection pool, per-host `time.AfterFunc` timeout
- **TCP fallback prober** for when no key is configured — port-22 reachability, useful in service-mesh environments
- **Per-host metrics to InfluxDB** — CPU / memory / disk as time series, async buffered writer (1024-entry buffer, 2 workers)
- **VM and network device coverage** — vSphere / KVM hypervisor discovery, SNMP / NETCONF for switches and routers, with `Fake*` mock clients for offline development
- **Active network discovery** — scan a subnet, probe TCP/22 and UDP/161, surface PENDING devices for human approval

### 🚀  CI/CD pipelines with real strategies

- **blue-green / canary / rolling** strategy planner (`internal/pipeline/strategy.go`) — three concrete plans, one interface
- **Phase-level tracking** — `strategy.Plan` resolves to ordered steps; a state machine watches each
- **Run history** — every run and every step lands in Postgres with `status` / `duration_ms` / `error_message` / `exit_code` / `output_truncated`
- **p50 / p95 / p99 duration percentiles** computed via `percentile_cont` in the dashboard SQL
- **Rollback on failure** — blue-green flips traffic back, canary halts the next weight step, rolling pauses for human approval

### ☸️  Multi-cluster Kubernetes

- **In-memory kubeconfig** — registered clusters get a per-cluster `ClientRegistry` (`internal/k8s/registry.go`); no kubeconfig written to disk
- **Real `client-go` typed clientset** (`internal/k8s/client.go`) — not a custom abstraction
- **Pod log streaming** — WebSocket + SSE + one-shot endpoints unified at `/api/v1/k8s/clusters/:clusterID/pods/:namespace/:pod/logs[/{stream,sse}]`
- **Pod exec over SPDY** — `k8s.pods.exec` permission, terminal widget in the frontend
- **Pod ↔ node ↔ project view** — see which business line owns the workload running on a given node, across all clusters
- **Historical log query** — Loki / Elasticsearch / K8s-native backends, switchable via `LOG_STORAGE_BACKEND`

### 📋  Service catalog with on-call and runbooks

- **Microservice entity** with `tier` (`critical` / `standard` / `internal`) and `derived_from` (`k8s_pod_health` / `last_pipeline_run` / `manual`)
- **Health rollup** — a gauge emits `0=unknown / 1=healthy / 2=degraded` per service, alertable
- **On-call rotation** + **runbook entries** with write APIs (`POST /api/v1/services/:id/oncall`, `POST /:id/runbook`)
- **Service ↔ pipeline relation** — bind a service to the pipeline that deploys it; one view shows health + deploy cadence

### 🔐  Auth and RBAC

- **LDAP or `dev_bypass`** — `internal/auth/middleware.go:137` short-circuits to a fake `RoleOperator` user when the `X-User: <name>` header is set in dev
- **JWT in prod** — `cmd/mint-dev-token` mints a development token
- **Per-permission matrix** (`internal/auth/rbac/matrix.go`) — 5 roles × 30+ permission keys, including the rare `PermissionViewAuditLogProject` for the `ScopedAuditor` role (sees audit logs filtered to their tenant, not others')
- **Per-project membership middleware** — `requireMembership` enforces "user is in the project → can touch the resource", even when the role matrix says yes

### 📊  Observability that earns its keep

- **Prometheus `/metrics`** — `http_requests_total{route, method, status}` + `request_duration_seconds` histogram + Go runtime metrics
- **Monitor-loop metrics** — `monitor_loop_iterations_total`, `monitor_loop_errors_total{host_id}`, `monitor_loop_last_tick_timestamp_seconds` (so you can alert on "the loop is stuck")
- **Service health metric** — `service_health_status{service_id, tier, derived_from}` (gauge 0/1/2)
- **Audit trail in Postgres** — `actor_id` from JWT subject (never from the body), `metadata` JSON, `ip_address`, `user_agent`, every write across 8 modules
- **Structured logs** — `log/slog` JSON output, request ID propagated via `X-Trace-Id`
- **OpenTelemetry tracing** — stdouttrace by default, OTLP optional, parent-based ratio sampling

### 🖥  Frontend that does not get in the way

- **Dark ops theme** — terminal-extension aesthetic, JetBrains Mono for data, Geist for UI
- **Real-time data** — WebSocket-driven service health, monitor status, log tail
- **14 pages**: Dashboard, Services, Pipelines, Physical Hosts, K8s Clusters, Logs, Metrics, Alerts, Audit, Projects, Devices, Discovery, Trace Detail, Login
- **Vitest** for React components, RTL for interaction, `tsc --noEmit` in CI

---

## Screenshots

### Ops view — for the on-call operator

[![](docs/screenshots/ops-view.png)](docs/screenshots/ops-view.png)

SLO row (traffic / error budget / p95 / monitor-loop staleness) → "What's broken" tables (top error routes, flapping hosts) → health trends (tick rate, 2xx/4xx/5xx stacked, pipeline failure rate) → alert state (services degraded, stuck-running pipelines, hosts offline %). Bottom row links to all 5 per-domain dashboards.

### Developer view — for the engineer shipping a feature

[![](docs/screenshots/developer-view.png)](docs/screenshots/developer-view.png)

Set `my_route` template var to your route prefix (e.g. `/api/v1/pipelines`) and `project` to your project UUID — top stats filter to your work, per-route top-10 charts stay global, "My project's pipelines" row scopes to your UUID, and the live error stream surfaces `level="error"` lines from the backend.

### Overview — system health at a glance

[![](docs/screenshots/overview.png)](docs/screenshots/overview.png)

Original entry point — HTTP traffic, p95 latency, error rate, pipeline outcomes, host availability. Good for the team standup screen.

### Physical host metrics — per-host time series

[![](docs/screenshots/physical-host-metrics.png)](docs/screenshots/physical-host-metrics.png)

Per-host CPU / memory / disk time series from InfluxDB, paired with stale-host detection from Postgres. (The dev compose does not include InfluxDB, so this panel shows the layout but no data — wire up InfluxDB in production to fill it.)

---

## Quick start

Needs **Docker** (or **Podman**) and **Go 1.25+**. One script brings up the full dev environment:

```bash
git clone https://github.com/your-org/devops-toolkit.git
cd devops-toolkit
bash scripts/dev-env-up.sh
```

The script:

1. Brings up the 7-service compose stack (Postgres, Loki, Prometheus, Grafana, Alertmanager, Redis, plus the backend)
2. Deploys the 8-node containerlab topology (2 DCs × 4 nodes: each DC has 1 core switch + 3 sshd hosts)
3. Generates an ed25519 prober keypair and injects the public key into the containerlab nodes
4. Registers every physical host as a `device` + `physical_hosts` row
5. Triggers one manual probe pass so dashboards have data before the first 1-minute tick
6. Prints the access URLs

Access after the script completes:

| Service | URL | Credentials |
|---|---|---|
| Backend API | http://localhost:3000 | header `X-User: alice` (dev_bypass) |
| Frontend | http://localhost:5173 | — |
| Grafana | http://localhost:3001 | `admin` / `admin` |
| Prometheus | http://localhost:9090 | — |
| Loki | (no UI; query through Grafana) | — |

Tear down: `bash scripts/dev-env-down.sh`.

### Just the binary, no full stack

```bash
go build -o devops-toolkit ./cmd/devops-toolkit
./devops-toolkit      # listens on :3000, SQLite at tests/fixtures/db/dev.db
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  Frontend (React 18 + TS + Vite, :5173)                          │
│  Zustand state · WebSocket (gorilla/ws) · 14 pages               │
│  i18next + react-i18next (en / zh-CN) · LanguageSwitcher         │
└──────────────────────────┬───────────────────────────────────────┘
                           │ REST + WS  (X-User: alice in dev)
┌──────────────────────────▼───────────────────────────────────────┐
│  Go backend (Gin, :3000)                                          │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │ device   │ │ pipeline │ │ k8s      │ │ physical │ │ logs   │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │ alerts   │ │ service  │ │ project  │ │ audit    │            │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘            │
│                                                                  │
│  internal/auth (LDAP/JWT/RBAC) · internal/observability (OTel)   │
│  internal/middleware (chain order: CORS→Recovery→Tracing→        │
│    Metrics→Auth→RBAC) · internal/ws (real-time hub)              │
└──────────────────────────┬───────────────────────────────────────┘
                           │
   ┌──────────┬──────────┼──────────┬──────────┐
   ▼          ▼          ▼          ▼          ▼
postgres   redis     prometheus   loki     influxdb
(5432)     (6379)     (9090)     (3100)    (8086)
                                          ▲
                                          │ devops-toolkit async writer
                                          │ (per-host CPU/mem/disk)
```

Deeper architecture: [ARCHITECTURE.md](ARCHITECTURE.md) · Specs: [openspec/specs/](openspec/specs/) · Per-module design: [docs/BACKEND.md](docs/BACKEND.md), [docs/FRONTEND.md](docs/FRONTEND.md).

The local dev environment is an 8-node containerlab topology (2 DCs × 4 nodes: 1 core switch + 3 sshd hosts per DC) — production scales the same REST + WebSocket APIs to **100+ nodes across multiple regions**.

---

## Module map

| Module | Path | What it does |
|---|---|---|
| Pipeline | `internal/pipeline` | blue-green / canary / rolling strategies, run history, phase tracking |
| K8s | `internal/k8s` | in-memory kubeconfig, multi-cluster registry, real `client-go` |
| K8s logs | `internal/k8s/logstream` | WS + SSE + one-shot pod log endpoints |
| Physical host | `internal/physicalhost` | state machine, SSH monitor loop, InfluxDB writer |
| Service catalog | `internal/servicecatalog` | microservices, on-call, runbook, health rollup |
| Project | `internal/project` | business line → system → project hierarchy, FinOps hooks |
| Device | `internal/device` | unified device inventory (network / container / physical) |
| Discovery | `internal/discovery` | active network scan (SNMP / SSH) → PENDING devices |
| Logs | `internal/logs` | multi-backend (Local / ES / Loki), retention, saved filters |
| Alerts | `internal/alerts` | multi-channel (Slack / Webhook / Email / Log) |
| Auth | `internal/auth` | LDAP + JWT + RBAC matrix + dev_bypass |
| Audit | `internal/audit` | every mutating action across 8 modules |
| Middleware | `internal/middleware` | CORS → Recovery → Tracing → Metrics → Auth → RBAC chain |
| Observability | `internal/observability` | Prometheus exporter + OpenTelemetry tracing |
| WebSocket | `internal/ws` | real-time event hub (gorilla/websocket) |
| Frontend | `frontend/src/` | React 18 + Vite + Zustand, 14 pages, dark ops theme |

Each module has deeper docs under `openspec/specs/<module>/` and `docs/<module>.md` when the implementation needs it. Start with [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md).

---

## Auth & RBAC quirks

A few things that trip up new contributors. All of them are intentional and have file:line citations:

- **Auth bypass in dev:** `X-User: alice` header → fake `RoleOperator` user, no JWT needed. The handler at `internal/auth/middleware.go:137` short-circuits when `LDAP.dev_bypass=true`. `scripts/seed-data.sh` used to be missing this header (401 on every POST); the bug is fixed, but if you write your own client, set the header.
- **Per-project access:** the role matrix says yes, but the `requireMembership` middleware says no if the user isn't in the project. Even `RoleSuperAdmin` (if you invent one) gets the project check. The bypass is `systemCtx` in the hostproject package.
- **Audit `actor_id` comes from the JWT subject**, never from the request body. This is a non-negotiable invariant — the audit trail's value depends on it. If you're tempted to take `actor_id` from `c.Query("actor_id")`, don't.
- **Route param names are load-bearing.** The k8s logstream handler at `internal/k8s/logstream/handler.go` uses `:clusterID` (not `:id`) to match the k8s handler's param name. Gin panics on param-name conflicts, so a refactor that "fixes" `:clusterID` to `:id` will break the server at startup.

---

## Development

```bash
# Tests
go test ./...                                 # 31 packages
go test -race ./...                           # race detector
cd frontend && npx vitest run                 # React components
npx tsc --noEmit                              # type check

# Regenerate i18n locales (CI doesn't run this — it's a developer tool)
# `keepRemoved: true` in i18next-parser.config.js ensures
# hand-translated content isn't wiped on every run.
cd frontend && npm run i18n:extract
```

Commit conventions:
- Module-level changes: `feat(physicalhost):` or `fix(pipeline):`
- Cross-cutting: `docs(readme):`, `chore(deps):`
- The `Co-Authored-By: Claude <noreply@anthropic.com>` trailer is expected on all commits authored with this tool. Don't strip it.

---

## Project status

Current version: **v0.5.1.0** (see [VERSION](VERSION))

- ✅ 31 backend packages, tests green (`go test ./...`)
- ✅ 7 production-shaped Grafana dashboards (4 per-domain + 3 system: overview, dev-view, ops-view)
- ✅ 5 Helm sub-charts scaffolded (cert-manager / external-secrets / sealed-secrets / monitoring / network-policies)
- ✅ Auth + RBAC + cross-tenant enforcement landed
- ✅ Production-readiness audit #1–#6 closed (2026-06-10)

Release history: [CHANGELOG.md](CHANGELOG.md) · Open work: [TODOS.md](TODOS.md)

---

## Versioning

Semantic: `MAJOR.MINOR.PATCH.BUILD` (e.g. `0.5.1.0`). Bumps happen on release commits, not per-merge. See [CHANGELOG.md](CHANGELOG.md) for the diff between versions; see [VERSION](VERSION) for the current value.

---

## Documentation

| Doc | When |
|---|---|
| [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md) | Start here — the doc tree, reading order, authority map |
| [PRD.md](PRD.md) | What we are building (product spec) |
| [openspec/specs/](openspec/specs/) | How we are building it (formal tech specs — authoritative) |
| [REQUIREMENTS.md](REQUIREMENTS.md) | Backend feature completion status |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture, request flow, data flow |
| [docs/BACKEND.md](docs/BACKEND.md) | Backend code conventions |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Frontend implementation spec |
| [DESIGN.md](DESIGN.md) | Design system (colors, type, motion) |
| [DEPLOY.md](DEPLOY.md) | Production deployment guide |
| [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md) | Bringing up the 8-node test environment |
| [docs/i18n/SPEC.md](docs/i18n/SPEC.md) | i18n spec (both doc and UI translation rules) |
| [TODOS.md](TODOS.md) | Open work items |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [CONTRIBUTING.md](CONTRIBUTING.md) | — *not yet written* (see TODOS) |

---

## Contributing

The codebase is small enough to read end-to-end in a weekend. To contribute:

1. **Pick an item from [TODOS.md](TODOS.md)** — P0 / P1 items have file:line references and audit context
2. **Branch from `main`** — `git checkout -b feat/<short-slug>`
3. **Change code** — if the package has a `_test.go` sibling, write a test; match the comment density of the surrounding code
4. **Run the suite** — `go test -count=1 -p 1 -timeout 600s ./...`
5. **Open a PR** — describe "what" + "why", link the TODO item or audit report
6. **Do not commit keys** — `deploy/secrets/prober/id_ed25519` is in `.gitignore`; the `.pub` is generated locally

Conventions are in the code, not in a style guide. Match the surrounding comment density, naming, and error-wrapping pattern (`fmt.Errorf("module.X: %w", err)`).

---

## License

[MIT](LICENSE) — Copyright (c) 2024 DevOps Toolkit contributors
