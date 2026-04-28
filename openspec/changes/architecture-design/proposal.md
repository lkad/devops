# Proposal: DevOps Toolkit Architecture Design

## Why

The DevOps Toolkit requires a comprehensive architecture design that serves as the single source of truth for all 13 modules. A well-designed architecture enables parallel development, clear interfaces between components, and a consistent developer experience. This design will define the complete system structure including data models, API contracts, database schema, and configuration management.

## What Changes

### Architecture Artifacts

1. **Directory Structure** — Clean Go project layout with `cmd/`, `internal/`, `pkg/`, `frontend/`
2. **Package Boundaries** — Clear separation between handlers, services, repositories, and domain models
3. **API Layer** — RESTful endpoints with consistent request/response patterns
4. **Database Layer** — PostgreSQL schema with versioned migrations
5. **Configuration System** — YAML-based config with environment variable overrides
6. **Middleware Stack** — Authentication, authorization, logging, error handling
7. **WebSocket Hub** — Channel-based real-time event broadcasting
8. **Module Integration Points** — How each of the 13 modules connects to the core

### Module Architecture

| Module | Package | Primary Entities | Key Interfaces |
|--------|---------|-----------------|----------------|
| CI/CD Pipeline | `internal/cicd` | Pipeline, Stage, Run | Executor, Strategy |
| Device Management | `internal/device` | Device, DeviceGroup, DeviceRelationship | StateMachine, ConfigRenderer |
| Logging | `internal/logs` | LogEntry, LogFilter, RetentionPolicy | StorageBackend |
| Monitoring | `internal/metrics` | Counter, Gauge, Histogram | Collector, Exporter |
| Alerts | `internal/alerts` | AlertChannel, AlertRule, AlertHistory | Notifier |
| WebSocket | `internal/websocket` | Client, Channel, Message | Hub |
| LDAP Auth | `internal/auth` | User, LDAPConfig, RoleMapping | Authenticator |
| RBAC | `internal/rbac` | Permission, Role, UserRole | Enforcer |
| Project Hierarchy | `internal/project` | BusinessLine, System, Project | HierarchyManager |
| K8s Management | `internal/k8s` | Cluster, Workload, Pod | ClusterClient |
| Physical Host | `internal/physicalhost` | Host, Metrics, SSHConnection | Monitor |
| Network Discovery | `internal/discovery` | DiscoveredDevice, ScanResult | Scanner |
| Audit Logging | `internal/audit` | AuditLog, AuditEntry | Logger |

## Capabilities

### New Capabilities
- `architecture-foundation`: Core architecture patterns, directory structure, and package design
- `api-contract`: Standardized REST API request/response formats with error codes
- `database-schema`: PostgreSQL schema design with migrations and indexes
- `config-management`: Configuration system with YAML and environment variable support
- `middleware-stack`: HTTP middleware for auth, logging, recovery, and CORS
- `websocket-hub`: WebSocket server with channel-based pub/sub

### Modified Capabilities
- None — this is a foundational change

## Impact

### New Files Structure

```
cmd/devops-toolkit/
  └── main.go                    # Application entry point

pkg/
  ├── config/                    # Configuration loading
  ├── database/                  # PostgreSQL connection pool
  ├── middleware/                # HTTP middleware stack
  ├── error/                     # Standardized error types
  └── response/                  # Unified response builders

internal/
  ├── cicd/                      # CI/CD pipeline module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   ├── executor.go
  │   ├── strategy.go            # Blue-Green, Canary, Rolling
  │   └── models.go
  ├── device/                    # Device management module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   ├── state_machine.go
  │   ├── config_renderer.go
  │   └── models.go
  ├── logs/                      # Logging module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   ├── backend.go             # Interface
  │   ├── local_backend.go
  │   ├── elasticsearch_backend.go
  │   ├── loki_backend.go
  │   └── models.go
  ├── metrics/                   # Prometheus metrics module
  │   ├── handler.go
  │   ├── service.go
  │   ├── collector.go
  │   └── models.go
  ├── alerts/                    # Alert notification module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   ├── notifier.go
  │   ├── channels/
  │   │   ├── slack.go
  │   │   ├── webhook.go
  │   │   ├── email.go
  │   │   └── log.go
  │   └── models.go
  ├── websocket/                  # WebSocket module
  │   ├── hub.go
  │   ├── client.go
  │   ├── channel.go
  │   └── handler.go
  ├── auth/                      # LDAP authentication module
  │   ├── handler.go
  │   ├── service.go
  │   ├── ldap_client.go
  │   └── models.go
  ├── rbac/                      # RBAC permissions module
  │   ├── middleware.go
  │   ├── service.go
  │   ├── enforcer.go
  │   └── models.go
  ├── project/                   # Project hierarchy module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   └── models.go
  ├── k8s/                       # Kubernetes management module
  │   ├── handler.go
  │   ├── service.go
  │   ├── cluster_client.go
  │   └── models.go
  ├── physicalhost/              # Physical host monitoring module
  │   ├── handler.go
  │   ├── service.go
  │   ├── repository.go
  │   ├── ssh_pool.go
  │   ├── monitor.go
  │   └── models.go
  ├── discovery/                 # Network discovery module
  │   ├── handler.go
  │   ├── service.go
  │   ├── scanner.go
  │   └── models.go
  └── audit/                     # Audit logging module
      ├── handler.go
      ├── service.go
      ├── repository.go
      └── models.go

migrations/
  ├── 000001_initial_schema.up.sql
  └── 000001_initial_schema.down.sql

frontend/
  └── src/
      ├── components/
      ├── pages/
      ├── hooks/
      └── services/
```

### API Surface

All endpoints follow consistent patterns:

| Pattern | Description |
|---------|-------------|
| `GET /api/{resource}` | List with pagination, filtering |
| `GET /api/{resource}/:id` | Get single resource |
| `POST /api/{resource}` | Create resource |
| `PUT /api/{resource}/:id` | Update resource |
| `DELETE /api/{resource}/:id` | Delete resource |
| `POST /api/{resource}/:id/{action}` | Perform action |

### Dependencies

- Go 1.21+
- PostgreSQL 15+
- Redis 7+
- React 18
- golang-migrate for database migrations
- gorilla/websocket for WebSocket
- go-ldap/ldap for LDAP
- k8s.io/client-go for Kubernetes
