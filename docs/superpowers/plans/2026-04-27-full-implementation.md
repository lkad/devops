# Full Implementation Plan - All OpenSpec Specs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Verify existing implementations against OpenSpec specs and implement any gaps to achieve 100% spec compliance.

**Architecture:** Modular verification approach - each subsystem (k8s, project, physicalhost, etc.) is verified independently, then gaps are fixed. Follows existing internal/ package structure.

**Tech Stack:** Go (existing), PostgreSQL, Kubernetes client-go, gorilla/websocket, JWT

---

## Overview

There are **21 OpenSpec specs** and **15 internal modules**. Most features in TODOS.md are marked ✅ complete, suggesting existing implementations. The work is:
1. **Verify** - Confirm existing code matches specs
2. **Gap-fill** - Implement missing requirements
3. **Test** - Ensure tests cover all spec scenarios

### Subsystem Groups

| Group | Specs | Priority |
|-------|-------|----------|
| Core Infra | api-contract, architecture-foundation, middleware-stack, database-schema | High |
| Auth | ldap-authentication, rbac-permissions | High |
| Data | device-management, log-aggregation, metrics-collection | High |
| Cloud | k8s-cluster-management, k8s-pod-log-streaming | High |
| Ops | physical-host-monitoring, network-discovery, alert-notification | Medium |
| Project | project-hierarchy, audit-logging | Medium |
| Pipeline | cicd-pipeline | Medium |
| Realtime | websocket-hub, websocket-realtime | Low |
| Config | config-management, test-environment | Low |

---

## Phase 1: Gap Analysis (Verification)

Before implementing, verify what exists vs what's required.

### Task 1: Verify Core Infrastructure Specs

**Files:**
- Check: `internal/config/loader.go`
- Check: `cmd/devops-toolkit/main.go`
- Check: `internal/*/models.go` (all modules)

- [ ] **Step 1: Verify api-contract spec compliance**
  - Check all endpoints use `/api/` prefix (not `/api/v1/`)
  - Verify standard response format: `{"data": ..., "meta": ...}`
  - Verify standard error format: `{"error": {"code": ..., "message": ...}}`
  - Check pagination on list endpoints

