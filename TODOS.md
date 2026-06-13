# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

# TODOS — DevOps Toolkit

Tracks deferred work that's clear but intentionally not done now.
Add new items at the top. Move done items to `## Completed` with a date.

The post-v0.2.0.0 audit (4 reports at `.context/audits/2026-06-10-*.md`)
identified a long backlog. Items below are sorted by
**production-readiness priority** (🔴 = blocker) → operator
value → polish. Source report is named in each item so the
audit context is recoverable.

## Open

### P0 — Auth + RBAC middleware on `/api/v1` (audit #3, item 1)

Currently every business endpoint is anonymous. The
middleware constructors are never called in `main.go`.
`internal/middleware/chain.go:12` even says "Auth and RBAC
are injected by their respective owners (Phase 2)".

Wire `auth.NewAuthMiddleware` and `rbac.RequirePermission`
on the `/api/v1` group, with per-module permission keys.
Tests: assert the production wiring rejects anonymous
requests with 401, and RBAC denies return 403. Source:
`.context/audits/2026-06-10-3-production-readiness.md` item 1.
Estimate: 1-2 days.

---

### P0 — Cross-tenant project-scope enforcement (audit #3, item 2)

A Developer-role user in project A can read/modify/delete
any project's resources. `internal/project/service.go:130-141`
`GetProjectWithRelations` returns everything regardless of
membership. Even with the auth middleware wired (#1 above),
the role matrix is global — needs a `rbac.HasPermissionInProject`
lookup in each service layer.

Source: `.context/audits/2026-06-10-3-production-readiness.md`
item 2. Estimate: 3-5 days.

---

### P0 — Audit log coverage across all mutating modules (audit #3, item 3)

Only `physicalhost.EnterMaintenance` / `ExitMaintenance`
call `audit.Service.RecordAction`. Project CRUD, member
grant/revoke, device CRUD, k8s cluster CRUD, pipeline
runs, alert rules, log saved-filters, discovery runs —
none audited.

Wire `audit.Service.RecordAction` from each service layer
with the caller's `ActorID` from the JWT subject (not
from the request body). Source: `.context/audits/2026-06-10-3-production-readiness.md`
item 3. Estimate: 2-3 days.

---

### P0 — `APP_JWT_SECRET` mandatory in production (audit #3, item 4)

`cmd/devops-toolkit/main.go:352-356` and again at 926
(WS hub signer) use a hard-coded dev fallback with only
`log.Warn`. `config.Validate` does not refuse to start when
unset. Same for `K8S_CRYPTO_KEY` at main.go:643.

Fix `internal/config/config.go:213` `Validate` to:
- refuse to boot when `APP_JWT_SECRET` is unset in `env == "production"`
- refuse to boot when `K8S_CRYPTO_KEY` is unset in prod
- refuse to boot when `ldap.dev_bypass: true` and
  `env == "production"`

Source: `.context/audits/2026-06-10-3-production-readiness.md`
item 4. Estimate: 2 hr.

---

### P0 — Deep `/health` (audit #3, item 5)

`cmd/devops-toolkit/main.go:230-232` returns 200 regardless
of DB, LDAP, K8s, Redis, InfluxDB state. Split into
`/live` (process up) and `/ready` (deps up). `/ready` fans
out to DB ping, LDAP ping, K8s clientset ping; returns
503 with per-dependency breakdown on any failure. Add
`healthcheck:` block to `deploy/docker-compose.yml:24-48`.
Source: `.context/audits/2026-06-10-3-production-readiness.md`
item 5. Estimate: 1 day.

---

### P0 — `deploy/docker-compose.yml` maturity (audit #3, item 6)

Add `mem_limit` / `cpus` on every service. Add `healthcheck:`
on the app container. Move hard-coded creds to env-var-only
(`LDAP_ADMIN_PASSWORD`, `APP__JWT_SECRET`). Enable mTLS
by default in prod. Add `scripts/backup-postgres.sh`
that does `pg_dump` to a dated file plus an S3 upload
+ restore drill. Source: `.context/audits/2026-06-10-3-production-readiness.md`
item 6. Estimate: 1 day compose + 1 day backup script.

