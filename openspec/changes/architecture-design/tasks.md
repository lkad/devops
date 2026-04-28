# Tasks: DevOps Toolkit Architecture Implementation

## 1. Project Setup

- [ ] 1.1 Initialize Go module: `go mod init github.com/devops-toolkit`
- [ ] 1.2 Create directory structure (cmd/, pkg/, internal/)
- [ ] 1.3 Add dependencies to go.mod (gorilla/mux, gorilla/websocket, go-ldap, pgx, redis, testify, etc.)
- [ ] 1.4 Create config.yaml with all sections documented
- [ ] 1.5 Create go.mod and go.sum

## 2. Package: Config

- [ ] 2.1 Create pkg/config/config.go with YAML loader
- [ ] 2.2 Implement environment variable override (${VAR} syntax)
- [ ] 2.3 Add config validation on load
- [ ] 2.4 Implement sensitive value masking for logging
- [ ] 2.5 Create config.yaml.example with all options documented
- [ ] 2.6 Write unit tests for config loading

## 3. Package: Error Handling

- [ ] 3.1 Create pkg/error/errors.go with AppError struct
- [ ] 3.2 Define standard error codes (VALIDATION_ERROR, NOT_FOUND, etc.)
- [ ] 3.3 Create pkg/error/response.go with error response formatting
- [ ] 3.4 Implement HTTP status code mapping
- [ ] 3.5 Write unit tests for error types

## 4. Package: Response

- [ ] 4.1 Create pkg/response/response.go with Response struct
- [ ] 4.2 Implement Success() helper (200/201/204)
- [ ] 4.3 Implement List() helper with pagination
- [ ] 4.4 Implement Error() helper
- [ ] 4.5 Write unit tests for response helpers

## 5. Package: Database

- [ ] 5.1 Create pkg/database/postgres.go with connection pool
- [ ] 5.2 Implement QueryRow, Query, Exec helpers
- [ ] 5.3 Add context propagation to queries
- [ ] 5.4 Create database migration framework setup
- [ ] 5.5 Create Redis client in pkg/database/redis.go
- [ ] 5.6 Write integration tests for database package

## 6. Middleware Stack

- [ ] 6.1 Create pkg/middleware/recovery.go (panic recovery)
- [ ] 6.2 Create pkg/middleware/logging.go (request logging)
- [ ] 6.3 Create pkg/middleware/cors.go (CORS headers)
- [ ] 6.4 Create pkg/middleware/metrics.go (HTTP metrics)
- [ ] 6.5 Create pkg/middleware/chain.go (middleware chaining)
- [ ] 6.6 Write unit tests for each middleware

## 7. Authentication

- [ ] 7.1 Create internal/auth/models.go (User, JWT claims)
- [ ] 7.2 Create internal/auth/jwt.go (token generation/validation)
- [ ] 7.3 Create internal/auth/ldap_client.go (LDAP connection pool)
- [ ] 7.4 Create internal/auth/service.go (authentication logic)
- [ ] 7.5 Create internal/auth/handler.go (/api/auth/login, /api/auth/logout, /api/auth/me)
- [ ] 7.6 Implement group-to-role mapping
- [ ] 7.7 Write unit tests for auth package

## 8. RBAC

- [ ] 8.1 Create internal/rbac/models.go (Role, Permission)
- [ ] 8.2 Create internal/rbac/enforcer.go (permission check logic)
- [ ] 8.3 Create internal/rbac/service.go (role/permission service)
- [ ] 8.4 Create internal/rbac/label_cache.go (label inheritance cache)
- [ ] 8.5 Create internal/rbac/middleware.go (permission enforcement)
- [ ] 8.6 Implement label-based access control
- [ ] 8.7 Write unit tests for RBAC package

## 9. WebSocket Hub

- [ ] 9.1 Create internal/websocket/hub.go (central hub)
- [ ] 9.2 Create internal/websocket/client.go (client representation)
- [ ] 9.3 Create internal/websocket/channel.go (subscription management)
- [ ] 9.4 Create internal/websocket/handler.go (HTTP upgrade handler)
- [ ] 9.5 Implement message broadcast to channels
- [ ] 9.6 Implement ping/pong heartbeat
- [ ] 9.7 Implement client disconnect cleanup
- [ ] 9.8 Write unit tests for WebSocket hub

## 10. Device Management

- [ ] 10.1 Create internal/device/models.go (Device, DeviceStateTransition)
- [ ] 10.2 Create internal/device/repository.go (CRUD operations)
- [ ] 10.3 Create internal/device/state_machine.go (state transitions)
- [ ] 10.4 Create internal/device/service.go (business logic)
- [ ] 10.5 Create internal/device/handler.go (HTTP handlers)
- [ ] 10.6 Implement device hierarchy (parent-child)
- [ ] 10.7 Implement device groups (flat, hierarchical, dynamic)
- [ ] 10.8 Implement configuration template rendering
- [ ] 10.9 Write unit tests for device state machine

## 11. Logging System

- [ ] 11.1 Create internal/logs/models.go (LogEntry, RetentionPolicy)
- [ ] 11.2 Create internal/logs/backend.go (StorageBackend interface)
- [ ] 11.3 Create internal/logs/local_backend.go (LocalStorageBackend)
- [ ] 11.4 Create internal/logs/elasticsearch_backend.go
- [ ] 11.5 Create internal/logs/loki_backend.go
- [ ] 11.6 Create internal/logs/repository.go (LogEntry CRUD)
- [ ] 11.7 Create internal/logs/service.go (business logic)
- [ ] 11.8 Create internal/logs/handler.go (HTTP handlers)
- [ ] 11.9 Implement log query delegation by backend type
- [ ] 11.10 Write unit tests for log backends

