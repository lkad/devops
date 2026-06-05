# 测试环境准备清单

**版本:** 1.0
**最后更新:** 2026-06-05
**基于:** [PRD.md §14](../PRD.md), [openspec/specs/test-environment/spec.md](../openspec/specs/test-environment/spec.md)

> **目的:** 重新开发时，一份可执行的测试环境准备手册。从空白机器到完整可用的三层测试环境，所有需要的文件、脚本、数据、配置。

---

## 0. 测试环境总览

```
┌─────────────────────────────────────────────────────────────┐
│  生产 (Production)        — 真实硬件，禁用                    │
├─────────────────────────────────────────────────────────────┤
│  集成 (Containerlab)      — 模拟双 DC，CI 用                  │
├─────────────────────────────────────────────────────────────┤
│  开发 (Local + Mock)      — 无依赖，单机，秒启                │
└─────────────────────────────────────────────────────────────┘
```

| 层级 | 启动时间 | 资源占用 | 真实性 | 用途 |
|------|---------|---------|--------|------|
| 开发 | < 30s | < 200MB | 低（Mock）| 单元测试、快速迭代 |
| 集成 | 2-5min | 2-4GB | 中（Containerlab）| E2E、预发验证、CI |
| 生产 | 不可用 | 真实硬件 | 高 | 真实流量 |

---

## 1. 需要准备的内容（总览）

| 类别 | 数量 | 路径 | 优先级 |
|------|------|------|--------|
| 1. 基础设施配置 | 3 个文件 | `deploy/` | P0 |
| 2. 编排脚本 | 11 个脚本 | `scripts/` | P0 |
| 3. 配置文件模板 | 4 个 | `configs/templates/` | P0 |
| 4. Mock/种子数据 | 6 套 | `tests/fixtures/` | P0 |
| 5. 验证脚本 | 3 个 | `scripts/verify/` | P1 |
| 6. CI 工作流 | 2 个 | `.github/workflows/` | P1 |
| 7. 故障排查手册 | 1 份 | `docs/TROUBLESHOOTING.md` | P2 |

**总计: ~30 个文件**

---

## 2. 基础设施配置（3 个文件）

### 2.1 Containerlab 拓扑

**`deploy/containerlab/topology.yml`**

```yaml
name: devops-toolkit-test
prefix: clab
mgmt:
  network: devops-mgmt
  ipv4-subnet: 172.30.30.0/24

topology:
  kinds:
    linux:
      cmd: /bin/bash
  nodes:
    # ===== DC1: 4 节点 =====
    dc1-core-switch:
      kind: linux
      image: network-multitool:latest
      labels:
        role: network_device
        dc: dc1
        env: test
      env:
        SNMP_COMMUNITY: devops-test
    dc1-web-21:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: physical_host
        dc: dc1
        env: test
    dc1-db-22:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: physical_host
        dc: dc1
        env: test
    dc1-app-23:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: container
        dc: dc1
        env: test

    # ===== DC2: 4 节点 =====
    dc2-core-switch:
      kind: linux
      image: network-multitool:latest
      labels:
        role: network_device
        dc: dc2
        env: test
    dc2-web-41:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: physical_host
        dc: dc2
        env: test
    dc2-db-42:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: physical_host
        dc: dc2
        env: test
    dc2-app-43:
      kind: linux
      image: linuxserver/openssh-server:latest
      labels:
        role: container
        dc: dc2
        env: test

  links:
    # DC1 trunk
    - endpoints: ["dc1-core-switch:eth1", "dc1-web-21:eth1"]
    - endpoints: ["dc1-core-switch:eth2", "dc1-db-22:eth1"]
    - endpoints: ["dc1-core-switch:eth3", "dc1-app-23:eth1"]
    # DC2 trunk
    - endpoints: ["dc2-core-switch:eth1", "dc2-web-41:eth1"]
    - endpoints: ["dc2-core-switch:eth2", "dc2-db-42:eth1"]
    - endpoints: ["dc2-core-switch:eth3", "dc2-app-43:eth1"]
    # DC1-DC2 trunk
    - endpoints: ["dc1-core-switch:eth4", "dc2-core-switch:eth4"]
```

### 2.2 Docker Compose 支持服务

