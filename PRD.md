# DevOps Toolkit - Product Requirements & Design Document

**Version:** 1.3
**Last Updated:** 2026-04-19

---

## Table of Contents

1. [Overview](#1-overview)
2. [CI/CD Pipeline](#2-cicd-pipeline)
3. [Device Management](#3-device-management)
4. [Logging System](#4-logging-system)
5. [Monitoring System](#5-monitoring-system)
6. [Alert Notification System](#6-alert-notification-system)
7. [WebSocket Real-Time Communication](#7-websocket-real-time-communication)
8. [Prometheus Metrics](#8-prometheus-metrics)
9. [LDAP Authentication & Role Mapping](#9-ldap-authentication--role-mapping)
10. [Permission Model](#10-permission-model)
11. [Architecture Overview](#11-architecture-overview)
12. [API Design](#12-api-design)

---

## 1. Overview

This document outlines the design for a comprehensive DevOps system that integrates CI/CD pipelines, device management, logging, and monitoring capabilities.

### System Components

```
┌────────────────────────────────────────────────────────────────────┐
│                      DevOps Toolkit                                 │
├────────────────────────────────────────────────────────────────────┤
│  CI/CD Pipeline  │  Device Management  │  Logging/Monitoring   │
│  - Pipeline CRUD  │  - Device Registry   │  - Log Aggregation    │
│  - Stage Execution│  - Config Deploy     │  - Metrics Collection │
│  - Run History    │  - Remote Actions   │  - Alert Routing      │
└────────────────────────────────────────────────────────────────────┘
```

---

## 2. CI/CD Pipeline

### 2.1 Core Flow

```
Push → Webhook Trigger → Build → Test → Package → Deploy → Verify
```

### 2.2 Build Stages

| Stage | Description |
|-------|-------------|
| validate | Code validation and linting |
| build | Multi-language build (Go, Python, Node.js, Java, .NET) |
| test | Unit tests with 80%+ coverage |
| security_scan | Trivy/Snyk vulnerability scanning |
| stage_deploy | Deploy to staging environment |
| smoke_test | Basic health verification |
| prod_deploy | Production deployment |
| verification | Post-deployment verification |

### 2.3 Deployment Strategies

- **Blue-Green Deployment**: Zero-downtime switching, fast rollback
- **Canary Release**: Gradual traffic allocation (1%→5%→25%→100%)
- **Rolling Update**: Maximum 20% instances upgraded simultaneously
- **Health Checks**: Readiness and liveness probes configuration

### 2.4 Pipeline YAML Structure

```yaml
stages:
  - validate
  - build
  - test
  - security_scan
  - stage_deploy
  - smoke_test
  - prod_deploy
  - verification
```

---

## 3. Device Management

### 3.1 Device Types

| Type | Description | Protocol | Discovery |
|------|-------------|----------|-----------|
| PhysicalHost | Physical servers/VMs | SSH, WinRM | Active registration |
| Container | Docker/K8s containers | StdIO, gRPC | Auto-discovery |
| NetworkDevice | Switches, routers | SNMP, NETCONF | Pull registration |
| LoadBalancer | HAProxy, Nginx | HTTP, REST API | Config import |
| CloudInstance | EC2, GCE, VMs | SSH, Custom | Cloud API discovery |
| IoT_Device | Sensors, edge devices | MQTT, HTTP | OTA registration |

### 3.2 Device State Machine

```
PENDING → AUTHENTICATED → REGISTERED → ACTIVE
ACTIVE → MAINTENANCE → SUSPENDED → RETIRE
```

### 3.3 Configuration Management

- **Template Engine**: Jinja2/Template2 with variables and logic
- **Inheritance Chain**: Base template → Type override → Instance override
- **Version Control**: Every change is traceable and reversible
- **Gradual Push**: Push configs in batches by tag groups

### 3.4 Group System

- **Flat Grouping**: Filter by tags (`label=env:prod`)
- **Hierarchical Grouping**: Parent group contains child groups with auto-inheritance
- **Dynamic Grouping**: Based on real-time attributes (`cluster=us-east && role=web`)
- **Overlapping Groups**: Devices can belong to multiple groups

### 3.5 Device Relationship Model

```
PhysicalHost (rack-01)
└─ VM (vm-web-001)
   └─ Container (nginx-abc123)
      └─ Container (app-xyz789)

NetworkDevice (core-01)
├─ VM (vm-jumpbox)
└─ PhysicalHost (firewall-01)
```

---

## 4. Logging System

### 4.1 Log Architecture

```
Sources → Collectors → Aggregator → Storage → Query/Search
     ↓        ↓              ↓         ↓         ↓
   Agents  Transport    Indexing    ES/S3   UI/REST
```

### 4.2 Storage Backend Support

| Backend | Environment Variable | Default Port | Use Case |
|---------|---------------------|--------------|----------|
| **Local** | `LOG_STORAGE_BACKEND=local` | - | Dev/Testing (default) |
| **Elasticsearch** | `LOG_STORAGE_BACKEND=elasticsearch` | 9200 | Production full-text search |
| **Loki** | `LOG_STORAGE_BACKEND=loki` | 3100 | Grafana ecosystem, log aggregation |

### 4.3 Configuration

```bash
# Common configuration
LOG_STORAGE_BACKEND=elasticsearch  # local | elasticsearch | loki
LOG_RETENTION_DAYS=30

# Elasticsearch configuration
ELASTICSEARCH_URL=http://localhost:9200
ELASTICSEARCH_INDEX=devops-logs

# Loki configuration
LOKI_URL=http://localhost:3100
```

### 4.4 Log Types

| Log Type | Source | Storage | Query Method |
|----------|--------|---------|--------------|
| **Operational Logs** | Application via LogManager | Local or Backend | Direct query |
| **Device/Container Logs** | External collectors (Filebeat) | Elasticsearch/Loki | Backend API delegation |
| **Audit Logs** | LDAP client events | Backend | Backend API delegation |

### 4.5 Log Query Delegation

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

### 4.6 Retention Policy

| Backend | Retention Method |
|---------|-----------------|
| **Local** | Application-level periodic cleanup |
| **Elasticsearch** | Index Lifecycle Management (ILM) Policy |
| **Loki** | Configure `chunk_target_size` and `retention_period` |

### 4.7 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/logs` | GET | Query logs |
| `/api/logs/stats` | GET | Get log statistics |
| `/api/logs/backend` | GET | Get storage backend health |
| `/api/logs/retention` | GET | Get retention policy |
| `/api/logs/retention` | PUT | Update retention policy |
| `/api/logs/retention/apply` | POST | Trigger retention cleanup |
| `/api/logs/alerts` | GET/POST | Alert rules CRUD |
| `/api/logs/filters` | GET/POST | Saved filters CRUD |

### 4.8 Filebeat Integration

External collectors (Filebeat) collect device and container logs:

```bash
./scripts/filebeat-test.sh setup              # Create config and sample logs
./scripts/filebeat-test.sh elasticsearch     # Configure Filebeat output
./scripts/filebeat-test.sh start             # Start Filebeat
./scripts/filebeat-test.sh loki              # Configure for Loki
```

---

## 5. Monitoring System

### 5.1 Metrics Collection

- **Pull Model**: Prometheus client, custom middleware
- **Push Model**: OTLP protocol, multi-language SDK support
- **Auto-Discovery**: K8s ServiceMonitor, NodeExporter discovery
- **Custom Collection**: Application instrumentation, business metrics

### 5.2 Metric Types

| Type | Description | Examples |
|------|-------------|----------|
| **Counter** | Monotonically increasing | Request count, error count |
| **Gauge** | Can go up and down | QPS, concurrency |
| **Histogram** | Value distribution | Request latency |
| **Summary** | Aggregated data | Total latency, percentiles |

### 5.3 Storage Layer

- **Short-term**: Prometheus TSDB (15 days)
- **Long-term**: Cortex/Thanos (90+ days)
- **Compression**: LevelDB + delta encoding
- **Sharding**: Horizontal by time range and tenant

---

## 6. Alert Notification System

### 6.1 Supported Channels

| Channel Type | Configuration | Description |
|-------------|---------------|-------------|
| `slack` | `webhookUrl`, `channel` | Slack webhook notifications |
| `webhook` | `url`, `headers` | Generic HTTP webhook |
| `email` | `recipients` | Email notifications (SMTP) |
| `log` | - | Log output only |

### 6.2 Rate Limiting

- **Time Window**: 60 seconds
- **Max Alerts**: 10 per window per alert name

### 6.3 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/alerts/channels` | GET | List notification channels |
| `/api/alerts/channels` | POST | Add notification channel |
| `/api/alerts/channels/:name` | DELETE | Remove channel |
| `/api/alerts/history` | GET | Query alert history |
| `/api/alerts/stats` | GET | Alert statistics |
| `/api/alerts/trigger` | POST | Trigger an alert |

### 6.4 Environment Variables

```bash
# Slack configuration
ALERT_SLACK_WEBHOOK=https://hooks.slack.com/services/xxx
ALERT_SLACK_CHANNEL=#alerts

# Generic Webhook
ALERT_WEBHOOK_URL=https://example.com/webhook
```

---

## 7. WebSocket Real-Time Communication

### 7.1 Supported Channels

| Channel | Event | Description |
|---------|-------|-------------|
| `log` | Log entry added | Real-time log streaming |
| `metric` | Metric update | Live metrics broadcast |
| `device_event` | Device status change | Device state updates |
| `pipeline_update` | Pipeline execution | Run progress notifications |
| `alert` | Alert triggered | Alert notifications |

### 7.2 Client Subscription

```javascript
const ws = new WebSocket('ws://localhost:3000/ws');
ws.send(JSON.stringify({ action: 'subscribe', channel: 'log' }));
ws.send(JSON.stringify({ action: 'subscribe', channel: 'alert' }));

// Receive messages
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.channel, data.data);
};
```

### 7.3 Message Format

```json
{
  "channel": "log",
  "type": "log",
  "data": { "id": "...", "message": "...", "level": "info" },
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

---

## 8. Prometheus Metrics

### 8.1 Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/metrics` | GET | Prometheus text format |
| `/api/metrics` | GET | JSON format |
| `/api/metrics/counter` | POST | Increment counter |
| `/api/metrics/gauge` | POST | Set/inc/dec gauge |
| `/api/metrics/histogram` | POST | Observe histogram value |

### 8.2 Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'devops-toolkit'
    static_configs:
      - targets: ['localhost:3000']
    metrics_path: '/metrics'
```

### 8.3 Available Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `devops_toolkit_info` | Gauge | service, version | System info |
| `http_requests_total` | Counter | endpoint, method, status | HTTP request count |
| `http_request_duration_ms` | Histogram | endpoint, method, status | Request latency |
| `logs_total` | Counter | level | Log count by level |
| `device_events_total` | Counter | type | Device events |
| `pipeline_events_total` | Counter | type, pipeline | Pipeline events |
| `alerts_total` | Counter | name, severity | Alert count |

---

## 9. LDAP Authentication & Role Mapping

### 9.1 Requirements

- **Authenticate** against LDAP server for user verification
- **Retrieve** group membership and map to internal roles
- **Expose** API (`Authenticate`, `GetGroups`, `GetRoles`)
- **Unit tested** against docker-based LDAP test environment
- **Production ready**: connection pooling, retry logic, graceful error handling

### 9.2 Group to Role Mapping

| LDAP Group | System Role | Permissions |
|------------|------------|-------------|
| `cn=IT_Ops,ou=Groups,dc=example,dc=com` | Operator | deploy, config-manage |
| `cn=DevTeam_Payments,ou=Groups,dc=example,dc=com` | Developer | read, test-deploy |
| `cn=Security_Auditors,ou=Groups,dc=example,dc=com` | Auditor | read, audit-read |
| `cn=SRE_Lead,ou=Groups,dc=example,dc=com` | SuperAdmin | all |

### 9.3 Business Objectives

| Objective | KPI | Target |
|----------|-----|--------|
| Secure Access Control | Auth failure block rate | 100% |
| Developer Velocity | Time to spin up dev env | ≤ 2 min |
| Auditability | Audit trail capture | 100% |
| Operational Reliability | Auth failure downtime | < 1 min |

---

## 10. Permission Model

### 10.1 User → AD Group → System Role Mapping

```
User → AD Groups → System Role → Base Permissions → Device Tags → Access Decision
```

### 10.2 Role Permissions Matrix

| Role | View Devices | Modify Config | Execute Commands | Remote Restart |
|------|-------------|--------------|------------------|----------------|
| Auditor | ✅ | ❌ | ❌ | ❌ |
| Developer | ✅ | ❌ | ❌ | ❌ |
| Operator | ✅ | ✅ | ✅ | ❌* |
| SuperAdmin | ✅ | ✅ | ✅ | ✅ |

*Operator can restart non-production devices only

### 10.3 Label Inheritance

- **Parent Group Labels**: Automatically inherited by child groups
- **Business Group Inheritance**: Child business groups can access parent resources
- **Permission Stacking**: Child groups add permissions on top of inherited ones

### 10.4 Access Decision Flow

```
1. Authentication Check → User authenticated? AD token valid?
2. Load User Groups → Get all AD groups user belongs to
3. Map to Roles → AD groups → Internal system roles
4. Determine Base Permissions → Role-based permissions
5. Device Tag Query → Get all tags of target device
6. Business Group Match → Check if user group matches device group
7. Label Permission Verify → Check label group permission inheritance
8. Operation Permission Check → Required operation vs user permissions
```

### 10.5 K8s Multi-Cluster Management

Multi-cluster Kubernetes management via kind for testing and development.

#### Supported Operations

| Operation | Description |
|-----------|-------------|
| Cluster Lifecycle | Create, delete, list clusters |
| Health Check | Nodes status, readiness probes |
| Workload Management | Deploy, scale, delete deployments |
| Metrics Collection | CPU/memory from nodes |
| Pod Operations | Logs retrieval, exec commands |
| Cross-Cluster | Broadcast config to all clusters |

#### kind Setup Script

```bash
# Setup all clusters (cluster-1, cluster-2, cluster-3)
bash devops-toolkit/scripts/kind-setup.sh setup

# Create specific cluster
bash devops-toolkit/scripts/kind-setup.sh create <name>

# Health check
bash devops-toolkit/scripts/kind-setup.sh health <name>

# Cleanup
bash devops-toolkit/scripts/kind-setup.sh cleanup
```

#### Kubeconfig Management

- Cluster kubeconfigs stored in `~/.kube/config-<cluster-name>`
- Mixed config for multi-cluster operations: `~/.kube/config-kind-mixed`

### 10.6 Physical Host Manager

Physical hosts use **agent-based metrics collection** with **data storage separation** from status tracking.

#### Core Design: Two-Layer Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                          NODE STATUS LAYER (本地跟踪)                           │
│                                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────────────┐  │
│  │  SSH Health │    │ Agent Heartbeat│   │ Data Freshness                    │  │
│  │  Check      │    │ (被动接收)     │    │ (超过30s无数据→OFFLINE)           │  │
│  │  (主动探测)  │    │              │    │                                  │  │
│  └──────┬──────┘    └──────┬──────┘    └──────────────┬───────────────────┘  │
│         │                  │                           │                        │
│         ▼                  ▼                           ▼                        │
│    [节点真的宕机?]    [节点真的宕机?]           [节点无数据但DB正常]           │
│                                                                                  │
│  用途: 告警触发、UI状态显示                                                        │
└──────────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────────┐
│                          DATA QUERY LAYER (按需查询)                                  │
│                                                                                      │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────────────┐  │
│  │ InfluxDB     │    │ Prometheus   │    │ Query Timeout/Error               │  │
│  │ (Telegraf后端)│    │ TSDB         │    │ (返回缓存数据或标记STALE)          │  │
│  │              │    │              │    │                                  │  │
│  └──────┬──────┘    └──────┬──────┘    └──────────────┬───────────────────┘  │
│         │                  │                           │                        │
│         ▼                  ▼                           ▼                        │
│    [DB故障恢复]      [DB故障恢复]             [不触发节点OFFLINE告警]         │
│                                                                                  │
│  用途: 指标详情查询、历史数据、图表                                                  │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

#### Boundary Separation: Node Status vs Data Status

| 状态类型 | 触发条件 | 影响范围 | 是否触发离线告警 |
|----------|---------|---------|-----------------|
| **节点 OFFLINE** | 30s 内无 agent 数据 **AND** SSH 检查失败 | 告警触发、UI 显示离线 | ✅ 是 |
| **数据 STALE** | 数据库查询超时/错误 | UI 显示"数据可能过期"、使用缓存 | ❌ 否 |
| **DB 故障** | InfluxDB/Prometheus 不可达 | 回退到本地缓存数据 | ❌ 否 |
| **节点正常运行但 DB 慢** | 查询延迟 > 5s | 超时返回，但仍显示节点在线 | ❌ 否 |

**关键原则:**
- 节点 OFFLINE ≠ 数据库查询失败
- 数据库故障不导致节点被标记为 offline
- 节点被标记 offline 时，数据仍然尝试从 DB 查询（用于生成历史告警）

#### Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                        PhysicalHostManager                                     │
│                      (本地状态 + 查询代理)                                    │
├────────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   节点注册                                                                │
│       │                                                                  │
│       ▼                                                                  │
│   ┌────────────────────────────────────────────────────────────────────┐   │
│   │                    LOCAL STATUS TRACKING                         │   │
│   │  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐  │   │
│   │  │ host.state   │  │lastHeartbeat │  │ lastAgentUpdate        │  │   │
│   │  │ (online/     │  │ (SSH探测)    │  │ (数据新鲜度)            │  │   │
│   │  │  offline)    │  │              │  │                        │  │   │
│   │  └──────────────┘  └──────────────┘  └────────────────────────┘  │   │
│   └────────────────────────────────────────────────────────────────────┘   │
│                                                                               │
│   数据查询                                                                │
│       │                                                                  │
│       ▼                                                                  │
│   ┌────────────────────────────────────────────────────────────────────┐   │
│   │                    DATA QUERY LAYER                                │   │
│   │                                                                   │   │
│   │  ┌─────────────────┐    ┌─────────────────────────────────────┐   │   │
│   │  │ InfluxDB Client │    │ Prometheus Query API Client         │   │   │
│   │  │ (时序数据查询)   │    │ (范围查询、即时查询)                   │   │   │
│   │  └────────┬────────┘    └────────┬────────────────────────────┘   │   │
│   │           │                      │                                │   │
│   │           └──────────┬─────────────┘                                │   │
│   │                      │                                              │   │
│   │                      ▼                                              │   │
│   │  ┌─────────────────────────────────────────────────────────────┐   │   │
│   │  │                 LOCAL CACHE (最近N分钟)                    │   │   │
│   │  │              用于快速响应 + DB故障回退                      │   │   │
│   │  └─────────────────────────────────────────────────────────────┘   │   │
│   └────────────────────────────────────────────────────────────────────┘   │
│                                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Data Flow

1. **Agent → InfluxDB (Telegraf)**:
   ```
   Telegraf Agent → InfluxDB (时序存储)
   ```

2. **Agent → Prometheus TSDB (native)**:
   ```
   Prometheus Agent → Prometheus TSDB (原生存储)
   ```

3. **DevOps Toolkit 查询**:
   ```
   UI请求 → PhysicalHostManager → InfluxDB/Prometheus API → 返回数据
                              ↓
                         本地缓存 (快速响应)
   ```

4. **状态跟踪 (独立于数据)**:
   ```
   Telegraf Agent 推送心跳 → PhysicalHostManager 更新 lastAgentUpdate
   30s 无心跳 → 标记为 offline → 触发告警
   ```

#### Design Principles

- **Storage Decoupling**: 数据存储在专业时序数据库(Telegraf→InfluxDB, Prometheus→TSDB)
- **Status Decoupling**: 节点状态由本地跟踪，不依赖数据库可用性
- **Cache Fallback**: DB 故障时使用本地缓存，保证服务可用性
- **Elastic Scaling**: 后端无状态，可水平扩展
- **Clear Boundaries**: 节点 down 和 DB 查询问题分开处理

#### Telegraf Agent Configuration (数据写入 InfluxDB)

```toml
[agent]
  interval = "10s"
  flush_interval = "10s"

# CPU metrics
[[inputs.cpu]]
  percpu = true
  totalcpu = true

# Memory metrics
[[inputs.mem]]

# Disk metrics
[[inputs.disk]]
  ignore_fs = ["tmpfs", "devtmpfs"]

# System uptime
[[inputs.uptime]]

# Output to InfluxDB
[[outputs.influxdb]]
  urls = ["http://influxdb:8086"]
  database = "telegraf"
  retention_policy = "autogen"
  timeout = "5s"
```

#### Prometheus Configuration (数据写入 Prometheus TSDB)

```yaml
# Prometheus agent (安装在节点上)
remote_write:
  - url: http://prometheus:9090/api/v1/write
    remote_timeout: 30s
    queue_config:
      capacity: 10000
      max_shards: 5
      min_shards: 1
```

#### Local Cache Strategy

| 场景 | 行为 |
|------|------|
| 正常情况 | 查询 DB，返回最新数据，更新本地缓存 |
| DB 故障 | 返回本地缓存数据，标记 `dataStatus: 'stale'` |
| 节点 offline | 仍然尝试查询 DB（用于历史分析），本地状态标记 offline |
| 缓存为空且 DB 不可用 | 返回 `dataStatus: 'unavailable'` |

#### REST API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/physical-hosts` | GET | List all physical hosts (本地状态) |
| `/api/physical-hosts` | POST | Register a physical host |
| `/api/physical-hosts/:id` | GET | Get physical host details |
| `/api/physical-hosts/:id` | DELETE | Remove physical host |
| `/api/physical-hosts/:id/metrics` | GET | Query metrics from DB (InfluxDB/Prometheus) |
| `/api/physical-hosts/:id/heartbeat` | POST | Trigger SSH heartbeat check |
| `/api/physical-hosts/summary` | GET | Get host summary |
| `/api/physical-hosts/cache` | GET | Get local cache status |

#### Physical Host Data Model

```javascript
{
  id: "host-1",
  hostname: "prod-web-server-01",
  ip: "192.168.1.1",
  port: 22,
  state: "online",           // 本地状态: online | offline
  lastHeartbeat: null,      // SSH探测时间
  lastAgentUpdate: "2026-04-18T06:30:00Z",  // 数据新鲜度
  dataStatus: "fresh",       // 数据状态: fresh | stale | unavailable
  metrics: {                // 本地缓存 (最近数据)
    cpu: { usage: 45.5, cores: 8 },
    memory: { total: 16384, used: 8192, usagePercent: 50 },
    disk: { disks: [...] },
    uptime: { value: 900000, formatted: "10d 10h 0m" }
  },
  registeredAt: "2026-04-18T00:00:00Z",
  metadata: {
    influxdbHost: "influxdb:8086",
    prometheusJob: "physical-hosts"
  }
}
```

#### Supported Operations

| Operation | Description |
|-----------|-------------|
| Host Management | Register, remove, list physical hosts |
| SSH Health Check | 主动探测节点连通性 |
| Data Freshness Check | 基于 lastAgentUpdate 判断节点状态 |
| Metrics Query | 从 InfluxDB/Prometheus 查询历史数据 |
| Local Cache | 快速响应 + DB故障回退 |
| Agent Installation | SSH 推送 Telegraf/Prometheus Agent 配置 |

#### Test Environment

```
test-physical-hosts/
├── docker-compose.yml           # InfluxDB + Prometheus + Grafana
├── telegraf.conf.example        # Telegraf 配置模板
├── prometheus.yml.example       # Prometheus 配置模板
└── mock_agent.js                # 模拟 Telegraf agent 用于测试
```

**Docker Compose 包含:**
- InfluxDB (端口 8086) — Telegraf 后端存储
- Prometheus (端口 9090) — 时序数据库 + Web UI
- Grafana (端口 3001) — 指标可视化
- Mock Agent — 模拟 Telegraf 行为用于测试

**测试场景:**
1. Agent → InfluxDB 数据写入 + 查询
2. Agent → Prometheus remote_write + 查询
3. DB 故障时本地缓存回退
4. 节点 offline 状态判断
5. dataStatus: fresh/stale/unavailable 状态转换

---

## 11. Architecture Overview

### 11.1 Core Principles

- **Modularity**: Loosely coupled components with well-defined interfaces
- **Scalability**: Horizontal scaling for all components
- **Resilience**: Fault tolerance and graceful degradation
- **Security**: Defense-in-depth with auth, authorization, encryption
- **Observability**: Comprehensive logging, monitoring, tracing

### 11.2 Technology Stack

| Layer | Technology |
|-------|------------|
| Backend | Node.js/Go microservices with gRPC/REST APIs |
| Frontend | React SPA |
| Database | PostgreSQL (relational), Redis (cache) |
| Message Queue | Apache Kafka/RabbitMQ |
| Container Orchestration | Kubernetes |
| Infrastructure | Terraform, Docker |

---

## 11.5 Test Environment

完整的模拟生产环境测试套件，用于发布后体验和开发验证。基于 Containerlab 实现双数据中心高可用拓扑。

### 11.5.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    Containerlab Network                                      │
│                                       172.30.30.0/24                                        │
│                                                                                             │
│  ┌─────────────────────────────┐                           ┌─────────────────────────────┐│
│  │        DC1 (Left)          │                           │       DC2 (Right)          ││
│  │       10.0.1.0/24          │                           │       10.0.2.0/24          ││
│  │                             │                           │                             ││
│  │      dc1-sw1 ──────────────╫═══════════════════════════╫───────────── dc2-sw1       ││
│  │       (Core)               ╫══════ Dual Trunk ══════════╫══        (Core)             ││
│  │          │                 ╫═══════════════════════════╫══          │                ││
│  │      dc1-sw2               ╫═══════════════════════════╫══      dc2-sw2               ││
│  │    (Distribution)           ╫═══════════════════════════╫══   (Distribution)        ││
│  │       │  │                  ╫═══════════════════════════╫══        │  │               ││
│  │       │  │                  ╫═══════════════════════════╫══        │  │               ││
│  │    ┌──┴──┐                ╫═══════════════════════════╫══     ┌──┴──┐               ││
│  │    │     │                ╫═══════════════════════════╫══     │     │               ││
│  │    │W    │D                ╫═══════════════════════════╫══     │W    │D               ││
│  │    │eb   │b                ╫═══════════════════════════╫══    │eb   │b               ││
│  │    │1    │1                ╫═══════════════════════════╫══    │2    │2               ││
│  │    └─────┘                ╫═══════════════════════════╫══    └─────┘               ││
│  └─────────────────────────────╫════════════════════════════════════════─────────────────┘│
│                                  │                                                           │
│                                  │          ┌───────────────────────────────────────┐      │
│                                  └──────────│        Time Series Databases          │      │
│                                             │  InfluxDB :8086  Prometheus :9090    │      │
│                                             │  Grafana  :3001                       │      │
│                                             └───────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

W = Web Server (Ubuntu + SSH running)
D = DB Server (Ubuntu + SSH running)
sw = Switch (Ubuntu + SNMP daemon running)
```

### 11.5.2 Node Inventory

| Node | Role | MGMT IP | Services | Ports |
|------|------|---------|----------|-------|
| dc1-sw1 | DC1 Core Switch | 172.30.30.11 | SNMP | UDP:161 |
| dc1-sw2 | DC1 Distribution | 172.30.30.12 | SNMP | UDP:161 |
| dc1-web | DC1 Web Server | 172.30.30.21 | SSH | TCP:22 |
| dc1-db | DC1 DB Server | 172.30.30.22 | SSH | TCP:22 |
| dc2-sw1 | DC2 Core Switch | 172.30.30.31 | SNMP | UDP:161 |
| dc2-sw2 | DC2 Distribution | 172.30.30.32 | SNMP | UDP:161 |
| dc2-web | DC2 Web Server | 172.30.30.41 | SSH | TCP:22 |
| dc2-db | DC2 DB Server | 172.30.30.42 | SSH | TCP:22 |

### 11.5.3 High Availability Features

1. **Dual Datacenters** - DC1 and DC2 deployed independently
2. **Dual Trunk Links** - Two trunk connections (eth2, eth3) between dc1-sw1 and dc2-sw1
3. **Switch Architecture** - Each DC has core switch + distribution switch
4. **SSH Services** - Each server runs real SSH daemon

### 11.5.4 Device Simulation Matrix

| Device Type | Simulation Method | Connection | Port | Purpose |
|-------------|-----------------|-----------|------|---------|
| PhysicalHost | Docker + openssh-server | SSH 真实连接 | TCP:22 | 指标采集、配置下发 |
| NetworkDevice | Docker + net-snmpd | SNMP 真实连接 | UDP:161 | 流量监控、端口状态 |
| Container | Docker native | Docker API | - | K8s 集群管理 |

### 11.5.5 Directory Structure

```
test-environment/
├── clab/                          # Containerlab 双数据中心拓扑
│   ├── topology.yml               # Containerlab 拓扑定义
│   ├── clab.sh                    # 管理脚本 (deploy/destroy/status/test)
│   ├── install.sh                 # Containerlab 安装脚本
│   ├── README.md                   # 详细文档
│   └── configs/
│       └── switch/
│           └── snmpd.conf         # 交换机 SNMP 配置
│
├── docker-compose.yml             # Docker 测试环境编排
├── Dockerfile.ssh-host            # SSH 物理主机镜像
├── Dockerfile.snmp-device         # SNMP 网络设备镜像
│
├── config/
│   ├── ssh/
│   │   └── authorized_keys        # SSH 授权密钥
│   ├── snmp/
│   │   ├── snmpd.conf            # SNMP 守护进程配置
│   │   └── snmpwalk.txt          # MIB 数据模板
│   └── haproxy/
│       └── haproxy.cfg           # HAProxy 配置
│
├── scripts/
│   ├── setup.sh                   # 环境初始化
│   ├── verify.sh                  # 连接验证
│   ├── register-devices.sh        # 自动注册设备到系统
│   └── cleanup.sh                 # 清理环境
│
└── docs/
    ├── ARCHITECTURE.md            # 测试环境架构说明
    └── SIMULATED_DEVICES.md       # 模拟设备详细说明
```

### 11.5.6 Containerlab Deployment

```bash
# 安装 Containerlab
cd /mnt/devops/devops-toolkit/test-environment/clab
sudo ./install.sh

# 部署拓扑
sudo ./clab.sh deploy

# 查看状态
sudo clab inspect -t topology.yml

# 测试连接
./clab.sh test

# 查看日志
./clab.sh logs dc1-sw1

# 销毁拓扑
sudo ./clab.sh destroy
```

### 11.5.7 Connection Verification

```bash
# SSH 连接验证
docker exec clab-devops-dc-ha-dc1-web hostname
docker exec clab-devops-dc-ha-dc2-web hostname

# SNMP 连接验证
docker exec clab-devops-dc-ha-dc1-sw1 snmpstatus -v 2c -c public localhost
docker exec clab-devops-dc-ha-dc2-sw1 snmpwalk -v 2c -c public localhost

# 容器间连通性
docker exec clab-devops-dc-ha-dc1-web ping -c 1 172.30.30.31
```

### 11.5.8 Simulated Metrics

**Physical Host (via SSH)**:
- CPU: 使用率、核心数、user/system/idle
- Memory: total/free/used/percent
- Disk: 设备、大小、已用空间、使用率
- Uptime: 运行时间
- Services: sshd, cron 等服务状态

**Network Device (via SNMP)**:
- ifNumber: 接口数量
- ifDescr: 接口描述
- ifSpeed: 接口速度
- ifOperStatus: 接口状态 (up/down)
- ifInOctets/ifOutOctets: 流量统计
- sysUpTime: 设备运行时间
- sysDescr: 系统描述
- sysContact/sysLocation: 联系人和位置信息

### 11.5.9 Device Auto-Registration

Devices in the test environment automatically register into the DevOps Toolkit using **pull-based discovery**. The DevOps Toolkit actively probes the network to discover devices, rather than devices pushing registration requests.

#### Discovery Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                         DevOps Toolkit (localhost:3000)                              │
│                                                                                     │
│  ┌─────────────────────┐    ┌──────────────────────┐    ┌──────────────────────┐ │
│  │  NetworkDiscovery   │───▶│   DeviceManager      │───▶│   WebSocket         │ │
│  │  (172.30.30.0/24)  │    │   (PENDING state)    │    │   (user notification)│ │
│  └─────────────────────┘    └──────────────────────┘    └──────────────────────┘ │
│           │                                                                           │
│           │ SNMP/SSH probes                                                           │
│           ▼                                                                           │
│  ┌─────────────────────┐                                                            │
│  │  Containerlab      │                                                            │
│  │  Network           │                                                            │
│  │  172.30.30.0/24    │                                                            │
│  └─────────────────────┘                                                            │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

#### Discovery Flow

1. **Scan Trigger**: User calls `POST /api/discovery/scan` or automated periodic scan
2. **Network Sweep**: NetworkDiscovery probes 172.30.30.0/24:
   - TCP port 22 (SSH) → discovered as `physical_host`
   - UDP port 161 (SNMP) → discovered as `network_device`
3. **Device Creation**: Discovered devices created in `PENDING` state via DeviceManager
4. **User Approval**: User reviews pending devices and calls `POST /api/discovery/register`
5. **State Transition**: PENDING → AUTHENTICATED → REGISTERED → ACTIVE

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/discovery/scan` | POST | Trigger network discovery scan |
| `/api/discovery/status` | GET | Get last scan status |
| `/api/discovery/register` | POST | Register discovered devices |

#### Discovery Manager

```javascript
// discovery/network_discovery.js
const discovery = new NetworkDiscovery({
  network: '172.30.30.0/24',  // Target network
  timeout: 2000               // 2s per host
});

// Scan network
const result = await discovery.scan();
// → { switches: [...], servers: [...] }

// Register devices
const devices = discovery.getDiscoveredDevices();
// → [{ id: 'clab-dc1-web-21', type: 'physical_host', ... }, ...]
```

#### Device ID Naming Convention

| Device | ID | Type | Datacenter |
|--------|-----|------|------------|
| DC1 Web Server | `clab-dc1-web-21` | physical_host | dc1 |
| DC1 DB Server | `clab-dc1-db-22` | physical_host | dc1 |
| DC1 Core Switch | `clab-dc1-core-11` | network_device | dc1 |
| DC2 Web Server | `clab-dc2-web-41` | physical_host | dc2 |
| DC2 Core Switch | `clab-dc2-core-31` | network_device | dc2 |

#### Testing Discovery

```bash
# Run discovery test script
cd /mnt/devops/devops-toolkit
node scripts/discovery-test.js

# Or use API directly
curl -X POST http://localhost:3000/api/discovery/scan
curl -X POST http://localhost:3000/api/discovery/register
```

---

## 11.3 Data Flow

**CI/CD Workflow**:
1. Developer pushes code
2. Webhook triggers CI pipeline
3. Build environment compiles and tests
4. Artifacts stored
5. Deployment pipeline promotes to staging/production
6. Monitoring validates deployment

**Device Management Flow**:
1. Device registers with management service
2. Configuration pushed from central repository
3. Device reports status and metrics
4. Alerts generated for anomalous behavior
5. Bulk operations via device groups

---

## 12. API Design

### 12.1 RESTful Endpoints

#### Device API (`/api/devices`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/devices` | GET | List all devices |
| `/api/devices` | POST | Register device |
| `/api/devices/:id` | GET | Get device details |
| `/api/devices/:id` | PUT | Update device |
| `/api/devices/:id` | DELETE | Remove device |
| `/api/devices/:id/metrics` | GET | Get device metrics |
| `/api/devices/:id/events` | GET | Get device events |
| `/api/devices/:id/actions` | POST | Execute action |
| `/api/devices/search` | GET | Search by tags |

#### Pipeline API (`/api/pipelines`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/pipelines` | GET | List all pipelines |
| `/api/pipelines` | POST | Create pipeline |
| `/api/pipelines/:id` | GET | Get pipeline |
| `/api/pipelines/:id` | PUT | Update pipeline |
| `/api/pipelines/:id` | DELETE | Delete pipeline |
| `/api/pipelines/:id/execute` | POST | Execute pipeline |
| `/api/pipelines/:id/runs` | GET | Get runs |
| `/api/runs` | GET | Get all recent runs |

#### Log API (`/api/logs`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/logs` | GET | Query logs |
| `/api/logs` | POST | Add log entry |
| `/api/logs/generate` | POST | Generate sample logs |
| `/api/logs/stats` | GET | Get statistics |
| `/api/logs/backend` | GET | Backend health |
| `/api/logs/retention` | GET/PUT | Retention config |
| `/api/logs/alerts` | GET/POST | Alert rules |

#### Metrics API (`/api/metrics`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/metrics` | GET | Get all metrics (JSON) |
| `/api/metrics/counter` | POST | Increment counter |
| `/api/metrics/gauge` | POST | Set gauge |
| `/api/metrics/histogram` | POST | Observe value |

#### Alert API (`/api/alerts`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/alerts/channels` | GET/POST | Notification channels |
| `/api/alerts/channels/:name` | DELETE | Remove channel |
| `/api/alerts/history` | GET | Alert history |
| `/api/alerts/stats` | GET | Alert statistics |
| `/api/alerts/trigger` | POST | Trigger alert |

#### Discovery API (`/api/discovery`)
| Endpoint | Method | Description |
|---------|--------|-------------|
| `/api/discovery/scan` | POST | Trigger network discovery scan |
| `/api/discovery/status` | GET | Get last scan status |
| `/api/discovery/register` | POST | Register discovered devices |

### 12.2 WebSocket

| Endpoint | Description |
|---------|-------------|
| `/ws` | WebSocket server for real-time events |

### 12.3 Prometheus

| Endpoint | Description |
|---------|-------------|
| `/metrics` | Prometheus text format metrics |

### 12.4 Health

| Endpoint | Description |
|---------|-------------|
| `/health` | Server health check |

---

## Appendix A: Docker Development Environment

```bash
# Start full infrastructure
docker-compose -f docker-compose.dev.yml up -d

# Service ports
# - Elasticsearch: http://localhost:9200
# - Kibana:       http://localhost:5601
# - Loki:         http://localhost:3100
# - Grafana:      http://localhost:3001
# - Prometheus:   http://localhost:9090
# - DevOps Toolkit: http://localhost:3000
```

---

## Appendix B: Test Coverage

### Test Suite Summary

| Metric | Value |
|--------|-------|
| **Total Tests** | 315 |
| **Test Files** | 15 |
| **Statement Coverage** | 30.12% |
| **Branch Coverage** | 22.57% |
| **Function Coverage** | 37.64% |
| **Line Coverage** | 31.11% |
| **Coverage Threshold** | 50% |

### Test Files

| Test File | Tests | Coverage Area |
|-----------|-------|---------------|
| `device_state_machine.test.js` | 19 | Device state transitions |
| `device_manager.test.js` | 16 | Device CRUD operations |
| `permission_middleware.test.js` | 21 | RBAC enforcement |
| `ldap_auth.test.js` | 24 | LDAP configuration |
| `auth_integration.test.js` | 24 | Authentication boundaries |
| `pipeline_manager.test.js` | 26 | CI/CD stage execution |
| `ldap_retry.test.js` | 20 | LDAP retry logic |
| `metrics_manager.test.js` | 23 | Prometheus metrics |
| `alerts_notification_manager.test.js` | 21 | Alert channels/rate limiting |
| `websocket_manager.test.js` | 17 | WebSocket events |
| `agent.test.js` | 21 | Device agent |
| `storage_backends.test.js` | 12 | Local/ES/Loki backends |
| `k8s_cluster_manager.test.js` | 25 | K8s cluster management |
| `k8s_multi_cluster.test.js` | 20 | Multi-cluster operations |
| `physical_host_manager.test.js` | 33 | Physical host SSH management |

### Coverage Gaps

80% coverage target is deferred. Requires extensive integration tests for:
- `server.js` (718 lines) - main HTTP server, needs full integration tests
- `log_manager.js` (509 lines) - log aggregation, needs backend mocking
- `main.js` - application entry point

---

**Document Status**: Active
**Owner**: DevOps Team
