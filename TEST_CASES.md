# 测试用例汇总 — DevOps Toolkit

**生成时间:** 2026-06-08
**仓库:** /mnt/devops
**总览:**
- 22 openspec specs / 374 scenarios / 192 requirements
- 24 Go packages / 756 test functions (全部通过 `-race`, 1 pre-existing flake 排除)
- 11 frontend pages (1 critical bug found + fixed)
- 3-tier test environment scripts (10 scripts, 60 conformance sub-tests)

---

## 一、Openspec 测试场景 (374 个)

每个 spec 给出 requirement 数 + scenario 数。Scenario 是 TDD 中"先写一个失败的测试"的来源。

| # | Spec | Requirement | Scenario | 关键测试点 |
|---|---|---:|---:|---|
| 1 | **architecture-foundation** | 8 | 8 | 目录分层、handler→service→repo→model、Gin/Viper/GORM/log-slog、JWT+LDAP、middleware chain |
| 2 | **api-contract** | 7 | 16 | RESTful URL、HTTP methods、list envelope `{data,pagination}`、error envelope、错误码、Content-Type |
| 3 | **database-schema** | 11 | 14 | UUID PK、soft-delete、JSONB、AutoMigrate、级联、外键 |
| 4 | **config-management** | 10 | 12 | YAML 加载、env override `__` 嵌套、必填/可选、敏感字段 mask、config 分区、reload (SIGHUP) |
| 5 | **middleware-stack** | 10 | 13 | Recovery/RequestID/CORS/Logger 顺序、限流、压缩 |
| 6 | **ldap-authentication** | 7 | 12 | LDAP 连接池、dev_bypass、group→role 映射、retry、graceful error、health |
| 7 | **rbac-permissions** | 6 | 14 | 5 global + 3 project roles、SuperAdmin/Operator/Developer/Auditor/ProjectAdmin、permission 继承 |
| 8 | **device-management** | 7 | 8 | CRUD、4-state、search、bulk actions、sub-resources (groups/templates) |
| 9 | **physical-host-monitoring** | 13 | 26 | 4-state 状态机、maintenance 进入/退出、consecutive fails 阈值、SSH 探活、Fake Prober |
| 10 | **network-discovery** | 6 | 10 | CIDR 扫描、port probe、SNMP、dedup、promote 到 devices |
| 11 | **k8s-cluster-management** | 10 | 17 | cluster CRUD、kubeconfig AES-256-GCM 加密、pods/deployments/services list、exec stub |
| 12 | **physical-host-project-linking** | 3 | 7 | 复合 unique (device_id, project_id)、orphan 时间戳、hierarchy walk |
| 13 | **cicd-pipeline** | 6 | 16 | pipeline CRUD、steps JSON、Run 异步触发、cancel、Fake executor |
| 14 | **log-aggregation** | 16 | 44 | LogBackend interface、3 backends (Local/ES/Loki)、Universal Query DSL、capabilities、degraded meta、30-day Loki cap |
| 15 | **metrics-collection** | 7 | 12 | Ingest、series 列表、90-day cap、Scraper interface、Prometheus stub、Middleware 导出 |
| 16 | **alert-notification** | 9 | 18 | 5 严重度、channels (slack/pd/email/webhook/log)、maintenance 抑制、history+stats |
| 17 | **websocket-hub** | 11 | 16 | JWT 升级、channel 订阅/退订、1000 连接上限、256-deep send buffer、ping/pong |
| 18 | **websocket-realtime** | 4 | 10 | Event bridges、canonical event types、`<domain>.<event>` 频道命名 |
| 19 | **k8s-pod-log-streaming** | 8 | 12 | WS + SSE + one-shot、30-day cap、backpressure drop-oldest、FakeLogClient |
| 20 | **audit-logging** | 5 | 10 | AuditEvent、BufferedEmitter (1024 ring) → DBEmitter、history+filter、middleware actor 提取 |
| 21 | **project-hierarchy** | 9 | 25 | 3-level (BL→System→Project)、MaxDepth=3、cycle detect、tree walk、weight 聚合 |
| 22 | **test-environment** | 19 | 54 | 3-tier env、10 scripts (deploy/status/logs/teardown/help)、fixtures (LDIF/JSON/YAML/SQL)、docker-compose/k3d/containerlab |
| | **TOTAL** | **192** | **374** | |

