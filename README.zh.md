<div align="center">

# DevOps Toolkit · 运维工具集

**管理多区域数据中心 100+ 节点的内部平台 —— CI/CD 流水线、多集群 Kubernetes、物理主机、含 on-call 与 runbook 的服务目录，以及 Prometheus 级别的可观测性栈。**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](#)
[![React 18](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](#)
[![PostgreSQL 14+](https://img.shields.io/badge/PostgreSQL-14+-336791?logo=postgresql&logoColor=white)](#)
[![License: MIT](https://img.shields.io/badge/License-MIT-22c55e)](#)
[![Version](https://img.shields.io/badge/version-0.5.1.0-22d3ee)](VERSION)

[快速开始](#快速开始) · [功能特性](#功能特性) · [截图](#截图) · [架构](#架构) · [参与贡献](#参与贡献)

</div>

> 📖 **Languages:** [English](README.md) · [简体中文](README.zh.md) — 项目的文档与 UI 双语处理详见 [i18n 规范](docs/i18n/SPEC.md)。

---

## 为什么需要 DevOps Toolkit

如果你曾把 **Jenkins + Argo + Prometheus + Loki + 自研审计日志 + 手写的设备状态机** 缝在一起，结果开着 14 个标签页到处切换，这个项目就是 SRE 团队经历过这种痛苦之后的产物。一个 Go 二进制 + 一个 React SPA：

- 拥有**设备生命周期**（状态机：`online` → `monitoring_issue` → `offline` → `maintenance`）
- 运行**blue-green / canary / rolling** 策略的流水线，phase 显式
- 用**真实的 `client-go`** 与已注册的 K8s 集群（k3d、kind、standard、EKS、TKE）通信
- 通过 WebSocket 流式推送**pod 日志**，并通过类型化终端**exec pod**
- 跨 8 个模块为每个写操作产生**审计轨迹**
- 开箱即用**7 张生产形态的 Grafana dashboard**

代码库本身就是实现，而不是 spec：~25,700 行 Go、33 个包、31 个测试包全绿；两张按角色划分的 Grafana 入口 dashboard——根据你是在被叫醒还是在上线功能，落点不同。

---

## 适用对象

| 你是谁 | 优先打开这张 dashboard |
|---|---|
| **凌晨 3 点被叫醒的 on-call operator** | [Ops 视图](docs/screenshots/ops-view.png) —— SLO、Top 错误路由、抖动主机、告警状态 |
| **上线流水线或接口的工程师** | [Developer 视图](docs/screenshots/developer-view.png) —— *我的*路由流量 / 5xx / p95，*我的*流水线成功率 |
| **检查整个机群的 SRE / 平台负责人** | [Overview](docs/screenshots/overview.png) —— 系统全局健康一览 |
| **跟踪某台主机的硬件 / DC 运维** | [Physical host metrics](docs/screenshots/physical-host-metrics.png) —— 单机 CPU / 内存 / 磁盘 |

两张角色化 dashboard（`developer-view`、`ops-view`）在 v0.5.1.0 加入 —— 它们是入口；另外 5 张是按域下钻的，从每张入口的底部 row 链接过去。

---

## 功能特性

### 🛠  能跑起来的基础设施

- **单一 Go 二进制** —— `go build -o devops-toolkit ./cmd/devops-toolkit` → 95MB 静态二进制，distroless 兼容
- **单页 React 前端** —— TypeScript + Vite + Zustand，深色 ops 主题，14 个页面
- **单进程部署** —— 监听 `:3000`，SPA + JSON API + WebSocket 在同一端口
- **PostgreSQL 或 SQLite** —— 同一套 GORM 模型，开发默认 SQLite，生产用 Postgres
- **TLS / mTLS 准备** —— `internal/server/tls.go` + `cmd/gen-mtls-cert/` 用于证书生成

### 🔄  设备与物理主机生命周期

- **状态机**（`internal/physicalhost/monitor.go:32`）：`online` → `monitoring_issue`（1 次失败）→ `offline`（连续 3 次）→ `online`（1 次成功），`maintenance` 作为并行 hold
- **真 SSH prober**（`internal/physicalhost/prober/sshprober.go`）：ed25519 密钥认证，可选连接池，`time.AfterFunc` 逐主机超时
- **TCP fallback prober**（未配置密钥时）—— 端口 22 可达性检查，监控服务网格有用
- **每主机指标写 InfluxDB** —— CPU / 内存 / 磁盘时序，异步缓冲 writer（1024 条缓冲，2 个 worker）

### 🚀  真实策略的 CI/CD 流水线

- **blue-green / canary / rolling** 策略规划器（`internal/pipeline/strategy.go`）—— 三种具体方案，同一接口
- **Phase 跟踪** —— strategy.Plan 解析为有序步骤，监控状态机观察每个
- **运行历史** —— 每次运行 + 每步都记入 Postgres，含 `status` / `duration_ms` / `error_message` / `exit_code` / `output_truncated`
- **p50 / p95 / p99 持续时间百分位** —— 通过 dashboard SQL 的 `percentile_cont` 计算

### ☸️  多集群 Kubernetes

- **内存中的 kubeconfig** —— 已注册集群获得每集群 `ClientRegistry`（`internal/k8s/registry.go`），不需要落盘 kubeconfig
- **真实的 `client-go` 类型化 clientset**（`internal/k8s/client.go`）—— 不是自定义抽象
- **Pod 日志流** —— `WS` + `SSE` + 一次性端点，统一在 `/api/v1/k8s/clusters/:clusterID/pods/:namespace/:pod/logs[/{stream,sse}]`
- **Pod exec** —— SPDY `k8s.pods.exec` 权限，前端的终端 widget

### 📋  含 on-call 与 runbook 的服务目录

- **微服务实体** 带 `tier`（`critical` / `standard` / `internal`）和 `derived_from`（`k8s_pod_health` / `last_pipeline_run` / `manual`）
- **健康汇总** —— gauge 发 `0=unknown / 1=healthy / 2=degraded` 给每个服务，可告警
- **On-call 轮值** + **runbook 条目**，有写 API（`POST /api/v1/services/:id/oncall`、`POST /:id/runbook`）
- **Service ↔ Pipeline** 关系 —— 把服务和它的部署流水线关联起来，在一个视图里看健康 + 部署节奏

### 🔐  认证与 RBAC

- **LDAP 或 dev_bypass** —— `internal/auth/middleware.go:137` 在 dev 下用 `X-User: <name>` 头短路到伪 `RoleOperator`
- **JWT**（生产）—— `cmd/mint-dev-token` 铸一个开发 token
- **逐权限矩阵**（`internal/auth/rbac/matrix.go`）—— 5 个角色 × 30+ 权限键，包括少见的 `PermissionViewAuditLogProject` 给 `ScopedAuditor` 角色用（按租户过滤审计，看不到其他租户的日志）
- **逐项目访问中间件** —— `requireMembership` 辅助函数强制「用户属于项目 → 可触碰资源」，即使角色矩阵说 yes

### 📊  物有所值的可观测性

- **Prometheus `/metrics`** —— `http_requests_total{route, method, status}` + `request_duration_seconds` 直方图 + Go runtime
- **Monitor-loop 指标** —— `monitor_loop_iterations_total`、`monitor_loop_errors_total{host_id}`、`monitor_loop_last_tick_timestamp_seconds`（可告警 "loop 卡死"）
- **服务健康指标** —— `service_health_status{service_id, tier, derived_from}`（gauge 0/1/2）
- **Postgres 中的审计轨迹** —— `actor_id` 取自 JWT subject（不是请求体里的字段），`metadata` JSON，`ip_address`，`user_agent`，跨 8 个模块覆盖每个写操作
- **结构化日志** —— `log/slog` JSON 输出，通过 `X-Trace-Id` 传播 request ID
- **OpenTelemetry tracing** —— 默认 stdouttrace，OTLP 可选，基于父级的比率采样

### 🖥  不挡路的前端

- **深色 ops 主题** —— 终端延伸美学，JetBrains Mono 用于数据，Geist 用于 UI
- **实时数据** —— WebSocket 驱动的服务健康、监控状态、日志尾
- **14 个页面**：Dashboard、Services、Pipelines、Physical Hosts、K8s Clusters、Logs、Metrics、Alerts、Audit、Projects、Devices、Discovery、Trace Detail、Login
- **Vitest** 测 React 组件，RTL 测交互，CI 里 `tsc --noEmit`

---

## 截图

### Ops 视图 —— 给 on-call operator

[![](docs/screenshots/ops-view.png)](docs/screenshots/ops-view.png)

SLO 行（流量 / 错误预算 / p95 / monitor-loop 卡死）→「什么坏了」表格（Top 错误路由、抖动主机）→ 健康趋势（tick 速率、2xx/4xx/5xx 堆叠、流水线失败率）→ 告警状态（服务降级、卡住的 running 流水线、离线主机 %）。底部 row 链接到所有 5 张按域的 dashboard。

### Developer 视图 —— 给工程师

[![](docs/screenshots/developer-view.png)](docs/screenshots/developer-view.png)

把 `my_route` 设成 `/api/v1/pipelines`（或任意路由前缀），把 `project` 设成你的项目 UUID —— 顶部 stat 过滤到你的工作，per-route top-10 图表保持全局，「My project's pipelines」行限定到你的 UUID，实时错误流显示后端的 `level="error"` 行。

### Overview —— 系统全局一览

[![](docs/screenshots/overview.png)](docs/screenshots/overview.png)

最早的入口 —— HTTP 流量、p95 延迟、错误率、流水线结果、主机可用性。适合贴在团队站会屏幕上。

### Physical host metrics —— 每主机时序

[![](docs/screenshots/physical-host-metrics.png)](docs/screenshots/physical-host-metrics.png)

每主机 CPU / 内存 / 磁盘时序来自 InfluxDB，配合 Postgres `physical_hosts` 表的「陈旧主机」检测。

---

## 快速开始

需要 **Docker**（或 **Podman**）和 **Go 1.25+**。完整开发环境只需一个脚本：

```bash
git clone https://github.com/your-org/devops-toolkit.git
cd devops-toolkit
bash scripts/dev-env-up.sh
```

这个脚本会：

1. 启动 8 服务的 compose 栈（Postgres、Loki、Prometheus、Grafana、Alertmanager、Redis，加上后端）
2. 部署 8 节点 containerlab 拓扑（2 DC × 4 节点：每 DC 1 个 core switch + 3 个 sshd 主机）
3. 自动生成 ed25519 prober 密钥对，把公钥注入 containerlab 节点
4. 把每个物理主机注册为 `device` + `physical_hosts` 行
5. 触发一次手动探针，让 dashboard 在第一个 1 分钟 tick 之前就有数据
6. 打印访问 URL

脚本完成后访问点：

| 服务 | URL | 凭据 |
|---|---|---|
| 后端 API | http://localhost:3000 | 头 `X-User: alice`（dev_bypass） |
| 前端 | http://localhost:5173 | — |
| Grafana | http://localhost:3001 | `admin` / `admin` |
| Prometheus | http://localhost:9090 | — |
| Loki | （无 UI；通过 Grafana 查询） | — |

完成后：

```bash
bash scripts/dev-env-down.sh
```

### 不跑完整 dev 栈 —— 只要二进制

```bash
go build -o devops-toolkit ./cmd/devops-toolkit
./devops-toolkit                  # 监听 :3000，SQLite 在 tests/fixtures/db/dev.db
```

### 测试

```bash
go test ./...                                  # 31 个包
go test -race ./...                            # race 检测
cd frontend && npx vitest run                  # React 组件
```

---

## 架构

```
┌──────────────────────────────────────────────────────────────────┐
│  Frontend (React 18 + TS + Vite, :5173)                         │
│  Zustand state · WebSocket (gorilla/ws) · 14 pages              │
│  i18next + react-i18next (en / zh-CN) · LanguageSwitcher         │
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

详细架构：[ARCHITECTURE.md](ARCHITECTURE.md) · 规格：[openspec/specs/](openspec/specs/) · 按模块设计：[docs/BACKEND.md](docs/BACKEND.md)、[docs/FRONTEND.md](docs/FRONTEND.md)

开发环境用 8 节点 containerlab 拓扑（2 DC × 4 节点：1 个 core switch + 3 个 sshd 主机）做本地验证 —— 生产部署通过同一套 REST + WebSocket API 扩展到**多区域数据中心的 100+ 节点**。

---

## 设计哲学

前端**有意不是** Material Design 风格的克隆。它是终端的延伸 —— 深色 `#0c1220` 背景，`#22d3ee` 青色作为重点，JetBrains Mono 用于所有应该是数据的部分（设备 ID、IP、SSH 命令、指标值），Geist 用于 UI 标签。美学是「这是工具，不是产品」：

- **功能胜过装饰** —— 边框、颜色、字重完成所有工作
- **数据密度优先** —— 表格宽，panel 垂直堆叠，没有留白 hero 区
- **状态色** 在整个系统里映射到同样的三种状态：`#22c55e`（online/healthy/success）、`#f59e0b`（warning/monitoring_issue）、`#ef4444`（offline/error）
- **数字用 tabular-nums** —— 数字在列里对齐；`JetBrains Mono` 自带 `font-variant-numeric: tabular-nums`

参考：[DESIGN.md](DESIGN.md)（设计系统的真相之源 —— 颜色、字号、间距、动效）。

---

## 模块地图

| 模块 | LOC | 功能 |
|---|---|---|
| `internal/device` | ~3k | 通用设备清单（网络 / 容器 / 物理） |
| `internal/pipeline` | ~5k | 流水线 CRUD + 运行历史 + 策略规划器 |
| `internal/k8s` | ~3k | 多集群 K8s client 注册（k3d、kind、EKS、TKE） |
| `internal/k8s/logstream` | ~1.5k | Pod 日志 via WS / SSE / 一次性 |
| `internal/physicalhost` | ~4k | 状态机、monitor loop、InfluxDB writer、audit adapter |
| `internal/physicalhost/prober` | ~1k | SSH prober（密钥）+ TCP fallback |
| `internal/logs` | ~2k | 多后端（local / ES / Loki）+ 保留 + 保存过滤器 |
| `internal/alerts` | ~2.5k | 多通道（Slack / Webhook / Email / Log） |
| `internal/servicecatalog` | ~3.5k | 微服务 + on-call + runbook + 健康汇总 |
| `internal/project` | ~2k | 业务线 → 系统 → 项目层级 + FinOps 钩子 |
| `internal/auth` | ~2k | LDAP + JWT + RBAC 矩阵 + dev_bypass |
| `internal/observability` | ~0.5k | Prometheus exporter + OTel tracing |
| `internal/middleware` | ~0.7k | Chain 顺序、request ID、logger、recovery、CORS |
| `internal/ws` | ~1.5k | 实时事件 hub（gorilla/websocket） |
| `internal/audit` | ~1k | 审计轨迹（项目变更） |
| `internal/discovery` | ~1.5k | SNMP/SSH 网络发现 |
| `cmd/devops-toolkit` | ~3k | 接线 + AutoMigrate + 路由注册 |
| `cmd/mint-dev-token` | ~0.1k | 开发 JWT 铸造器 |
| `cmd/ws-smoke` | ~0.1k | WebSocket 烟雾测试 |
| `cmd/gen-mtls-cert` | ~0.1k | mTLS 证书生成器 |
| 前端（`frontend/src/`） | ~10k | React 18 + 14 个页面 + 10 个通用组件 |

---

## 项目状态

当前版本：**v0.5.1.0**（见 [VERSION](VERSION)）

- ✅ 31 个后端包，测试全绿（`go test ./...`）
- ✅ 1 个已知 unreachable warning 在 `internal/audit/repository.go:77`（不在范围内，审计报告里已排除）
- ✅ 7 张生产形态的 Grafana dashboard（4 张按域 + 3 张系统：overview、dev-view、ops-view）
- ✅ 5 个 C 子项目（Helm chart 脚手架、5 个子 chart 给 cert-manager / external-secrets / sealed-secrets / monitoring / network-policies）
- ✅ 认证 + RBAC + 跨租户强制落地（B 子项目，3 个 phase）
- ✅ 2026-06-10 生产就绪审计的 #1-#6 关闭

发布历史见 [CHANGELOG.md](CHANGELOG.md)，未完成项见 [TODOS.md](TODOS.md)。

---

## 文档

| 文档 | 何时读 |
|---|---|
| **[DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)** | 首先读 —— 文档树、阅读顺序、权威性图 |
| [PRD.md](PRD.md) | 我们要建什么（产品 spec） |
| [openspec/specs/](openspec/specs/) | 怎么建（形式化技术规格 —— 权威） |
| [REQUIREMENTS.md](REQUIREMENTS.md) | 后端功能完成状态 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 系统架构、请求流、数据流 |
| [docs/BACKEND.md](docs/BACKEND.md) | 后端代码规范 |
| [docs/FRONTEND.md](docs/FRONTEND.md) | 前端实现规格 |
| [DESIGN.md](DESIGN.md) | 设计系统（颜色、字号、动效） |
| [DEPLOY.md](DEPLOY.md) | 部署指南 |
| [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md) | 启动 8 节点测试环境 |
| [TEST_CASES.md](TEST_CASES.md) | 测试用例清单 |
| [docs/CONFLICTS.md](docs/CONFLICTS.md) | 文档历史冲突（考古用） |
| [TODOS.md](TODOS.md) | 未完成工作项 |
| [CHANGELOG.md](CHANGELOG.md) | 发布历史 |
| **[docs/i18n/SPEC.md](docs/i18n/SPEC.md)** | **i18n 规范**（双轨：文档 + UI） |

---

## 参与贡献

这个项目小到可以在一个周末从头到尾读完。要贡献：

1. **从 [TODOS.md](TODOS.md) 里挑一项** —— P0/P1 项有 file:line 引用和审计上下文
2. **从 `main` 拉分支** —— `git checkout -b feat/<short-slug>`
3. **改代码** —— 如果包里有 `_test.go` 兄弟就写一个测试；注释密度匹配周围代码
4. **跑套件** —— `go test -count=1 -p 1 -timeout 600s ./...`
5. **开 PR** —— 描述「做了什么」+「为什么」，链上 TODO 项或审计报告
6. **不要提交密钥** —— `deploy/secrets/prober/id_ed25519` 在 gitignore 里；`.pub` 是本地生成

代码规约写在代码里，不在 style guide 里。匹配周围的注释密度、命名风格、错误包装模式（`fmt.Errorf("module.X: %w", err)`）。

两条不明显的规则：

- **`auth.Bearer` 永远来自 JWT subject**，永远不要来自请求体字段 —— 审计轨迹依赖这个
- **不要改 `internal/k8s/handler.go` 里的路由参数名** —— k8s logstream handler 依赖 `:clusterID`；gin 在参数名冲突时会 panic

---

## 许可证

[MIT](LICENSE) —— Copyright (c) 2024 DevOps Toolkit 贡献者
