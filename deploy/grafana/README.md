# DevOps Toolkit — Grafana

Grafana is provisioned with **4 datasources** and **7 dashboards** by the
`docker-compose` stack under `deploy/`. The dashboard JSONs in
`dashboards/` are auto-loaded by the file-based provider at
`provisioning/dashboards/dashboards.yml` (30s refresh); UI edits are
NOT persisted (the provider runs with `allowUiUpdates: false`).

## Datasources (provisioned, not user-created)

| Name | Type | URL | Used by |
|------|------|-----|---------|
| `Prometheus` | prometheus | `http://prometheus:9090` | HTTP traffic, monitor loop, service health |
| `Postgres` | postgres | `postgres:5432` | Pipeline runs, physical hosts |
| `Loki` | loki | `http://loki:3100` | Live log streams |
| `InfluxDB` | influxdb | `http://influxdb:8086` | Per-host CPU / memory / disk / uptime |

Datasource credentials are env-var driven — see `provisioning/datasources/datasources.yml`
for the `${POSTGRES_PASSWORD}` / `${INFLUXDB_TOKEN}` references.

## Dashboards (7 total)

Two **role-based entry points** + five **per-domain drill-downs**.

### Entry points (open one of these first)

| Dashboard | Audience | What it answers in 10s |
|-----------|----------|------------------------|
| **[Developer view](dashboards/developer-view.json)** | Engineer working on the platform | "Is my code working today?" — per-route traffic, errors, latency, my pipelines, live error stream |
| **[Ops view](dashboards/ops-view.json)** | On-call operator | "Is the platform SLO-compliant right now?" — error budget, monitor-loop staleness, top error routes, flapping hosts, alert-state stats |

### Per-domain drill-downs (linked from both entry points)

| Dashboard | Source | What it covers |
|-----------|--------|----------------|
| [DevOps Toolkit overview](dashboards/devops-toolkit-overview.json) | Prom + PG | System-level health (1 stat row + 4 trends) |
| [Pipeline execution](dashboards/pipeline-execution.json) | PG | Per-pipeline success / duration percentiles / step-level fail rate |
| [Logs overview](dashboards/logs-overview.json) | Loki | Service-level log volume, error ratio, live tail |
| [Service catalog](dashboards/service-catalog.json) | Prom + PG | Service inventory, on-call / runbook, health rollup |
| [Physical host metrics](dashboards/physical-host-metrics.json) | Influx + PG | Per-host CPU / memory / disk time series |

## Role-based navigation

- **Engineer on the platform** → open `Developer view`, set `my_route` template
  variable to your route prefix (e.g. `/api/v1/pipelines`), set `project` to
  your project UUID. Drill into per-domain dashboards from the bottom row.
- **Operator on call** → open `Ops view`. Read the SLO row first, then the
  "What's broken" tables, then the "Alert state" row. Drill into per-domain
  dashboards for the entity matching the alert.

## Template variables

| Variable | Where | Type | Default | Effect |
|----------|-------|------|---------|--------|
| `datasource` | dev + ops | prom | Prometheus | PromQL target |
| `pg` | dev + ops | postgres | Postgres | SQL target |
| `loki` | dev | loki | Loki | LogQL target |
| `my_route` | dev | textbox | empty | Per-route PromQL filter (regex prefix, blank = all) |
| `project` | dev | textbox | empty | Project UUID for PG panel filters (blank = all) |
| `error_route_filter` | dev | textbox | empty | Narrow the "Top 10" route time series |
| `route_filter` | ops | textbox | `/api/v1/.*` | Regex for the Top error routes table |

## Adding a new dashboard

1. Drop a `your-dashboard.json` in `dashboards/`. Folder = "DevOps Toolkit"
   (from the `dashboards.yml` provider), tags `["devops-toolkit", "..."]`.
2. Reference datasources by `uid: "${datasource}"` (NOT hardcoded UIDs).
3. Test JSON syntax locally: `python3 -m json.tool your-dashboard.json > /dev/null`
4. Commit; the file-based provider picks it up on the next 30s cycle
   (or after a Grafana restart).

## Editing in the Grafana UI

The dashboards are managed-as-code — UI changes are **not** persisted to
the JSON files. To change a panel:

1. Edit the JSON in this repo.
2. Re-deploy (or `docker restart grafana`).
3. The file-based provider reloads the JSON.

The provider runs with `allowUiUpdates: false`; if you need a temporary
edit, change the provider config to `allowUiUpdates: true` and remember
to revert before merging.
