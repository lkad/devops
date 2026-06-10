# service-catalog

## Purpose

Make the microservice a first-class entity in the DevOps Toolkit so an
on-call operator can answer "what services are unhealthy right now, and
which deploy last touched this one" in under a minute.

Today the system models the things that *contain* a microservice (hosts,
K8s clusters, pipelines, alerts) but the service itself is not modelled.
This spec introduces the Service entity, a CRUD + filter API, a derived
health rollup, a Service ↔ Pipeline association, and the corresponding
frontend page.

## Requirements

### Requirement: Service Entity

The system SHALL persist a `Service` row per microservice under
management. A Service is the unit the on-call operator triages against.

#### Scenario: Create a service

- **WHEN** user sends `POST /api/v1/services` with body
  `{name: "payments-api", tier: "critical", owner: "team-payments@example.com"}`
- **THEN** system persists a new `Service` row with a generated `id`,
  `created_at`, `updated_at` and returns 201 with the row
- **AND** `name` is unique across non-deleted rows (409 on conflict)

#### Scenario: Reject invalid tier

- **WHEN** user sends `POST /api/v1/services` with `tier: "super-critical"`
- **THEN** system returns 400 with a validation error
- **AND** the enum is `critical` | `important` | `standard`

#### Scenario: Reject blank name

- **WHEN** user sends `POST /api/v1/services` with `name: ""` or
  whitespace-only
- **THEN** system returns 400 with a validation error

#### Scenario: Reject name that exceeds 64 chars

- **WHEN** user sends `POST /api/v1/services` with a 65-char name
- **THEN** system returns 400 with a validation error

### Requirement: Service List & Filter

The system SHALL list services with optional filters.

#### Scenario: List all services

- **WHEN** user sends `GET /api/v1/services`
- **THEN** system returns 200 with a paginated envelope `{data: [...], pagination: {...}}`
- **AND** each row includes `id, name, description, owner,
  repository_url, tier, created_at, updated_at`

#### Scenario: Filter by tier

- **WHEN** user sends `GET /api/v1/services?tier=critical`
- **THEN** system returns only services with `tier = "critical"`

#### Scenario: Filter by name contains

- **WHEN** user sends `GET /api/v1/services?q=pay`
- **THEN** system returns only services whose `name` contains `pay`
  (case-insensitive)

#### Scenario: Filter by owner

- **WHEN** user sends `GET /api/v1/services?owner=team-payments@example.com`
- **THEN** system returns only services owned by that address

#### Scenario: 404 on missing service

- **WHEN** user sends `GET /api/v1/services/missing-id`
- **THEN** system returns 404 with the standard error envelope

### Requirement: Service Update

The system SHALL support partial updates of a Service.

#### Scenario: Update description only

- **WHEN** user sends `PUT /api/v1/services/:id` with `{description: "new"}`
- **THEN** system updates only the description field; other fields
  unchanged
- **AND** returns 200 with the updated row

#### Scenario: Update owner

- **WHEN** user sends `PUT /api/v1/services/:id` with `{owner: "new@example.com"}`
- **THEN** system updates the owner and bumps `updated_at`

### Requirement: Service Soft Delete

The system SHALL soft-delete services (preserve the row + pipeline
history).

#### Scenario: Soft delete

- **WHEN** user sends `DELETE /api/v1/services/:id`
- **THEN** system sets `deleted_at = now()` and returns 204
- **AND** a subsequent `GET /api/v1/services` does not return the row
- **AND** any pipelines with `service_id = :id` have
  `service_id = NULL` (ON DELETE SET NULL)

#### Scenario: Delete non-existent

- **WHEN** user sends `DELETE /api/v1/services/missing-id`
- **THEN** system returns 404

### Requirement: Service ↔ Pipeline Association

The system SHALL allow a pipeline to declare which Service it deploys.
A pipeline without a `service_id` is the legacy linear path.

#### Scenario: Create a pipeline with a service

- **WHEN** user sends `POST /api/v1/pipelines` with body including
  `service_id: "svc-abc"`