---

### P1 — Service catalog on-call + runbook write UI (audit #2, item 11)

Backend models, repository, and CurrentOnCall/ListRunbook
all exist. **No HTTP routes are registered** for the
writes. Services.tsx renders the embedded oncall +
runbook read-only. This is a real product gap; on-call
rotation is a feature headline.

Add `POST/DELETE /api/v1/services/:id/oncall` and
`POST/DELETE /api/v1/services/:id/runbook` + UI buttons.
Source: `.context/audits/2026-06-10-2-frontend-backend-gaps.md`
item 11. Estimate: 1 day.

---

### P1 — K8s pod log streaming inside the cluster modal (audit #2, item 3)

`/api/v1/k8s/clusters/:id/pods/:namespace/:pod/logs[/{stream,sse}]`
endpoints exist (WS + SSE + one-shot). K8sClusters.tsx
shows pods but no "Logs" action. Drill-in from pod row →
log viewer. Source: `.context/audits/2026-06-10-2-frontend-backend-gaps.md`
item 3. Estimate: 2 days for WS panel; SSE version faster.

---

### P1 — K8s pod exec ("Shell into pod") UI (audit #2, item 1)

`POST /api/v1/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec`
endpoints exist end-to-end. No frontend page calls it.
K8sClusters.tsx renders pods but no row has a "Shell" or
"Exec" action. xterm.js terminal widget. Source:
`.context/audits/2026-06-10-2-frontend-backend-gaps.md`
item 1. Estimate: 2-3 days.

---

### P1 — K8s log persistence (audit #1, item 1)

`k8s-pod-log-streaming` requires every K8s pod line to
land in `log_entries` via `logsService.CreateLogEntry`.
Not implemented. Operators lose every K8s log line the
moment it scrolls past the WS buffer.

Wire `internal/k8s/logstream/service.go:Stream` to call
`logsService.CreateLogEntry`. Source: `.context/audits/2026-06-10-1-spec-coverage.md`
item 1. Estimate: 1-2 days.

---

### P1 — Middleware chain order + CORS (audit #4, item 1, audit #1 item 3)

`CORS → Recovery → Logging → Metrics → Auth → RBAC` order
mandated by the spec; current runtime is
`Recovery → Tracing → Metrics → Auth` (no CORS at all
in the chain). X-Trace-Id missing on 404 paths because
the NoRoute handler runs outside the tracing middleware.
Source: `.context/audits/2026-06-10-4-architecture.md`
and `.context/audits/2026-06-10-1-spec-coverage.md`.
Estimate: 1 day.

---

### P1 — `monitor_loop.go` graceful shutdown (audit #4, item 1)

`internal/physicalhost/monitor_loop.go:56-72` is started
with `context.Background()` in `main.go:542`. On SIGTERM
the loop is not cancelled before `srv.Shutdown(ctx)`
returns. On a 200-host fleet the per-host 5s sequential
check holds for 1000s+ after shutdown. No rate limiting
beyond per-host 5s. No Prometheus metric for
`loop_iterations_total`, `loop_errors_total`.

Move to `signal.NotifyContext` shared with the HTTP
server. Add a single Counter + Gauge to
`internal/observability/metrics.go`. Source:
`.context/audits/2026-06-10-4-architecture.md` item 1.
Estimate: 1 day.

---

### P2 — `writeAPIError` consolidation (audit #4, item 5)

Eight near-identical `writeAPIError` helpers across
modules. Move into `internal/handler/response.go` as
`handler.WriteAPIError(w, err)`. Source:
`.context/audits/2026-06-10-4-architecture.md` item 5.
Estimate: 1 hr.

---

### P2 — `database.MapNotFound` helper (audit #4, item 5)

11 places do `if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrNotFound }`.
One helper. Source: `.context/audits/2026-06-10-4-architecture.md`
item 5. Estimate: 30 min.

---

### P2 — Dev-default secrets → single const block (audit #4, item 6)