- [ ] **Step 2: Verify architecture-foundation spec compliance**
  - Confirm each module has handler.go, service.go, repository.go, models.go
  - Confirm domain models use json: tags, no db tags
  - Confirm no cross-module imports (internal/* → internal/*)

- [ ] **Step 3: Verify middleware-stack spec compliance**
  - Confirm middleware order: CORS → Recovery → Logging → Metrics → Auth → RBAC
  - Check public routes skip auth: /health, /metrics, /api/auth/*

### Task 2: Verify Auth & Permissions Specs

**Files:**
- Check: `internal/auth/handler.go`
- Check: `internal/auth/middleware.go`
- Check: `internal/auth/ldap/` (entire dir)
- Check: `internal/rbac/permissions.go`

- [ ] **Step 1: Verify ldap-authentication spec compliance**
  - Check POST /api/auth/login with LDAP bind
  - Verify JWT token generation on successful auth
  - Confirm group membership retrieval
  - Confirm group-to-role mapping from config
  - Check connection pooling
  - Check retry logic on transient failures

- [ ] **Step 2: Verify rbac-permissions spec compliance**
  - Check 4 roles: SuperAdmin, Operator, Developer, Auditor
  - Verify permission matrix matches spec
  - Check label-based access control
  - Check environment restrictions (prod/dev/test)

### Task 3: Verify Kubernetes Specs

**Files:**
- Check: `internal/k8s/cluster_manager.go`
- Check: `internal/k8s/models.go`

- [ ] **Step 1: Verify k8s-cluster-management spec compliance**
  - Check POST /api/k8s/clusters with kubeconfig validation
  - Verify cluster listing and details
  - Check health checks (GET /api/k8s/clusters/:id/health)
  - Verify node listing with CPU/memory
  - Check workload listing (deployments, statefulsets)
  - Verify pod logs retrieval
  - Check kubeconfig NOT exposed in API responses

- [ ] **Step 2: Verify k8s-pod-log-streaming spec compliance**
  - Check WebSocket channel `container_log` exists
  - Verify log message format with clusterId, namespace, pod, container
  - Check log persistence via logsService
  - Verify log level inference (error/warn/info)
  - Check multi-container support

### Task 4: Verify Project & Audit Specs

**Files:**
- Check: `internal/project/manager.go`
- Check: `internal/project/repository.go`
- Check: `internal/project/audit.go`

- [ ] **Step 1: Verify project-hierarchy spec compliance**
  - Check Business Line CRUD: POST/GET/PUT/DELETE /api/org/business-lines
  - Check System CRUD: /api/org/business-lines/:id/systems
  - Check Project CRUD: /api/org/systems/:id/projects
  - Verify resource linking: POST/GET/DELETE /api/org/projects/:id/resources
  - Check permission management: /api/org/projects/:id/permissions
  - Verify FinOps report: GET /api/org/reports/finops

- [ ] **Step 2: Verify audit-logging spec compliance**
  - Check audit log creation on all CRUD operations
  - Verify audit fields: id, timestamp, username, action, entity_type, entity_id, changes
  - Check audit query API: GET /api/org/audit-logs with filters

### Task 5: Verify Remaining Specs

**Files:**
- Check: `internal/device/` (all files)
- Check: `internal/logs/` (all files)
- Check: `internal/metrics/` (all files)
- Check: `internal/physicalhost/` (all files)
- Check: `internal/alerts/` (all files)
- Check: `internal/pipeline/` (all files)
- Check: `internal/discovery/` (all files)
- Check: `internal/websocket/` (all files)

- [ ] **Step 1: Verify device-management spec**
  - State machine: PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE
  - Device types support
  - Parent-child hierarchy
  - Device groups (flat, hierarchical, dynamic)
  - Configuration templates
  - Device search and actions

- [ ] **Step 2: Verify log-aggregation spec**
  - Storage backends: local, elasticsearch, loki
  - Log query API with filters
  - Log statistics
  - Retention policy
  - Alert rules
  - Saved filters

- [ ] **Step 3: Verify metrics-collection spec**
  - Prometheus /metrics endpoint
  - Counter, Gauge, Histogram types
  - HTTP metrics middleware

- [ ] **Step 4: Verify physical-host-monitoring spec**
  - Host registration with SSH credentials
  - Connection pooling
  - Health checks
  - Metrics collection (CPU, memory, disk, uptime)
  - Service monitoring via systemctl
  - Configuration push via SSH
  - Two-layer architecture (status independent of DB)
  - Local cache for fallback

- [ ] **Step 5: Verify alert-notification spec**
  - Channel management (slack, webhook, email, log)
  - Alert triggering
  - Rate limiting (per name)
  - Alert history

- [ ] **Step 6: Verify cicd-pipeline spec**
  - Pipeline CRUD
  - Stage execution engine
  - Run history
  - Cancel support

- [ ] **Step 7: Verify network-discovery spec**
  - SNMP/SSH scanning
  - Device discovery
  - Protocol detection

- [ ] **Step 8: Verify websocket-hub spec**
  - /ws endpoint with upgrade
  - Channel subscription (log, metric, device_event, pipeline_update, alert, container_log)
  - Broadcast to subscribers
  - Ping/pong heartbeat
  - Client disconnect handling

---

## Phase 2: Gap Implementation

After verification, implement missing requirements.

### Task 6: Implement Identified Gaps

**Priority Order (High → Low):**

1. **container_log WebSocket channel** (if missing from websocket-hub)
2. **Kubeconfig masking** in cluster details response
3. **Permission inheritance** in project hierarchy
4. **State change events** in physical host monitoring
5. **Log level inference** in k8s-pod-log-streaming
6. **Historical log navigation** link in k8s-pod-log-streaming

### Task 7: Add Missing Tests

For each gap identified, add corresponding tests following TDD:

- [ ] **Step 1: Write failing test for each gap**
- [ ] **Step 2: Implement minimal code to pass**
- [ ] **Step 3: Run tests to verify**
- [ ] **Step 4: Commit**

---

## Phase 3: Final Verification

### Task 8: End-to-End Verification

- [ ] **Step 1: Run all tests** `go test ./...`
- [ ] **Step 2: Verify no TODO/placeholder comments in code**
- [ ] **Step 3: Verify all spec requirements have corresponding tests**
- [ ] **Step 4: Final commit with verification results**

---

## Implementation Order

Given the scope, execute in this order:

1. **Phase 1 Tasks 1-5** (Gap Analysis) - Can be done in parallel by subagents
2. **Phase 2 Task 6** (Gap Implementation) - Sequential, by priority
3. **Phase 3 Tasks 7-8** (Testing & Verification) - Final sequential steps

---

## Execution Options

**Option 1: Subagent-Driven (recommended)**
- Create worktree for this effort
- Dispatch parallel subagents for Phase 1 gap analysis per subsystem group
- Each subagent verifies one group and reports gaps
- Then implement gaps by priority

**Option 2: Sequential Implementation**
- One task at a time in this session
- Use checkpoints between phases

**Which approach?**