## 12. Metrics Collection

- [ ] 12.1 Create internal/metrics/models.go (Counter, Gauge, Histogram)
- [ ] 12.2 Create internal/metrics/collector.go (Prometheus collector)
- [ ] 12.3 Create internal/metrics/service.go (metric recording)
- [ ] 12.4 Create internal/metrics/handler.go (/metrics endpoint)
- [ ] 12.5 Implement HTTP request metrics middleware integration
- [ ] 12.6 Write unit tests for metrics

## 13. Alert Notification

- [ ] 13.1 Create internal/alerts/models.go (AlertChannel, AlertHistory)
- [ ] 13.2 Create internal/alerts/channels/channel.go (Channel interface)
- [ ] 13.3 Create internal/alerts/channels/slack.go
- [ ] 13.4 Create internal/alerts/channels/webhook.go
- [ ] 13.5 Create internal/alerts/channels/email.go
- [ ] 13.6 Create internal/alerts/channels/log.go
- [ ] 13.7 Create internal/alerts/rate_limiter.go (10 per 60s)
- [ ] 13.8 Create internal/alerts/repository.go
- [ ] 13.9 Create internal/alerts/service.go
- [ ] 13.10 Create internal/alerts/handler.go
- [ ] 13.11 Write unit tests for rate limiter

## 14. CI/CD Pipeline

- [ ] 14.1 Create internal/cicd/models.go (Pipeline, Run, Stage)
- [ ] 14.2 Create internal/cicd/repository.go
- [ ] 14.3 Create internal/cicd/service.go
- [ ] 14.4 Create internal/cicd/executor.go (stage execution)
- [ ] 14.5 Create internal/cicd/strategy.go (Blue-Green, Canary, Rolling)
- [ ] 14.6 Create internal/cicd/handler.go
- [ ] 14.7 Implement YAML config parsing
- [ ] 14.8 Write unit tests for pipeline executor

## 15. Project Hierarchy

- [ ] 15.1 Create internal/project/models.go (BusinessLine, System, Project)
- [ ] 15.2 Create internal/project/repository.go
- [ ] 15.3 Create internal/project/service.go
- [ ] 15.4 Create internal/project/handler.go
- [ ] 15.5 Implement FinOps CSV export
- [ ] 15.6 Write unit tests for project hierarchy

## 16. Audit Logging

- [ ] 16.1 Create internal/audit/models.go (AuditLog)
- [ ] 16.2 Create internal/audit/repository.go
- [ ] 16.3 Create internal/audit/service.go
- [ ] 16.4 Create internal/audit/handler.go
- [ ] 16.5 Implement audit log creation on CRUD operations
- [ ] 16.6 Write unit tests for audit logging

## 17. K8s Cluster Management

- [ ] 17.1 Create internal/k8s/models.go (Cluster, Workload)
- [ ] 17.2 Create internal/k8s/cluster_client.go (k3d client)
- [ ] 17.3 Create internal/k8s/service.go
- [ ] 17.4 Create internal/k8s/handler.go
- [ ] 17.5 Implement cluster lifecycle (create/delete/list)
- [ ] 17.6 Implement health checks
- [ ] 17.7 Write integration tests with real k3d clusters

## 18. Physical Host Monitoring

- [ ] 18.1 Create internal/physicalhost/models.go (Host, Metrics)
- [ ] 18.2 Create internal/physicalhost/ssh_pool.go (SSH connection pool)
- [ ] 18.3 Create internal/physicalhost/repository.go
- [ ] 18.4 Create internal/physicalhost/monitor.go (metrics collection)
- [ ] 18.5 Create internal/physicalhost/cache.go (local cache)
- [ ] 18.6 Create internal/physicalhost/service.go
- [ ] 18.7 Create internal/physicalhost/handler.go
- [ ] 18.8 Implement two-layer architecture (status + data)
- [ ] 18.9 Write unit tests for SSH pool

## 19. Network Discovery

- [ ] 19.1 Create internal/discovery/models.go (DiscoveredDevice, ScanResult)
- [ ] 19.2 Create internal/discovery/scanner.go (SNMP/SSH probing)
- [ ] 19.3 Create internal/discovery/service.go
- [ ] 19.4 Create internal/discovery/handler.go
- [ ] 19.5 Implement device type detection
- [ ] 19.6 Write unit tests for scanner

## 20. Database Migrations

- [ ] 20.1 Create migration: 000001_create_users.sql
- [ ] 20.2 Create migration: 000002_create_devices.sql
- [ ] 20.3 Create migration: 000003_create_pipelines.sql
- [ ] 20.4 Create migration: 000004_create_alerts.sql
- [ ] 20.5 Create migration: 000005_create_projects.sql
- [ ] 20.6 Create migration: 000006_create_physical_hosts.sql
- [ ] 20.7 Create migration: 000007_create_audit_logs.sql
- [ ] 20.8 Test all migrations (up and down)

## 21. Main Application

- [ ] 21.1 Create cmd/devops-toolkit/main.go
- [ ] 21.2 Wire up all middleware in correct order
- [ ] 21.3 Register all HTTP routes
- [ ] 21.4 Initialize database connection
- [ ] 21.5 Initialize Redis connection
- [ ] 21.6 Start WebSocket hub
- [ ] 21.7 Implement graceful shutdown
- [ ] 21.8 Create Dockerfile
- [ ] 21.9 Create docker-compose.yml for local dev

## 22. Frontend Setup

- [ ] 22.1 Create frontend directory structure
- [ ] 22.2 Set up React with TypeScript
- [ ] 22.3 Create API client service
- [ ] 22.4 Create WebSocket client hook
- [ ] 22.5 Create basic layout components
- [ ] 22.6 Set up routing