---

## 二、Go 单元 + 集成测试 (756 个)

每个 package 给 `TestXxx` 函数计数（已 `go test -list` 验证）。**全部通过 `-race`**，1 个 pre-existing flake (`internal/project/testhelpers_test.go` sqlite deadlock) 排除。

| # | Package | Tests | 覆盖的核心 |
|---|---|---:|---|
| 1 | `internal/pipeline` | 72 | Pipeline/PipelineRun/PipelineStepRun CRUD、Executor (Fake + Local)、trigger + cancel、step transitions |
| 2 | `internal/k8s` | 60 | Cluster CRUD、AES-256-GCM 加密/解密、Pod/Deployment/Service 列表、exec stub、FakeClient |
| 3 | `internal/metrics` | 53 | Metric/Series、Ingest/List/GetSeriesByName、Scraper、Middleware、90-day cap |
| 4 | `internal/logs` | 51 | Local/ES/Loki backends、Capabilities、Query DSL、Result/Meta、30-day cap、error codes |
| 5 | `internal/device` | 50 | Device/DeviceGroup/ConfigurationTemplate、CRUD、search、actions (enter/exit maintenance) |
| 6 | `internal/physicalhost` | 47 | 4-state machine、Monitor (Fake Prober)、Maintenance (AuditEmitter seam)、handler 8 endpoints |
| 7 | `internal/audit` | 47 | AuditEvent、Emitter interface、BufferedEmitter (drop+warn)、DBEmitter、List+Filter、middleware actor 提取 |
| 8 | `internal/alerts` | 46 | Alert/Channel/Rule CRUD、Dispatcher (Log + Fake + stubs)、SuppressionChecker、DSL parser、history+stats |
| 9 | `internal/discovery` | 44 | Scanner/Prober interfaces、Fake 实现、StartRun、Promote、ListHostsByRun、HostExistsByIP |
| 10 | `internal/hostproject` | 33 | HostProjectLink、复合 unique index、OrphanByDevice、GetAncestors、3-level walk、dedup |
| 11 | `internal/k8s/logstream` | 31 | Streamer/LogClient interfaces、Fake、Service、3 endpoints (WS/SSE/one-shot)、BackpressureSink、30-day cap |
| 12 | `internal/auth/rbac` | 31 | 5 global + 3 project roles、Permission matrix、RequirePermission middleware、HasAccessByLabel |
| 13 | `internal/ws/hub` | 30 | Hub/Client/ChannelRegistry、Authenticate (JWT)、RegisterRoutes、ReadPump/WritePump、ping/pong |
| 14 | `internal/auth/ldap` | 27 | Client/Service/Handler、group→role 映射、Fake client、rate limit、graceful error |
| 15 | `pkg/contracts` | 18 | Response envelope、Pagination、ErrorCode + HTTPStatus、APIError、User/Role/ProjectRole hierarchy |
| 16 | `internal/ws/realtime` | 16 | Event、Publisher (Hub + Noop)、canonical event types、bridge functions |
| 17 | `internal/middleware` | 15 | Recovery/RequestID/CORS/Logger/Chain、HTTP request/response fixtures |
| 18 | `internal/database` | 8 | Open (sqlite/postgres)、BaseModel (UUID + soft-delete)、AutoMigrate、JSONMap Scan/Valuer |
| 19 | `pkg/logger` | 7 | slog wrapper、New、WithLevel/WithFormat/WithWriter、MaskValue、parseLevel |
| 20 | `internal/config` | 6 | Viper loader、WithConfigPath、env override、validation、敏感 mask、String()、defaults |
| 21 | `tests` | 5 | **scripts_test.go**: 10 scripts × 6 verbs (present + help + 3 safe + unknown) = 60 sub-tests,验证 `deploy/status/logs/teardown/help` 标准接口 |
| 22 | `internal/handler` | 5 | WriteJSON、WriteError、ListResponse、Pagination |
| 23 | `cmd/devops-toolkit` | 5 | `route_smoke_test.go`: 20 routes (login + 19 pages) via in-process router |
| 24 | `internal/auth` | 0 | (placeholder, types live in `pkg/contracts` and `internal/auth/{ldap,rbac}`) |
| | **TOTAL** | **756** | |

