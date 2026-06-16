<div align="center">

# DevOps Toolkit

**The internal platform for managing 100+ nodes across multi-region data centers — CI/CD pipelines, multi-cluster Kubernetes, physical hosts, service catalog with on-call & runbooks, and a Prometheus-grade observability stack.**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](#)
[![React 18](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](#)
[![PostgreSQL 14+](https://img.shields.io/badge/PostgreSQL-14+-336791?logo=postgresql&logoColor=white)](#)
[![License: MIT](https://img.shields.io/badge/License-MIT-22c55e)](#)
[![Version](https://img.shields.io/badge/version-0.5.1.0-22d3ee)](VERSION)

[Quick Start](#quick-start) · [Features](#features) · [Screenshots](#screenshots) · [Architecture](#architecture) · [Contributing](#contributing)

</div>

> 📖 **Languages:** [English](README.md) · [简体中文](README.zh.md) — see the [i18n spec](docs/i18n/SPEC.md) for how the project handles translations of both docs and the UI.

---

## Why DevOps Toolkit

If you've ever stitched together **Jenkins + Argo + Prometheus + Loki + a custom audit trail + a hand-rolled device state machine** and ended up with 14 tabs open, this project is the result of that pain being felt across an SRE team. It's a single Go binary + a React SPA that:

- Owns the **device lifecycle** (state machine: `online` → `monitoring_issue` → `offline` → `maintenance`)
- Runs **blue-green / canary / rolling** pipelines with explicit phases
- Speaks **real `client-go`** to registered K8s clusters (k3d, kind, standard, EKS, TKE)
- Streams **pod logs over WebSocket** and **pods exec** through a typed terminal
- Emits an **audit trail** for every mutating action across 8 modules
- Ships with **7 production-shaped Grafana dashboards** out of the box

The codebase is the implementation, not a spec: ~25,700 lines of Go, 33 packages, 31 test packages green, two role-based Grafana entry points that drop you into the right view depending on whether you're paged or shipping a feature.

---

## Who is it for

| If you are… | Open this dashboard first |
|---|---|
| **On-call operator** paged at 3am | [Ops view](docs/screenshots/ops-view.png) — SLO, top error routes, flapping hosts, alert state |
| **Engineer** shipping a pipeline or endpoint | [Developer view](docs/screenshots/developer-view.png) — *my* route's traffic / 5xx / p95, *my* pipeline's success |
| **SRE / platform owner** checking the fleet | [Overview](docs/screenshots/overview.png) — system-wide health at a glance |
| **Hardware / DC operator** tracking a host | [Physical host metrics](docs/screenshots/physical-host-metrics.png) — CPU / memory / disk per host |

The two role-based dashboards (`developer-view`, `ops-view`) were added in v0.5.1.0 — they're the entry points, the other five are per-domain drill-downs linked from the bottom row of each.

---

## Features

### 🛠  Infrastructure that runs

- **Single Go binary** — `go build -o devops-toolkit ./cmd/devops-toolkit` → 95MB static binary, distroless-compatible
- **Single-page React frontend** — TypeScript + Vite + Zustand, dark ops theme, 14 pages
- **One-process deployment** — bind to `:3000`, serve the SPA + JSON API + WebSocket on the same port
- **PostgreSQL or SQLite** — same GORM models, dev defaults to SQLite, prod to Postgres
- **TLS / mTLS ready** — `internal/server/tls.go` + `cmd/gen-mtls-cert/` for cert generation

### 🔄  Device & physical-host lifecycle

- **State machine** (`internal/physicalhost/monitor.go:32`): `online` → `monitoring_issue` (1 fail) → `offline` (3 consecutive) → `online` (1 success), with `maintenance` as a parallel hold
- **Real SSH prober** (`internal/physicalhost/prober/sshprober.go`): ed25519 key auth, connection pooling optional, `time.AfterFunc` per-host timeout
- **TCP fallback prober** when no key is configured — port 22 reachability check, useful for monitoring a service mesh
- **Per-host metrics to InfluxDB** — CPU / memory / disk time series, async buffered writer (1024 entries, 2 workers)

### 🚀  CI/CD pipelines with real strategies

- **Blue-green / canary / rolling** strategy planners (`internal/pipeline/strategy.go`) — three concrete plans, one interface
- **Phase tracking** — strategy.Plan resolves to ordered steps, monitor state machine observes each
- **Run history** — every run + step recorded in Postgres with `status` / `duration_ms` / `error_message` / `exit_code` / `output_truncated`
- **p50 / p95 / p99 duration percentiles** via `percentile_cont` in the dashboard SQL

### ☸️  Multi-cluster Kubernetes

- **In-memory kubeconfig** — registered clusters get a per-cluster `ClientRegistry` (`internal/k8s/registry.go`), no on-disk kubeconfig required
- **Real `client-go` typed clientset** (`internal/k8s/client.go`) — not a custom abstraction
- **Pod log streaming** — `WS` + `SSE` + one-shot endpoints, all under `/api/v1/k8s/clusters/:clusterID/pods/:namespace/:pod/logs[/{stream,sse}]`
- **Pod exec** — SPDY `k8s.pods.exec` permission, terminal widget on the frontend

### 📋  Service catalog with on-call & runbooks

- **Microservice entities** with `tier` (`critical` / `standard` / `internal`) and `derived_from` (`k8s_pod_health` / `last_pipeline_run` / `manual`)
- **Health rollup** — gauge that emits `0=unknown / 1=healthy / 2=degraded` for every service, alertable
- **On-call rotation** + **runbook entries** with write API (`POST /api/v1/services/:id/oncall`, `POST /:id/runbook`)
- **Service ↔ Pipeline** relationship — link a service to its deploying pipeline, see health + deploy cadence in one view

### 🔐  Auth & RBAC

- **LDAP or dev_bypass** — `internal/auth/middleware.go:137` short-circuits to a fake `RoleOperator` on `X-User: <name>` header in dev
- **JWT** in production — `cmd/mint-dev-token` mints a dev token
- **Per-permission matrix** (`internal/auth/rbac/matrix.go`) — 5 roles × 30+ permission keys, including the rare `PermissionViewAuditLogProject` for the `ScopedAuditor` role (per-tenant audit filter, no other tenant's logs visible)
- **Per-project access middleware** — the `requireMembership` helper enforces "user in project → can touch resource" even when the role matrix says yes

### 📊  Observability that earns its keep

- **Prometheus `/metrics`** — `http_requests_total{route, method, status}` + `request_duration_seconds` histogram + Go runtime
- **Monitor-loop metrics** — `monitor_loop_iterations_total`, `monitor_loop_errors_total{host_id}`, `monitor_loop_last_tick_timestamp_seconds` (alertable for "loop is wedged")
- **Service-health metrics** — `service_health_status{service_id, tier, derived_from}` (gauge 0/1/2)
- **Audit trail in Postgres** — `actor_id` from the JWT subject (not the request body), `metadata` JSON, `ip_address`, `user_agent`, every mutating action across 8 modules
- **Structured logging** — `log/slog` JSON output, request ID propagation via `X-Trace-Id`
- **OpenTelemetry tracing** — stdouttrace default, OTLP optional, parent-based ratio sampling

### 🖥  Frontend that doesn't get in the way

- **Dark ops theme** — terminal-extension aesthetic, JetBrains Mono for data, Geist for UI
- **Live data** — WebSocket-driven service health, monitor state, log tail
- **14 pages**: Dashboard, Services, Pipelines, Physical Hosts, K8s Clusters, Logs, Metrics, Alerts, Audit, Projects, Devices, Discovery, Trace Detail, Login
- **Vitest** for the React components, RTL for interactions, tsc --noEmit in CI

---

## Screenshots

### Ops view — for the on-call operator

[![](docs/screenshots/ops-view.png)](docs/screenshots/ops-view.png)

SLO row (traffic / error budget / p95 / monitor-loop staleness) → "What's broken" tables (top error routes, flapping hosts) → health trends (tick rate, 2xx/4xx/5xx stacked, pipeline failure rate) → alert state (services degraded, stuck-running pipelines, hosts offline %). Drill-down row links to all 5 per-domain dashboards.

### Developer view — for the engineer

[![](docs/screenshots/developer-view.png)](docs/screenshots/developer-view.png)

Set `my_route` to `/api/v1/pipelines` (or any route prefix) and `project` to your project UUID — the top stats filter to your work, the per-route top-10 charts stay global, the "My project's pipelines" row is scoped to your UUID, and the live error stream surfaces `level="error"` lines from the backend.

### Overview — system health at a glance

[![](docs/screenshots/overview.png)](docs/screenshots/overview.png)

The original entry point — HTTP traffic, p95 latency, error rate, pipeline outcomes, host availability. Good for the team standup screen.

### Physical host metrics — per-host time series

[![](docs/screenshots/physical-host-metrics.png)](docs/screenshots/physical-host-metrics.png)

Per-host CPU / memory / disk time series from InfluxDB, with stale-host detection from the Postgres `physical_hosts` table.

---

## Quick start

You need **Docker** (or **Podman**) and **Go 1.25+**. The full dev environment is one script:

```bash
git clone https://github.com/your-org/devops-toolkit.git
cd devops-toolkit
bash scripts/dev-env-up.sh
```

This will:
1. Bring up an 8-service compose stack (Postgres, Loki, Prometheus, Grafana, Alertmanager, Redis, plus the backend)
2. Deploy an 8-node containerlab topology (2 DC × 4 nodes: 1 core switch + 3 sshd hosts each)
3. Auto-generate an ed25519 prober keypair, inject the public key into the containerlab nodes
4. Register every physical host as a `device` + `physical_hosts` row
5. Trigger one manual probe so the dashboards have data on the first 1-minute tick
6. Print the access URLs

Access points after the script completes:

| Service | URL | Credentials |
|---|---|---|
| Backend API | http://localhost:3000 | header `X-User: alice` (dev_bypass) |
| Frontend | http://localhost:5173 | — |
| Grafana | http://localhost:3001 | `admin` / `admin` |
| Prometheus | http://localhost:9090 | — |
| Loki | (no UI; query via Grafana) | — |

When you're done:

```bash
bash scripts/dev-env-down.sh
```

### Without the dev stack — just the binary

```bash
go build -o devops-toolkit ./cmd/devops-toolkit
./devops-toolkit                  # listens on :3000, SQLite at tests/fixtures/db/dev.db
```

### Tests

```bash
go test ./...                                  # 31 packages
go test -race ./...                            # race detector
cd frontend && npx vitest run                  # React components
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  Frontend (React 18 + TS + Vite, :5173)                         │
│  Zustand state · WebSocket (gorilla/ws) · 14 pages              │
└──────────────────────────┬───────────────────────────────────────┘
                           │ REST + WS  (X-User: alice in dev)
┌──────────────────────────▼───────────────────────────────────────┐
│  Go backend (Gin, :3000)                                         │
│                                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐│
│  │ device   │ │ pipeline │ │ k8s      │ │ physical │ │ logs    ││
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └─────────┘│
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐             │
│  │ alerts   │ │ service  │ │ project  │ │ audit    │             │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘             │
│                                                                   │
│  internal/auth (LDAP/JWT/RBAC) · internal/observability (OTel)   │
│  internal/middleware (chain order: CORS→Recovery→Tracing→       │
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

Detailed architecture: [ARCHITECTURE.md](ARCHITECTURE.md) · Specs: [openspec/specs/](openspec/specs/) · Per-module design: [docs/BACKEND.md](docs/BACKEND.md), [docs/FRONTEND.md](docs/FRONTEND.md)

The dev environment ships an 8-node containerlab topology (2 DC × 4 nodes: 1 core switch + 3 sshd hosts each) for local validation — production deployments scale to **100+ nodes across multi-region data centers** through the same REST + WebSocket API.

---

## Design philosophy

The frontend is intentionally **not** a Material Design clone. It's a terminal-extension — dark `#0c1220` background, `#22d3ee` cyan for accent, JetBrains Mono for everything that should be data (device IDs, IPs, SSH commands, metric values), Geist for UI labels. The aesthetic is "this is a tool, not a product":

- **Function over decoration** — borders, color, font weight do all the work
- **Data density wins** — tables are wide, panels stack vertically, no whitespace hero sections
- **Status colors** map to the same three states across the entire system: `#22c55e` (online/healthy/success), `#f59e0b` (warning/monitoring_issue), `#ef4444` (offline/error)
- **Tabular-nums everywhere** — numbers align in columns; `JetBrains Mono` has `font-variant-numeric: tabular-nums` baked in

Reference: [DESIGN.md](DESIGN.md) (the design system source of truth — colors, type scale, spacing, motion).

---

## Module map

| Module | LOC | What it does |
|---|---|---|
| `internal/device` | ~3k | Generic device inventory (network / container / physical) |
| `internal/pipeline` | ~5k | Pipeline CRUD + run history + strategy planners |
| `internal/k8s` | ~3k | Multi-cluster K8s client registry (k3d, kind, EKS, TKE) |
| `internal/k8s/logstream` | ~1.5k | Pod logs over WS / SSE / one-shot |
| `internal/physicalhost` | ~4k | State machine, monitor loop, InfluxDB writer, audit adapter |
| `internal/physicalhost/prober` | ~1k | SSH prober (key-based) + TCP fallback |
| `internal/logs` | ~2k | Multi-backend (local / ES / Loki) + retention + saved filters |
| `internal/alerts` | ~2.5k | Multi-channel (Slack / Webhook / Email / Log) |
| `internal/servicecatalog` | ~3.5k | Microservices + on-call + runbook + health rollup |
| `internal/project` | ~2k | Business line → system → project hierarchy + FinOps hooks |
| `internal/auth` | ~2k | LDAP + JWT + RBAC matrix + dev_bypass |
| `internal/observability` | ~0.5k | Prometheus exporter + OTel tracing |
| `internal/middleware` | ~0.7k | Chain order, request ID, logger, recovery, CORS |
| `internal/ws` | ~1.5k | Real-time event hub (gorilla/websocket) |
| `internal/audit` | ~1k | Audit trail (project mutations) |
| `internal/discovery` | ~1.5k | SNMP/SSH network discovery |
| `cmd/devops-toolkit` | ~3k | Wiring + AutoMigrate + route registration |
| `cmd/mint-dev-token` | ~0.1k | Dev JWT minter |
| `cmd/ws-smoke` | ~0.1k | WebSocket smoke test |
| `cmd/gen-mtls-cert` | ~0.1k | mTLS cert generator |
| Frontend (`frontend/src/`) | ~10k | React 18 + 14 pages + 10 common components |

---

## Project status

Current version: **v0.5.1.0** (see [VERSION](VERSION))

- ✅ 31 backend packages, all tests green (`go test ./...`)
- ✅ 1 known unreachable warning in `internal/audit/repository.go:77` (out of scope, audit reports excluded it)
- ✅ 7 production-shaped Grafana dashboards (4 per-domain + 3 system: overview, dev-view, ops-view)
- ✅ 5 C sub-projects (Helm chart scaffolding, 5 sub-charts for cert-manager / external-secrets / sealed-secrets / monitoring / network-policies)
- ✅ Auth + RBAC + cross-tenant enforcement landed (B sub-project, 3 phases)
- ✅ Production-readiness audit 2026-06-10 closed for items #1-#6

See [CHANGELOG.md](CHANGELOG.md) for the release history and [TODOS.md](TODOS.md) for what's still open.

---

## Documentation

| Doc | When to read |
|---|---|
| **[DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)** | First — the doc tree, reading order, authority map |
| [PRD.md](PRD.md) | What we're building (product spec) |
| [openspec/specs/](openspec/specs/) | How we're building it (formal tech specs — authoritative) |
| [REQUIREMENTS.md](REQUIREMENTS.md) | Backend feature completion status |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture, request flow, data flow |
| [docs/BACKEND.md](docs/BACKEND.md) | Backend code conventions |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Frontend implementation spec |
| [DESIGN.md](DESIGN.md) | Design system (colors, type, motion) |
| [DEPLOY.md](DEPLOY.md) | Deployment guide |
| [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md) | Bringing up the 8-node test environment |
| [TEST_CASES.md](TEST_CASES.md) | Test case inventory |
| [docs/CONFLICTS.md](docs/CONFLICTS.md) | Doc history conflicts (for archeology) |
| [TODOS.md](TODOS.md) | Open work items |
| [CHANGELOG.md](CHANGELOG.md) | Release history |

---

## Contributing

This project is small enough to read end-to-end in a weekend. To contribute:

1. **Pick an item from [TODOS.md](TODOS.md)** — P0/P1 items have file:line citations and audit context
2. **Branch off `main`** — `git checkout -b feat/<short-slug>`
3. **Make the change** — write a test if the package has a `_test.go` sibling; match the comment density of the surrounding code
4. **Run the suite** — `go test -count=1 -p 1 -timeout 600s ./...`
5. **Open a PR** — describe the what + the why, link the TODO item or audit report
6. **Don't commit secrets** — `deploy/secrets/prober/id_ed25519` is gitignored; the `.pub` is generated locally

Coding conventions are encoded in the code, not in a style guide. Match the surrounding comment density, the surrounding naming, and the surrounding error-wrapping pattern (`fmt.Errorf("module.X: %w", err)`).

The two non-obvious rules:
- **`auth.Bearer` always comes from the JWT subject**, never from a request body field — the audit trail depends on this
- **Don't change route param names in `internal/k8s/handler.go`** — the k8s logstream handler depends on `:clusterID`; gin panics on param-name conflicts

---

## License

[MIT](LICENSE) — Copyright (c) 2024 DevOps Toolkit contributors
