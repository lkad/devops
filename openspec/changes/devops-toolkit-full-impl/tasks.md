# Tasks: DevOps Toolkit Full Implementation

## 1. Project Setup

- [x] 1.1 Initialize Go module with go mod init
- [x] 1.2 Create directory structure (cmd/, internal/, pkg/, frontend/)
- [x] 1.3 Create config.yaml with all configuration sections
- [x] 1.4 Create database migration framework
- [x] 1.5 Create PostgreSQL schema for all entities
- [x] 1.6 Create package/shared for config, database, middleware

## 2. Foundation Layer

- [x] 2.1 Implement config loader (YAML + env overrides)
- [x] 2.2 Implement PostgreSQL connection pool
- [ ] 2.3 Implement Redis connection (for WebSocket scaling)
- [x] 2.4 Create HTTP middleware (auth, logging, recovery, cors)
- [x] 2.5 Implement JWT token handling
- [x] 2.6 Create base handler/service/repository patterns
- [x] 2.7 Implement health check endpoint (/health)

## 3. WebSocket Server

- [x] 3.1 Implement WebSocket endpoint (/ws)
- [x] 3.2 Implement channel subscription model
- [x] 3.3 Implement message broadcast to channels
- [x] 3.4 Integrate WebSocket with log aggregator
- [ ] 3.5 Integrate WebSocket with alert manager
- [ ] 3.6 Integrate WebSocket with device manager
- [ ] 3.7 Integrate WebSocket with pipeline manager

## 4. Metrics Collection

- [x] 4.1 Implement Prometheus metrics endpoint (/metrics)
- [x] 4.2 Implement counter metrics API
- [x] 4.3 Implement gauge metrics API
- [x] 4.4 Implement histogram metrics API
- [ ] 4.5 Add HTTP request metrics middleware
- [ ] 4.6 Add module event metrics (device, pipeline, alert)

## 5. LDAP Authentication

- [x] 5.1 Implement LDAP connection pool
- [x] 5.2 Implement user authentication against LDAP
- [x] 5.3 Implement group membership retrieval
- [x] 5.4 Implement group-to-role mapping
- [ ] 5.5 Implement retry logic for LDAP failures
- [x] 5.6 Create auth endpoints (/api/auth/login, /api/auth/logout)
- [ ] 5.7 Add LDAP health check

## 6. RBAC Permissions

- [x] 6.1 Define permission matrix in config
- [x] 6.2 Implement permission middleware
- [ ] 6.3 Implement label-based access control
- [ ] 6.4 Implement label inheritance for groups
- [ ] 6.5 Implement environment restrictions
- [ ] 6.6 Add permission checks to all protected endpoints

## 7. Device Management

- [x] 7.1 Implement device state machine with transitions
- [x] 7.2 Implement device CRUD repository
- [x] 7.3 Implement device service with business logic
- [x] 7.4 Implement device HTTP handlers
- [ ] 7.5 Implement device hierarchy (parent-child)
- [ ] 7.6 Implement device groups (flat, hierarchical, dynamic)
- [ ] 7.7 Implement configuration templates with Jinja2
- [ ] 7.8 Implement device search by tags
- [ ] 7.9 Implement device actions endpoint

## 8. Log Aggregation

- [x] 8.1 Implement LocalStorageBackend
- [ ] 8.2 Implement ElasticsearchBackend interface
- [ ] 8.3 Implement LokiBackend interface
- [x] 8.4 Implement LogManager with backend delegation
- [x] 8.5 Implement log query API with filters
- [ ] 8.6 Implement log statistics endpoint
- [ ] 8.7 Implement retention policy management
- [ ] 8.8 Implement alert rules for logs
- [ ] 8.9 Implement saved filters
- [ ] 8.10 Implement sample log generation

## 6. RBAC Permissions

- [ ] 6.1 Define permission matrix in config
- [ ] 6.2 Implement permission middleware
- [ ] 6.3 Implement label-based access control
- [ ] 6.4 Implement label inheritance for groups
- [ ] 6.5 Implement environment restrictions
- [ ] 6.6 Add permission checks to all protected endpoints

## 7. Device Management

- [ ] 7.1 Implement device state machine with transitions
- [ ] 7.2 Implement device CRUD repository
- [ ] 7.3 Implement device service with business logic
- [ ] 7.4 Implement device HTTP handlers
- [ ] 7.5 Implement device hierarchy (parent-child)
- [ ] 7.6 Implement device groups (flat, hierarchical, dynamic)
- [ ] 7.7 Implement configuration templates with Jinja2
- [ ] 7.8 Implement device search by tags
- [ ] 7.9 Implement device actions endpoint

## 8. Log Aggregation