**`deploy/docker-compose.yml`** — PostgreSQL, Loki, ES, Prometheus, Grafana, InfluxDB

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: devops
      POSTGRES_USER: devops
      POSTGRES_PASSWORD: devops
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "devops"]
      interval: 5s
      timeout: 5s
      retries: 5

  loki:
    image: grafana/loki:2.8.0
    command: -config.file=/etc/loki/loki-config.yaml
    ports: ["3100:3100"]
    volumes: ["./loki-config.yaml:/etc/loki/loki-config.yaml"]

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      discovery.type: single-node
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
      xpack.security.enabled: "false"
    ports: ["9200:9200"]
    volumes: ["esdata:/usr/share/elasticsearch/data"]

  prometheus:
    image: prom/prometheus:latest
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --storage.tsdb.retention.time=7d
    ports: ["9090:9090"]
    volumes: ["./prometheus.yml:/etc/prometheus/prometheus.yml"]

  grafana:
    image: grafana/grafana:latest
    ports: ["3001:3000"]
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    depends_on: [prometheus]

  influxdb:
    image: influxdb:2.7
    ports: ["8086:8086"]
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
      DOCKER_INFLUXDB_INIT_USERNAME: admin
      DOCKER_INFLUXDB_INIT_PASSWORD: admin-devops
      DOCKER_INFLUXDB_INIT_ORG: devops
      DOCKER_INFLUXDB_INIT_BUCKET: metrics
    volumes: ["influxdata:/var/lib/influxdb2"]

  ldap:
    image: bitnami/openldap:2.6
    environment:
      LDAP_ROOT: dc=example,dc=com
      LDAP_ADMIN_USERNAME: admin
      LDAP_ADMIN_PASSWORD: admin
    ports: ["389:1389"]
    volumes: ["./bootstrap.ldif:/ldifs/bootstrap.ldif:ro"]

volumes:
  pgdata: {}
  esdata: {}
  influxdata: {}
```

### 2.3 k3d 集群配置

**`deploy/k3d/cluster.yaml`**

```yaml
apiVersion: k3d.io/v1alpha4
kind: Simple
metadata:
  name: dev-cluster-1
servers: 1
agents: 2
image: rancher/k3s:v1.28.4-k3s2
ports:
  - port: 6550:6550  # K8s API
    nodeFilters: [loadbalancer]
options:
  k3s:
    extraArgs:
      - arg: --disable=traefik
        nodeFilters: [server:0:0]
```

---

## 3. 编排脚本（11 个）

放在 `scripts/` 目录。

### 3.1 顶层脚本

| 脚本 | 用途 | 入口 |
|------|------|------|
| `setup.sh` | 一键启动完整集成环境 | dev / ci |
| `teardown.sh` | 一键清理所有资源 | dev / ci |
| `reset.sh` | 清数据但保留服务 | dev |
| `status.sh` | 显示所有组件健康状态 | dev / ops |

### 3.2 组件脚本

| 脚本 | 用途 | 依赖 |
|------|------|------|
| `clab.sh` | Containerlab 部署/销毁 | Docker |
| `k3d-setup.sh` | k3d 集群创建/删除 | Docker |
| `db-setup.sh` | PostgreSQL 初始化 + AutoMigrate | postgres 容器 |
| `ldap-seed.sh` | LDAP 种子用户/组导入 | ldap 容器 |
| `seed-data.sh` | 业务种子数据（设备/项目/告警）| db |
| `verify.sh` | 连接性验证 | 全部 |

### 3.3 脚本接口规范

所有脚本必须实现：

```bash
# 标准接口
./script.sh deploy    # 部署
./script.sh destroy   # 销毁
./script.sh status    # 状态
./script.sh logs      # 查看日志
./script.sh help      # 帮助
```

**通用约束：**
- 使用 `set -euo pipefail`
- 颜色输出（green=ok, red=fail, yellow=warn, blue=info）
- 每个动作前显示 `[STEP] xxx`
- 失败时退出码非 0，错误信息到 stderr
- 日志同时写到 `./logs/script-name.log`

### 3.4 脚本示例

**`scripts/setup.sh`**

```bash
#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/log.sh"