- **THEN** system persists the pipeline with that foreign key
- **AND** `service_id` is included in the response body
- **AND** if `service_id` does not exist (or is soft-deleted), system
  returns 400

#### Scenario: Read a pipeline shows its service

- **WHEN** user sends `GET /api/v1/pipelines/:id` for a pipeline with
  `service_id = "svc-abc"`
- **THEN** the response body includes `service_id: "svc-abc"`

#### Scenario: Update pipeline to set service

- **WHEN** user sends `PUT /api/v1/pipelines/:id` with `{service_id: "svc-xyz"}`
- **THEN** the pipeline is updated and the next `GET` reflects the new
  service association
- **AND** the `service_id` is validated as above (400 on unknown)

#### Scenario: Backwards compatibility — pipeline without service

- **WHEN** user sends `POST /api/v1/pipelines` without a `service_id`
- **THEN** system creates the pipeline with `service_id = NULL`
  (legacy behaviour preserved)

### Requirement: Service Health Rollup

The system SHALL expose a derived health view per service. The rollup is
**derived** at read time from the most recent pipeline runs — no stored
health column.

#### Scenario: Healthy service

- **WHEN** the most recent pipeline run for the service has
  `status = "succeeded"`
- **THEN** `GET /api/v1/services/:id/health` returns
  `{status: "healthy", derived_from: "last_pipeline_run", last_run: {...}}`

#### Scenario: Degraded service

- **WHEN** the most recent pipeline run for the service has
  `status = "failed"` OR is `status = "running"` and `started_at` is
  older than 10 minutes
- **THEN** `GET /api/v1/services/:id/health` returns
  `{status: "degraded", ...}`

#### Scenario: Unknown service

- **WHEN** the service has no pipeline runs at all
- **THEN** `GET /api/v1/services/:id/health` returns
  `{status: "unknown", reason: "no_pipeline_runs"}`

#### Scenario: Recent runs included

- **WHEN** `GET /api/v1/services/:id/health` is called
- **THEN** the response includes the last 5 pipeline runs for the
  service in `recent_runs` (newest first), each with
  `{id, status, started_at, duration_ms, triggered_by}`

### Requirement: List Pipeline Runs Filtered by Service

The system SHALL support `GET /api/v1/services/:id/runs` as a
shorthand for `GET /api/v1/pipelines?service_id=...`.

#### Scenario: Get runs for a service

- **WHEN** user sends `GET /api/v1/services/:id/runs`
- **THEN** system returns the same paginated envelope as
  `/api/v1/pipelines/:id/runs`, filtered to pipelines where
  `service_id = :id`

### Requirement: Frontend Service Page

The frontend SHALL render a `/services` page (added to the SPA nav)
with list + detail views.

#### Scenario: List view

- **WHEN** user navigates to `/services`
- **THEN** the page fetches `GET /api/v1/services` and renders a
  table with: name, tier badge, owner, derived health badge
  (green/yellow/red), last-deploy relative time, View button
- **AND** a search input filters the list by name (client-side)

#### Scenario: Detail modal

- **WHEN** user clicks "View" on a service row
- **THEN** a modal opens showing: name, description, owner,
  repository link (anchor), tier badge
- **AND** a "Recent deploys" section listing the last 5 runs (id,
  status, started, duration), each clickable to open the
  Pipelines run detail

#### Scenario: Empty state

- **WHEN** the service list is empty
- **THEN** an EmptyState component shows "No services yet — create
  one with the + New Service button"

## Notes

- Soft-delete uses GORM's `gorm.DeletedAt` and is invisible to
  default queries. Foreign key on `pipelines.service_id` is
  `ON DELETE SET NULL` so pipeline history is preserved.
- The 10-minute "running" threshold for degraded state is a const
  in the health service; it is not a stored field and not user-tunable
  in P0.
- The health rollup is intentionally a derived signal. P1 will swap
  in real K8s/probe data behind the same wire shape; nothing the
  frontend renders needs to change.