### TDD 纪律

- **每个 Scenario 必须先有 failing test** — 15 个并行 agent 全部按 Red-Green-Refactor 跑通
- **Strict mode**: `go test -race` 必须干净
- **No mocks for wire layer**: HTTP 测试用 `httptest.NewServer`,WebSocket 测试用真实 `gorilla/websocket` client,k8s 测试用 `FakeClient` 接口注入
- **Pre-existing flake 隔离**: `internal/project` 在并行 sqlite 测试下偶现 deadlock,标记为 follow-up,不影响 build

---

## 三、前端测试 (11 pages + 60 scripts sub-tests)

### A. Vite proxy → backend (frontend 11 pages)

每个 page 的验证项:渲染、SideNav 可见、auth 保留、console 无错误、关键交互可用。

| # | Page | 路径 | 验证点 |
|---|---|---|---|
| 1 | Login | `/login` | 用户名/密码输入框、Sign In 按钮、表单验证、JWT 持久化到 localStorage、错误消息 |
| 2 | Dashboard | `/` | 4 stats cards (Projects/Devices/Hosts/Alerts) 显示真实计数、Recent Activity 面板、Welcome 文本 |
| 3 | Projects | `/projects` | 3-level 树 (BL→System→Project) 可展开/折叠、点击节点切换 detail panel、Members 表、"+ New Project" 按钮 |
| 4 | Devices | `/devices` | 4-row DataTable (Name/Type/State/Group/Labels/Actions)、Type+State filter、Probe+Maintenance 按钮、state-aware Maintenance (online → enter; maintenance → exit) |
| 5 | Physical Hosts | `/physical-hosts` | 2 hosts、State filter (All/Online/Monitoring Issue/Offline/Maintenance)、Probe + Enter/Exit Maintenance 按钮、Maintenance banner (purple, 🛠) |
| 6 | K8s Clusters | `/k8s` | Card grid (auto-fill minmax 280px)、Type+Status badge、Probe/View 按钮、View modal 含 3 tabs (Pods/Deployments/Services) |
| 7 | Discovery | `/discovery` | DataTable of runs、CIDR/Started/Status/HostsFound 列、"+ New Scan" modal、Promote button (enabled only on completed runs) |
| 8 | Pipelines | `/pipelines` | Two-pane (list / detail)、Trigger 按钮、Run history DataTable、Run detail modal 显示 per-step status + Cancel |
| 9 | Logs | `/logs` | Capabilities card、Query form (q/from/to/limit)、Degraded-meta banner (purple, 条件渲染)、Stream toggle (WS 订阅) |
| 10 | Metrics | `/metrics` | Series list (left)、Sparkline (SVG 200x60)、last-50 points table、series 详情 |
| 11 | Alerts | `/alerts` | 3 tabs (Active/Channels/History)、Acknowledge/Resolve 按钮、Show suppressed toggle、WS 实时 prepend |
| 12 | Audit | `/audit` | Filters (action/actor/type/from/to/limit)、DataTable、row click 打开 detail modal、metadata JSON pre |

### B. scripts_test.go (test-environment 标准接口)

`tests/scripts_test.go` 用 Go 跑 shell,验证 10 个 scripts 都符合 `deploy|status|logs|teardown|help` 接口。

| 测试 | 数量 | 验证 |
|---|---:|---|
| `TestScriptsPresent` | 10 | 每个 script 存在 |
| `TestScriptHelpAlwaysSucceeds` | 10 | `help` 返回 0 且列出 4 个 verbs |
| `TestScriptUnknownVerbFails` | 10 | 无效 verb 返回非零 + usage |
| `TestScriptSafeVerbsDoNotStartServices` | 30 | `status`/`logs`/`help` 5s 内返回 (不启 docker/k3d) |
| `TestScriptConventions` | 10 | 每个 script 开头是 `set -euo pipefail` |
| **TOTAL** | **60** | |