main() {
  local env="${1:-dev}"
  
  log_step "Starting setup for env: $env"
  
  case "$env" in
    dev)
      # 仅启动 mock 模式
      log_step "Starting dev environment (mock only)"
      "$SCRIPT_DIR/db-setup.sh" deploy
      "$SCRIPT_DIR/seed-data.sh" minimal
      ;;
    ci)
      # 完整集成环境
      log_step "Starting CI environment (full stack)"
      "$SCRIPT_DIR/db-setup.sh" deploy
      "$SCRIPT_DIR/clab.sh" deploy
      "$SCRIPT_DIR/k3d-setup.sh" deploy
      "$SCRIPT_DIR/ldap-seed.sh" apply
      "$SCRIPT_DIR/seed-data.sh" full
      "$SCRIPT_DIR/verify.sh" full
      ;;
    prod)
      log_error "Production setup is disabled in this script"
      exit 1
      ;;
    *)
      log_error "Unknown env: $env"
      exit 1
      ;;
  esac
  
  log_ok "Setup complete for env: $env"
  log_info "Run: $SCRIPT_DIR/status.sh"
}

main "$@"
```

---

## 4. 配置文件模板（4 个）

放在 `configs/templates/`，每层环境一个。

### 4.1 开发环境 — `config-dev.yaml`

```yaml
environment: dev
server:
  host: "0.0.0.0"
  port: 3000
database:
  host: "localhost"
  port: 5432
  user: "devops"
  password: "devops"
  name: "devops_dev"
ldap:
  url: "ldap://localhost:389"
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_password: "admin"
auth:
  dev_bypass: true  # ⚠️ 仅开发环境
  jwt_secret: "dev-secret-do-not-use-in-prod"
  jwt_expiry: 86400
logs:
  backend: "local"
use_mock:
  vmware: true
  snmp: true
  ssh: true
seed_data: "minimal"
```

### 4.2 集成环境 — `config-ci.yaml`

```yaml
environment: ci
server:
  host: "0.0.0.0"
  port: 3000
database:
  url: "postgres://devops:devops@postgres:5432/devops_ci"
ldap:
  url: "ldap://ldap:389"
  base_dn: "dc=example,dc=com"
auth:
  dev_bypass: false
  jwt_secret: "ci-secret-rotate-me"
logs:
  backend: "loki"
  loki_url: "http://loki:3100"
use_mock:
  vmware: false
  snmp: false
  ssh: false
containerlab:
  topology_file: "/workspace/deploy/containerlab/topology.yml"
  kubeconfig_path: "/root/.kube/config"
seed_data: "full"
```

### 4.3 生产环境 — `config-prod.yaml`（禁用模板）

```yaml
# ⚠️ 此为模板，生产部署必须：
# 1. 移除所有硬编码密码
# 2. 从 Vault/SOPS 注入密钥
# 3. 启用 mTLS
environment: production
server:
  host: "0.0.0.0"
  port: 3000
  tls:
    enabled: true
    cert_file: "/etc/devops-toolkit/tls/cert.pem"
    key_file: "/etc/devops-toolkit/tls/key.pem"
database:
  url: "${DATABASE_URL}"  # 必填环境变量
ldap:
  url: "${LDAP_URL}"
  base_dn: "${LDAP_BASE_DN}"
auth:
  dev_bypass: false
  jwt_secret: "${JWT_SECRET}"  # 必填
  jwt_expiry: 3600
logs:
  backend: "elasticsearch"
  elasticsearch_url: "${ELASTICSEARCH_URL}"
use_mock: {}  # 空
seed_data: "none"
audit:
  enabled: true
  retention_days: 365
```

### 4.4 配置加载逻辑

```go
// 启动时根据 ENV 变量选择配置
func LoadConfig() (*Config, error) {
    env := os.Getenv("ENV")
    if env == "" {
        env = "dev"
    }
    
    configPath := filepath.Join("configs", fmt.Sprintf("config-%s.yaml", env))
    if _, err := os.Stat(configPath); err != nil {
        return nil, fmt.Errorf("config for env=%s not found: %w", env, err)
    }
    
    // 加载 + 环境变量覆盖
    cfg, err := loadFromYAML(configPath)
    if err != nil {
        return nil, err
    }
    overrideFromEnv(cfg)
    
    // 校验
    if env == "production" {
        if err := validateProductionConfig(cfg); err != nil {
            return nil, fmt.Errorf("production config invalid: %w", err)
        }
    }
    return cfg, nil
}
```

---

## 5. Mock/种子数据（6 套）

放在 `tests/fixtures/`。

### 5.1 目录结构

```
tests/fixtures/
├── ldap/
│   ├── users.ldif             # 10 用户
│   ├── groups.ldif            # 5 组（覆盖 4 角色）
│   └── bootstrap.ldif         # 入口
├── db/
│   ├── seed-dev.sql           # 开发用最小数据集
│   ├── seed-ci.sql            # CI 完整数据集
│   └── migrations/            # GORM AutoMigrate 顺序
├── devices/
│   ├── 8-clab-nodes.json      # 8 个 Containerlab 节点
│   ├── fake-vmware.json       # vSphere Mock 数据
│   └── fake-snmp.json         # SNMP Mock 数据
├── logs/
│   ├── sample-100.json        # 100 条样例日志
│   └── alerts-rules.json      # 5 条告警规则
├── metrics/
│   └── prometheus-scrape.yml  # 采集配置
└── projects/
    ├── business-lines.json    # 3 个 BL
    ├── systems.json           # 6 个 System
    └── projects.json          # 12 个 Project
