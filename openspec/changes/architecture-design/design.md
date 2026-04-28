# Design: DevOps Toolkit Architecture

## Context

**Background**: DevOps Toolkit is a Go-based internal platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts. The archived Node.js implementation had architectural issues: mixed concerns, inconsistent error handling, and no clear boundaries.

**Current State**: Empty workspace (`archive/archived-20260427/` contains archived code). PRD v1.4 defines 13 modules with requirements. Need to design a clean architecture before implementation.

**Constraints**:
- Go 1.21+ for backend (performance, single-binary deployment)
- PostgreSQL 15+ for relational data
- Redis 7+ for session/cache and WebSocket scaling
- React 18 SPA frontend
- Must integrate with existing infrastructure (LDAP, Kubernetes)
- 13 modules to implement with clean separation

**Stakeholders**: DevOps teams managing CI/CD pipelines, physical hosts, and multi-cluster Kubernetes environments.

## Goals / Non-Goals

**Goals:**
- Clean layered architecture: Handler → Service → Repository
- Clear package boundaries enforced by Go module system
- Standardized API contract with consistent request/response format
- PostgreSQL schema with versioned migrations
- Configuration system with YAML + environment variable overrides
- HTTP middleware stack for cross-cutting concerns
- WebSocket hub with channel-based pub/sub
- Comprehensive error handling with proper HTTP status codes
- ASCII diagrams in code comments for complex state machines and data flows

**Non-Goals:**
- GraphQL API (REST is sufficient)
- Multi-tenancy (single organization model)
- Microservices (monolithic Go binary)
- Legacy protocol support beyond SNMPv2c

## Directory Structure

```
devops-toolkit/
├── cmd/
│   └── devops-toolkit/
│       └── main.go              # Entry point, server bootstrap
│
├── pkg/                         # Shared libraries (no internal dependencies)
│   ├── config/
│   │   └── config.go            # YAML config loader with env overrides
│   ├── database/
│   │   ├── postgres.go          # Connection pool management
│   │   └── redis.go             # Redis client
│   ├── middleware/
│   │   ├── auth.go              # JWT validation
│   │   ├── logging.go           # Request logging
│   │   ├── recovery.go          # Panic recovery
│   │   ├── cors.go              # CORS headers
│   │   └── metrics.go           # HTTP metrics middleware
│   ├── error/
│   │   ├── errors.go            # Standard error types
│   │   └── response.go          # Error response formatting
│   └── response/
│       └── response.go          # Unified response builders
│
├── internal/                    # Core modules (no cross-module imports)
│   ├── cicd/
│   │   ├── handler.go           # HTTP handlers
│   │   ├── service.go           # Business logic
│   │   ├── repository.go        # Data access
│   │   ├── executor.go          # Stage execution engine
│   │   ├── strategy.go          # Deployment strategies
│   │   └── models.go            # Domain models
│   │
│   ├── device/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── state_machine.go     # Device state transitions
│   │   ├── config_renderer.go   # Jinja2 template rendering
│   │   └── models.go
│   │
│   ├── logs/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── backend.go           # StorageBackend interface
│   │   ├── local_backend.go
│   │   ├── elasticsearch_backend.go
│   │   └── loki_backend.go
│   │
│   ├── metrics/
│   │   ├── handler.go           # /metrics Prometheus endpoint
│   │   ├── service.go
│   │   ├── collector.go         # Prometheus collector
│   │   └── models.go
│   │
│   ├── alerts/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── notifier.go
│   │   ├── rate_limiter.go
│   │   ├── channels/
│   │   │   ├── channel.go      # Channel interface
│   │   │   ├── slack.go
│   │   │   ├── webhook.go
│   │   │   ├── email.go
│   │   │   └── log.go
│   │   └── models.go
│   │
│   ├── websocket/
│   │   ├── hub.go              # Central hub managing all connections
│   │   ├── client.go           # WebSocket client representation
│   │   ├── channel.go          # Channel subscription management
│   │   └── handler.go          # HTTP upgrade handler
│   │
│   ├── auth/
│   │   ├── handler.go          # /api/auth/login, logout
│   │   ├── service.go
│   │   ├── ldap_client.go      # LDAP connection pool
│   │   ├── jwt.go              # JWT token handling
│   │   └── models.go
│   │
│   ├── rbac/
│   │   ├── middleware.go       # Permission enforcement middleware
│   │   ├── service.go
│   │   ├── enforcer.go         # Policy evaluation
│   │   ├── label_cache.go      # Label inheritance cache
│   │   └── models.go
│   │
│   ├── project/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── models.go
│   │
│   ├── k8s/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── cluster_client.go    # k3d client wrapper
│   │   └── models.go
│   │
│   ├── physicalhost/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── ssh_pool.go         # SSH connection pooling
│   │   ├── monitor.go         # Metrics collection
│   │   ├── cache.go            # Local cache with TTL
│   │   └── models.go
│   │
│   ├── discovery/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── scanner.go          # SNMP/SSH probing
│   │   └── models.go
│   │
│   └── audit/
│       ├── handler.go
│       ├── service.go
│       ├── repository.go
│       └── models.go
│
├── migrations/                  # Database migrations
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   ├── 000002_create_devices.up.sql
│   └── ...
│
├── frontend/                   # React SPA
│   ├── public/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── services/           # API clients
│   │   └── App.tsx
│   └── package.json
│
├── config.yaml                 # Application configuration
├── go.mod
├── go.sum
└── Dockerfile
```

