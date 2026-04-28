# Design: DevOps Toolkit Full Implementation

## Context

**Background**: The archived implementation at `archive/archived-20260427/` was a Node.js-based monolithic application with 13 loosely integrated modules. The rewrite targets a clean Go backend with React frontend, proper layered architecture, and comprehensive API coverage.

**Current State**: All 13 modules are documented in PRD v1.4. The TODOS shows everything as "complete" but the code structure has architectural issues: mixed concerns, inconsistent error handling, and no clear boundaries between layers.

**Constraints**:
- Go 1.21+ for backend (performance, single-binary deployment)
- PostgreSQL 15+ for persistent storage
- Redis for session/cache
- React 18 for frontend SPA
- Must integrate with existing infrastructure (LDAP, Kubernetes)

**Stakeholders**: DevOps teams managing CI/CD pipelines, physical hosts, and multi-cluster Kubernetes environments.

## Goals / Non-Goals

**Goals:**
- Clean layered architecture: Handler → Service → Repository
- RESTful API with consistent request/response patterns
- Pluggable storage backends (Local, Elasticsearch, Loki)
- Comprehensive error handling with proper HTTP status codes
- WebSocket for real-time events across all modules
- Full test coverage with unit and integration tests
- PostgreSQL for all persistent data
- Audit logging for compliance-critical operations

**Non-Goals:**
- GraphQL API (REST is sufficient for all use cases)
- Multi-tenancy (single organization model)
- Cloud-native specific features beyond K8s integration
- Legacy protocol support (SNMPv1/v2c only)

## Decisions

### 1. Go Backend with Layered Architecture

**Decision**: Use `cmd/` → `internal/` → `pkg/` structure with Handler/Service/Repository layers.

**Rationale**:
- Go's module system enforces import boundaries
- Handler layer handles HTTP/WebSocket concerns
- Service layer contains business logic
- Repository layer abstracts data access
- `pkg/` for truly shared code (config, middleware, database)

**Alternatives Considered**:
- Monolithic `internal/` with package-level separation — rejected due to unclear boundaries
- Domain-driven design with aggregates — overkill for this scale

### 2. PostgreSQL as Primary Database

**Decision**: Use PostgreSQL for all relational data, with module-specific storage for time-series (InfluxDB/Prometheus).

**Rationale**:
- Rich SQL features (CTEs, window functions) for complex queries
- ACID compliance for transactional integrity
- JSONB for semi-structured data (device metadata, pipeline config)
- Proven reliability and tooling

**Alternatives Considered**:
- SQLite — rejected (concurrency limitations)
- MongoDB — rejected (want strong typing and SQL for reporting)

### 3. Pluggable Storage Backends for Logs

**Decision**: LogManager delegates to LocalStorageBackend, ElasticsearchBackend, or LokiBackend based on configuration.

**Rationale**:
- LocalStorageBackend for dev/testing (zero infrastructure)
- Elasticsearch for production full-text search
- Loki for Grafana ecosystem integration
- Clean interface allows adding backends without changing callers

### 4. Device State Machine with Explicit Transitions

**Decision**: Strict state machine with validated transitions. State stored in PostgreSQL with transition logging.

**Rationale**:
- Prevents invalid state transitions (e.g., RETIRE → ACTIVE)
- Audit trail of all state changes
- Clear error messages for invalid transitions

### 5. LDAP Authentication with Local RBAC

**Decision**: LDAP for authentication only. Permissions are managed locally in PostgreSQL, not synced to LDAP.

**Rationale**:
- LDAP groups map to roles (Operator, Developer, Auditor, SuperAdmin)
- Local permission model allows fine-grained control without LDAP changes
- Simpler security model: auth ≠ authorization

### 6. Two-Layer Physical Host Architecture

**Decision**: Separate Node Status Layer (local, for online/offline) from Data Query Layer (InfluxDB/Prometheus, for metrics).

**Rationale**:
- Node status must be available even when DB is down
- Metrics data is inherently eventually-consistent
- Clear separation of concerns prevents confusion

### 7. Project Management with Organizational Hierarchy

**Decision**: Business Line → System → Project hierarchy stored in PostgreSQL with inheritance.

**Rationale**:
- Natural organizational structure
- Permissions cascade down (editor on BL can edit all children)
- Resource linking allows FinOps reporting across DevOps resources

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope too large | 13 modules may take months | Phase 1: Core (Device, Logs, Alerts, Auth); Phase 2: Advanced (K8s, PhysicalHost, Discovery) |
| PostgreSQL schema migrations | Breaking changes during dev | Use golang-migrate with versioned SQL files |
| LDAP integration complexity | Connection pooling, retries, timeouts | Separate auth package with thorough testing |
| WebSocket scalability | Many concurrent connections | Use Redis Pub/Sub for horizontal scaling |
| K8s API compatibility | Multiple k3d versions | Pin k3d version, test against multiple versions |

## Open Questions

1. **Containerlab for test environment** — Should test environment be part of the main repo or a separate `devops-toolkit-test-env` repository?
2. **Prometheus remote_write** — Should we implement a lightweight agent for Prometheus data collection, or rely entirely on existing exporters?
3. **Filebeat integration** — PRD mentions Filebeat for log collection. Should we bundle a Filebeat config or just document integration?
4. **Frontend build** — React SPA in `frontend/` or move to separate repo for cleaner separation?
5. **API versioning** — Start with `/api/v1` or just `/api`? PRD shows unversioned paths.

## Technical Stack Summary

```
┌─────────────────────────────────────────────────────────────┐
│                         Frontend                             │
│                    React 18 SPA (frontend/)                 │
├─────────────────────────────────────────────────────────────┤
│                         API Layer                            │
│              HTTP Handlers (internal/*/handler.go)           │
├─────────────────────────────────────────────────────────────┤
│                      Service Layer                           │
│               Business Logic (internal/*/service.go)          │
├─────────────────────────────────────────────────────────────┤
│                    Repository Layer                          │
│              Data Access (internal/*/repository.go)          │
├─────────────────────────────────────────────────────────────┤
│                         Database                             │
│        PostgreSQL (main) │ InfluxDB/Prometheus (metrics)      │
└─────────────────────────────────────────────────────────────┘
```
