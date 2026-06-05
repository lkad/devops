# Architecture — DevOps Toolkit

> **⚠️ 文档警告:** 本文档部分内容已过时（迁移前状态）。
>
> 重新开发时，请同时参考：
> - [REQUIREMENTS.md](REQUIREMENTS.md) — 当前技术规格
> - [docs/CONFLICTS.md](docs/CONFLICTS.md) — 详细冲突清单
> - [openspec/specs/](openspec/specs/) — 权威形式化规格
>
> 主要冲突：HTTP 框架（gorilla/mux → Gin）、配置库（YAML → Viper）、项目 Type（内嵌枚举 → 独立表）

## Overview

DevOps Toolkit is a Go-based internal platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts. The system is organized around an **organizational hierarchy** (Business Line → System → Project) that provides project management and FinOps reporting capabilities.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      cmd/devops-toolkit                         │
│                    (HTTP Server, gorilla/mux)                  │
└─────────────────────────────────────────────────────────────────┘
         │           │           │           │           │
         ▼           ▼           ▼           ▼           ▼
┌─────────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────┐
│   device    │ │pipeline │ │  logs   │ │ alerts  │ │   project   │
│  manager    │ │ manager │ │ manager │ │ manager │ │   manager   │
└─────────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────────┘
         │                    │                             │
         ▼                    ▼                             ▼
┌─────────────────┐  ┌─────────────────┐         ┌─────────────────┐
│   PostgreSQL    │  │   File/ES/Loki  │         │   PostgreSQL    │
│  (devices)      │  │   (logs)        │         │  (projects)     │
└─────────────────┘  └─────────────────┘         └─────────────────┘
```

## Modules

### Core Modules

| Module | Purpose | Persistence |
|--------|---------|------------|
| `internal/device` | Device state machine, CRUD, SQLite/PostgreSQL | PostgreSQL |
| `internal/pipeline` | CI/CD pipeline orchestration, stage runner | In-memory + JSON |
| `internal/logs` | Log ingestion, multi-backend (local/ES/Loki) | File/ES/Loki |
| `internal/metrics` | Prometheus collector, /metrics endpoint | In-memory |
| `internal/alerts` | Notification channels, rate limiting, history | In-memory |
| `internal/k8s` | Kubernetes cluster management (k3d/kind for testing, standard k8s for production) | kubeconfig |
| `internal/discovery` | SNMP/SSH network discovery | In-memory |
| `internal/physicalhost` | SSH host monitoring, metrics collection | In-memory |
| `internal/websocket` | Pub/sub hub for real-time events | In-memory |
| `internal/auth/ldap` | LDAP authentication, connection pooling | LDAP server |

### Project Management Module (NEW)

`internal/project/` — Organizational hierarchy and FinOps reporting.

#### Data Model

```
BusinessLine
├── id (UUID, PK)
├── name (string, unique)
├── description (string)
├── created_at, updated_at

System
├── id (UUID, PK)
├── business_line_id (FK → BusinessLine, CASCADE)
├── name (string)
├── description (string)
├── created_at, updated_at

Project
├── id (UUID, PK)
├── system_id (FK → System, CASCADE)
├── name (string)
├── project_type_id (FK → ProjectType, CASCADE)  <!-- ⚠️ 独立表设计，详见 CONFLICTS.md #3 -->
├── description (string)
├── created_at, updated_at

ProjectResource (link table)
├── id (UUID, PK)
├── project_id (FK → Project, CASCADE)
├── resource_type (enum: device, pipeline, log_source, alert_channel, physical_host)
├── resource_id (string) — external ID from source manager
├── created_at
-- UNIQUE(project_id, resource_type, resource_id)

ProjectPermission
├── id (UUID, PK)
├── level (string) — "project" | "system" | "business_line"
├── project_id (UUID, nullable, FK)
├── system_id (UUID, nullable, FK)
├── business_line_id (UUID, nullable, FK)
├── role (enum: viewer, editor, admin)
├── subject (string) — LDAP user DN or group DN
├── created_at
-- INDEX on (subject, level)
```

#### Enums

> **⚠️ 冲突 #3:** REQUIREMENTS.md v2.1 采用独立 `project_types` 表 + `GORMProjectType` 模型，而非内嵌枚举。本节保留旧设计供参考，新实现应采用独立表。

```go
type ProjectType string
const (
    ProjectTypeFrontend ProjectType = "frontend"
    ProjectTypeBackend  ProjectType = "backend"
)