`APP_JWT_SECRET` default at main.go:354, 926 (TWO
places), `K8S_CRYPTO_KEY` at 643, `INFLUX_*` at 506-518,
`LOG_STORAGE_DIR` at 870, `PROBER_*` at 288-319. Extract
to one const block + `envOrWarn` helper. Source:
`.context/audits/2026-06-10-4-architecture.md` item 6.
Estimate: 1 file, ~30 lines.

---

### P2 — K8s `Client` interface split (audit #4, item 3)

`internal/k8s/client.go:139-178` — 6 methods, all
consumed by the same Service. The `servicecatalog`
walker already narrows via inline adapter. Split into
`Lister` (List* + Ping), `LogReader` (GetLogsBySelector),
`Exec` (ExecInPod). Source: `.context/audits/2026-06-10-4-architecture.md`
item 3. Estimate: 2-3 hr + test updates.

---

### P2 — Layering cracks in 3 packages (audit #4, item 2)

`internal/servicecatalog/handler.go:183,190` reaches
`h.cat.repo` directly (2 sites, 30 min). `internal/project/handler.go:198`
reaches `h.repo` once (15 min). `internal/physicalhost/handler.go`
reaches `h.repo` 8 times (1 day for a Service).
Source: `.context/audits/2026-06-10-4-architecture.md` item 2.
Estimate: 3-5 days for the full sweep, or 1 hr to document
the exception.

---

### P3 — `/devops-toolkit` 49 MB binary in repo (audit #4, item 7)

Add to `.gitignore`, `git rm`. Source: `.context/audits/2026-06-10-4-architecture.md`
item 7. Estimate: 1 min.

---

### P3 — Untracked `internal/k8s/registry.go` (audit #4 stray files)

`registry.go` and `registry_test.go` are referenced from
`main.go` but untracked. `git add` them. Estimate: 1 min.

---

### P3 — Dashboard `?status=open` silently ignored (audit #2 small bug)

`alerts/handler.go:344-372` reads `state` not `status`.
The "open alerts" count on the dashboard is always 0.
5-min fix. Source: `.context/audits/2026-06-10-2-frontend-backend-gaps.md`
end-of-list small bug.

---

### P3 — Concurrency bugs (audit #4 smaller findings)

- `Hub.Publish` TOCTOU on second channel send (hub.go:202-216).
  30 min.
- `BufferedEmitter` drain race (emitter.go:177-201). 2 hr.
- No concurrent-access test for `defaultRegistry`. 30 min.

---

### P3 — Configuration secrets masking list incomplete (audit #4 config)

`(*Config).String()` masks only `password`; misses
`kubeconfig`, `bind_password`, `token`, `secret`. Move the
set of names to `pkg/logger`. 1 hr.

---

### P3 — Run promtool check rules pre-merge on alert YAMLs (audit #1)

`deploy/prometheus/rules/catalog-degraded.yml` was not run
through `promtool check rules`. PromQL silently accepts
typoes. 15 min CC (install promtool in CI).

---

## Completed

### D 子项目 — UI 空白补全 + Middleware CORS (2026-06-13)

D 子项目 5 项 UI/Middleware 空白全部补全,4 frontend agents + 1 backend CORS 主 session,~3 小时完成。
关键 commit:d1 `5db94b3e`+`a1ed5cf3` (PodLogPanel) / d3 `c34fdaf6` (OnCall/Runbook editors) / d4 `d6a91394` (Audit filters) / d5 `5c5c4324` (CORS)。

- Phase 1 (Agent 1, d1) K8s pod log UI — 2 commits + 4 vitest
- Phase 2 (Agent 2, d2) K8s pod exec UI — 之前 session 已实施,no-op
- Phase 3 (Agent 3, d3) Service catalog 写 UI — 1 commit + 11 vitest
- Phase 4 (Agent 4, d4) Audit log UI 增强 — 1 commit (无新 test 文件)
- Phase 5 (主 session, d5) CORS middleware 增强 — 1 commit
- 31 packages Go + 28+ vitest 全绿,1 vet warning 既有

