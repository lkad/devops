# Product Requirements Document – LDAP Authentication & Role Mapping

## 1. Purpose & Scope

Implement a Go‑based LDAP client that authenticates users and resolves their system roles for the DevOps platform. The client must:

- **Authenticate** against a test LDAP server (osixia/openldap) for development and a production LDAP for deployment.
- **Retrieve** group membership and map LDAP groups to internal system roles:
  - `cn=IT_Ops,ou=Groups,dc=example,dc=com` → `Operator`
  - `cn=DevTeam_Payments,ou=Groups,dc=example,dc=com` → `Developer`
  - `cn=Security_Auditors,ou=Groups,dc=example,dc=com` → `Auditor`
  - `cn=SRE_Lead,ou=Groups,dc=example,dc=com` → `SuperAdmin`
- **Expose** a simple API (`Authenticate`, `GetGroups`, `GetRoles`) for integration with other services.
- **Be thoroughly unit‑tested** against a docker‑based LDAP test environment.
- **Be production‑ready**: connection pooling, retry logic, graceful error handling, environment‑driven configuration, and compliance with OWASP top‑10 mitigations.

---

## 2. Business Objectives

| Objective | KPI | Target |
|-----------|-----|--------|
| **Secure Access Control** | Percentage of auth failures that are correctly blocked | 100 % |
| **Developer Velocity** | Time to spin up a new developer environment | ≤ 2 min |
| **Auditability** | Complete audit trail of role resolution events | 100 % of successful logins captured |
| **Operational Reliability** | Downtime during authentication failures | < 1 min for fallback mechanisms |

---

## 3. Target Audience

- **SRE Engineers** requiring programmatic LDAP integration.
- **DevOps Platform Services** that need to resolve user roles during deployment workflows.
- **Security Team** for role‑based access policies.
- **Developers** onboarding with LDAP credentials.

---

## 4. Key Features

| Feature | Description | Acceptance Criteria |
|---------|-------------|---------------------|
| **Environment‑Driven Config** | Load LDAP URL, admin DN, password, and base DN from environment variables. | Defaults used when variables are unset; required variables fail with clear error messages. |
| **Authentication** | Bind with user DN and password; return boolean success. | Returns `true` for valid credentials; `false` for invalid credentials. |
| **Group Retrieval** | Search for `groupOfNames` entries where `member` matches user DN. | Returns exact slice of group DNs; empty slice if none. |
| **Role Mapping** | Map group DNs to internal roles. | Returns unique slice of role strings; no duplicates. |
| **Retry & Timeout** | Retry LDAP operations with exponential backoff up to 3 attempts. | Operations fail after 3 attempts with detailed error logs. |
| **Unit Tests** | Spin up test LDAP server via Docker Compose; run tests against it. | ≥ 100 % test coverage for LDAP client functions. |
| **Demo CLI** | Simple command‑line app demonstrating authentication and role retrieval. | CLI prints username, authentication result, and resolved roles. |
| **Documentation** | README with setup, usage, testing instructions. | README includes Docker Compose commands, example `go run`, and test instructions. |
| **Git Workflow** | Initialize repository, commit, tag. | Git commit includes all source and test files; tag `ldap-client-setup`. |

---

## 5. User Stories

| Story | Acceptance Criteria |
|-------|----------------------|
| **SRE Engineer** | As an SRE, I can authenticate against LDAP and retrieve my system roles to enforce RBAC in the deployment pipeline. |
| **Developer** | As a developer, I can quickly set up a local LDAP test environment and run unit tests against it. |
| **Security Analyst** | As a security analyst, I can audit role assignments by inspecting logs of `GetRoles` invocations. |

---

## 6. Implementation Overview

| Step | Tool | Output |
|------|------|--------|
| 1 | `mkdir -p /mnt/devops/auth/ldap` | Package skeleton |
| 2 | `go mod init github.com/example/devops/auth/ldap` | `go.mod` |
| 3 | Create `config.go` | Loads env vars, validates required ones |
| 4 | Create `client.go` | Implements `Authenticate`, `GetGroups`, `GetRoles` |
| 5 | Create `client_test.go` | Tests using Docker Compose to spin up test LDAP | 
| 6 | Create `cmd/testldap/main.go` | Demo CLI |
| 7 | Create `README.md` | Setup, usage, testing instructions |
| 8 | `git init` | Repo creation |
| 9 | Commit all files | Git history |
| 10 | `git tag -a ldap-client-setup -m "Initial LDAP client setup"` | Tagging |

---

## 7. Testing & Verification

| Test | Tool | Description | Pass Criteria |
|------|------|-------------|---------------|
| **Unit Tests** | `go test ./... -cover` | Runs all tests against a running test LDAP container. | 100 % code coverage; all tests pass. |
| **Integration Test** | `docker compose up -d && go test ./auth/ldap -run TestIntegration` | Verifies real LDAP interactions. | No connection errors; correct role mapping. |
| **Performance** | `go test -run TestAuthPerformance` | Benchmarks authentication under load. | Latency < 50 ms; CPU < 5 % per request. |
| **Security Scan** | `gstack -run cso` | Performs OWASP Top‑10 scanning on the codebase. | No critical vulnerabilities. |

---