type ResourceType string
const (
    ResourceTypeDevice        ResourceType = "device"
    ResourceTypePipeline     ResourceType = "pipeline"
    ResourceTypeLogSource     ResourceType = "log_source"
    ResourceTypeAlertChannel  ResourceType = "alert_channel"
    ResourceTypePhysicalHost  ResourceType = "physical_host"
)

type Role string
const (
    RoleViewer Role = "viewer"
    RoleEditor Role = "editor"
    RoleAdmin  Role = "admin"
)
```

#### Permission Inheritance

Business Line → System → Project. Permission check walks up:
1. Check project-level permission
2. If none, check system-level permission
3. If none, check business line-level permission
4. If LDAP user is member of `config.yaml` → `ldap.super_admin_group` → full access

#### API Endpoints

All list endpoints support pagination (`?page=1&per_page=50`).

**Business Lines**
- `GET /api/org/business-lines` — list all
- `POST /api/org/business-lines` — create
- `GET /api/org/business-lines/:id` — get one with systems
- `PUT /api/org/business-lines/:id` — update
- `DELETE /api/org/business-lines/:id` — delete (CASCADE)

**Systems**
- `GET /api/org/business-lines/:bl_id/systems` — list
- `POST /api/org/business-lines/:bl_id/systems` — create
- `GET /api/org/systems/:id` — get one with projects
- `PUT /api/org/systems/:id` — update
- `DELETE /api/org/systems/:id` — delete (CASCADE)

**Projects**
- `GET /api/org/systems/:sys_id/projects` — list
- `POST /api/org/systems/:sys_id/projects` — create
- `GET /api/org/projects/:id` — get one with linked resources
- `PUT /api/org/projects/:id` — update
- `DELETE /api/org/projects/:id` — delete (CASCADE)

**Resource Linking**
- `POST /api/org/projects/:id/resources` — link resource
- `DELETE /api/org/projects/:id/resources/:resource_id` — unlink
- `GET /api/org/projects/:id/resources` — list linked resources

**Permissions**
- `GET /api/org/projects/:id/permissions` — list
- `POST /api/org/projects/:id/permissions` — grant
- `DELETE /api/org/permissions/:perm_id` — revoke

**FinOps Export**
- `GET /api/org/reports/finops?period=2026-04` — CSV export

#### FinOps CSV Format

```csv
Business Line,System,Project Type,Project,Resource Type,Count,Unit
电商事业部,订单系统,Backend,order-backend,VM,3,nodes
电商事业部,订单系统,Backend,order-backend,Storage,500,GB
电商事业部,订单系统,Backend,order-backend,Alerts,12,channels
```

## Integration Points

### Resource Linking

Projects link to existing resources via `ProjectResource`:
- `device` → `internal/device/manager.go` (device ID)
- `pipeline` → `internal/pipeline/manager.go` (pipeline ID)
- `log_source` → `internal/logs/manager.go` (log source identifier)
- `alert_channel` → `internal/alerts/manager.go` (channel name)
- `physical_host` → `internal/physicalhost/manager.go` (host ID)

### WebSocket Channels

The project hierarchy can be viewed in real-time via WebSocket subscriptions:
- `log` — log events
- `metric` — Prometheus metrics
- `device_event` — device state changes
- `pipeline_update` — pipeline run status
- `alert` — alert notifications

### LDAP Integration

- Authentication: LDAP bind for login
- Authorization: Local RBAC permissions (not synced to LDAP)
- SuperAdmin: Members of `ldap.super_admin_group` in `config.yaml` get full access

## Technical Stack

> **⚠️ 部分字段已过时。** HTTP Server 已迁移到 Gin，DB 驱动已迁移到 GORM，详见 [docs/CONFLICTS.md](docs/CONFLICTS.md) 冲突 #1、#2。

| Component | Technology | 状态 |
|-----------|------------|------|
| HTTP Server | `net/http` + `gorilla/mux` | ⚠️ 已迁移到 Gin |
| WebSocket | `gorilla/websocket` | ✅ |
| Database | PostgreSQL (`lib/pq`) | ⚠️ 已迁移到 GORM |
| SSH | `golang.org/x/crypto/ssh` | ✅ |
| K8s | k3d/kind CLI + `client-go` | ✅ |
| Config | YAML + environment overrides | ⚠️ 已改用 Viper |
| Logging | `log/slog` (stdlib) | ✅ |
| Testing | Go `testing` package | ✅ |

## Cross-Cutting Patterns

### 1. Backend Capability Abstraction (LogBackend)

日志子系统需要支持 Local / Elasticsearch / Loki 三种后端，三者查询语法差异巨大。**架构原则：后端可换，客户端零改动**。

**三层架构:**

```
┌─────────────────────────────────────────┐
│  Client (Frontend / 第三方)             │
│  - 用 Universal Query DSL 请求           │
│  - 先调 /capabilities 知道能用啥         │
│  - 看响应 meta 知道是否降级              │
└─────────────────────────────────────────┘
                  ↕ HTTP API (稳定契约)
