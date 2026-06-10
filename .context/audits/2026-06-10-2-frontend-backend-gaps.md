# Audit 2 — Frontend / backend feature gaps (v0.2.0.0)

Captured 2026-06-10. Read-only recon of frontend pages vs
backend routes. Full report at the agent JSONL log; this
is the decision-relevant summary.

## Frontend surface: 15 pages, all routed (no dead pages)

`Login`, `Dashboard`, `Projects`, `Devices`, `PhysicalHosts`,
`K8sClusters`, `Discovery`, `Pipelines`, `Services`,
`TraceDetail`, `Logs`, `Metrics`, `Alerts`, `Audit`. The
SideNav lists 12; `TraceDetail` and `Login` are deep-link /
pre-shell.

## Backend surface: ~95 routes, 10 modules

Project, Device, DeviceGroups, ConfigurationTemplates,
HostProjectLink, PhysicalHost, Discovery, K8sClusters,
K8sLogStream, Pipeline, ServiceCatalog, Logs, Metrics,
Alerts, Audit.

## Big gaps: backend has it, frontend doesn't show

1. **POST .../pods/:pod/exec** (`internal/k8s/handler.go:259`)
   — operator can shell into a pod via API, but no frontend
   "Shell" or "Exec" action on the K8sClusters page.
   Value: very high. 2-3 days for a real terminal widget
   (xterm.js).

2. **GET .../namespaces/:ns/logs?labelSelector=…** — the
   P2 follow-up "log search by label selector" endpoint.
   Logs page only uses the global `/api/v1/logs/query`.
   1-2 days for the panel.

3. **WS / SSE / one-shot .../pods/.../logs[/stream|/sse]**
   (`internal/k8s/logstream/handler.go:82-86`) — per-pod
   log stream. Logs page subscribes to WS channels `logs.*`
   and `container_log` but never opens a per-pod stream. The
   canonical "tail pod logs" feature; today you cannot drill
   into a specific pod. 2 days for the WS panel; SSE is
   faster.

4. **GET /api/v1/physical-hosts/:id/maintenance-history** —
   wired, spec calls out a "last 5 maintenance windows"
   timeline; the host detail modal shows only the *current*
   banner. 30 min.

5. **GET /api/v1/pipelines/:id/stats** — success rate /
   avg duration / last 10 runs. Pipelines.tsx never calls
   it. ~1 hr to add a stats column.

6. **GET /api/v1/alerts/stats** — aggregate stats endpoint.
   Alerts.tsx has 3 tabs (active/channels/history) but no
   stats tab. 30 min.

7. **PUT/DELETE /api/v1/alerts/channels/:id** — only POST
   and GET are used. No edit/delete on the Channels tab.
   1 hr.

8. **Device groups + configuration templates** — entire
   `internal/device/groups.go` and `templates.go` subsystems
   are dead from the frontend's perspective. No sidebar
   link, no page. 1 day to add a single "Inventory → Device
   Groups" tab and a "Templates" page.

9. **Host-project linking UI** — 5 routes under
   `internal/hostproject/` are unused by the Devices page
   (just shows a `group_name` column). 2 hr.

10. **Project write routes** — backend exposes PUT/DELETE on
    projects and members, but Projects.tsx only reads +
    creates. No edit, no delete, no member add/remove. 2-3 hr
    for the read paths; writes need modals.

11. **Service catalog OnCall / Runbook edit UI** — backend
    models, repository, and CurrentOnCall/ListRunbook all
    exist. **No HTTP routes are registered** for the
    writes. Services.tsx renders the embedded oncall +
    runbook read-only. This is a real product gap; on-call
    rotation is a feature headline. Add
    `POST/DELETE /api/v1/services/:id/oncall` and
    `POST/DELETE /api/v1/services/:id/runbook` + UI
    buttons. ~1 day.

12. **GET /api/v1/logs/streams** — list of log streams
    picker. Logs.tsx has no streams picker. 1 hr.

## Small bug

- Dashboard's `?status=open` on `/api/v1/alerts` is silently
  ignored: alerts/handler.go:344-372 reads `state` not
  `status`. The "open alerts" count on the dashboard is
  always 0. 5-min fix.

## Top 5 to close (operator value)

1. **Service catalog on-call + runbook edit UI** — 1 day,
   headline feature finally complete.
2. **K8s pod log streaming inside the cluster modal** —
   2 days, drill-in from pod row → log viewer.
3. **K8s pod exec ("Shell into pod") UI** — 2-3 days,
   xterm.js widget, first thing you want on a stuck pod.
4. **Device groups + configuration templates pages** —
   1 day, two long-dead subsystems come to life.
5. **Pipeline stats row in the pipeline list** — 1 hr,
   small column with success rate / avg duration.

Honourable mentions (5 min CC each): maintenance-history
panel, alerts stats card, log streams picker, host-project
linking UI.