## Layer Architecture

Each module follows a consistent three-layer pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                      HANDLER LAYER                          │
│  (internal/*/handler.go)                                   │
│  - HTTP request/response handling                          │
│  - Input validation (structural)                            │
│  - Response formatting                                      │
│  - Calls Service layer                                      │
├─────────────────────────────────────────────────────────────┤
│                      SERVICE LAYER                          │
│  (internal/*/service.go)                                    │
│  - Business logic                                          │
│  - Transaction management                                   │
│  - Calls Repository layer                                   │
├─────────────────────────────────────────────────────────────┤
│                    REPOSITORY LAYER                         │
│  (internal/*/repository.go)                                 │
│  - Database operations                                      │
│  - SQL query execution                                      │
│  - Data mapping (rows → structs)                            │
└─────────────────────────────────────────────────────────────┘
```

### Handler → Service → Repository Flow

```
HTTP Request
    │
    ▼
┌─────────────┐
│   Handler   │  ← Validates input structure
│             │  ← Formats response
│             │  ← Calls Service method
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Service   │  ← Business logic
│             │  ← Transaction boundaries
│             │  ← Calls Repository methods
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Repository  │  ← SQL execution
│             │  ← Row scanning
└─────────────┘
       │
       ▼
   PostgreSQL
```

## Package Boundaries (Enforced by Go Modules)

```
pkg/         → Can be imported by any package. Contains shared utilities.
              No internal/ package dependencies.

internal/*/  → Can only be imported by cmd/ and other internal/* packages.
              Cannot be imported by pkg/ (Go enforces this).

cmd/         → Application entry point. Imports internal/* packages.
```

**Cross-module communication diagram:**

```
cmd/main.go
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│                    internal/* packages                  │
│                                                         │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐│
│  │  cicd   │   │ device  │   │  logs   │   │ alerts  ││
│  └────┬────┘   └────┬────┘   └────┬────┘   └────┬────┘│
│       │             │             │             │      │
│       └─────────────┴─────────────┴─────────────┘      │
│                         │                              │
│                    ┌────┴────┐                        │
│                    │ pkg/*   │  ← Shared utilities   │
│                    │(config, │    (no internal deps)  │
│                    │ db, err)│                        │
│                    └─────────┘                        │
└─────────────────────────────────────────────────────────┘
```

**Forbidden patterns:**
- `internal/device` cannot import `internal/cicd`
- `internal/cicd` cannot import `internal/logs`
- Modules communicate only through the API layer (HTTP)

## Database Schema Design

### Core Tables

```sql
-- Version: 000001
-- Description: Initial schema with users, devices, pipelines

-- Users (local RBAC, LDAP for auth only)
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(255) UNIQUE NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    role        VARCHAR(50) NOT NULL CHECK (role IN ('super_admin', 'operator', 'developer', 'auditor')),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Devices
CREATE TABLE devices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL CHECK (type IN ('physical_host', 'container', 'network_device', 'load_balancer', 'cloud_instance', 'iot_device')),
    state           VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'authenticated', 'registered', 'active', 'maintenance', 'suspended', 'retire')),
    parent_id       UUID REFERENCES devices(id),
    metadata        JSONB DEFAULT '{}',
    labels          JSONB DEFAULT '{}',           -- {"env": "prod", "role": "web"}
    config_template TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_devices_state ON devices(state);
CREATE INDEX idx_devices_type ON devices(type);
CREATE INDEX idx_devices_labels ON devices USING GIN(labels);

-- Device state transitions log
CREATE TABLE device_state_transitions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id     UUID NOT NULL REFERENCES devices(id),
    from_state    VARCHAR(50) NOT NULL,
    to_state      VARCHAR(50) NOT NULL,
    triggered_by  VARCHAR(255),
    reason        TEXT,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Pipelines
CREATE TABLE pipelines (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    yaml_config TEXT NOT NULL,                      -- Full YAML definition
    status      VARCHAR(50) DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'success', 'failed', 'cancelled')),
    created_by  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Pipeline runs
CREATE TABLE pipeline_runs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id  UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    status      VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled')),
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    trigger      VARCHAR(50) DEFAULT 'manual',      -- manual, webhook, scheduled
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Pipeline run stages
CREATE TABLE pipeline_run_stages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id      UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    stage_name  VARCHAR(100) NOT NULL,
    status      VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed', 'skipped')),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    logs        TEXT,
    error       TEXT
);

-- Alert channels
CREATE TABLE alert_channels (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) UNIQUE NOT NULL,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('slack', 'webhook', 'email', 'log')),
    config      JSONB NOT NULL,                      -- {"webhookUrl": "...", "channel": "..."}
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Alert history
CREATE TABLE alert_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    severity    VARCHAR(20) NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    message     TEXT NOT NULL,
    channel     VARCHAR(255) NOT NULL,
    status      VARCHAR(20) DEFAULT 'sent' CHECK (status IN ('sent', 'rate_limited', 'failed')),
    triggered_by VARCHAR(255),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Business lines (project hierarchy)
CREATE TABLE business_lines (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE systems (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_line_id UUID NOT NULL REFERENCES business_lines(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id   UUID NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('frontend', 'backend')),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Project resources (linking table)
CREATE TABLE project_resources (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    resource_type VARCHAR(50) NOT NULL,              -- device, pipeline, log_source, alert_channel, physical_host
    resource_id UUID NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, resource_type, resource_id)
);

-- Project permissions (local RBAC)
CREATE TABLE project_permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    username    VARCHAR(255) NOT NULL,
    role        VARCHAR(50) NOT NULL CHECK (role IN ('viewer', 'editor', 'admin')),
    granted_by  VARCHAR(255),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, username)
);

-- Audit logs
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(255) NOT NULL,
    action      VARCHAR(50) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID NOT NULL,
    entity_name VARCHAR(255),
    changes     TEXT,
    old_value   TEXT,
    new_value   TEXT,
    ip_address  INET,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_username ON audit_logs(username);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at);

-- Physical hosts
CREATE TABLE physical_hosts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname        VARCHAR(255) NOT NULL,
    ip              INET NOT NULL,
    port            INTEGER DEFAULT 22,
    state           VARCHAR(50) DEFAULT 'online' CHECK (state IN ('online', 'offline')),
    last_heartbeat  TIMESTAMPTZ,
    last_agent_update TIMESTAMPTZ,
    data_status     VARCHAR(50) DEFAULT 'fresh' CHECK (data_status IN ('fresh', 'stale', 'unavailable')),
    ssh_user        VARCHAR(255),
    ssh_password    VARCHAR(255),                    -- Encrypted
    registered_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Discovered devices (pending registration)
CREATE TABLE discovered_devices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip          INET NOT NULL,
    port        INTEGER,
    device_type VARCHAR(50) NOT NULL,               -- physical_host, network_device
    protocol    VARCHAR(50) NOT NULL,               -- ssh, snmp
    metadata    JSONB DEFAULT '{}',
    scan_id     UUID,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- K8s clusters
CREATE TABLE k8s_clusters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) UNIQUE NOT NULL,
    kubeconfig  TEXT NOT NULL,
    status      VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'error')),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);
```

## Configuration Management

### config.yaml Structure

```yaml
app:
  name: "devops-toolkit"
  host: "0.0.0.0"
  port: 8080
  env: "development"  # development, production

database:
  host: "localhost"
  port: 5432
  username: "devops"
  password: "${DB_PASSWORD}"  # Environment variable override
  name: "devops"
  max_connections: 25
  ssl_mode: "disable"

redis:
  host: "localhost"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0

logs:
  storage_backend: "local"  # local, elasticsearch, loki
  retention_days: 30
  elasticsearch:
    url: "http://localhost:9200"
    index: "devops-logs"
  loki:
    url: "http://localhost:3100"

ldap:
  host: "ldap.example.com"
  port: 389
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_password: "${LDAP_BIND_PASSWORD}"
  user_filter: "(uid=%s)"
  group_mapping:
    "cn=IT_Ops,ou=Groups,dc=example,dc=com": "operator"
    "cn=DevTeam_Payments,ou=Groups,dc=example,dc=com": "developer"
    "cn=Security_Auditors,ou=Groups,dc=example,dc=com": "auditor"
    "cn=SRE_Lead,ou=Groups,dc=example,dc=com": "super_admin"

alerts:
  rate_limit:
    window_seconds: 60
    max_alerts: 10
  slack:
    enabled: true
  webhook:
    enabled: true
  email:
    enabled: false
    smtp_host: "smtp.example.com"
    smtp_port: 587

k8s:
  default_kubeconfig_path: "~/.kube/config"

physicalhost:
  ssh_pool_size: 10
  health_check_interval: 30
  data_freshness_threshold: 30

websocket:
  read_buffer_size: 1024
  write_buffer_size: 1024
  ping_interval: 30
  channels:
    - log
    - metric
    - device_event
    - pipeline_update
    - alert
```

### Environment Variable Override Rules

1. Any config value can be overridden by environment variable
2. Format: `${ENV_VAR_NAME}` in YAML
3. Nested values: `database.host` → `DATABASE_HOST`
4. Arrays: `alerts.channels[0]` → `ALERTS_CHANNELS_0`

## Middleware Stack

```
Request Flow:
──────────────────────────────────────────────────────────────

  HTTP Request
      │
      ▼
┌──────────────────┐
│     CORS         │  ← Set CORS headers
│   middleware     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│    Recovery      │  ← Panic recovery, returns 500
│   middleware     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│    Logging       │  ← Log request method, path, duration
│   middleware     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│    Metrics       │  ← Record HTTP request metrics
│   middleware     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│     Auth         │  ← Validate JWT, extract user
│   middleware     │    (skip for /health, /metrics, /api/auth/*)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│    RBAC          │  ← Check permissions based on route
│   middleware     │
└────────┬─────────┘
         │
         ▼
   Handler
```

## WebSocket Hub Design

```
┌─────────────────────────────────────────────────────────────┐
│                      WebSocket Hub                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐   │
│   │ Client1 │   │ Client2 │   │ Client3 │   │ ClientN │   │
│   │  (WS)   │   │  (WS)   │   │  (WS)   │   │  (WS)   │   │
│   └────┬────┘   └────┬────┘   └────┬────┘   └────┬────┘   │
│        │             │             │             │         │
│        └─────────────┴──────┬──────┴─────────────┘         │
│                             │                              │
│                      ┌──────┴──────┐                       │
│                      │ Subscription │                      │
│                      │   Manager    │                       │
│                      └──────┬──────┘                       │
│                             │                              │
│   Channels: log | metric | device_event | pipeline_update | alert │
│                             │                              │
│   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐  │
│   │log      │   │metric   │   │device_  │   │pipeline │  │
│   │channel  │   │channel  │   │event    │   │_update  │  │
│   │         │   │         │   │channel  │   │channel  │  │
│   └─────────┘   └─────────┘   └─────────┘   └─────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
                    Broadcast to all
                    subscribed clients
```

### Hub State

```go
type Hub struct {
    // Registered clients by client ID
    clients map[string]*Client

    // Channels with subscribed client IDs
    channels map[string]map[string]*Client  // channel name → client ID → client

    // Mutex for thread-safe access
    mu sync.RWMutex

    // Register requests from clients
    register chan *Client

    // Unregister requests from clients
    unregister chan *Client

    // Broadcast channel for messages
    broadcast chan *Message
}
```

## Error Handling Strategy

### Error Types

```go
// pkg/error/errors.go

type AppError struct {
    Code    string      `json:"code"`     // Machine-readable code
    Message string      `json:"message"`  // Human-readable message
    Details interface{} `json:"details"` // Additional context
    Status  int         `json:"-"`        // HTTP status code
}

func (e *AppError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error codes
const (
    ErrCodeValidation    = "VALIDATION_ERROR"
    ErrCodeNotFound      = "NOT_FOUND"
    ErrCodeUnauthorized  = "UNAUTHORIZED"
    ErrCodeForbidden     = "FORBIDDEN"
    ErrCodeConflict      = "CONFLICT"
    ErrCodeInternal      = "INTERNAL_ERROR"
    ErrCodeRateLimited   = "RATE_LIMITED"
    ErrCodeInvalidState  = "INVALID_STATE"
)
```

### Error Response Format

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Invalid request parameters",
        "details": {
            "field": "email",
            "reason": "invalid email format"
        }
    }
}
```

### HTTP Status Code Mapping

| Error Code | HTTP Status | When Used |
|------------|-------------|-----------|
| VALIDATION_ERROR | 400 | Invalid input |
| UNAUTHORIZED | 401 | Missing/invalid token |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource doesn't exist |
| CONFLICT | 409 | State conflict (e.g., duplicate) |
| INVALID_STATE | 422 | Invalid state transition |
| RATE_LIMITED | 429 | Rate limit exceeded |
| INTERNAL_ERROR | 500 | Unexpected server error |

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope too large (13 modules) | Months to complete | Phase 1: Core (Device, Logs, Alerts, Auth); Phase 2: Advanced (K8s, PhysicalHost, Discovery) |
| PostgreSQL schema migrations | Breaking changes during dev | Use golang-migrate with versioned SQL, always provide DOWN migration |
| Cross-module dependencies | Tight coupling, hard to test | Strict package boundary enforcement, modules communicate via HTTP only |
| LDAP integration complexity | Connection issues, timeouts | Separate auth package with connection pooling, retry logic |
| WebSocket scalability | Memory pressure with many connections | Redis Pub/Sub for horizontal scaling (defer to Phase 2) |
| K8s API compatibility | Version mismatches | Pin k3d version, test against multiple k8s versions |
| Configuration complexity | Environment-specific configs | Clear override rules, document env var mapping |

## Open Questions

1. **API Versioning**: Start with `/api/v1` or just `/api`? PRD shows unversioned paths. Recommendation: `/api/v1` for future-proofing.

2. **Prometheus Metrics Storage**: PRD mentions Prometheus TSDB for time-series data. Should physical host metrics use Prometheus remote_write or direct InfluxDB? Recommendation: Support both, let users choose.

3. **Frontend Build**: React SPA in `frontend/` within same repo or separate repository? Recommendation: Same repo for now, easier to develop.

4. **Containerlab Test Environment**: Part of main repo or separate `devops-toolkit-test-env`? Recommendation: Same repo for easier testing.

5. **Filebeat Integration**: Bundle Filebeat config or document only? Recommendation: Document integration, provide example configs.