```

### 5.2 LDAP 种子数据

**`tests/fixtures/ldap/users.ldif`**（节选）

```ldif
dn: uid=alice,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: alice
cn: Alice Operator
sn: Operator
mail: alice@example.com
userPassword: alice123

dn: uid=bob,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: bob
cn: Bob Developer
sn: Developer
mail: bob@example.com
userPassword: bob123

dn: uid=carol,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: carol
cn: Carol Auditor
sn: Auditor
mail: carol@example.com
userPassword: carol123

dn: uid=dave,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
uid: dave
cn: Dave SuperAdmin
sn: SuperAdmin
mail: dave@example.com
userPassword: dave123
```

**`tests/fixtures/ldap/groups.ldif`**

```ldif
dn: cn=sre-lead,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: sre-lead
member: uid=dave,ou=users,dc=example,dc=com

dn: cn=dev-team,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: dev-team
member: uid=alice,ou=users,dc=example,dc=com
member: uid=bob,ou=users,dc=example,dc=com

dn: cn=audit,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: audit
member: uid=carol,ou=users,dc=example,dc=com
```

### 5.3 设备种子数据

**`tests/fixtures/devices/8-clab-nodes.json`**

```json
[
  {"id": "clab-dc1-web-21", "name": "dc1-web-21", "type": "physical_host", "ip": "172.30.30.21", "dc": "dc1", "ssh_port": 22, "ssh_user": "root", "ssh_password": "root"},
  {"id": "clab-dc1-db-22", "name": "dc1-db-22", "type": "physical_host", "ip": "172.30.30.22", "dc": "dc1", "ssh_port": 22},
  {"id": "clab-dc1-app-23", "name": "dc1-app-23", "type": "container", "ip": "172.30.30.23", "dc": "dc1"},
  {"id": "clab-dc1-core-11", "name": "dc1-core-switch", "type": "network_device", "ip": "172.30.30.11", "dc": "dc1", "snmp_community": "devops-test", "snmp_port": 161},
  {"id": "clab-dc2-web-41", "name": "dc2-web-41", "type": "physical_host", "ip": "172.30.30.41", "dc": "dc2", "ssh_port": 22},
  {"id": "clab-dc2-db-42", "name": "dc2-db-42", "type": "physical_host", "ip": "172.30.30.42", "dc": "dc2", "ssh_port": 22},
  {"id": "clab-dc2-app-43", "name": "dc2-app-43", "type": "container", "ip": "172.30.30.43", "dc": "dc2"},
  {"id": "clab-dc2-core-31", "name": "dc2-core-switch", "type": "network_device", "ip": "172.30.30.31", "dc": "dc2", "snmp_community": "devops-test", "snmp_port": 161}
]
```

### 5.4 项目种子数据

**`tests/fixtures/projects/business-lines.json`**

```json
[
  {"id": "bl-ecommerce", "name": "电商事业部", "description": "电商核心业务", "weight": 0.6},
  {"id": "bl-finance", "name": "金融事业部", "description": "支付与对账", "weight": 0.3},
  {"id": "bl-platform", "name": "基础平台部", "description": "中间件与基础设施", "weight": 0.1}
]
```

**`tests/fixtures/projects/systems.json`**

```json
[
  {"id": "sys-order", "name": "订单系统", "business_line_id": "bl-ecommerce"},
  {"id": "sys-payment", "name": "支付系统", "business_line_id": "bl-finance"},
  {"id": "sys-mq", "name": "消息队列", "business_line_id": "bl-platform"},
  {"id": "sys-k8s", "name": "K8s平台", "business_line_id": "bl-platform"},
  {"id": "sys-monitor", "name": "监控系统", "business_line_id": "bl-platform"},
  {"id": "sys-cdn", "name": "CDN", "business_line_id": "bl-ecommerce"}
]
```

**`tests/fixtures/projects/projects.json`**

```json
[
  {"id": "proj-order-backend", "name": "order-backend", "system_id": "sys-order", "type_id": "pt-backend"},
  {"id": "proj-order-frontend", "name": "order-frontend", "system_id": "sys-order", "type_id": "pt-frontend"},
  {"id": "proj-payment-gateway", "name": "payment-gateway", "system_id": "sys-payment", "type_id": "pt-backend"},
  {"id": "proj-mq-kafka", "name": "mq-kafka", "system_id": "sys-mq", "type_id": "pt-backend"},
  {"id": "proj-k8s-prod", "name": "k8s-prod", "system_id": "sys-k8s", "type_id": "pt-backend"},
  {"id": "proj-monitor-prom", "name": "monitor-prometheus", "system_id": "sys-monitor", "type_id": "pt-backend"}
]
```

### 5.5 告警规则种子

**`tests/fixtures/logs/alerts-rules.json`**

```json
[
  {"name": "high-error-rate", "condition": "level=error", "window": "5m", "threshold": 10, "channel": "slack-ops"},
  {"name": "host-offline", "condition": "source=device_event AND state=offline", "window": "1m", "threshold": 1, "channel": "webhook-oncall"},
  {"name": "pipeline-failed", "condition": "source=pipeline_update AND status=failed", "window": "1m", "threshold": 1, "channel": "email-leads"},
  {"name": "k8s-pod-restart", "condition": "source=k8s_event AND reason=Restart", "window": "10m", "threshold": 3, "channel": "log"},
  {"name": "host-in-maintenance", "condition": "source=device_event AND state=maintenance", "window": "0", "threshold": 1, "channel": "log", "suppress_external": true}
]
```

### 5.6 日志种子

**`tests/fixtures/logs/sample-100.json`**（节选）

```json
[
  {"timestamp": "2026-06-05T10:00:00Z", "level": "info", "source": "api", "message": "request processed", "host": "dc1-web-21"},
  {"timestamp": "2026-06-05T10:00:01Z", "level": "warn", "source": "api", "message": "slow query detected", "host": "dc1-db-22", "duration_ms": 1234},
  {"timestamp": "2026-06-05T10:00:02Z", "level": "error", "source": "worker", "message": "job failed", "job_id": "job-123", "error": "connection refused"},
  ...
]
```

---

## 6. 验证脚本（3 个）

放在 `scripts/verify/`。

### 6.1 `verify-conn.sh` — 组件连通性

```bash
#!/bin/bash
# 验证所有组件可达
check() {
  local name="$1"
  local cmd="$2"
  if eval "$cmd" >/dev/null 2>&1; then
    log_ok "$name"
    return 0
  else
    log_fail "$name"
    return 1
  fi
}

