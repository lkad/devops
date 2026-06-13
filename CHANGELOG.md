# Changelog

All notable changes to this project will be documented in this file.

## [0.3.0.0] - 2026-06-13

B 子项目 — 鉴权+多租户硬化 落地。关 P0 #1 (Auth+RBAC middleware 接到 /api/v1) + P0 #2 (Service 层跨租户强制) + P0 #3 (Audit 覆盖全部 mutating 模块) + scoped-Auditor RBAC matrix 接线。3 个 phase 并行实施(1 主 session + 2 background agents),2-3 天完成。

### Added

- **scoped-Auditor RBAC matrix** — `pkg/contracts/user.go` 加 `RoleScopedAuditor` 角色,`internal/auth/rbac/matrix.go` 加 `PermissionViewAuditLogProject` 权限。`audit.Service.ListForCaller` 走 per-tenant 路径已经存在(scoped-Auditor P0 follow-up,A 子项目之前完成)。
- **Service 层跨租户强制 (P0 #2)** — 8 modules (project/device/k8s/pipeline/servicecatalog/physicalhost/alerts/logs) 加 `*WithCaller(ctx, id)` 新方法 + `requireMembership` helper。`caller.IsMemberOf` + SuperAdmin bypass + fail-closed (`ErrUnauthenticated`/`ErrForbidden`)。
- **Audit 覆盖全部 mutating 模块 (P0 #3)** — 8 modules 全部 emit `audit.Service.RecordAction` (project/pipeline 加实际 emit;其他模块加 test pinning 既有 emit)。Project emit 从 `handler.go` 移到 `service.go`(避免 double emit)。
- **hostproject system operations** — 加 `systemCtx` helper 合成 SuperAdmin caller 给 `Link/Unlink/Bulk*` 路径(系统操作),允许 hostproject 跨项目操作而不破坏 P0 #2 fail-closed 模型。

### Fixed

- **Pre-existing build break in hostproject** — `hostproject/service.go` 调 `s.projectSvc.GetProject(string)` 缺 `context.Context` 参数。已修:4 处调用加 ctx(Link/Unlink/BulkLink/BulkUnlink 用 `systemCtx`,List paths 也用 `systemCtx`)。
- **P0 #2 fail-closed 触发意外 500** — `route_smoke_test.go` (Phase 3 integration test) 不接 AuthMiddleware,导致 service-layer caller check 失败。已修:test 加 `devBypassAuth` 中间件(从 X-User header 合成 SuperAdmin caller)。

### Internal

- `pipeline/service.go` 加 `caller` import + `membershipChecker` 字段 + `SetMembershipChecker` method。
- `hostproject/service.go` 加 `caller` import + `assertProjectExists(ctx, id)` 接 ctx。
- `hostproject/service_test.go` 加 `context` import + `newServiceWithDB` 设 membershipChecker(allow any)。
- 31 packages 全绿,0 fail,1 vet warning(`internal/audit/repository.go:89` 既有)。
- ~30 atomic commits from 3 phase:Phase 1 (1) + Phase 3 (9) + Phase 4 (8) + manual merge resolution (1) + Phase 5 docs (this commit)。

### Notes for next phase

- **scoped-Auditor role usage** — `RoleScopedAuditor` 是新角色,需要 LDAP group → role 映射配置(在 `internal/auth/ldap` 处,留 D 阶段)。
- **caller injection for admin paths** — hostproject `systemCtx` 模式可推广给其他 admin-only paths(如 `internal/auth/...` 的 admin endpoints),如果后续需要。

---

## [0.2.1.0] - 2026-06-12

A 子项目 — 后端生产化 (Production-Readiness) 落地。5 个并行 agent 实施,3.5 小时完成。所有 P0 #4 / #5 / #6 + P1 monitor_loop + P2 收敛(8 module-local writeAPIError / dev-default secrets / secret masking 名单 / K8s Lister 接口) + P3 杂项(`?status=open` bug 修 / 49MB binary .gitignore / promtool CI),加 Helm chart 骨架 + backup/restore 脚本。

### Added

- **Health endpoints** — `/live` (liveness) + `/ready` (DB+LDAP+K8s fan-out 探活) + `/health` (兼容旧调用方)。`internal/health/health.go` + tests。
- **Production secret enforcement** — `config.Validate()` 在 `env == "production"` 时强制 `ldap.url` / `database.password != "devops"` / `ldap.dev_bypass == false`。JWT/K8S-crypto 密钥 env-only,在 main.go 启动检查。
- **Dev-default secrets 集中** — `internal/config/secrets_block.go`:5 个 `DevDefault*` 常量 + `EnvOrWarn(envName, devDefault)` helper。生产 fail-fast 关键支撑。
- **Secret masking 完整名单** — `pkg/logger/secret_keys.go`:15 个 key (password/kubeconfig/bind_password/token/secret/jwt_secret/k8s_crypto_key/private_key/ssh_key/client_secret/api_key/ldap_bind_pw/tls_cert 等) + `ShouldMask(key)` helper。`IsSensitiveField` 委托到 `ShouldMask`,Single source of truth。
- **monitor_loop 优雅停机** — `signal.NotifyContext` 在 main.go 接管 SIGINT/SIGTERM,与 HTTP server 共享 root context。3 个 Prometheus metrics:`physicalhost_loop_iterations_total` / `physicalhost_loop_errors_total` / `physicalhost_loop_last_success_timestamp_seconds`。
- **K8s Client interface 拆分** — `Lister` (Ping + ListPods/ListDeployments/ListServices) / `LogReader` (GetLogsBySelector) / `Execer` (ExecInPod)。`k8sClientAdapter.client: k8s.Lister` 走窄接口,`ListerFor` registry method。
- **Helm chart 骨架** — `deploy/helm/` (13 文件):Chart.yaml + values.yaml + 9 templates (deployment/service/ingress/configmap/secret/serviceaccount/hpa/cronjob-backup/prometheusrule) + _helpers.tpl + README。livenessProbe→/live,readinessProbe→/ready,cert-manager/ExternalSecret 接入点已留。
- **Backup/Restore 脚本** — `scripts/backup-postgres.sh` + `scripts/restore-postgres.sh`,3 target/source (local/s3/nfs),`set -euo pipefail`,路径遍历 sanitization,dump 验证。

### Fixed

- **dashboard `?status=open` 静默忽略** — `internal/alerts/handler.go:373-381` 改用 `state` + `status` synonym 方式,前端发 `?status=open` 现在能正确返 open alerts。
- **Plan 级别 bug** — configmap template 原用 `quote` 处理 nested config map,`helm template` 失败。改为 toYaml 输出 nested config 到 `config.yaml` 文件 key,顶层 scalar 转 env 注入。
- **`.gitignore` 49MB binary 误匹配目录** — `**/devops-toolkit` 改 `*/devops-toolkit`(单星,避免误匹配 `cmd/devops-toolkit/` 目录)。
- **`handler.WriteAPIError` 合并** — 8 module-local `writeAPIError` helper 删除,改调统一 `handler.WriteAPIError`。`servicecatalog.writeAPIError` 保留(做 typed-error mapping,fit 不进 pass-through 语义)。

### Changed

- **Layering 改进** — 5 module `hostproject` + `project` 改用 `handler.WriteAPIError`,`servicecatalog` 维持自己(typed-error mapping 需求)。
- **CI** — `.github/workflows/ci.yml` 加 promtool `check rules` + `check config` 两步,验证 `prometheus.yml` scrape config + 规则文件。
- **docker-compose** — `deploy/docker-compose.yml` 加 backup 容器(postgres:15 + cron + /backups volume)+ mem_limit/cpus/healthcheck 资源限制。

### Internal

- **`database.MapNotFound` helper** — 之前 session 已实施(`internal/database/notfound.go`),接受 sentinel 参数。Task 2 无新增 commit。
- **Task 8 monitor_loop + Task 12 K8s interface split** — 之前 session 已实施,Agent 3 验证 + 扩展(`ListerFor` registry method + 4 个 test file stub)。

### For contributors

- 新 spec 文档: `docs/superpowers/specs/2026-06-12-A-production-readiness-design.md` (746 行)
- 新 plan 文档: `docs/superpowers/plans/2026-06-12-A-production-readiness.md` (2509 行, 14 task × 105 step)
- 13 个 atomic commit + 4 个 merge commit on main
- 测试:31 packages 全绿,0 fail,1 vet warning (`internal/audit/repository.go:89` 既有,已 exclude)

---

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