┌─────────────────────────────────────────┐
│  API 适配层 (Go)                         │
│  - 接收标准 LogQuery，校验，补充默认值   │
│  - 调 backend.Query(Query) → LogPage    │
│  - 错误码映射 (backend err → 标准 err)  │
│  - 加 meta 信息到响应                    │
└─────────────────────────────────────────┘
                  ↕ LogBackend interface
┌─────────────────────────────────────────┐
│  后端实现                                │
│  - LocalBackend (子串匹配，内存扫)        │
│  - ElasticsearchBackend (翻译成 Lucene)   │
│  - LokiBackend (翻译成 LogQL)            │
└─────────────────────────────────────────┘
```

**核心不变式:**

1. **LogEntry schema 永远不变** — 字段名/类型/含义固定
2. **所有后端都能跑 Universal 子集** — 这是 SLA
3. **能力查询永远先于高级查询** — 客户端应先调 `/capabilities`
4. **错误永远可重试判断** — 标准错误码 (UNSUPPORTED_FEATURE / TIME_RANGE_EXCEEDED / BACKEND_UNAVAILABLE 等)
5. **降级永远透明** — `meta.degraded_features` 必须有
6. **后端切换 = 0 客户端代码改动**

**形式化规格:** [openspec/specs/log-aggregation/spec.md](openspec/specs/log-aggregation/spec.md) (16 requirements)
**详细设计:** [docs/LOG-QUERY-API.md](docs/LOG-QUERY-API.md) (13 章节)

### 2. Maintenance Mode (Alert Suppression Pattern)

物理主机可主动进入 `maintenance` 状态，期间的告警**抑制到外部通道**（Slack/PD/Email）但**保留到 log 通道**用于审计。

**核心设计:**

| 层 | 行为 |
|----|------|
| 状态机 | 在 online/monitoring_issue/offline 之上叠加 maintenance flag |
| 告警 | 维护中的主机产生的告警 `suppress_external=true`、仍记录到 log |
| 审计 | 维护进入/退出写审计日志，含 reason / expected_end_time |
| 审计员 | 通过 `/api/alerts/suppressed?host_id=X` 查看被抑制的告警 |

**为什么用 4 态而不是 5 态:** maintenance 与运行态正交（维护中监控仍可正常/异常），用 flag 模式避免 N×M 笛卡尔积。

**形式化规格:** [openspec/specs/physical-host-monitoring/spec.md](openspec/specs/physical-host-monitoring/spec.md) (Maintenance Mode, Maintenance Audit Trail)
**告警侧:** [openspec/specs/alert-notification/spec.md](openspec/specs/alert-notification/spec.md) (Maintenance Mode Alert Suppression)

### 3. Three-Tier Test Environment

三层环境策略是 dev/ci/prod 跨层可复用的模式，**架构层面**关注的不是工具而是**抽象契约**：

| 层 | 契约 | 启动时间 | 适用 |
|----|------|---------|------|
| dev | `use_mock=all`, `dev_bypass=true` | < 30s | 单元测试、快速迭代 |
| ci | 真实后端集成 + Containerlab | 2-5min | E2E、CI 集成 |
| prod | 真实硬件 + 完整监控 | - | 真实流量 |

**核心规则:** 同一份代码 + 不同 env config = 不同层。配置加载在启动时根据 `ENV` 环境变量选择 `config-{env}.yaml`，不修改代码。

**形式化规格:** [openspec/specs/test-environment/spec.md](openspec/specs/test-environment/spec.md) (19 requirements, 54 scenarios)
**详细清单:** [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md)

## Directory Structure

```
/mnt/devops/
├── cmd/
│   └── devops-toolkit/
│       └── main.go              # HTTP server, route wiring
├── internal/
│   ├── config/                  # YAML + env config loader
│   ├── device/                  # Device state machine, CRUD
│   ├── pipeline/                # CI/CD orchestration
│   ├── logs/                   # Log manager, multi-backend
│   ├── metrics/                # Prometheus collector
│   ├── alerts/                # Notification channels
│   ├── k8s/                   # k3d/kind cluster management
│   ├── discovery/              # SNMP/SSH network discovery
│   ├── physicalhost/           # SSH host monitoring
│   ├── project/               # Organizational hierarchy (NEW)
│   ├── websocket/              # Pub/sub hub
│   └── auth/
│       └── ldap/               # LDAP authentication
├── scripts/                     # Shell scripts (k3d, kind setup)
└── config.yaml                 # Configuration
```

## Design Documents

- [DESIGN.md](DESIGN.md) — Frontend design system (colors, typography, spacing)
- [ARCHITECTURE.md](ARCHITECTURE.md) — This file (backend architecture)
- [PRD.md](PRD.md) — Product requirements

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-24 | Add project management module with 3-level hierarchy | Enable FinOps reporting by Business Line → System → Project |
| 2026-04-24 | Local RBAC permissions, LDAP only for auth | Keep org hierarchy management in DevOps, not in LDAP |
| 2026-04-24 | PostgreSQL for project hierarchy persistence | Align with existing PostgreSQL usage for device manager |
| 2026-04-24 | Resource linking via explicit ProjectResource table | Enable FinOps aggregation across all resource types |
| 2026-04-24 | Permission inheritance: BL → System → Project | Simplify permission management, inherit from parent level |
| 2026-04-25 | 前端API路径使用相对地址 | 支持反向代理部署，路径可能是根路径或子路径 |
| 2026-04-25 | K8s集群Type字段替代Provider字段 | k3d/kind仅用于测试环境，生产环境使用标准k8s集群 |
| 2026-05-01 | K8s历史日志时间范围限制30天 | Loki查询最大支持约721小时(30天)，超出返回400错误；前端DatePicker自动限制选择范围 |

## 开发准则

### 1. API路径规范

**前端所有API调用必须使用相对路径，禁止使用绝对路径。**

原因：实际部署时可能通过反向代理（如 Nginx、Traefik）访问服务，代理路径可能是：
- `/` (根路径)
- `/devops/` (子路径)
- `/api/devops-toolkit/` (自定义路径)

使用绝对路径 `/api/...` 会绕过代理的前端路由，导致 404 错误。

**正确示例：**
```javascript
// ✅ 使用 path-relative 路径（无前导斜杠）
// basePath 来自 Vite 的 import.meta.env.BASE_URL
const basePath = import.meta.env.BASE_URL || '/';
const API_BASE = `${basePath}api`;  // 部署在 /devops/ 时为 'devops/api'

