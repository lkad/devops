# Service Layer — Design

**Status:** Draft
**Last updated:** 2026-06-10
**Builds on:** [PRD.md v2.1](../PRD.md), [ARCHITECTURE.md](../../ARCHITECTURE.md), [openspec/specs/](../../openspec/specs/)
**Spec:** [openspec/specs/service-catalog/spec.md](../../openspec/specs/service-catalog/spec.md)

---

## 1. Problem statement

The DevOps Toolkit models the things that *contain* a microservice —
physical hosts, K8s clusters, deployment pipelines, alert rules — but the
microservice itself is not a first-class entity. With 30+ microservices in
production, the operator on call cannot answer the only question that
matters in the first five minutes of an incident:

> **"What services are unhealthy right now, and which one broke most recently?"**

The current on-call path through the UI is:
PhysicalHosts → Pipelines → Logs → K8sClusters → Alerts — a tree-walk that
takes longer than the alert SLA. A list of services with current health
state, a one-click drill-down, and a "what deploy last touched this" answer
do not exist.

This document is the P0 design for closing that gap. P1+ (distributed
tracing, alertmanager wiring, runbook pages) is out of scope here.

## 2. Goals (P0)

| # | Goal | Success criterion |
|---|------|-------------------|
| G1 | Microservice is a first-class entity in the data model | `services` table exists, CRUD endpoints work, list page renders |
| G2 | One-click answer to "what's unhealthy right now" | `/services` page shows a rollup column with green/yellow/red per service |
| G3 | Drill-down shows recent deploys for the service | `/services/:id` page lists the last N pipeline runs for the service |
| G4 | "Who owns this and how do I reach them" | Owner field on Service, displayed on detail page |
| G5 | All existing functionality preserved | `go test ./...` and `vitest run` both green throughout |

## 3. Non-goals (P0)

- Distributed tracing (OpenTelemetry / Jaeger) — P1
- Real K8s pod health integration (querying deployment.status, pod counts) — P1; P0 uses a derived "last pipeline outcome" as a stand-in
- alertmanager.yml + on-call routing — P1
- Runbook pages per service — P2
- Service ↔ log/trace/alert correlation — P2

## 4. Data model

### 4.1 New table: `services`

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | text (uuid) | PK, not null | |
| `name` | text | unique, not null, ≤ 64 chars | e.g. `payments-api` |
| `description` | text | nullable, ≤ 1024 chars | |
| `owner` | text | nullable, ≤ 256 chars | email or team name; shown on detail page |
| `repository_url` | text | nullable, ≤ 512 chars | link to source repo |
| `tier` | text | not null, default `'standard'` | enum: `critical` / `important` / `standard` |
| `created_at` | timestamptz | not null | GORM auto |
| `updated_at` | timestamptz | not null | GORM auto |
| `deleted_at` | timestamptz | nullable | GORM soft delete |

### 4.2 Foreign key on `pipelines`

Add `service_id` column on the existing `pipelines` table:

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `service_id` | text | nullable, FK → services(id) ON DELETE SET NULL | existing rows have NULL |

The FK is `ON DELETE SET NULL` so removing a service does not cascade-delete
the deployment history. The pipeline run history is the audit trail; the
service is just a label.

### 4.3 Derived health state

The `health_status` field on a Service is **derived, not stored**. It is
computed at read time from the most recent data sources available:

```
no pipeline runs yet           → "unknown"
last run succeeded             → "healthy"
last run failed / running > 10m → "degraded"
```

The threshold for "running > 10m" is a const for now. Future iterations can
swap in real K8s / probe data without changing the wire shape.

## 5. API surface

All routes live under `/api/v1/services` and require the same auth as the
rest of the v1 API.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/services` | List with optional `tier`, `owner`, `q` (name contains) filters |
| `POST` | `/services` | Create one (validation: name regex, tier enum) |
| `GET` | `/services/:id` | Detail |
| `PUT` | `/services/:id` | Partial update (same validation as create) |
| `DELETE` | `/services/:id` | Soft delete |
| `GET` | `/services/:id/health` | Health rollup: derived status + last 5 pipeline runs |
| `GET` | `/services/:id/runs` | Paginated pipeline runs filtered to this service |

The Pipeline model gets a corresponding read surface:

| Method | Path | Change |
|--------|------|--------|
| `POST` | `/pipelines` | New `service_id` field accepted in request body |
| `GET` | `/pipelines/:id` | Response now includes `service_id` |

## 6. Frontend

A new page at `/services` is added to the SPA nav. Two views:

**List view** — table with: name, tier badge, owner, derived health
status (green/yellow/red Badge), last deploy relative time, "View"
button. A search box at the top filters by name.

**Detail view** — opens in a modal (matches the existing Pipelines /
Alerts page pattern). Shows: name, description, owner, repository
link, tier badge. Below the header, a "Recent deploys" section lists
the last 5 pipeline runs (id, status, started, duration) with click
to open the run detail.

## 7. Risks & open questions

| Risk | Mitigation |
|------|------------|
| Soft-deleted service FK on pipelines leaves dangling `service_id` | Use `ON DELETE SET NULL`; the pipeline run page shows the service name as "—" when service_id is null |
| `tier` enum vs free-form text | Enum on the model; validation rejects unknown values with 400 |
| "Health" is a derived signal that will be wrong until P1 | Surface it as `derived_from: "last_pipeline_run"` in the health response so consumers know |
| Existing pipelines have no service_id | All existing rows default to NULL; the list view surfaces "Unassigned" if filtering by service returns zero rows for a service |

## 8. Test plan (TDD)

Each test in this work is written first, watched to fail, then made green
with the smallest possible production change.

| Layer | Test target | Initial red signal |
|-------|-------------|-------------------|
| `internal/servicecatalog/repository_test.go` | repo CRUD + filter | compile error: undefined `Repository` |
| `internal/servicecatalog/service_test.go` | service create/list/update + validation | compile error |
| `internal/servicecatalog/handler_test.go` | HTTP routes return correct envelopes | 404 on `GET /api/v1/services` |
| `internal/pipeline/handler_test.go` (extend) | `POST /pipelines` accepts `service_id`; `GET /pipelines/:id` includes it | response body lacks `service_id` |
| `internal/servicecatalog/health_test.go` | health rollup returns derived state + last 5 runs | route 404 |
| `frontend/src/pages/Services.test.tsx` | list renders rows; detail modal opens | component missing |
| Full repo | `go test ./...` + `vitest run` | (eventual green) |

The first failing test for each layer is the entry point — once it
exists and the failure is confirmed, the production change is written
to satisfy *that* test, not the whole layer in one go.
