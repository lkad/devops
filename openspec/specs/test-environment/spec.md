# test-environment

## Purpose

Define the complete test environment infrastructure for the DevOps Toolkit: a three-tier strategy (dev/ci/prod) with deterministic mock data, reproducible Containerlab topologies, support service stacks, and CI workflow integration. The goal is that any developer can spin up a full-featured test environment in under 5 minutes, and CI can run the same setup deterministically.

## Requirements

### Requirement: Three-Tier Test Environment
The system SHALL provide three distinct test environments: dev (mock-only), integration (Containerlab), and production-template (disabled).

#### Scenario: Dev environment
- **WHEN** user runs `./scripts/setup.sh dev`
- **THEN** system starts only PostgreSQL + mock data layer
- **AND** all external dependencies (LDAP, ES, Loki, Containerlab) are stubbed
- **AND** total startup time is under 30 seconds

#### Scenario: Integration environment
- **WHEN** user runs `./scripts/setup.sh ci`
- **THEN** system deploys full Containerlab topology + k3d cluster + all support services
- **AND** total startup time is 2-5 minutes
- **AND** environment is suitable for E2E and CI integration tests

#### Scenario: Production-template
- **WHEN** user runs `./scripts/setup.sh prod`
- **THEN** system returns error "Production setup is disabled in this script"
- **AND** points user to DEPLOY.md for actual production deployment

### Requirement: Containerlab Topology
The system SHALL provide Containerlab-based dual-datacenter test topology.

#### Scenario: Deploy topology
- **WHEN** user runs `clab.sh deploy`
- **THEN** Containerlab creates dual-datacenter topology with 8 nodes

#### Scenario: Node inventory
- **WHEN** topology is deployed
- **THEN** system has 8 nodes: 4 per DC (1 core switch, 2 physical hosts, 1 container)
- **AND** node naming follows pattern: dc{1|2}-{role}-{n}

#### Scenario: Management network
- **WHEN** topology is deployed
- **THEN** management network `devops-mgmt` (172.30.30.0/24) is created
- **AND** all nodes are reachable from the host on this subnet

### Requirement: Dual Datacenter Architecture
The system SHALL simulate two independent datacenters with trunk links.

#### Scenario: DC network isolation
- **WHEN** DC1 and DC2 are deployed
- **THEN** DC1 uses 10.0.1.0/24, DC2 uses 10.0.2.0/24 for application traffic
- **AND** both DCs share 172.30.30.0/24 management subnet

#### Scenario: Trunk links
- **WHEN** topology is deployed
- **THEN** at least one inter-DC trunk connection exists between DC1 and DC2 core switches

### Requirement: Simulated Device Types
The system SHALL simulate PhysicalHost, NetworkDevice, and Container.

#### Scenario: PhysicalHost simulation
- **WHEN** containerlab physical host node is deployed
- **THEN** SSH is available on port 22 with real openssh-server

#### Scenario: NetworkDevice simulation
- **WHEN** containerlab switch node is deployed
- **THEN** SNMP is available on UDP:161 with net-snmpd
- **AND** SNMP community is configurable via `SNMP_COMMUNITY` env var

#### Scenario: Container simulation
- **WHEN** containerlab container node is deployed
- **THEN** Docker API is accessible for native container operations

### Requirement: Support Service Stack
The system SHALL provide a Docker Compose stack for all support services.

#### Scenario: PostgreSQL available
- **WHEN** docker-compose is started
- **THEN** PostgreSQL 15+ is available on port 5432
- **AND** database `devops` (or `devops_ci` for integration) is auto-created
- **AND** GORM AutoMigrate runs on first start

#### Scenario: Loki available
- **WHEN** docker-compose is started
- **THEN** Loki is available on port 3100 with `/ready` health endpoint

#### Scenario: Elasticsearch available
- **WHEN** docker-compose is started
- **THEN** Elasticsearch is available on port 9200 with `/\_cluster/health` endpoint

