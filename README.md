# DevOps Toolkit

Go-based internal DevOps platform for managing infrastructure, CI/CD pipelines, logs, alerts, physical hosts, multi-cluster Kubernetes, and a microservice catalog with on-call and runbooks.

> **📖 新接手代码请先读 [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)** — 文档结构、阅读顺序、权威性。

**Current version:** see [VERSION](VERSION) (v0.2.0.0 as of 2026-06-10). Recent changes in [CHANGELOG.md](CHANGELOG.md).

---

## 项目概述

- **类型:** 内部 SRE/DevOps 平台
- **目标用户:** DevOps/SRE 工程师 + on-call operator
- **核心能力:** 设备、流水线、日志、告警、多集群 K8s、物理主机、项目管理、微服务目录(含 on-call + runbook)、分布式追踪、Prometheus 指标 + Grafana 仪表盘
- **技术栈:** Go 1.25 + Gin + GORM + PostgreSQL (后端) ; React + TypeScript + Vite (前端)
- **代码规模:** ~25,700 行 Go 后端 + React 前端 (本仓库就是当前实现,不是"待重建")

---

## 文档入口

| 文档 | 用途 |
|------|------|
| **[DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)** | **文档索引(推荐入口)** |
| [PRD.md](PRD.md) | 产品需求(要做什么)|
| [openspec/specs/](openspec/specs/) | 形式化技术规格(怎么做,权威)|
| [ARCHITECTURE.md](ARCHITECTURE.md) | 后端架构 (含历史迁移说明) |
| [REQUIREMENTS.md](REQUIREMENTS.md) | 后端技术规格 + 功能完成状态 |
| [docs/FRONTEND.md](docs/FRONTEND.md) | 前端实现规格 |
| [DESIGN.md](DESIGN.md) | 前端设计系统 |
| [DEPLOY.md](DEPLOY.md) | 部署指南 |
| [docs/CONFLICTS.md](docs/CONFLICTS.md) | 文档冲突清单(历史溯源,选择性参考) |
| [CHANGELOG.md](CHANGELOG.md) | 变更日志 |
| [TEST_CASES.md](TEST_CASES.md) | 测试用例总览 |

---

## 核心模块 (v0.2.0.0)

| 模块 | 用途 | 状态 |
|------|------|------|
| `internal/device` | 物理/虚拟/网络设备统一管理(状态机) | ✅ |
| `internal/pipeline` | CI/CD 编排、策略(blue-green / canary / rolling)、运行历史 | ✅ |
| `internal/servicecatalog` | 微服务目录、健康 rollup、Service ↔ Pipeline 关联、on-call rotation、runbook | ✅ (P0–P2) |
| `internal/logs` | 多后端日志(local / ES / Loki)、保留策略、stats | ✅ |
| `internal/metrics` | Prometheus 指标采集 + `/metrics` endpoint | ✅ |
| `internal/observability` | OpenTelemetry tracing 中间件 + `X-Trace-Id` 响应头 | ✅ |
| `internal/alerts` | 多通道告警(Slack/Webhook/Email/Log)+ alertmanager wiring + Prometheus alert rules | ✅ |
| `internal/k8s` | 多集群 K8s 管理 (k3d/kind/standard/tke/eks) + 真实 client-go + per-cluster ClientRegistry | ✅ (P1.6) |
| `internal/k8s/logstream` | K8s pod 日志一次性 + WebSocket 流式查询 | ✅ |
| `internal/physicalhost` | SSH 物理主机监控、prober、InfluxDB writer、metrics cache | ✅ |
| `internal/project` | 业务线 → 系统 → 项目 组织层级 + FinOps | ✅ |
| `internal/auth` | LDAP + JWT 中间件 + RBAC 权限矩阵 | ✅ |
| `internal/server` | TLS + mTLS 配置 | ✅ |
| `internal/ws/{hub,realtime}` | WebSocket 实时事件推送 | ✅ |
| `internal/audit` | 审计日志 (项目管理变动) | ✅ |
| `internal/discovery` | SNMP/SSH 网络发现 | ✅ |

前端 (`frontend/`): React 18 + TypeScript + Vite + Zustand,主要页面包括 Services / Pipelines / Physical Hosts / K8s Clusters / Logs / Trace Detail / Metrics Panel。

---

## 技术栈 (v0.2.0.0)

| 层级 | 技术 |
|------|------|
| 后端语言 | Go 1.25 (toolchain 1.26) |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL 14+ (生产) / SQLite (测试) |
| 配置 | Viper |
| 日志 | log/slog (Go 标准库) |
| 认证 | LDAP + JWT |
| Tracing | OpenTelemetry (stdouttrace + 可选 OTLP) |
| K8s client | k8s.io/client-go v0.36.1 (typed clientset, in-memory kubeconfig) |
| 前端 | React 18 + TypeScript + Vite |
| 状态管理 | Zustand |
| 实时 | gorilla/websocket |
| 测试 | go test (race) + vitest |
| 部署 | Docker Compose / k3d / Containerlab |
| 监控栈 | Prometheus + Alertmanager + Grafana + Loki |

---

## 快速开始

```bash
# 后端
go build -o devops-toolkit ./cmd/devops-toolkit
./devops-toolkit                          # 默认 :3000

# 前端
cd frontend
npm install
npm run dev                               # Vite 默认 :5173, 代理 /api → :3000

# 开发用 JWT
./cmd/mint-dev-token/mint-dev-token --user alice --role developer

# 健康检查
curl http://localhost:3000/health

# Metrics
curl http://localhost:3000/metrics

# 全套测试
go test ./...                             # 28 包
cd frontend && npx vitest run             # 前端
```

完整部署见 [DEPLOY.md](DEPLOY.md);本地集成栈见 [deploy/docker-compose.yml](deploy/docker-compose.yml) + [deploy/grafana/dashboards/](deploy/grafana/dashboards/)。

---

## 测试

```bash
# 单元 + 集成
go test ./...

# Race detector
go test -race ./...

# Load test (需服务运行)
LOAD_BASE_URL=http://localhost:3000 LOAD_TOKEN=dev-bypass \
  LOAD_VUS=50 LOAD_DURATION=20s \
  go test -tags=load -run TestLoadPhysicalHosts ./tests/load/...

# 前端
cd frontend && npx vitest run && npx tsc --noEmit
```

SLO 强制: load test 触发 p95 > 500ms 或 错误率 > 1% 时 fail。详见 [tests/load/loadgen_test.go](tests/load/loadgen_test.go)。

---

## 文档状态

- ✅ PRD v2.1 + openspec/specs/ 为权威来源
- ✅ README + CHANGELOG + VERSION 与代码同步 (v0.2.0.0)
- ⚠️ ARCHITECTURE.md 顶部带有迁移前/后说明,核心架构图反映当前 Gin/GORM 状态
- 📦 docs/archive/ 为历史归档

---

## 许可证

参见 [LICENSE](LICENSE)。