check "PostgreSQL" "pg_isready -h localhost -p 5432 -U devops"
check "Loki"        "curl -sf http://localhost:3100/ready"
check "ES"          "curl -sf http://localhost:9200/_cluster/health"
check "Prometheus"  "curl -sf http://localhost:9090/-/ready"
check "InfluxDB"    "curl -sf http://localhost:8086/health"
check "LDAP"        "ldapsearch -x -H ldap://localhost:389 -b dc=example,dc=com"
check "Containerlab" "docker ps | grep -q clab-devops-toolkit-test"
check "k3d dev-cluster-1" "k3d cluster list | grep -q dev-cluster-1"
check "App health"  "curl -sf http://localhost:3000/health"
```

### 6.2 `verify-data.sh` — 数据完整性

```bash
# 验证种子数据已加载
psql -U devops -d devops -c "SELECT COUNT(*) FROM devices"     # >= 8
psql -U devops -d devops -c "SELECT COUNT(*) FROM business_lines" # >= 3
psql -U devops -d devops -c "SELECT COUNT(*) FROM systems"      # >= 6
psql -U devops -d devops -c "SELECT COUNT(*) FROM projects"     # >= 6
psql -U devops -d devops -c "SELECT COUNT(*) FROM project_types" # >= 2
```

### 6.3 `verify-flow.sh` — 端到端流程

```bash
# E2E smoke test
test_login() {
  TOKEN=$(curl -sX POST http://localhost:3000/api/auth/login \
    -d '{"username":"alice","password":"alice123"}' | jq -r .token)
  [ -n "$TOKEN" ]
}