### B 子项目 — 鉴权+多租户硬化 (2026-06-13)

B 子项目 P0 #1+#2+#3 + scoped-Auditor RBAC matrix 全部落地,3 phase (1 主 session + 2 background agents) 并行实施,~30 atomic commits。
关键 commit:`a84db132` (b1 merge) + `466fac6b` (b3 merge) + `c0b29165` (b4 merge) + `8699d7cb` (merge resolution) + `41e3f1fc` (spec) + `1f14fe28` (plan) + `70c4c916` (release v0.2.1.0)。

- P0 #1 (Auth+RBAC middleware 接到 /api/v1) — 之前 session 已实施 (`authMW.RequireAuth()` 在 `cmd/devops-toolkit/main.go:149`)
- P0 #2 (跨租户强制) — 8 modules × `*WithCaller` 新方法 + `requireMembership` helper (Agent 2)
- P0 #3 (Audit 覆盖全部 mutating) — 8 modules (Project + Pipeline 加实际 emit;其他加 test pinning;Agent 3)
- scoped-Auditor matrix — `RoleScopedAuditor` + `PermissionViewAuditLogProject` (Phase 1 主 session)
- hostproject `systemCtx` helper — 合成 SuperAdmin caller 给 admin paths (Link/Unlink/Bulk*);fix pre-existing build break + P0 #2 误触发
- 31 packages 全绿,0 fail,1 vet warning 既有

注: Agent 1 (Phase 2 Auth+RBAC) 实际无新 commit 工作 — Phase 2 之前 session 已完整实施 (`auth.NewAuthMiddleware` + `rbac.RBACMiddleware` 都已经存在)。Agent 2 和 3 各做各的 phase 实质工作,2 个 stop hallucination 重试后成功。

### A 子项目 — 后端生产化 (2026-06-12)

A 子项目 12 项全部落地,5 个 agent 并行实施,3.5 小时完成。
具体见 `openspec/specs/production-readiness/` (镜像本 spec) + git log。
关键 commit:`d954e86d` (a1 merge) + `ff90443e` (a3 merge) + `37c88180` (a4 merge) + `79b44124` (a5 merge) + `6a09794b` (plan) + `cb5ea906` (spec)。

- P0 #4 (APP_JWT_SECRET/K8S_CRYPTO_KEY 强制生产) — `internal/config/config.go` Validate + main.go env check
- P0 #5 (`/health` 拆 `/live`/`/ready`/`/health` + fan-out 探活 DB+LDAP+K8s) — `internal/health/health.go`
- P0 #6 (docker-compose mem_limit/cpus/healthcheck/backup 容器) — `deploy/docker-compose.yml`
- P1 monitor_loop 优雅停机 + 3 Prometheus metrics — `signal.NotifyContext` 共享,既已实施
- P2 `handler.WriteAPIError` 合并 (8 sites → 2 真 site,hostproject/project 真实)
- P2 `database.MapNotFound` helper (已存在,Task 2 无 commit)
- P2 dev-default secrets 集中 (5 const + EnvOrWarn) — `internal/config/secrets_block.go`
- P2 K8s Client interface 拆 (Lister/LogReader/Execer) — 已存在
- P2 secret masking 名单 (15 key) — `pkg/logger/secret_keys.go`
- P3 49MB binary `.gitignore` 精确模式
- P3 dashboard `?status=open` bug (handler.go:373-381 synonym 方式)
- P3 `promtool check rules` + `check config` CI step
- 新增 Helm chart 骨架(13 文件,9 templates) + configmap flatten 修
- 新增 `scripts/backup-postgres.sh` + `scripts/restore-postgres.sh`

### Audit reports persisted 2026-06-10

`/mnt/devops/.context/audits/2026-06-10-{1,2,3,4}-*.md` —
4 full audit reports (spec coverage, frontend/backend gaps,
production-readiness, architecture). Captured 2026-06-10
as the v0.2.0.0 baseline; future /retro runs should
sample the open-item list to measure progress.