fetch(`${API_BASE}/k8s/clusters`)
fetch(`${API_BASE}/org/business-lines`)

// ✅ WebSocket 使用相对路径
const WS_URL = `${basePath}ws`
```

**错误示例：**
```javascript
// ❌ 禁止使用绝对路径（以 / 开头的路径）
fetch('/api/k8s/clusters')
fetch('/api/org/business-lines')
const WS_URL = `ws://${window.location.host}/ws`  // 使用绝对路径的 WebSocket
```

**适用于：**
- `fetch()` 请求 - 使用 path-relative 路径
- `WebSocket` 连接 - 使用 path-relative 路径
- 静态资源路径 - 使用 path-relative 路径

**代理配置要求：**

使用相对路径后，反向代理只需将请求路由到后端服务：

```nginx
# 子路径部署示例：/devops/
# 所有 /devops/* 请求都会被代理，包括 /devops/api/* 和 /devops/ws
location /devops/ {
    rewrite ^/devops/(.*) /$1 break;
    proxy_pass http://localhost:3000;
}
```

当浏览器访问 `http://example.com/devops/k8s` 时：
1. 前端返回 SPA 应用
2. 浏览器执行 `fetch('devops/api/k8s/clusters')`
3. 请求发送到 `http://example.com/devops/api/k8s/clusters`
4. 代理将 `/devops/api/k8s/clusters` 重写为 `/api/k8s/clusters` 并转发到后端