test_list_devices() {
  curl -sH "Authorization: Bearer $TOKEN" \
    http://localhost:3000/api/devices | jq -e '.data | length > 0'
}

test_query_logs() {
  curl -sH "Authorization: Bearer $TOKEN" \
    "http://localhost:3000/api/logs?level=error" | jq -e '.data | length > 0'
}

test_trigger_alert() {
  curl -sX POST -H "Authorization: Bearer $TOKEN" \
    http://localhost:3000/api/alerts/trigger \
    -d '{"name":"test","severity":"info","message":"smoke test"}' | jq -e .ok
}
```

---

## 7. CI 集成（2 个工作流）

### 7.1 `.github/workflows/test-unit.yml`

```yaml
name: Unit Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: devops
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.21" }
      - run: go mod download
      - run: go test ./internal/... -race -coverprofile=coverage.out
      - run: go test ./cmd/... -race
      - uses: codecov/codecov-action@v3
```

### 7.2 `.github/workflows/test-integration.yml`

```yaml
name: Integration Tests
on: [pull_request]
jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
      - name: Start full environment
        run: ./scripts/setup.sh ci
      - name: Wait for healthy
        run: ./scripts/status.sh --wait
      - name: Run integration tests
        run: go test ./tests/integration/... -tags=integration
      - name: Verify flow
        run: ./scripts/verify/verify-flow.sh
      - name: Upload logs
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: test-logs
          path: logs/
```

---

## 8. 重置 / 清理流程

### 8.1 三档重置

| 命令 | 范围 | 保留 |
|------|------|------|
| `scripts/reset.sh` | 数据 | 服务、配置 |
| `scripts/teardown.sh` | 全部 | 配置、源码 |
| `scripts/clab.sh destroy && k3d-setup.sh destroy && docker-compose down -v` | 容器 | 镜像 |

### 8.2 reset.sh 实现

```bash
#!/bin/bash
set -euo pipefail
log_warn "This will DELETE all data in postgres, loki, es, influxdb"
read -p "Continue? (yes/no): " confirm
[ "$confirm" = "yes" ] || exit 1

# 1. 停止应用
docker-compose stop app

# 2. 清数据库
docker-compose exec -T postgres psql -U devops -c "DROP DATABASE IF EXISTS devops; CREATE DATABASE devops;"

# 3. 清日志
docker-compose exec -T loki rm -rf /loki/boltdb-shipper-cache /loki/boltdb-shipper-active /loki/boltdb-shipper-compactor
docker-compose restart loki

# 4. 清 ES
curl -XDELETE "http://localhost:9200/logs-*"

# 5. 清 InfluxDB
docker-compose exec -T influxdb influx delete --bucket metrics --start 1970-01-01T00:00:00Z --stop $(date -u +%Y-%m-%dT%H:%M:%SZ)

# 6. 重新 AutoMigrate
./scripts/db-setup.sh migrate

# 7. 重新种子
./scripts/seed-data.sh

# 8. 重启应用
docker-compose start app
```

---

## 9. 故障排查手册

**`docs/TROUBLESHOOTING.md`** 应包含：

### 9.1 常见问题

| 症状 | 原因 | 处置 |
|------|------|------|
| `connection refused: 5432` | postgres 容器未起 | `docker-compose up -d postgres` |
| `LDAP bind failed` | 容器未就绪 | `sleep 5 && ldapsearch -x ...` |
| `Clab: image not found` | 未 pull | `docker pull network-multitool` |
| `k3d: port already in use` | 旧集群残留 | `k3d cluster delete --all` |
| `permission denied on /var/run/docker.sock` | 用户不在 docker 组 | `sudo usermod -aG docker $USER` |
| `seed: foreign key violation` | AutoMigrate 未跑 | `./scripts/db-setup.sh migrate` |
| `alert trigger no notification` | channel 未配 | 检查 `slack-ops` 是否存在 |
| `auth: dev_bypass not working` | config 未加载 | 检查 `ENV=dev` 或 config-dev.yaml |

### 9.2 调试命令

```bash
# 看所有容器状态
docker ps -a