#### Scenario: Prometheus available
- **WHEN** docker-compose is started
- **THEN** Prometheus is available on port 9090 with `/-/ready` endpoint

#### Scenario: InfluxDB available
- **WHEN** docker-compose is started
- **THEN** InfluxDB v2 is available on port 8086 with bucket `metrics` initialized

#### Scenario: Grafana available
- **WHEN** docker-compose is started
- **THEN** Grafana is available on port 3001

#### Scenario: LDAP available
- **WHEN** docker-compose is started
- **THEN** bitnami/openldap is available on port 389
- **AND** bootstrap.ldif is loaded with users and groups

### Requirement: k3d Cluster Integration
The system SHALL provide a k3d-based Kubernetes cluster for K8s feature testing.

#### Scenario: k3d cluster creation
- **WHEN** user runs `k3d-setup.sh deploy`
- **THEN** k3d creates cluster `dev-cluster-1` with 1 server and 2 agents
- **AND** K8s API is exposed on port 6550
- **AND** kubeconfig is written to `~/.kube/config`

#### Scenario: Traefik disabled
- **WHEN** k3d cluster is created
- **THEN** Traefik ingress controller is disabled
- **AND** cluster is left ready for custom ingress setup

### Requirement: Standard Script Interface
All orchestration scripts SHALL implement a standard subcommand interface.

#### Scenario: Standard subcommands
- **WHEN** any orchestration script is invoked
- **THEN** it accepts one of: `deploy`, `destroy`, `status`, `logs`, `help`
- **AND** unknown subcommands return non-zero exit code with usage info

#### Scenario: Script conventions
- **WHEN** any script runs
- **THEN** it uses `set -euo pipefail`
- **AND** uses color output: green=ok, red=fail, yellow=warn, blue=info
- **AND** prefixes each action with `[STEP] xxx`
- **AND** writes logs to `./logs/<script-name>.log`
- **AND** exits non-zero on any failure with error to stderr

#### Scenario: Top-level scripts
- **WHEN** user runs top-level scripts
- **THEN** `setup.sh {dev|ci|prod}` orchestrates the full environment
- **AND** `teardown.sh` removes all resources except config/source
- **AND** `reset.sh` clears data but keeps services running (with confirm prompt)
- **AND** `status.sh` shows health of all components

#### Scenario: Component scripts
- **WHEN** user runs component scripts
- **THEN** `clab.sh`, `k3d-setup.sh`, `db-setup.sh`, `ldap-seed.sh`, `seed-data.sh`, `verify.sh` each manage one subsystem

### Requirement: Configuration Templates
The system SHALL provide one config template per environment.

#### Scenario: Dev config
- **WHEN** `ENV=dev` is set
- **THEN** system loads `configs/templates/config-dev.yaml`
- **AND** `auth.dev_bypass=true` is enabled
- **AND** `use_mock: {vmware, snmp, ssh}` are all true
- **AND** `seed_data: minimal`

#### Scenario: CI config
- **WHEN** `ENV=ci` is set
- **THEN** system loads `configs/templates/config-ci.yaml`
- **AND** `auth.dev_bypass=false` (real LDAP auth)
- **AND** `use_mock` are all false (real integrations)
- **AND** `seed_data: full`

#### Scenario: Prod config template
- **WHEN** `ENV=production` is set
- **THEN** system loads `configs/templates/config-prod.yaml`
- **AND** all secrets must come from environment variables (no hardcoded values)
- **AND** `auth.jwt_secret`, `database.url`, `ldap.url` are required env vars
- **AND** mTLS is enabled by default

#### Scenario: Config validation
- **WHEN** config is loaded
- **THEN** system fails fast if required env vars are missing in production mode
- **AND** logs a warning if `dev_bypass=true` in non-dev mode

### Requirement: LDAP Seed Data
The system SHALL provide LDAP seed data covering 4 roles and 10 users.

