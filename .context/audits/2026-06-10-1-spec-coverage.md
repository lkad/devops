# Audit 1 — Spec coverage gaps (v0.2.0.0)

Captured 2026-06-10. Read-only recon of `openspec/specs/` vs the
codebase. Full report at the agent JSONL log; this is the
decision-relevant summary.

## Status of all 25 specs

| Spec | Status |
|------|--------|
| alert-notification | 🟡 stubs (Slack/Webhook), no real HTTP |
| api-contract | 🟡 /api/v1 prefix everywhere vs spec's /api/... |
| architecture-foundation | 🟡 main.go cross-injects modules |
| audit-logging | 🟡 spec's filter columns not implemented, no retention |
| cicd-pipeline | 🟡 no typed stage semantics |
| config-management | 🟡 no SIGHUP hot-reload |
| database-schema | 🟡 no versioned migration CLI |
| device-management | 🟡 flat /devices vs spec's nested /devices/{physical,vm,network} |
| **k8s-cluster-management** | ✅ |
| **k8s-pod-exec** | ✅ |
| **k8s-pod-health** | ✅ |
| k8s-pod-log-streaming | 🟡 persistence + reconnect-with-since missing |
| ldap-authentication | 🟡 connection pool, retry, generic 503 missing |
| **log-aggregation** | ✅ |
| metrics-collection | 🟡 spec asks for dedicated POST counter/gauge/histogram |
| middleware-stack | 🟡 CORS not wired, order wrong, 404 bypasses X-Trace-Id |
| **network-discovery** | 🟡 spec endpoint names don't match code |
| **observability-p1** | ✅ |
| physical-host-monitoring | 🟡 4-state vs spec's 8-state |
| **physical-host-project-linking** | ✅ |
| project-hierarchy | 🟡 flattened to one Project table, no FinOps CSV |
| rbac-permissions | 🟡 no label inheritance, no prod env gate |
| **service-catalog** | ✅ |
| **test-environment** | ✅ |
| **websocket-hub** | ✅ |
| **websocket-realtime** | ✅ |

✅ = 10/25 fully done. 🟡 = 15/25 partial (the rest of this doc).

## Top 5 by operator impact

1. **K8s log persistence** — `k8s-pod-log-streaming` requires every K8s pod
   line to land in `log_entries` via `logsService.CreateLogEntry`.
   Not implemented. Operators lose every K8s log line the moment
   it scrolls past the WS buffer.
   File: `internal/k8s/logstream/service.go` (no calls to logs
   service in the stream path).

2. **FinOps CSV export** — `project-hierarchy` requires
   `GET /api/org/reports/finops?period=YYYY-MM` returning CSV.
   No route, no CSV writer. Cost-allocation data model has
   no on-ramp.
   File: missing entirely.

3. **Middleware chain + CORS** — CORS struct exists but never
   registered on `/api/v1`; preflight from the SPA fails in
   any non-dev deployment. Re-order also gets X-Trace-Id on
   404 paths for free.
   File: `cmd/devops-toolkit/main.go:163-197`.

4. **Network discovery endpoint rename** — spec says
   `/api/discovery/{scan,status,register}`; code has
   `/api/v1/discovery/runs[/...]`. A spec-following client
   gets 404s. Either rename the routes or update the spec.

5. **Audit log filters + retention** — handler exposes only
   `List` + `Get`; spec's `entity_type/entity_id/username`
   filters and the retention sweep are unimplemented. Admins
   cannot answer "who deleted project X last week".

## Not implemented (long tail)

- `database-schema`: versioned `NNNNNN_*.up.sql/down.sql` files
  + `migrate up/down` CLI.
- `config-management`: SIGHUP-triggered hot reload.
- `k8s-pod-log-streaming`: per-line persistence,
  reconnect-with-since.
- `network-discovery`: literal spec endpoint names.
- `project-hierarchy`: FinOps CSV export.
- `audit-logging`: filter columns, retention sweep.
- `ldap-authentication`: connection pool, retry, generic 503.
- `architecture-foundation`: cross-module repo injection in
  main.go:627-628, 657, 712-755 (violates "modules SHALL
  communicate only through HTTP").
- `rbac-permissions`: label inheritance, env-based prod gate.
- `middleware-stack`: CORS not registered, order wrong.
- `observability-p1`: X-Trace-Id missing on 404 (NoRoute bypasses
  the tracing middleware).
