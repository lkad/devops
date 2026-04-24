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
| `internal/k8s` | k3d/kind cluster management via CLI | kubeconfig |
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