#### Scenario: Users loaded
- **WHEN** ldap-seed.sh runs
- **THEN** 10 users are created under `ou=users,dc=example,dc=com`
- **AND** passwords are simple (e.g. `alice123`) for easy manual testing

#### Scenario: Groups loaded
- **WHEN** ldap-seed.sh runs
- **THEN** at least 5 groups are created covering: sre-lead, dev-team, audit, readonly, custom
- **AND** each group has at least 1 member

#### Scenario: Bootstrap ordering
- **WHEN** LDAP container starts
- **THEN** `bootstrap.ldif` references `users.ldif` and `groups.ldif` in correct order
- **AND** containers can bind immediately after `ldapsearch` returns success

### Requirement: Device Seed Data
The system SHALL provide JSON fixtures for all 8 Containerlab nodes plus VMware and SNMP mocks.

#### Scenario: Containerlab node fixture
- **WHEN** system loads `tests/fixtures/devices/8-clab-nodes.json`
- **THEN** it contains 8 devices matching the Containerlab topology (4 per DC)
- **AND** each entry has: id, name, type, ip, dc, plus type-specific fields (ssh_port, snmp_community, etc.)

#### Scenario: VMware mock fixture
- **WHEN** system runs in dev mode (`use_mock.vmware=true`)
- **THEN** it loads `tests/fixtures/devices/fake-vmware.json` instead of querying real vCenter

#### Scenario: SNMP mock fixture
- **WHEN** system runs in dev mode (`use_mock.snmp=true`)
- **THEN** it loads `tests/fixtures/devices/fake-snmp.json` instead of real SNMP polls

### Requirement: Project Hierarchy Seed Data
The system SHALL provide 3 Business Lines, 6 Systems, and 12 Projects as seed data.

#### Scenario: Business lines fixture
- **WHEN** seed-data.sh runs
- **THEN** `business-lines.json` contains 3 entries with weight summing to 1.0
- **AND** example: 电商事业部 (0.6), 金融事业部 (0.3), 基础平台部 (0.1)

#### Scenario: Systems fixture
- **WHEN** seed-data.sh runs
- **THEN** `systems.json` contains 6 systems linked to business lines
- **AND** each system has a `business_line_id` foreign key

#### Scenario: Projects fixture
- **WHEN** seed-data.sh runs
- **THEN** `projects.json` contains 6+ projects linked to systems
- **AND** each project has a `system_id` and `type_id`

### Requirement: Alert and Log Seed Data
The system SHALL provide alert rule fixtures and sample log entries.

#### Scenario: Alert rules fixture
- **WHEN** seed-data.sh runs
- **THEN** `alerts-rules.json` contains 5 alert rules
- **AND** rules cover: error rate, host offline, pipeline failed, k8s pod restart, host-in-maintenance (with suppress_external)

#### Scenario: Sample logs fixture
- **WHEN** seed-data.sh runs
- **THEN** `sample-100.json` contains 100 log entries spanning all 4 levels
- **AND** entries are timestamped within the last 24 hours
- **AND** hostnames reference real Containerlab nodes

### Requirement: Connection Verification Scripts
The system SHALL provide scripts to verify all component connections.

#### Scenario: Component health check
- **WHEN** user runs `verify-conn.sh`
- **THEN** it checks health of: PostgreSQL, Loki, ES, Prometheus, InfluxDB, LDAP, Containerlab, k3d, App
- **AND** returns 0 only if all checks pass
- **AND** outputs per-check status (ok/fail) to stdout

#### Scenario: Data integrity check
- **WHEN** user runs `verify-data.sh`
- **THEN** it counts rows in key tables (devices, business_lines, systems, projects, project_types)
- **AND** fails if any count is below the expected minimum

#### Scenario: End-to-end flow check
- **WHEN** user runs `verify-flow.sh`
- **THEN** it executes: login, list devices, query logs, trigger alert
- **AND** returns 0 only if all flow steps succeed
- **AND** each step uses real HTTP calls (no shortcuts)

