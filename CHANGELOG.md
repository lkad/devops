# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0.0] - 2026-04-26

### Added

- **LDAP Authentication** — Full LDAP auth with connection pooling, retry logic, group resolution, and role mapping
- **JWT Middleware** — Bearer token validation on all protected routes with user context propagation
- **RBAC Permissions** — Role-based access control (Auditor, Developer, Operator, SuperAdmin) with permission enforcement middleware
- **Auth Handler** — Login, logout, and /me endpoints for session management
- **Config Loader** — YAML config loading with environment variable overrides
- **Project Management** — Full CRUD for BusinessLine → System → Project hierarchy with PostgreSQL persistence
- **Project Audit Logging** — All project management changes are now recorded in the audit log
- **K8s Multi-cluster Filtering** — Multi-cluster support with environment color coding
- **Physical Host Bug Fix** — Fixed mux.Vars path parameter extraction in GetHostHTTP and DeleteHostHTTP
- **Integration Tests** — Comprehensive HTTP integration tests for device, physicalhost, logs, and project modules

### Fixed

- Auth middleware blocking static files — now excludes /static and assets paths
- Auth store rehydration — JWT now properly restored on page reload
- Repository list result slices — initialized to empty arrays instead of nil
- Project integration tests — cleanup before tests to remove stale test data
- K8s container bounds check — panic prevention in GetPods
- Physical host path parameters — uses mux.Vars instead of query params

### Changed

- DevOps Toolkit frontend now served from /devops/ sub-path
- Vite base configuration set to './' for relative asset loading
- ARCHITECTURE.md updated with nginx reverse proxy sub-path configuration