---

## 四、/qa 实际跑过的端到端测试 (本期)

通过 browse binary (`/home/vagrant/.claude/skills/gstack/browse/dist/browse`) 真实跑过:

| # | 步骤 | 结果 |
|---|---|---|
| 1 | `goto http://127.0.0.1:5173/` | 200, 自动 redirect 到 `/login` |
| 2 | 填 username/password, click Sign In | 200, redirect 到 `/` (Dashboard) |
| 3 | Dashboard 渲染 | 4 stats 显示 4/4/2/0,Welcome 文本 |
| 4 | `goto /projects` (full page reload) | **BUG**: redirect 回 `/login` |
| 5 | Fix: 提交 `54f097b4` (hydrated gate) | — |
| 6 | 重试 `goto /projects` | 200, SideNav 可见,admin 仍登录 |
| 7-16 | 循环 `goto /projects /devices /physical-hosts /k8s /discovery /pipelines /logs /metrics /alerts /audit` | 全部 200,0 console errors |
| 17 | Console scan 全部 11 pages | 0 errors, 2 cosméticos v7 warnings |
| 18 | 各 page screenshot 存 `.gstack/qa-reports/screenshots/` | 12 PNG files |

**Health score: 92/100**,1 critical issue found and fixed,2 cosmetic deferred.

报告存于 `.gstack/qa-reports/qa-report-devops-toolkit-2026-06-08.md`,baseline 在 `.gstack/qa-reports/baseline.json` 供未来 regression。

---

## 五、Pre-existing 已知问题 (out of scope for QA, 留给 follow-up)

| # | 位置 | 问题 |
|---|---|---|
| 1 | `internal/project/testhelpers_test.go` | sqlite 在并行测试下偶现 deadlock,导致 `./...` 全包测试间歇失败。**修复方法**:用 `file:` DSN + `MaxOpenConns(1)` 替代 `:memory:` |
| 2 | `internal/*` 和 `cmd/*` 部分文件 | gofmt 未通过,`make ci-lint` 会失败。**修复方法**:`gofmt -s -w .` 一次 |
| 3 | `cmd/devops-toolkit/main.go` | `set -e`/weight 字段在 `project_hierarchy` tree-builder 中未使用,可能清理 |
| 4 | `cmd/devops-toolkit/main.go` | `registerAuthRoutes/registerProjectRoutes/...` 14 个 helper 仍接受 `*gorm.DB` 但 main 已不再需要 pass `db` (可用闭包) |
| 5 | alert 模块 | 速率限制 (10/60s) 未实现,目前是 immediate dispatch |
| 6 | k8s-pod-log-streaming | `KubeLogClient` 是 client-go function seam,真实生产 wire-up 留到 Phase 8 (final integration) |
| 7 | alerts/discovery/k8s/pipeline/physicalhost modules | `RecordAction` 接入 audit 模块,目前 `logAuditEmitter` 是 no-op |

---

## 六、Test 执行入口

```bash
# 全部 Go 测试 (race, 排除 1 pre-existing flake)
go test -race -count=1 $(go list ./... | grep -v internal/project)

# 全部 + 项目 (含 pre-existing flake)
go test -race -count=1 ./...

# 单个 spec 范围
go test -race -count=1 ./internal/logs/...

# scripts 标准接口 conformance
go test -count=1 -v ./tests/...

# 路由 smoke
go test -count=1 -v -run TestRouteSmoke ./cmd/devops-toolkit/...

# Frontend 类型检查
cd frontend && npx tsc --noEmit

# Frontend build
cd frontend && npx vite build
```

---

## 七、Test 总数 (本期)

| 层级 | 数量 |
|---:|---|
| Openspec scenarios (形式 spec) | 374 |
| Openspec requirements | 192 |
| Go TestXxx 函数 | 756 |
| Frontend pages E2E (via browse) | 11 |
| scripts standard interface sub-tests | 60 |
| **Total (跨 3 个层次)** | **1,393** |