# 看应用日志
docker logs -f devops-toolkit

# 实时看 db 查询
docker exec postgres pg_isready -U devops
docker exec postgres psql -U devops -c "SELECT * FROM pg_stat_activity"

# 测 LDAP
ldapsearch -x -H ldap://localhost:389 -b dc=example,dc=com -D "cn=admin,dc=example,dc=com" -w admin

# 测 Containerlab 节点
docker exec -it clab-devops-toolkit-test-dc1-web-21 bash
ssh root@172.30.30.21  # 应该能登入

# 测 k3d
KUBECONFIG=$(k3d kubeconfig write dev-cluster-1) kubectl get nodes
```

### 9.3 完全清理（救火）

```bash
# 杀掉一切
docker ps -aq | xargs docker stop
docker ps -aq | xargs docker rm
docker network prune -f
docker volume prune -f

# 重建
./scripts/setup.sh ci
```

---

## 10. 重新开发实施清单

按优先级：

### P0 — 基础（必须先有）

- [ ] `deploy/containerlab/topology.yml`
- [ ] `deploy/docker-compose.yml`
- [ ] `configs/templates/config-dev.yaml`
- [ ] `configs/templates/config-ci.yaml`
- [ ] `configs/templates/config-prod.yaml`
- [ ] `scripts/setup.sh`
- [ ] `scripts/teardown.sh`
- [ ] `scripts/clab.sh`
- [ ] `scripts/db-setup.sh`
- [ ] `scripts/seed-data.sh`
- [ ] `tests/fixtures/ldap/{users,groups,bootstrap}.ldif`
- [ ] `tests/fixtures/devices/8-clab-nodes.json`
- [ ] `tests/fixtures/projects/*.json`
- [ ] `tests/fixtures/logs/alerts-rules.json`

### P1 — 集成（CI 跑通）

- [ ] `deploy/k3d/cluster.yaml`
- [ ] `scripts/k3d-setup.sh`
- [ ] `scripts/ldap-seed.sh`
- [ ] `scripts/verify/verify-conn.sh`
- [ ] `scripts/verify/verify-data.sh`
- [ ] `scripts/verify/verify-flow.sh`
- [ ] `scripts/status.sh`
- [ ] `scripts/reset.sh`
- [ ] `.github/workflows/test-unit.yml`
- [ ] `.github/workflows/test-integration.yml`
- [ ] `tests/fixtures/logs/sample-100.json`

### P2 — 完善（生产前）

- [ ] `docs/TROUBLESHOOTING.md`
- [ ] `scripts/verify/verify-ha.sh`（高可用测试）
- [ ] `tests/load/k6-*.js`（压力测试）
- [ ] `tests/e2e/playwright/*.spec.ts`（E2E）
- [ ] `tests/fixtures/metrics/prometheus-scrape.yml`
- [ ] `deploy/prometheus.yml`
- [ ] `deploy/loki-config.yaml`
- [ ] Grafana dashboards JSON

### P3 — 加分（可选）

- [ ] Chaos engineering 脚本（杀容器、模拟网络）
- [ ] 性能基线测试
- [ ] 备份/恢复流程
- [ ] 跨 region 测试

---

## 11. 验证环境就绪

完成后跑这套 checklist：

```bash
# 1. 启动
./scripts/setup.sh ci

# 2. 验证
./scripts/verify/verify-conn.sh    # 所有组件 green
./scripts/verify/verify-data.sh    # 种子数据完整
./scripts/verify/verify-flow.sh    # 端到端流程通过

# 3. 跑测试
go test ./...                     # 单测
go test -tags=integration ./...   # 集成

# 4. 看 UI
open http://localhost:3000        # 用 alice/alice123 登录
```

全部通过 = 环境就绪。

---

## 12. 参考

- [openspec/specs/test-environment/spec.md](../openspec/specs/test-environment/spec.md) — Containerlab 场景
- [PRD.md §14](../PRD.md) — 三层测试策略
- [DEPLOY.md](../DEPLOY.md) — 云平台部署（生产参考）
- [docs/BACKEND.md](BACKEND.md) — 后端配置规范