## 8. Deployment & Ops

- **Docker Image**: Build a minimal Go binary (`auth/ldap`) and publish to `ghcr.io/example/devops/auth-ldap:latest`.
- **Kubernetes Secret**: Store LDAP credentials (`LDAP_ADMIN_PASSWORD`) in a sealed secret.
- **Helm Chart**: Provide a chart that installs the binary, sets env vars, and mounts the Docker Compose LDIF for dev environments.
- **Observability**: Export metrics (auth success rate, latency) via Prometheus client.

---

## 9. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Credential Leak** | Sensitive LDAP passwords may be exposed in logs or config files. | Store credentials in sealed secrets; never log raw passwords. |
| **LDAP Schema Drift** | Production LDAP schema may differ from test, causing runtime failures. | Validate schema on startup; support schema version checks. |
| **Connection Failure** | LDAP server unreachable → authentication outage. | Implement retry with backoff; fallback to cached roles for a short grace period. |
| **Role Mismatch** | Incorrect group‑to‑role mapping leads to privilege escalation. | Unit tests cover all groups; audit logs capture mapping decisions. |

---

## 10. Logging & Storage Backend Integration

The LDAP client audit logs are integrated with the DevOps Toolkit's centralized logging system, supporting multiple storage backends.

### 10.1 Supported Storage Backends

| Backend | Environment Variable | Default Port | Use Case |
|---------|---------------------|--------------|----------|
| **Local** | `LOG_STORAGE_BACKEND=local` | - | Development/testing, default |
| **Elasticsearch** | `LOG_STORAGE_BACKEND=elasticsearch` | 9200 | Production full-text search |
| **Loki** | `LOG_STORAGE_BACKEND=loki` | 3100 | Grafana ecosystem, log aggregation |

### 10.2 Configuration

```bash
# Common configuration
LOG_STORAGE_BACKEND=elasticsearch  # local | elasticsearch | loki
LOG_RETENTION_DAYS=30              # Log retention in days, default 30

# Elasticsearch configuration
ELASTICSEARCH_URL=http://localhost:9200
ELASTICSEARCH_INDEX=devops-logs
ELASTICSEARCH_USERNAME=
ELASTICSEARCH_PASSWORD=

# Loki configuration
LOKI_URL=http://localhost:3100
```

### 10.3 Log Query Delegation

The system uses a query delegation pattern based on storage backend type:

- **Local backend**: Queries operational logs from local JSON array
- **Elasticsearch/Loki backend**: Delegates queries to backend API for external logs (device/container logs collected by Filebeat)

```javascript
// Query routing in LogManager
queryLogs(options = {}) {
  const backendType = this.backend.constructor.name;
  if (backendType === 'LocalStorageBackend') {
    return this.queryLogsLocal(options);
  } else {
    return this.queryLogsFromBackend(options);
  }
}
```

### 10.4 Log Types

| Log Type | Source | Storage | Query Method |
|----------|--------|---------|--------------|
| **Operational Logs** | Application via LogManager | Local or Backend | Direct query |
| **Device/Container Logs** | External collectors (Filebeat) | Elasticsearch/Loki | Backend API delegation |
| **Audit Logs** | LDAP client events | Backend | Backend API delegation |

### 10.5 Retention Policy

| Backend | Retention Method |
|---------|-----------------|
| **Local** | Application-level periodic cleanup |
| **Elasticsearch** | Index Lifecycle Management (ILM) Policy |
| **Loki** | Configure `chunk_target_size` and `retention_period` |

### 10.6 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/logs/retention` | GET | Get retention policy configuration |
| `/api/logs/retention` | PUT | Update retention policy |
| `/api/logs/retention/apply` | POST | Manually trigger retention cleanup |
| `/api/logs/backend` | GET | Get storage backend health status |
| `/api/logs` | GET | Query logs |
| `/api/logs/stats` | GET | Get log statistics |

### 10.7 Docker Development Environment

```bash
# Start full logging infrastructure
docker-compose -f docker-compose.dev.yml up -d

# Service ports
# - Elasticsearch: http://localhost:9200
# - Kibana:       http://localhost:5601
# - Loki:         http://localhost:3100
# - Grafana:      http://localhost:3001
# - Prometheus:   http://localhost:9090
```

---

## 11. Timeline (Estimated)

| Milestone | Date | Owner |
|-----------|------|-------|
| Design Sign‑off | 2026‑04‑20 | DevOps Lead |
| Package Skeleton | 2026‑04‑21 | Engineer |
| Unit Tests & Demo CLI | 2026‑04‑25 | Engineer |
| Integration & Security Tests | 2026‑04‑28 | Engineer |
| Review & Sign‑off | 2026‑04‑30 | DevOps Lead |
| Merge & Deploy | 2026‑05‑02 | Release Manager |

---

**Attachments**

- `docker-compose.yml` – Test LDAP setup
- `bootstrap.ldif` – LDAP initial data
- `auth/ldap/` – Go client source
- `cmd/testldap/` – Demo CLI
- `client_test.go` – Unit tests
- `README.md` – Build & run instructions

---

_This PRD is a living document and may be updated as the project evolves._

---

[Go back to top](#product-requirements-document--ldap-authentication--role-mapping)