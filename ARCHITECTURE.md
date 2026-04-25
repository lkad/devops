# Architecture — DevOps Toolkit

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
├── type (enum: frontend, backend)
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

| Component | Technology |
|-----------|------------|
| HTTP Server | `net/http` + `gorilla/mux` |
| WebSocket | `gorilla/websocket` |
| Database | PostgreSQL (`lib/pq`) |
| SSH | `golang.org/x/crypto/ssh` |
| K8s | k3d/kind CLI + `client-go` |
| Config | YAML + environment overrides |
| Testing | Go `testing` package |

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

## 开发准则

### 1. API路径规范

**前端所有API调用必须使用相对路径，禁止使用绝对路径。**

原因：实际部署时可能通过反向代理（如 Nginx、Traefik）访问服务，代理路径可能是：
- `/` (根路径)
- `/devops/` (子路径)
- `/api/devops-toolkit/` (自定义路径)

如果前端使用绝对路径 `/api/...`，当代理配置为子路径时会导致请求失败。

**正确示例：**
```javascript
// ✅ 使用相对路径
const API_BASE = 'api/org';
fetch('api/k8s/clusters')
fetch('api/org/business-lines')

// ✅ 使用模板字符串
fetch(`api/k8s/clusters/${clusterName}/nodes`)
```

**错误示例：**
```javascript
// ❌ 使用绝对路径
fetch('/api/k8s/clusters')
fetch('/api/org/business-lines')
```

**适用于：**
- `fetch()` 请求
- `window.location.href` 跳转
- WebSocket 连接
- 静态资源路径（除非明确知道代理配置）

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

### 3. 集成测试规范

**测试阶段使用真实环境数据，不使用Mock。**

测试原则：
- K8s测试：连接真实的k3d集群进行功能验证
- 测试覆盖：节点操作、Pod管理、日志获取、Cordon/Uncordon等
- 不使用mock数据，确保测试反映真实使用场景
- 集成测试使用`skipIfNoK8s()`辅助函数，无集群时自动跳过

```go
func skipIfNoK8s(t *testing.T) string {
    // 检查kubeconfig是否存在
    kubeconfig := os.Getenv("HOME") + "/.kube/config-dev-cluster-1"
    if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
        t.Skip("Skipping: no k3d cluster")
    }
    return kubeconfig
}
```

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

For sub-path deployment (`/devops/`):

```nginx
location /devops/ {
    rewrite ^/devops/(.*) /$1 break;
    proxy_pass http://localhost:3000;
}
```

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