### 2. Kubernetes集群类型区分

**集群类型 (Type) 字段用于区分集群用途：**

| Type值 | 用途 | 说明 |
|--------|------|------|
| `k3d` | 测试/开发环境 | 本地k3d集群，用于功能测试 |
| `kind` | 测试/开发环境 | 本地kind集群，用于功能测试 |
| `standard` | 生产环境 | 标准Kubernetes集群 |

设计原则：
- k3d/kind仅用于测试开发环境，不代表生产集群类型
- 生产环境按标准K8s集群处理，所有操作相同
- Cluster数据结构的`Type`字段替代原有的`Provider`字段

### 3. K8s历史日志时间范围限制

**限制原因：**
- Grafana Loki 查询时间范围最大支持约 721 小时（30天）
- 超过此范围 Loki 返回 400 错误：`query time range exceeds the limit`

**前端限制：**
- DatePicker 组件自动限制日期选择范围不超过 30 天
- 调整开始日期时，自动缩短结束日期确保不超过 30 天
- 调整结束日期时，自动延长开始日期确保不超过 30 天

**后端限制：**
- `internal/k8s/log_query.go` 验证时间范围
- 超出限制返回 400 错误：`time range exceeds maximum of 30 days (requested X hours)`

**环境变量配置：**
```bash
LOG_STORAGE_BACKEND=loki   # 使用Loki存储 (可选: elasticsearch, 空=默认k8s原生)
LOKI_URL=http://localhost:3100  # Loki服务器地址
ELASTICSEARCH_URL=http://localhost:9200  # ES服务器地址
ELASTICSEARCH_INDEX=k8s-logs-*  # ES索引名
```

### 5. 日志查询功能

**通用日志存储后端配置：**

| Backend | 说明 | 配置项 |
|---------|------|--------|
| `local` | 本地内存存储（默认） | 无需额外配置 |
| `elasticsearch` | ES存储 | `ELASTICSEARCH_URL` |
| `loki` | Loki存储 | `LOKI_URL` |

**API端点：**
- `GET /api/logs` — 查询日志（支持分页、level、source、search过滤）
- `GET /api/logs/stats` — 日志统计
- `POST /api/logs` — 创建日志
- `POST /api/logs/generate` — 生成示例日志（开发测试用）

**查询参数：**
| 参数 | 说明 | 示例 |
|------|------|------|
| `level` | 日志级别 | `error`, `warn`, `info`, `debug` |
| `source` | 来源 | `api`, `web`, `worker`, `database` |
| `search` | 搜索消息内容 | `error` |
| `limit` | 返回数量（默认50，最大100） | `100` |
| `offset` | 偏移量 | `0` |

**前端组件：**
- `frontend/src/pages/logs/LogViewer.tsx` — 日志查看器页面

**测试环境：**
```bash
# 启动Loki（如未运行）
docker run -d --name devops-loki -p 3100:3100 grafana/loki:2.8.0

# 使用local后端测试（默认）
curl -X POST http://localhost:3000/api/logs/generate -d '{"count": 10}'

# 切换到Loki后端
export LOG_STORAGE_BACKEND=loki
export LOKI_URL=http://localhost:3100
```

### 6. 测试规范

测试分为两类：

#### 4.1 开发测试 (DEV Tests)
- 文件命名：`*_test.go`
- 目的：本地快速开发验证
- **允许使用 httptest mock**
- 适用于：单元测试、handler逻辑测试

#### 4.2 QA测试 (QA Tests)
- 文件命名：`*_integration_test.go`
- 目的：真实环境验证，CI/CD使用
- **禁止使用 mock，必须真实HTTP请求**
- 必须连接真实依赖服务（PostgreSQL、k3d等）
- 使用 `skipIf*()` 辅助函数，依赖不可用时自动跳过

