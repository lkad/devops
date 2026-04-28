# Proposal: DevOps Toolkit Full Implementation

## Why

The current DevOps Toolkit implementation has accumulated technical debt and architectural inconsistencies across 13 major modules. A complete rewrite based on the PRD v1.4 will provide a clean, well-structured foundation with proper separation of concerns, comprehensive API design, and modern development practices. This reset enables maintainability, testability, and future extensibility.

## What Changes

### Architecture & Foundation
- **Clean Go backend structure** — `cmd/`, `internal/`, `pkg/` with clear boundaries
- **RESTful API layer** — All 13 modules expose consistent REST endpoints
- **WebSocket server** — Unified real-time communication for all modules
- **Configuration management** — YAML-based config with environment variable overrides
- **Database layer** — PostgreSQL for relational data, with pluggable storage backends

### Module Implementation

#### 1. CI/CD Pipeline
- Pipeline CRUD with YAML-defined stages
- Execution engine with stage sequencing (validate → build → test → security_scan → stage_deploy → smoke_test → prod_deploy → verification)
- Deployment strategies: Blue-Green, Canary, Rolling Update
- Run history and statistics

#### 2. Device Management
- Device state machine: PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE
- Device types: PhysicalHost, Container, NetworkDevice, LoadBalancer, CloudInstance, IoT_Device
- Hierarchical parent-child relationships
- Device groups (flat/hierarchical/dynamic) with tag-based filtering
- Configuration templates with Jinja2 inheritance

#### 3. Logging System
- Pluggable storage backends: Local (default), Elasticsearch, Loki
- Log query delegation based on backend type
- Retention policy management
- Alert rules and saved filters
- Filebeat integration support

#### 4. Monitoring System
- Prometheus metrics endpoint (`/metrics`)
- Metric types: Counter, Gauge, Histogram, Summary
- Pull model (Prometheus client) and Push model (OTLP)
- Auto-discovery via K8s ServiceMonitor

#### 5. Alert Notification System
- Notification channels: Slack, Webhook, Email, Log
- Rate limiting: 10 alerts per 60 seconds per alert name
- Alert history and statistics
- Trigger API for programmatic alerting

#### 6. WebSocket Real-Time Communication
- Channel-based subscription: log, metric, device_event, pipeline_update, alert
- Unified message format with channel, type, data, timestamp

#### 7. LDAP Authentication & Role Mapping
- LDAP server authentication with connection pooling
- Group-to-role mapping (Operator, Developer, Auditor, SuperAdmin)
- Retry logic and graceful error handling

#### 8. Permission Model
- RBAC with base permissions per role
- Label-based access control with inheritance
- Environment restrictions (prod/dev/test)
- Business group hierarchy inheritance

#### 9. Project Management Module
- Organizational hierarchy: Business Line → System → Project
- Resource linking (devices, pipelines, log sources, alert channels, physical hosts)
- Local RBAC permissions (viewer, editor, admin)
- FinOps CSV export for billing reports
- Audit logging for all CRUD operations

#### 10. K8s Multi-Cluster Management
- Kind/k3d cluster lifecycle (create, delete, list)
- Health checks (nodes, deployments, pods)
- Workload management (deploy, scale, delete)
- Metrics collection and log retrieval
- Cross-cluster operations

#### 11. Physical Host Manager
- SSH connection pooling and health checks
- Agent-based metrics (Telegraf → InfluxDB, Prometheus Agent → TSDB)
- Two-layer architecture: Node Status Layer (local tracking) + Data Query Layer (DB queries)
- Local cache with DB故障 fallback
- Service monitoring via systemctl

#### 12. Network Discovery
- Pull-based discovery scanning (SNMP/SSH probing)
- Device auto-registration workflow: Scan → Create PENDING → User Approval → Registered
- Support for TCP:22 (SSH) and UDP:161 (SNMP)

#### 13. Test Environment
- Containerlab-based dual-datacenter topology
- Simulated devices: PhysicalHost (SSH), NetworkDevice (SNMP), Container (Docker)
- Network topology with dual trunk links between DCs
- Connection verification scripts

## Capabilities

### New Capabilities
- `cicd-pipeline`: CI/CD pipeline execution with multi-stage support and deployment strategies
- `device-management`: Device registry, state machine, groups, relationships, and configuration templates
- `log-aggregation`: Unified logging with pluggable backends and query delegation
- `metrics-collection`: Prometheus-format metrics with counter/gauge/histogram support
- `alert-notification`: Multi-channel alerting with rate limiting and history
- `websocket-realtime`: Channel-based real-time event broadcast system
- `ldap-authentication`: LDAP authentication with group-to-role mapping
- `rbac-permissions`: Role-based access control with label inheritance
- `project-hierarchy`: Business Line → System → Project organizational structure with resource linking
- `k8s-cluster-management`: Multi-cluster Kubernetes operations via kind/k3d
- `physical-host-monitoring`: SSH-based host monitoring with two-layer architecture
- `network-discovery`: SNMP/SSH network scanning and auto-registration
- `test-environment`: Containerlab dual-datacenter simulation environment
- `audit-logging`: Comprehensive audit trail for compliance

### Modified Capabilities
- None — this is a fresh implementation

## Impact

### New Directories
- `cmd/devops-toolkit/` — Application entry point
- `internal/` — Core business logic packages
  - `internal/cicd/` — Pipeline execution engine
  - `internal/device/` — Device management
  - `internal/logs/` — Log aggregation and storage
  - `internal/metrics/` — Prometheus metrics
  - `internal/alerts/` — Alert notification system
  - `internal/websocket/` — WebSocket server
  - `internal/auth/` — LDAP authentication
  - `internal/rbac/` — Permission enforcement
  - `internal/project/` — Project management
  - `internal/k8s/` — Kubernetes cluster management
  - `internal/physicalhost/` — Physical host monitoring
  - `internal/discovery/` — Network discovery
- `pkg/` — Shared libraries (database, config, middleware)
- `frontend/` — React SPA

### API Surface
- All REST endpoints as defined in PRD Section 12
- WebSocket endpoint at `/ws`
- Prometheus metrics at `/metrics`
- Health check at `/health`

### Dependencies
- Go 1.21+
- PostgreSQL 15+
- Redis (cache)
- React 18 (frontend)