### Requirement: CI Workflow Integration
The system SHALL provide GitHub Actions workflows for unit and integration tests.

#### Scenario: Unit test workflow
- **WHEN** PR is opened or commit is pushed
- **THEN** `.github/workflows/test-unit.yml` runs `go test ./... -race -cover`
- **AND** uses PostgreSQL as a service container
- **AND** uploads coverage to Codecov

#### Scenario: Integration test workflow
- **WHEN** PR is opened
- **THEN** `.github/workflows/test-integration.yml` runs `setup.sh ci` then `go test -tags=integration ./...`
- **AND** uploads logs as artifacts on failure
- **AND** has 30-minute timeout

### Requirement: Reset and Cleanup Procedure
The system SHALL provide a safe reset/cleanup flow with confirmation prompts.

#### Scenario: Three-tier cleanup
- **WHEN** user wants to clean up
- **THEN** `reset.sh` clears data only (services stay)
- **AND** `teardown.sh` removes everything except config/source
- **AND** `docker-compose down -v && clab.sh destroy && k3d-setup.sh destroy` removes containers + volumes + clusters

#### Scenario: Reset confirmation
- **WHEN** `reset.sh` is invoked
- **THEN** it prompts "This will DELETE all data... Continue? (yes/no):"
- **AND** exits 1 unless user types `yes`

#### Scenario: Reset sequence
- **WHEN** `reset.sh` proceeds after confirmation
- **THEN** it: stops app → drops/recreates DB → clears Loki → clears ES indices → clears InfluxDB → re-runs AutoMigrate → re-seeds data → restarts app

### Requirement: Auto-Registration of Test Devices
The system SHALL automatically register discovered Containerlab devices.

#### Scenario: Discovery flow
- **WHEN** containerlab topology is running and DevOps Toolkit is active
- **THEN** NetworkDiscovery scans 172.30.30.0/24 and creates PENDING devices
- **AND** each discovered device has `source=containerlab` tag

### Requirement: Simulated Metrics
The system SHALL generate realistic metrics for testing.

#### Scenario: Physical host metrics
- **WHEN** metrics are collected from physical host
- **THEN** system returns CPU, memory, disk, uptime data

#### Scenario: Network device metrics
- **WHEN** metrics are collected from network device via SNMP
- **THEN** system returns interface status, traffic octets, uptime

### Requirement: Troubleshooting Manual
The system SHALL provide a TROUBLESHOOTING.md document with common failures and recovery steps.

#### Scenario: Common failures documented
- **WHEN** user encounters a problem
- **THEN** TROUBLESHOOTING.md has a symptom → cause → fix table covering: port conflicts, container failures, LDAP bind errors, k3d port reuse, AutoMigrate missing, alert channel misconfig

#### Scenario: Debug commands documented
- **WHEN** user needs to debug
- **THEN** TROUBLESHOOTING.md lists exact commands for: `docker ps`, `docker logs`, `pg_isready`, `ldapsearch`, `docker exec clab-...-dc1-web-21 bash`, `kubectl get nodes`

#### Scenario: Nuclear cleanup option
- **WHEN** environment is in a broken state
- **THEN** TROUBLESHOOTING.md documents: `docker ps -aq | xargs docker stop` + `docker network prune -f` + `docker volume prune -f` + `./scripts/setup.sh ci`

### Requirement: Environment Readiness Checklist
The system SHALL provide a final checklist to confirm the environment is ready.

#### Scenario: Readiness verification
- **WHEN** user runs the full readiness flow
- **THEN** `./scripts/setup.sh ci` starts the environment
- **AND** `./scripts/verify/verify-conn.sh` returns 0 (all components green)
- **AND** `./scripts/verify/verify-data.sh` returns 0 (seed data complete)
- **AND** `./scripts/verify/verify-flow.sh` returns 0 (E2E passes)
- **AND** `go test ./...` and `go test -tags=integration ./...` both pass
- **AND** `http://localhost:3000` shows login page (user can log in as alice/alice123)