```go
// DEV测试 - 可以使用mock
func TestManager_QueryLogsHTTP(t *testing.T) {
    m := NewManager(cfg, nil)  // 直接实例化，不走网络
    req := httptest.NewRequest("GET", "/api/logs", nil)
    w := httptest.NewRecorder()
    m.QueryLogsHTTP(w, req)  // 直接调用handler
}

// QA测试 - 必须真实HTTP请求
func TestProjectAPI_BusinessLines_CreateAndList(t *testing.T) {
    baseURL := skipIfNoProjectDeps(t)  // 检查依赖
    resp, err := http.Get(baseURL + "/api/org/business-lines")  // 真实HTTP
    // ...
}
```

测试原则：
- K8s测试：连接真实的k3d集群进行功能验证
- Metrics测试：从实际运行的服务器抓取 `/metrics` 端点
- 使用 `skipIfNoK8s()` / `skipIfNoServer()` 等辅助函数，无环境时自动跳过

### 4. 代码设计原则

**通用原则：**
- 所有配置必须可通过YAML文件或环境变量覆盖
- 错误处理：关键操作返回error，便于调用方处理
- 日志记录：重要操作应有日志，便于调试
- 接口设计：HTTP handler使用`mux.Vars(r)`获取路径参数，不使用`r.URL.Query().Get(":param")`

**前端开发原则：**
- 所有API路径使用相对路径
- 用户操作反馈：成功/失败需有Toast提示
- 危险操作（如删除节点）需二次确认
- 列表操作需考虑空状态展示

## Deployment

### Binary Deployment

```bash
# Build
go build -o devops-toolkit ./cmd/devops-toolkit

# Run
./devops-toolkit
```

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o devops-toolkit ./cmd/devops-toolkit

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/devops-toolkit .
COPY config.yaml .
EXPOSE 3000
CMD ["./devops-toolkit"]
```

```bash
docker build -t devops-toolkit .
docker run -p 3000:3000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  devops-toolkit
```

### systemd

```ini
[Unit]
Description=DevOps Toolkit
After=network.target

[Service]
Type=simple
User=devops
Group=devops
WorkingDirectory=/opt/devops-toolkit
ExecStart=/opt/devops-toolkit/devops-toolkit
Restart=always
RestartSec=5

# Environment overrides
Environment=DEVOPS_SERVER_PORT=3000
Environment=DEVOPS_DATABASE_HOST=localhost

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable devops-toolkit
sudo systemctl start devops-toolkit
```

### Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name devops.example.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

For sub-path deployment (`/devops/`), the proxy must route BOTH the frontend app AND the API/WebSocket endpoints:

```nginx
# 前端 SPA (必须在 /api/ 之后)
location /devops/ {
    rewrite ^/devops/(.*) /$1 break;
    proxy_pass http://localhost:3000;
}

# API 路径重写 (关键！将 /devops/api/* 转为 /api/*)
location /devops/api/ {
    rewrite ^/devops/api/(.*) /api/$1 break;
    proxy_pass http://localhost:3000;
}

# WebSocket 路径重写
location /devops/ws {
    rewrite ^/devops/ws /ws break;
    proxy_pass http://localhost:3000;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

**工作原理**:
1. 前端使用 `base: './'` 和相对路径 `'api/clusters'`
2. 浏览器在 `/devops/k8s` 页面时，`fetch('api/clusters')` 解析为 `/devops/api/clusters`
3. nginx 将 `/devops/api/clusters` 重写为 `/api/clusters` 并转发到后端

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEVOPS_SERVER_HOST` | Listen address | `0.0.0.0` |
| `DEVOPS_SERVER_PORT` | Listen port | `3000` |
| `DEVOPS_DATABASE_HOST` | PostgreSQL host | from config |
| `DEVOPS_DATABASE_PORT` | PostgreSQL port | `5432` |
| `DEVOPS_DATABASE_USER` | PostgreSQL user | from config |
| `DEVOPS_DATABASE_PASSWORD` | PostgreSQL password | from config |
| `DEVOPS_DATABASE_NAME` | PostgreSQL database | `devops` |
| `DEVOPS_LDAP_HOST` | LDAP server host | from config |
| `DEVOPS_LDAP_PORT` | LDAP server port | `389` |