- [ ] 8.1 Implement LocalStorageBackend
- [ ] 8.2 Implement ElasticsearchBackend interface
- [ ] 8.3 Implement LokiBackend interface
- [ ] 8.4 Implement LogManager with backend delegation
- [ ] 8.5 Implement log query API with filters
- [ ] 8.6 Implement log statistics endpoint
- [ ] 8.7 Implement retention policy management
- [ ] 8.8 Implement alert rules for logs
- [ ] 8.9 Implement saved filters
- [ ] 8.10 Implement sample log generation

## 9. Alert Notification

- [ ] 9.1 Implement notification channel CRUD
- [ ] 9.2 Implement Slack channel sender
- [ ] 9.3 Implement Webhook channel sender
- [ ] 9.4 Implement Email channel sender
- [ ] 9.5 Implement Log channel sender
- [ ] 9.6 Implement rate limiting (10 per 60s per name)
- [ ] 9.7 Implement alert history storage
- [ ] 9.8 Implement alert statistics
- [ ] 9.9 Implement alert trigger API

## 10. CI/CD Pipeline

- [ ] 10.1 Implement pipeline YAML parser
- [ ] 10.2 Implement pipeline CRUD repository
- [ ] 10.3 Implement pipeline service
- [ ] 10.4 Implement pipeline HTTP handlers
- [ ] 10.5 Implement stage execution engine
- [ ] 10.6 Implement Blue-Green deployment strategy
- [ ] 10.7 Implement Canary deployment strategy
- [ ] 10.8 Implement Rolling update strategy
- [ ] 10.9 Implement run history tracking
- [ ] 10.10 Implement pipeline statistics

## 11. Project Hierarchy

- [ ] 11.1 Implement Business Line CRUD
- [ ] 11.2 Implement System CRUD
- [ ] 11.3 Implement Project CRUD
- [ ] 11.4 Implement resource linking
- [ ] 11.5 Implement local RBAC permissions
- [ ] 11.6 Implement permission inheritance
- [ ] 11.7 Implement FinOps CSV export
- [ ] 11.8 Implement audit logging integration

## 12. K8s Cluster Management

- [ ] 12.1 Implement k3d cluster lifecycle (create/delete/list)
- [ ] 12.2 Implement cluster health checks
- [ ] 12.3 Implement workload deployment
- [ ] 12.4 Implement workload scaling
- [ ] 12.5 Implement workload deletion
- [ ] 12.6 Implement metrics collection
- [ ] 12.7 Implement pod logs retrieval
- [ ] 12.8 Implement pod exec command
- [ ] 12.9 Implement cross-cluster operations
- [ ] 12.10 Create k3d-setup.sh script

## 13. Physical Host Monitoring

- [ ] 13.1 Implement SSH connection pool
- [ ] 13.2 Implement host registration
- [ ] 13.3 Implement SSH health check
- [ ] 13.4 Implement metrics collection (CPU, memory, disk, uptime)
- [ ] 13.5 Implement service monitoring via systemctl
- [ ] 13.6 Implement config push via SSH
- [ ] 13.7 Implement two-layer architecture (status + data)
- [ ] 13.8 Implement local cache with DB fallback
- [ ] 13.9 Implement state change WebSocket events

## 14. Network Discovery

- [ ] 14.1 Implement NetworkDiscovery scanner
- [ ] 14.2 Implement SNMP probing
- [ ] 14.3 Implement SSH probing
- [ ] 14.4 Implement device type detection
- [ ] 14.5 Implement pending device workflow
- [ ] 14.6 Implement discovery status endpoint
- [ ] 14.7 Implement auto-registration flow

## 15. Test Environment

- [ ] 15.1 Create containerlab topology.yml
- [ ] 15.2 Create clab.sh management script
- [ ] 15.3 Create Docker images for simulated devices
- [ ] 15.4 Create SNMP configuration
- [ ] 15.5 Create SSH authorized keys
- [ ] 15.6 Setup InfluxDB/Prometheus/Grafana docker-compose
- [ ] 15.7 Create connection verification scripts

## 16. Frontend

- [ ] 16.1 Initialize React project
- [ ] 16.2 Create device management UI
- [ ] 16.3 Create pipeline dashboard UI
- [ ] 16.4 Create log viewer UI
- [ ] 16.5 Create alert management UI
- [ ] 16.6 Create project hierarchy UI
- [ ] 16.7 Create audit log viewer UI
- [ ] 16.8 Implement WebSocket client for real-time updates

## 17. Integration & Testing

- [ ] 17.1 Write unit tests for all services
- [ ] 17.2 Write integration tests for handlers
- [ ] 17.3 Test LDAP authentication flow
- [ ] 17.4 Test device state machine transitions
- [ ] 17.5 Test pipeline execution
- [ ] 17.6 Test WebSocket broadcast
- [ ] 17.7 Test k3d cluster operations
- [ ] 17.8 Test physical host monitoring
- [ ] 17.9 Test network discovery
