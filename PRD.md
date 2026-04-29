# DevOps Toolkit - 产品需求文档

**Version:** 2.1
**Last Updated:** 2026-04-28

---

## 修订说明 (v2.1)

| 日期 | 修订内容 |
|------|---------|
| 2026-04-28 | 统一 Project 权限命名为 auditor/developer/operator/super_admin |
| 2026-04-28 | 简化物理主机状态管理，只保留 online/offline |
| 2026-04-28 | 物理主机状态优先监控，SSH 确认，区分 monitoring_issue 状态 |
| 2026-04-28 | K8s 模块增加 Pod 与节点关系视图，关联 Project |
| 2026-04-28 | 增加告警条件 DSL 规范 |
| 2026-04-28 | 增加 CI/CD 回滚机制、错误响应格式 |
| 2026-04-28 | 状态机增加触发事件表 |

---

## 目录

1. [系统概述](#1-系统概述)
2. [CI/CD Pipeline](#2-cicd-pipeline)
3. [Device Management](#3-device-management)
4. [Logging System](#4-logging-system)
5. [Monitoring System](#5-monitoring-system)
6. [Alert Notification System](#6-alert-notification-system)
7. [WebSocket Real-Time Communication](#7-websocket-real-time-communication)
8. [Prometheus Metrics](#8-prometheus-metrics)
9. [LDAP Authentication & Role Mapping](#9-ldap-authentication--role-mapping)
10. [Permission Model](#10-permission-model)
11. [K8s Multi-Cluster Management](#11-k8s-multi-cluster-management)
12. [Physical Host Management](#12-physical-host-management)
13. [Project Management Module](#13-project-management-module)
14. [Test Environment](#14-test-environment)

---

## 1. 系统概述

### 1.1 定位

内部 DevOps 平台，用于管理双数据中心基础设施（基于 Containerlab 的测试环境）。

### 1.2 核心功能

| 模块 | 用途 |
|------|------|
| CI/CD Pipeline | 流水线执行、部署策略 |
| Device Management | 物理/虚拟/网络设备统一管理 |
| Logging | 日志收集、存储、查询 |
| Monitoring | Prometheus 指标采集 |
| Alerts | 多通道通知（Slack/Webhook/Email） |
| WebSocket | 实时事件推送 |
| K8s Multi-Cluster | 多集群生命周期管理 |
| Physical Host | SSH 监控主机 |
| Project Management | 组织层级 + FinOps |

### 1.3 技术栈

| 层级 | 技术 |
|------|------|
| Backend | Go + Gin 框架 |
| Database | PostgreSQL + GORM |
| Frontend | React SPA（静态部署） |
| Container | Docker + Containerlab |
| Metrics | Prometheus + Grafana |

### 1.4 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      DevOps Toolkit                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│   │   CI/CD     │  │   Device    │  │   Logs      │        │
│   │  Pipeline   │  │  Management │  │   System    │        │
│   └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                              │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│   │   K8s       │  │  Physical   │  │   Project   │        │
│   │  Multi-Cluster│  │   Host     │  │  Management │        │
│   └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                              │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│   │   Metrics   │  │   Alerts    │  │   WebSocket │        │
│   │  (Prometheus)│  │            │  │             │        │
│   └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. CI/CD Pipeline

### 2.1 概述

CI/CD Pipeline 管理应用的构建、测试、部署流程，支持多种部署策略。

### 2.2 构建阶段

| 阶段 | 说明 | 超时 |
|------|------|------|
| validate | 代码验证和 lint | 5 min |
| build | 多语言构建 (Go, Python, Node.js, Java, .NET) | 10 min |
| test | 单元测试，80%+ 覆盖率 | 15 min |
| security_scan | Trivy/Snyk 漏洞扫描 | 10 min |
| stage_deploy | 部署到 staging 环境 | 10 min |
| smoke_test | 健康检查验证 | 5 min |
| prod_deploy | 生产部署（根据策略） | 15 min |
| verification | 部署后验证 | 5 min |

### 2.3 回滚机制

| 策略 | 回滚触发条件 | 回滚操作 |
|------|-------------|---------|
| Blue-Green | 验证阶段失败 / 人工触发 | 切换流量到原环境 |
| Canary | 指标异常（错误率>1%） | 停止下一阶段，保留原版本 |
| Rolling | 单批次失败 | 暂停批次，等待人工确认 |

### 2.4 部署策略

| 策略 | 说明 | 适用场景 |
|------|------|---------|
| Blue-Green | 零停机切换，快速回滚 | 有状态服务 |
| Canary | 渐进式流量分配 (1%→5%→25%→100%) | 无状态服务 |
| Rolling | 最大 20% 实例同时升级 | 一般场景 |

> **注意**: Canary 部署需要外部负载均衡器支持流量权重控制（如 HAProxy、Envoy、云 LB）。

### 2.4 Pipeline YAML 结构

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

deployment:
  strategy: canary
  canary:
    steps:
      - weight: 1
      - weight: 5
      - weight: 25
      - weight: 100
```

### 2.5 数据模型

```go
type Pipeline struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Stages      []string  `json:"stages"`
    DeployConfig DeployConfig `json:"deploy_config"`
    CreatedAt   time.Time `json:"created_at"`
}

type PipelineRun struct {
    ID          string    `json:"id"`
    PipelineID  string    `json:"pipeline_id"`
    Status      string    `json:"status"` // pending, running, success, failed, cancelled
    Stage       string    `json:"stage"`
    StartedAt   time.Time `json:"started_at"`
    FinishedAt  time.Time `json:"finished_at"`
    Logs        []string  `json:"logs"`
}
```

### 2.6 标准错误响应

所有 API 错误返回统一格式：

```json
{
  "code": "PIPELINE_NOT_FOUND",
  "message": "流水线不存在",
  "details": {
    "id": "pipeline-123"
  }
}
```

### 2.7 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/pipelines` | GET | 列出所有流水线 |
| `/api/pipelines` | POST | 创建流水线 |
| `/api/pipelines/:id` | GET | 获取流水线详情 |
| `/api/pipelines/:id` | PUT | 更新流水线 |
| `/api/pipelines/:id` | DELETE | 删除流水线 |
| `/api/pipelines/:id/execute` | POST | 执行流水线 |
| `/api/pipelines/:id/runs` | GET | 获取运行历史 |

### 2.7 测试矩阵

| 测试场景 | 输入 | 预期结果 | 测试类型 |
|---------|------|---------|---------|
| 流水线创建 | 有效 YAML 配置 | 流水线创建成功，状态 pending | 单元测试 |
| 流水线执行 | 存在且有效的流水线 | 阶段顺序执行，最终状态 success/failed | 集成测试 |
| 阶段超时 | 阶段执行超过超时时间 | 阶段标记为 timeout，流水线失败 | 单元测试 |
| 并发执行 | 同一流水线多次触发 | 拒绝重复执行，返回错误 | 单元测试 |
| 部署策略验证 | Canary 配置 | 各阶段权重正确应用 | 单元测试 |
| 日志记录 | 流水线各阶段输出 | 日志正确追加到 run.logs | 集成测试 |
| 取消执行 | 运行中的流水线 | 立即停止，状态为 cancelled | 集成测试 |

---

## 3. Device Management

### 3.1 概述

统一管理三类设备：物理主机、虚拟机、网络设备。支持设备发现、状态机、配置管理。

### 3.2 设备类型

| 类型 | 说明 | 协议 | 发现方式 |
|------|------|------|---------|
| PhysicalHost | 物理服务器/虚拟机 | SSH, IPMI | 主动注册 |
| VM | 虚拟机器 | vSphere, KVM, Xen, Hyper-V | Hypervisor 发现 |
| NetworkDevice | 交换机、路由器、防火墙 | SNMP, NETCONF | Pull 注册 |

### 3.3 通用状态机

```
PENDING → AUTHENTICATED → REGISTERED → ACTIVE
                                 ↓
                         MAINTENANCE ↔ SUSPENDED
                                 ↓
                              RETIRED
```

**触发事件:**

| 当前状态 | 事件 | 下一状态 | 触发条件 |
|---------|------|---------|---------|
| PENDING | AUTHENTICATED | AUTHENTICATED | 认证成功 |
| PENDING | AUTH_FAILED | PENDING | 认证失败（保持，重试） |
| AUTHENTICATED | REGISTER | REGISTERED | 注册完成 |
| REGISTERED | ACTIVATE | ACTIVE | 激活使用 |
| ACTIVE | MAINTENANCE | MAINTENANCE | 进入维护 |
| MAINTENANCE | ACTIVATE | ACTIVE | 维护完成 |
| ACTIVE | SUSPEND | SUSPENDED | 暂停使用 |
| SUSPENDED | RESUME | ACTIVE | 恢复使用 |
| SUSPENDED | RETIRE | RETIRED | 退役 |
| MAINTENANCE | RETIRE | RETIRED | 维护中退役 |
| ACTIVE | RETIRE | RETIRED | 直接退役 |

### 3.4 虚拟机状态机

```
PENDING → RUNNING → SUSPENDED
              ↓          ↓
           STOPPED    (resume)
              ↓
           RETIRED
```

**触发事件:**

| 当前状态 | 事件 | 下一状态 | 触发条件 |
|---------|------|---------|---------|
| PENDING | START | RUNNING | 虚拟机启动 |
| RUNNING | SUSPEND | SUSPENDED | 暂停（内存挂起） |
| RUNNING | STOP | STOPPED | 正常关机 |
| STOPPED | START | RUNNING | 开机 |
| SUSPENDED | RESUME | RUNNING | 恢复运行 |
| STOPPED | RETIRE | RETIRED | 退役 |
| RUNNING | RETIRE | RETIRED | 运行中退役 |

### 3.5 网络设备状态机

```
PENDING → DISCOVERED → ACTIVE
                      ↓
               MAINTENANCE → RETIRED
```

**触发事件:**

| 当前状态 | 事件 | 下一状态 | 触发条件 |
|---------|------|---------|---------|
| PENDING | DISCOVER | DISCOVERED | SNMP/SSH 发现成功 |
| DISCOVERED | ACTIVATE | ACTIVE | 激活使用 |
| ACTIVE | MAINTENANCE | MAINTENANCE | 进入维护 |
| MAINTENANCE | ACTIVATE | ACTIVE | 维护完成 |
| MAINTENANCE | RETIRE | RETIRED | 维护中退役 |
| ACTIVE | RETIRE | RETIRED | 直接退役 |

### 3.6 物理主机数据模型

| 字段 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| Name | string | 是 | 主机名，1-64字符 | web-server-01 |
| SerialNo | string | 是 | 唯一序列号 | SN123456789 |
| Manufacturer | string | 是 | 枚举: Dell, HP, Lenovo, Huawei, Inspur, SuperMicro | Dell |
| Model | string | 否 | 型号 | PowerEdge R740 |
| BIOSVersion | string | 否 | BIOS 版本 | 2.5.3 |
| CPUModel | string | 否 | CPU 型号 | Intel Xeon Gold 6248 |
| CPUCores | int | 否 | 物理核心数 | 32 |
| CPUThreads | int | 否 | 线程数 | 64 |
| MemoryGB | int | 否 | 内存总容量 | 128 |
| DiskTotalGB | int | 否 | 总存储 | 2000 |
| MgmtIP | string | 是 | 有效 IPv4 地址 | 192.168.1.101 |
| IPMIIP | string | 否 | IPMI/BMC IP | 192.168.1.100 |
| Location | string | 否 | 数据中心/机房 | DC1-A |
| Rack | string | 否 | 格式: `^[A-Z]-\d{2}-\d{2}$` | A-01-20 |
| AssetNo | string | 否 | 资产编号 | AST-2024-0001 |
| PurchaseDate | date | 否 | 采购日期 | 2024-01-15 |
| WarrantyExpire | date | 否 | 保修到期 | 2027-01-15 |

### 3.7 虚拟机数据模型

| 字段 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| VMID | string | 是 | 唯一标识 | vm-100 |
| Name | string | 是 | 名称，1-128字符 | web-server-01 |
| HypervisorType | string | 是 | 枚举: vsphere, kvm, xen, hyperv | vsphere |
| HypervisorHost | string | 是 | 宿主物理机 ID | host-1 |
| ResourcePool | string | 否 | 资源池 | RP-Production |
| Cluster | string | 否 | 所属集群 | Cluster-01 |
| VCPU | int | 是 | 1-256 | 4 |
| MemoryMB | int | 是 | 512-65536 | 8192 |
| DiskTotalGB | int | 否 | 总磁盘容量 | 100 |
| IPAddresses | JSON | 否 | IP 地址列表 | ["192.168.1.10"] |
| MACAddress | string | 否 | 主 MAC 地址 | 00:0c:29:ab:cd:ef |
| PowerState | string | 否 | on, off, suspended | on |
| GuestOS | string | 否 | 操作系统 | ubuntu 22.04 |

### 3.8 网络设备数据模型

| 字段 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| DeviceType | string | 是 | switch, router, firewall, loadbalancer, wireless, storage, other | switch |
| Vendor | string | 是 | Cisco, Juniper, Huawei, H3C, Arista, Fortinet, PaloAlto, other | Cisco |
| Model | string | 否 | 型号 | Catalyst 9300 |
| OSVersion | string | 否 | 操作系统版本 | 17.6.1 |
| SerialNo | string | 是 | 序列号 | FCW2233L0AA |
| MgmtIP | string | 是 | 管理网 IP | 192.168.1.1 |
| ConfigBackupStatus | string | 否 | success, failed, pending | success |

### 3.9 网络接口结构

```json
{
  "name": "GigabitEthernet0/0/1",
  "description": "To-Core-SW-01",
  "admin_status": "up",
  "oper_status": "up",
  "speed": "10G",
  "vlan_access": 100,
  "ip_address": "10.0.1.2/24",
  "mac_address": "aabb.ccdd.eeff",
  "counters": {
    "in_bytes": 1234567890,
    "out_bytes": 9876543210
  }
}
```

### 3.10 设备关系模型

```
PhysicalHost (rack-01)
└─ VM (vm-web-001)
   └─ Container (nginx-abc123)
      └─ Container (app-xyz789)

NetworkDevice (core-01)
├─ VM (vm-jumpbox)
└─ PhysicalHost (firewall-01)
```

### 3.11 配置管理

- **Template Engine**: Jinja2/Template2 with variables and logic
- **Inheritance Chain**: Base template → Type override → Instance override
- **Version Control**: Every change is traceable and reversible
- **Gradual Push**: Push configs in batches by tag groups

### 3.12 设备组系统

| 组类型 | 说明 | 示例 |
|--------|------|------|
| Flat Grouping | 按标签过滤 `label=env:prod` | 环境分组 |
| Hierarchical | 父子继承，自动传递标签 | DC1 → Rack-A |
| Dynamic | 基于实时属性动态计算 | `cluster=us-east && role=web` |
| Overlapping | 设备可属于多个组 | 同时属于 prod 和 web |

### 3.13 Mock 测试框架

用于在没有真实硬件的环境下进行开发和测试。

#### 客户端接口

```go
// HypervisorClient - 虚拟化平台客户端
type HypervisorClient interface {
    ListVMs(ctx context.Context, hostID string) ([]*VM, error)
    GetVM(ctx context.Context, vmID string) (*VM, error)
    GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    GetHostInfo(ctx context.Context, hostID string) (*GORMDevice, error)
    GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
    GetHostPowerState(ctx context.Context, hostID string) (string, error)
    SetHostPowerState(ctx context.Context, hostID string, state string) error
}

// NetworkDeviceClient - 网络设备客户端
type NetworkDeviceClient interface {
    ListDevices(ctx context.Context) ([]*GORMDevice, error)
    GetDevice(ctx context.Context, deviceID string) (*GORMDevice, error)
    GetDeviceInterfaces(ctx context.Context, deviceID string) ([]*NetworkInterface, error)
    GetDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
    BackupConfig(ctx context.Context, deviceID string) (string, error)
}

// MetricsCollector - 指标采集客户端
type MetricsCollector interface {
    CollectVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    CollectHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
    CollectNetworkDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
}
```

#### Mock 客户端实现

| 实现 | 用途 | 数据 |
|------|------|------|
| FakeVMwareClient | vSphere 模拟 | 3 台 VM (web-server-01, db-server-01, app-server-01) |
| FakeKVMClient | KVM 模拟 | 2 台 VM (kvm-vm-01, kvm-vm-02) |
| FakeIPMIClient | IPMI 模拟 | 物理主机电源控制 |
| FakeNetworkDeviceClient | SNMP 模拟 | 3 台网络设备 (core-switch-01, access-switch-01, edge-firewall-01) |
| FakeMetricsCollector | 指标模拟 | CPU/内存/磁盘/网络 |

#### 使用示例

```go
// 创建带模拟客户端的设备管理器
manager := NewManagerWithClients(
    db,
    &fake.FakeVMwareClient{},  // 模拟 vSphere
    &fake.FakeMetricsCollector{},
    &fake.FakeNetworkDeviceClient{},
)

// 使用 manager 进行操作
vms, err := manager.DiscoverVMsFromHost(ctx, "host-1")
```

### 3.14 API 端点

#### 通用设备 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/devices` | GET | 列表（分页、过滤） |
| `/api/devices` | POST | 创建/注册 |
| `/api/devices/:id` | GET | 获取单个 |
| `/api/devices/:id` | PUT | 更新 |
| `/api/devices/:id` | DELETE | 删除 |
| `/api/devices/search` | GET | 标签搜索 |
| `/api/devices/:id/metrics` | GET | 获取监控指标 |

#### 物理主机 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/devices/physical` | GET | 列表 |
| `/api/devices/physical` | POST | 创建 |
| `/api/devices/physical/:id` | GET | 获取 |
| `/api/devices/physical/:id` | PUT | 更新 |
| `/api/devices/physical/:id` | DELETE | 删除 |
| `/api/devices/physical/:id/discover` | POST | 触发发现 |
| `/api/devices/physical/:id/metrics` | GET | 采集指标 |

#### 虚拟机 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/devices/vm` | GET | 列表 |
| `/api/devices/vm` | POST | 创建 |
| `/api/devices/vm/:id` | GET | 获取 |
| `/api/devices/vm/:id/power-on` | POST | 开机 |
| `/api/devices/vm/:id/power-off` | POST | 关机 |
| `/api/devices/vm/:id/suspend` | POST | 暂停 |
| `/api/devices/vm/:id/resume` | POST | 恢复 |
| `/api/devices/vm/:id/metrics` | GET | 获取监控指标 |
| `/api/devices/vm/discover` | POST | 从 hypervisor 发现 |

#### 网络设备 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/devices/network` | GET | 列表 |
| `/api/devices/network` | POST | 创建 |
| `/api/devices/network/:id` | GET | 获取 |
| `/api/devices/network/:id/interfaces` | GET | 获取端口列表 |
| `/api/devices/network/:id/vlans` | GET | 获取 VLAN 配置 |
| `/api/devices/network/:id/backup` | POST | 触发配置备份 |
| `/api/devices/network/:id/metrics` | GET | 采集指标 |
| `/api/devices/network/:id/discover` | POST | 触发发现 |

### 3.15 测试矩阵

#### 状态机测试

| 测试场景 | 当前状态 | 动作 | 预期下一状态 | 验证 |
|---------|---------|------|-------------|------|
| 正常注册流程 | PENDING | AUTHENTICATED 事件 | AUTHENTICATED | 验证状态转换记录 |
| 注册失败 | PENDING | AUTHENTICATED 事件(失败) | PENDING | 验证错误日志，保留原状态 |
| 维护模式进入 | ACTIVE | MAINTENANCE 事件 | MAINTENANCE | 验证通知触发 |
| 维护模式恢复 | MAINTENANCE | ACTIVE 事件 | ACTIVE | 验证状态更新 |
| 退役流程 | SUSPENDED | RETIRED 事件 | RETIRED | 验证资源释放 |
| 无效转换 | ACTIVE | SUSPENDED 事件(不允许) | ACTIVE | 验证拒绝，记录错误 |

#### CRUD 测试

| 测试场景 | 输入 | 预期结果 | 验证点 |
|---------|------|---------|-------|
| 创建物理主机 | 完整有效数据 | 创建成功，返回设备对象 | DB 记录，状态=PENDING |
| 创建虚拟机 | 缺少必填字段 VMID | 返回 400 错误 | 错误信息明确 |
| 更新设备 | 存在的设备 ID + 新数据 | 更新成功 | DB 值匹配，审计日志 |
| 删除设备 | 存在的设备 ID | 软删除成功 | deleted_at 设置，API 查询不到 |
| 查询设备列表 | 分页参数 page=1, limit=10 | 返回分页结果 | 总数、页码、每页数量正确 |
| 搜索设备 | 标签过滤 env=prod | 返回匹配设备 | 结果数量、标签正确 |

#### Mock 客户端测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| FakeVMwareClient.ListVMs | hostID="host-1" | 返回 3 台 VM | VM 数量、名称匹配 |
| FakeKVMClient.GetVM | vmID="kvm-vm-01" | 返回 VM 对象 | 字段值匹配 |
| FakeNetworkDeviceClient.GetDeviceInterfaces | deviceID="core-switch-01" | 返回接口列表 | 接口数量、名称匹配 |
| FakeMetricsCollector.CollectVMMetrics | vmID="web-server-01" | 返回指标数据 | CPU/内存字段存在 |
| 电源控制 | power-on 命令 | VM 电源状态变为 on | PowerState 更新 |

---

## 4. Logging System

### 4.1 概述

日志系统负责收集、存储、查询应用和设备日志，支持多种后端存储。

### 4.2 日志架构

```
Sources → Collectors → Aggregator → Storage → Query/Search
     ↓        ↓              ↓         ↓         ↓
   Agents  Transport    Indexing    ES/S3   UI/REST
```

### 4.3 存储后端支持

| 后端 | 环境变量 | 默认端口 | 用途 |
|------|---------|---------|------|
| Local | `LOG_STORAGE_BACKEND=local` | - | 开发/测试（默认） |
| Elasticsearch | `LOG_STORAGE_BACKEND=elasticsearch` | 9200 | 生产全文搜索 |
| Loki | `LOG_STORAGE_BACKEND=loki` | 3100 | Grafana 生态 |

### 4.4 日志类型

| 类型 | 来源 | 存储 | 查询方式 |
|------|------|------|---------|
| Operational Logs | Application via LogManager | Local 或 Backend | 直接查询 |
| Device/Container Logs | External collectors (Filebeat) | Elasticsearch/Loki | Backend API 委托 |
| Audit Logs | LDAP client events | Backend | Backend API 委托 |

### 4.5 日志查询委托

```javascript
queryLogs(options = {}) {
  const backendType = this.backend.constructor.name;
  if (backendType === 'LocalStorageBackend') {
    return this.queryLogsLocal(options);
  } else {
    return this.queryLogsFromBackend(options);
  }
}
```

### 4.6 保留策略

| 后端 | 保留方法 |
|------|---------|
| Local | 应用层周期清理 |
| Elasticsearch | Index Lifecycle Management (ILM) Policy |
| Loki | chunk_target_size + retention_period 配置 |

### 4.7 数据模型

```go
type LogEntry struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"` // debug, info, warn, error
    Message   string    `json:"message"`
    Source    string    `json:"source"`
    Metadata  map[string]interface{} `json:"metadata"`
}
```

### 4.8 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/logs` | GET | 查询日志 |
| `/api/logs` | POST | 添加日志 |
| `/api/logs/stats` | GET | 获取统计 |
| `/api/logs/backend` | GET | 后端健康状态 |
| `/api/logs/retention` | GET | 获取保留策略 |
| `/api/logs/retention` | PUT | 更新保留策略 |
| `/api/logs/retention/apply` | POST | 触发保留清理 |
| `/api/logs/alerts` | GET/POST | 告警规则 CRUD |
| `/api/logs/filters` | GET/POST | 保存的过滤器 CRUD |
| `/api/logs/generate` | POST | 生成样本日志 |

### 4.9 测试矩阵

#### 存储后端测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| Local 后端写入 | LogEntry 对象 | 写入成功 | 文件存在，内容匹配 |
| ES 后端写入 | LogEntry 对象 | 写入成功 | ES 索引存在，文档可查询 |
| Loki 后端写入 | LogEntry 对象 | 写入成功 | Loki 查询返回结果 |
| 后端健康检查 | - | 返回后端状态 | 字段: status, latency_ms |
| 查询委托 | 跨后端查询 | 正确路由 | 结果格式一致 |

#### 日志查询测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 按级别过滤 | level=error | 只返回 error 日志 | 结果数量、日志级别一致 |
| 按时间范围 | start=2026-04-01, end=2026-04-28 | 范围内日志 | 时间戳在范围内 |
| 按来源过滤 | source=device-manager | 匹配来源的日志 | 结果来源正确 |
| 分页查询 | page=2, limit=20 | 第 21-40 条日志 | 偏移量正确 |
| 全文搜索 | keyword=error | 匹配 keyword 的日志 | 结果包含 keyword |
| 组合过滤 | level=error, source=api | 同时满足条件的日志 | 所有条件匹配 |

#### 保留策略测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 获取保留策略 | - | 返回当前策略 | retention_days, max_size |
| 更新保留策略 | retention_days=30 | 更新成功 | 新值生效 |
| 触发清理 | - | 执行清理 | 过期日志删除，统计更新 |

#### 告警规则测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 创建告警规则 | 有效规则配置 | 创建成功 | 规则保存，ID 分配 |
| 触发告警 | 匹配条件的日志 | 告警发送 | 回调函数调用，事件记录 |
| 告警去重 | 同一规则短时间多次触发 | 只发送一次 | rate limiting 生效 |
| 删除告警规则 | 存在的规则 ID | 删除成功 | 规则不再触发 |

### 4.10 Filebeat 集成

```bash
# 配置和测试 Filebeat
./scripts/filebeat-test.sh setup              # 创建配置和样本日志
./scripts/filebeat-test.sh elasticsearch     # 配置 Filebeat 输出
./scripts/filebeat-test.sh start             # 启动 Filebeat
./scripts/filebeat-test.sh loki              # 配置为 Loki
```

---

## 5. Monitoring System

### 5.1 概述

Prometheus 指标的采集、存储和查询系统。

### 5.2 采集模式

| 模式 | 说明 | 场景 |
|------|------|------|
| Pull | Prometheus client, 自定义中间件 | 应用暴露 /metrics |
| Push | OTLP 协议, 多语言 SDK | 无法拉取的服务 |
| Auto-Discovery | K8s ServiceMonitor, NodeExporter | K8s 环境 |

### 5.3 指标类型

| 类型 | 说明 | 示例 |
|------|------|------|
| Counter | 单调递增 | 请求计数、错误计数 |
| Gauge | 可增可减 | QPS、并发数 |
| Histogram | 值分布 | 请求延迟 |
| Summary | 聚合数据 | 总延迟、百分位数 |

### 5.4 存储层

| 层级 | 存储 | 保留 |
|------|------|------|
| 短期 | Prometheus TSDB | 15 天 |
| 长期 | Cortex/Thanos | 90+ 天 |

### 5.5 数据模型

```go
type Metric struct {
    Name   string            `json:"name"`
    Type   string            `json:"type"` // counter, gauge, histogram, summary
    Labels map[string]string `json:"labels"`
    Value  float64          `json:"value"`
}
```

### 5.6 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/metrics` | GET | Prometheus text 格式 |
| `/api/metrics` | GET | JSON 格式 |
| `/api/metrics/counter` | POST | 递增计数器 |
| `/api/metrics/gauge` | POST | 设置/增减仪表 |
| `/api/metrics/histogram` | POST | 观察直方图值 |

### 5.7 可用指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `devops_toolkit_info` | Gauge | service, version | 系统信息 |
| `http_requests_total` | Counter | endpoint, method, status | HTTP 请求计数 |
| `http_request_duration_ms` | Histogram | endpoint, method, status | 请求延迟 |
| `logs_total` | Counter | level | 按级别日志计数 |
| `device_events_total` | Counter | type | 设备事件 |
| `pipeline_events_total` | Counter | type, pipeline | 流水线事件 |
| `alerts_total` | Counter | name, severity | 告警计数 |

### 5.8 Prometheus 配置

```yaml
scrape_configs:
  - job_name: 'devops-toolkit'
    static_configs:
      - targets: ['localhost:3000']
    metrics_path: '/metrics'
```

### 5.9 测试矩阵

#### 指标采集测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| Counter 递增 | counter name + increment | 计数器增加 | GET /metrics 显示正确值 |
| Gauge 设置 | gauge name + value | gauge 设置为值 | GET /api/metrics 包含值 |
| Histogram 观察 | histogram name + value | 样本添加 | 百分位数计算正确 |
| 标签过滤 | query with labels | 匹配标签的指标 | 结果标签匹配 |

#### HTTP 中间件测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 请求记录 | GET /api/devices | http_requests_total 增加 | 计数器 +1 |
| 延迟记录 | GET /api/devices | http_request_duration_ms 增加 | 直方图样本增加 |
| 状态码记录 | GET /health (200) | status=200 标签增加 | 按状态码分组计数 |

#### Prometheus 导出测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| Text 格式 | GET /metrics | Prometheus 格式输出 | 解析正确 |
| JSON 格式 | GET /api/metrics | JSON 格式输出 | 结构正确 |
| 查询表达式 | /api/metrics?query=http_requests | 过滤后的指标 | 结果匹配 |

---

## 6. Alert Notification System

### 6.1 概述

多通道告警通知系统，支持 Slack、Webhook、Email、Log。

### 6.2 支持的通道

| 类型 | 配置 | 说明 |
|------|------|------|
| `slack` | webhookUrl, channel | Slack webhook 通知 |
| `webhook` | url, headers | 通用 HTTP webhook，headers 用于 Authorization Bearer token |
| `email` | recipients, smtp_host, smtp_port | Email 通知 (SMTP) |
| `log` | - | 仅日志输出 |

**Webhook Payload 格式:**

```json
{
  "alert_name": "high-cpu",
  "severity": "critical",
  "message": "CPU usage above 90%",
  "timestamp": "2026-04-28T10:00:00Z",
  "labels": {
    "host": "web-server-01",
    "env": "prod"
  }
}
```

Content-Type: `application/json`

### 6.3 限流规则

| 参数 | 值 |
|------|------|
| 时间窗口 | 60 秒 |
| 每告警名称上限 | 10 条/窗口 |

### 6.4 数据模型

```go
type AlertChannel struct {
    Name    string            `json:"name"`
    Type    string            `json:"type"` // slack, webhook, email, log
    Config  map[string]string `json:"config"`
    Enabled bool              `json:"enabled"`
}

type AlertRule struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Condition string    `json:"condition"`
    Severity  string    `json:"severity"` // critical, warning, info
    Channel   string    `json:"channel"`
    Enabled   bool      `json:"enabled"`
}
```

### 6.4.1 告警条件 DSL

AlertRule.Condition 使用简单表达式语言：

| 操作符 | 说明 | 示例 |
|--------|------|------|
| `==` | 等于 | `level == "error"` |
| `!=` | 不等于 | `source != "test"` |
| `>` `>=` `<` `<=` | 数值比较 | `count > 5` |
| `&&` | 逻辑与 | `level == "error" && count > 3` |
| `\|\|` | 逻辑或 | `level == "error" \|\| level == "warn"` |
| `()` | 优先级 | `(level == "error") && (count > 5)` |

可用字段：
- `level` — 日志级别 (debug, info, warn, error)
- `source` — 日志来源
- `count` — 时间窗口内匹配次数
- `message` — 消息内容（支持 `contains()` 函数）

示例：
```
level == "error" && count > 5
message contains "timeout" && count > 1
(level == "warn" || level == "error") && count > 10
```

### 6.5 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/alerts/channels` | GET | 列出通知通道 |
| `/api/alerts/channels` | POST | 添加通道 |
| `/api/alerts/channels/:name` | DELETE | 删除通道 |
| `/api/alerts/history` | GET | 查询告警历史 |
| `/api/alerts/stats` | GET | 告警统计 |
| `/api/alerts/trigger` | POST | 触发告警 |

### 6.6 测试矩阵

#### 通道管理测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 创建 Slack 通道 | 有效 webhook URL | 创建成功 | 通道可查询 |
| 创建无效通道 | 缺少必填字段 | 返回 400 | 错误信息明确 |
| 删除通道 | 存在的通道名 | 删除成功 | 告警不再发送 |
| 禁用通道 | 存在的通道名 | 禁用成功 | 告警跳过 |

#### 告警触发测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 触发告警 | 有效告警配置 | 通知发送 | 回调函数调用 |
| 限流触发 | 同名称 15 次触发 | 前 10 成功，后 5 限流 | 超出计数被拒绝 |
| 通道故障 | webhook 返回 500 | 记录错误，不阻塞 | 错误日志，状态更新 |

#### 历史查询测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 查询历史 | 无过滤 | 返回所有历史 | 分页正确 |
| 按名称过滤 | name=high-cpu | 匹配名称的历史 | 结果过滤正确 |
| 按时间过滤 | start, end | 时间范围内的历史 | 时间戳正确 |

---

## 7. WebSocket Real-Time Communication

### 7.1 概述

WebSocket 提供实时事件推送，包括日志、指标、设备事件等。

### 7.2 支持的通道

| 通道 | 事件 | 说明 |
|------|------|------|
| `log` | Log entry added | 实时日志流 |
| `metric` | Metric update | 实时指标更新 |
| `device_event` | Device status change | 设备状态更新 |
| `pipeline_update` | Pipeline execution | 运行进度通知 |
| `alert` | Alert triggered | 告警通知 |

### 7.3 客户端订阅

```javascript
const ws = new WebSocket('ws://localhost:3000/ws');
ws.send(JSON.stringify({ action: 'subscribe', channel: 'log' }));
ws.send(JSON.stringify({ action: 'subscribe', channel: 'alert' }));

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log(data.channel, data.data);
};
```

### 7.4 消息格式

```json
{
    "channel": "log",
    "type": "log",
    "data": { "id": "...", "message": "...", "level": "info" },
    "timestamp": "2026-04-28T00:00:00.000Z"
}
```

### 7.5 API 端点

| 端点 | 说明 |
|------|------|
| `/ws` | WebSocket 服务器 |

### 7.6 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 建立连接 | WebSocket 客户端连接 | 连接成功 | connection open 事件 |
| 订阅通道 | subscribe action | 订阅成功 | subscription confirmed |
| 取消订阅 | unsubscribe action | 取消成功 | 不再接收消息 |
| 消息广播 | 服务器广播消息 | 所有订阅客户端收到 | 消息内容匹配 |
| 多通道订阅 | 同时订阅 log 和 alert | 两个通道都收到 | 通道区分正确 |
| 断线重连 | 网络中断后重连 | 重连成功 | 恢复订阅 |

---

## 8. LDAP Authentication & Role Mapping

### 8.1 概述

LDAP 认证和组到角色的映射系统。

### 8.2 认证流程

```
User → LDAP Server → Token/JWT → 角色映射 → 权限
```

### 8.3 组到角色映射

| LDAP 组 | 系统角色 | 权限 |
|---------|---------|------|
| `cn=IT_Ops,ou=Groups,dc=example,dc=com` | Operator | deploy, config-manage |
| `cn=DevTeam_Payments,ou=Groups,dc=example,dc=com` | Developer | read, test-deploy |
| `cn=Security_Auditors,ou=Groups,dc=example,dc=com` | Auditor | read, audit-read |
| `cn=SRE_Lead,ou=Groups,dc=example,dc=com` | SuperAdmin | all |

### 8.4 业务目标

| 目标 | KPI | 目标值 |
|------|-----|-------|
| 安全访问控制 | 认证失败阻止率 | 100% |
| 开发者效率 | 开发环境启动时间 | ≤ 2 min |
| 审计追踪 | 审计日志采集率 | 100% |
| 运维可靠性 | 认证失败停机时间 | < 1 min |

### 8.5 数据模型

```go
type User struct {
    Username string   `json:"username"`
    Groups   []string `json:"groups"`
    Role     string   `json:"role"`
    Token    string   `json:"token"`
}
```

### 8.6 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/auth/login` | POST | LDAP 登录 |
| `/api/auth/logout` | POST | 登出 |
| `/api/auth/me` | GET | 获取当前用户信息 |

### 8.7 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 有效登录 | 正确的 LDAP 凭证 | 返回 JWT token | token 有效，可用于后续请求 |
| 无效登录 | 错误的 LDAP 凭证 | 返回 401 | 错误信息明确 |
| 组映射 | 用户属于多个组 | 映射到多个角色 | 权限叠加正确 |
| Token 刷新 | 过期 token | 返回新 token | 新 token 可用 |
| 连接重试 | LDAP 服务器暂时不可达 | 重试 3 次后失败 | 错误日志，重试计数 |

---

## 9. Permission Model

### 9.1 概述

基于标签和业务层级的 RBAC 权限系统。

### 9.2 访问决策流程

```
1. 认证检查 → 用户已认证？AD token 有效？
2. 加载用户组 → 获取用户所有 AD 组
3. 映射角色 → AD 组 → 内部系统角色
4. 确定基础权限 → 基于角色的权限
5. 设备标签查询 → 获取目标设备的所有标签
6. 业务组匹配 → 检查用户组是否匹配设备组
7. 标签权限验证 → 检查标签组权限继承
8. 操作权限检查 → 所需操作 vs 用户权限
```

### 9.3 角色权限矩阵

| 角色 | 查看设备 | 修改配置 | 执行命令 | 远程重启 |
|------|---------|---------|---------|---------|
| Auditor | ✅ | ❌ | ❌ | ❌ |
| Developer | ✅ | ❌ | ❌ | ❌ |
| Operator | ✅ | ✅ | ✅ | ❌* |
| SuperAdmin | ✅ | ✅ | ✅ | ✅ |

*Operator 仅可重启非生产设备

### 9.4 标签继承

- **父组标签**: 自动继承给子组
- **业务组继承**: 子业务组可访问父资源
- **权限叠加**: 子组在继承基础上添加权限

### 9.5 资源级别权限

资源级别权限用于对特定资源（如特定设备）进行精细化控制。

**决策规则:**

| 规则 | 说明 |
|------|------|
| 最小颗粒优先级最高 | 资源级别权限 > 项目级别权限 > 业务组级别权限 |
| 无需审批 | 资源权限申请无需审批流程 |
| 永久有效 | 资源权限不设置过期时间 |

**权限优先级矩阵:**

| 级别 | 范围 | 审批 | 有效期 | 优先级 |
|------|------|------|--------|--------|
| 业务组 | BL 级别 | 有 | 可设置 | 最低 |
| 项目 | Project 级别 | 有 | 可设置 | 中 |
| 资源 | 单个资源 | 无 | 永久 | 最高 |

**示例:**
- 用户在项目级别是 Developer，但在特定设备 A 上被授予 Operator → 对设备 A 有 Operator 权限
- 用户在项目级别被拒绝访问 X 服务，但资源级别允许访问设备 B → 对设备 B 有访问权限

### 9.6 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| Auditor 查看设备 | Auditor 角色 | 允许查看 | 返回设备数据 |
| Auditor 修改配置 | Auditor 角色 | 拒绝修改 | 返回 403 |
| Operator 生产重启 | Operator 角色 + prod 设备 | 拒绝 | 返回 403 |
| Operator 非生产重启 | Operator 角色 + dev 设备 | 允许 | 操作成功 |
| SuperAdmin 全操作 | SuperAdmin 角色 | 全部允许 | 所有操作成功 |
| 标签继承 | 子组继承父组标签 | 继承的标签可访问 | 权限检查通过 |

---

## 10. K8s Multi-Cluster Management

### 10.1 概述

通过 kind/k3d 实现多集群 Kubernetes 管理。

### 10.2 支持的操作

| 操作 | 说明 |
|------|------|
| Cluster Lifecycle | 创建、删除、列出集群 |
| Health Check | 节点状态、就绪探针 |
| Workload Management | 部署、扩缩容、删除 |
| Metrics Collection | CPU/内存采集 |
| Pod Operations | 日志获取、exec 命令 |
| Cross-Cluster | 广播配置到所有集群 |
| Node Workload View | 查看节点上运行的业务（前端/后端 Pod） |

### 10.2.1 Pod 与节点关系

用于显示物理机上运行的业务：

```
PhysicalHost (node-01)
├── Pod: frontend-order-web-xxx → Project: order-frontend
├── Pod: backend-order-api-xxx → Project: order-backend
└── Pod: frontend-payment-web-xxx → Project: payment-frontend
```

**视图用途:**
- 查看节点资源使用情况
- 追踪业务部署分布
- 容量规划

**数据结构:**

```go
type PodInfo struct {
    PodName      string `json:"pod_name"`
    Namespace    string `json:"namespace"`
    NodeName     string `json:"node_name"`
    Status       string `json:"status"`
    ProjectID    string `json:"project_id"`    // 关联的 Project ID
    ProjectName  string `json:"project_name"`  // 用于显示
    ProjectType  string `json:"project_type"` // frontend | backend
    CreatedAt    string `json:"created_at"`
}
```

**API 扩展:**

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/k8s/clusters/:name/nodes/:node/pods` | GET | 获取节点上所有 Pod |
| `/api/k8s/clusters/:name/pods/by-project` | GET | 按 Project 分组的 Pod |
| `/api/physical-hosts/:id/k8s-pods` | GET | 物理主机上的 K8s Pod（跨集群） |

### 10.3 kind/k3d 设置脚本

```bash
# 设置所有集群 (cluster-1, cluster-2, cluster-3)
bash devops-toolkit/scripts/kind-setup.sh setup

# 创建特定集群
bash devops-toolkit/scripts/kind-setup.sh create <name>

# 健康检查
bash devops-toolkit/scripts/kind-setup.sh health <name>

# 清理
bash devops-toolkit/scripts/kind-setup.sh cleanup
```

### 10.4 Kubeconfig 管理

- 集群 kubeconfigs 存储在 `~/.kube/config-<cluster-name>`
- 混合配置用于多集群操作: `~/.kube/config-kind-mixed`

### 10.5 数据模型

```go
type K8sCluster struct {
    Name    string            `json:"name"`
    Type    string            `json:"type"` // dev, test, uat, prod
    Kubeconfig string         `json:"kubeconfig"`
    Status  string            `json:"status"` // healthy, unhealthy, unknown
}
```

### 10.6 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/k8s/clusters` | GET | 列出集群 |
| `/api/k8s/clusters` | POST | 创建集群 |
| `/api/k8s/clusters/:name` | DELETE | 删除集群 |
| `/api/k8s/clusters/:name/health` | GET | 健康检查 |
| `/api/k8s/clusters/:name/nodes` | GET | 获取节点 |
| `/api/k8s/clusters/:name/namespaces` | GET | 获取命名空间 |
| `/api/k8s/clusters/:name/pods` | GET | 获取 Pod |
| `/api/k8s/clusters/:name/pods/:pod/logs` | GET | 获取 Pod 日志 |
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs` | GET | 获取带命名空间的 Pod 日志 |
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/exec` | POST | 在 Pod 中执行命令 |
| `/api/k8s/clusters/:name/nodes/:node/pods` | GET | 获取节点上所有 Pod |
| `/api/k8s/clusters/:name/pods/by-project` | GET | 按 Project 分组获取 Pod |
| `/api/k8s/clusters/:name/metrics` | GET | 获取集群指标 |
| `/api/k8s/maintenance` | POST | 维护操作 |
| `/api/physical-hosts/:id/k8s-pods` | GET | 物理主机上的 K8s Pod（跨集群） |

### 10.7 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 列出集群 | - | 返回所有集群 | 集群数量、名称正确 |
| 创建集群 | 有效配置 | 集群创建成功 | kind/k3d cluster exists |
| 健康检查 | 存在的集群名 | 返回健康状态 | nodes ready, components healthy |
| 获取节点 | 存在的集群名 | 返回节点列表 | 节点数量、资源状态 |
| 获取 Pod | 集群名 + 命名空间 | 返回 Pod 列表 | Pod 数量、状态正确 |
| 获取 Pod 日志 | 集群名 + 命名空间 + Pod 名 | 返回日志内容 | 日志内容非空 |
| Pod exec | 有效命令 | 命令执行成功 | 输出返回 |
| 部署应用 | deployment 配置 | 部署成功 | replicas ready |
| 扩缩容 | deployment + replicas | 扩缩成功 | replicas 数量匹配 |
| 删除集群 | 存在的集群名 | 集群删除 | kind/k3d cluster 不存在 |
| 获取节点 Pod 列表 | 集群名 + 节点名 | 返回节点上所有 Pod | Pod 数量、Project 信息正确 |
| 按 Project 分组 | 集群名 | 返回按 Project 分组的 Pod | 分组正确、包含前端/后端 |
| 物理主机跨集群 Pod | 物理主机 ID | 返回该主机上所有 K8s Pod | 跨集群聚合正确 |

---

## 11. Physical Host Management

### 11.1 概述

通过 SSH 连接管理物理主机，实现指标采集和配置下发。

**状态判断优先级:** 监控数据为主，SSH 为辅。

### 11.2 状态管理

```
状态: online | monitoring_issue | offline

判断规则:
- 监控正常 → online
- 监控显示 DOWN，SSH 确认成功 → monitoring_issue (监控异常，但主机正常)
- 监控显示 DOWN，SSH 也失败 → offline (主机确实离线)
```

**状态说明:**

| 状态 | 监控数据 | SSH 检查 | UI 显示 | 告警 |
|------|---------|---------|---------|------|
| online | 正常 | 成功 | ✅ 在线 | 无 |
| monitoring_issue | DOWN | 成功 | ⚠️ 监控异常 | 可选 |
| offline | DOWN | 失败 | ❌ 离线 | 是 |

**设计理由:**
- 监控数据可能因网络、采集 agent 问题而丢失
- SSH 确认可以区分"主机真的挂了"和"监控本身的问题"
- monitoring_issue 状态让运维人员知道需要检查监控本身

### 11.3 指标采集

通过 SSH 连接采集系统指标，数据存储在本地缓存用于展示。

| 指标 | 来源 | 说明 |
|------|------|------|
| CPU | SSH 执行 `top`/`mpstat` | 使用率、核心数 |
| Memory | SSH 执行 `free` | total, used, usagePercent |
| Disk | SSH 执行 `df` | 设备、大小、已用空间 |
| Uptime | SSH 执行 `uptime` | 运行时间 |
| Services | SSH 执行 `systemctl` | 服务状态 |

### 11.4 数据模型

```javascript
{
    id: "host-1",
    hostname: "prod-web-server-01",
    ip: "192.168.1.1",
    port: 22,
    state: "online",           // online | monitoring_issue | offline
    monitoringStatus: "up",    // up | down (来自监控系统)
    lastHeartbeat: "2026-04-28T10:00:00Z",  // SSH 最后成功时间
    lastMonitoringDown: null,  // 最近一次监控 DOWN 的时间
    metrics: {
        cpu: { usage: 45.5, cores: 8 },
        memory: { total: 16384, used: 8192, usagePercent: 50 },
        disk: { disks: [...] },
        uptime: { value: 900000, formatted: "10d 10h 0m" }
    },
    registeredAt: "2026-04-28T00:00:00Z"
}
```

### 11.5 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/physical-hosts` | GET | 列出所有主机 |
| `/api/physical-hosts` | POST | 注册主机 |
| `/api/physical-hosts/:id` | GET | 获取主机详情 |
| `/api/physical-hosts/:id` | DELETE | 删除主机 |
| `/api/physical-hosts/:id/metrics` | GET | 查询指标 |
| `/api/physical-hosts/:id/heartbeat` | POST | SSH 心跳检查 |
| `/api/physical-hosts/summary` | GET | 获取主机摘要 |
| `/api/physical-hosts/cache` | GET | 获取本地缓存状态 |

### 11.6 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 监控正常 | 监控系统数据正常 | state=online | monitoringStatus=up |
| 监控 DOWN，SSH 正常 | 监控显示 DOWN，SSH 确认成功 | state=monitoring_issue | 主机正常，告警可选 |
| 监控 DOWN，SSH 失败 | 监控显示 DOWN，SSH 也失败 | state=offline | 触发告警 |
| SSH 心跳成功 | 正常主机 SSH 连接 | state 保持/更新 | lastHeartbeat 刷新 |
| SSH 心跳失败 | 监控也显示 DOWN | state=offline | 触发告警 |
| 指标采集 | 正常主机 | 返回 CPU/内存/磁盘 | 字段存在，值合理 |
| 配置推送 | 有效配置 | SSH 推送成功 | 远程配置更新 |

---

## 12. Project Management Module

### 12.1 概述

组织层级管理（Business Line → System → Project），用于 FinOps 报表。

### 12.2 层级结构

```
Business Line (事业群/产品线)
└── System (系统)
    └── Project (项目)
        ├── frontend (前端项目)
        └── backend (后端项目)
```

### 12.3 资源链接

Projects 链接 DevOps 资源：
- `device` → 物理/虚拟机
- `pipeline` → CI/CD 流水线
- `log_source` → 日志聚合源
- `alert_channel` → 告警通知通道
- `physical_host` → SSH 监控主机

**共享资源:**
- 资源可以链接到多个 Project（跨项目共享）
- 共享资源使用 `weight`（权重）作为财务分割基数
- 权重表示该 Project 占用的资源比例（百分比或分数）

```go
type ProjectResource struct {
    ProjectID   string  `json:"project_id"`
    ResourceID  string  `json:"resource_id"`
    ResourceType string `json:"resource_type"` // device, pipeline, etc.
    Weight      float64 `json:"weight"`        // 0.0-1.0 或 0-100，表示分摊比例
    LinkedAt    time.Time `json:"linked_at"`
}
```

**权重分配示例:**

| 设备 | Project A | Project B | Project C |
|------|-----------|-----------|-----------|
| shared-server-01 | 权重 0.5 (50%) | 权重 0.3 (30%) | 权重 0.2 (20%) |
| shared-storage-01 | 权重 1.0 (100%) | - | - |

**FinOps 计算:**
- 设备费用 = 设备总成本 × 权重
- Project A 的 shared-server-01 费用 = $1000 × 0.5 = $500

### 12.4 权限模型

| 角色 | 说明 |
|------|------|
| auditor | 读取层级和链接的资源 |
| developer | 读取层级，编辑项目资源 |
| operator | 编辑层级和链接资源，运维操作 |
| super_admin | 删除层级和管理权限 |

权限继承：Business Line → System → Project。Business Line 的 operator 可操作其下所有内容。

**注意:** LDAP 仅用于认证，权限在 DevOps 本地管理。LDAP 组到角色的映射：
- LDAP `IT_Ops` 组 → `operator`
- LDAP `DevTeam_*` 组 → `developer`
- LDAP `Security_Auditors` 组 → `auditor`
- LDAP `SRE_Lead` 组 → `super_admin`

### 12.4.1 软删除与恢复

**删除行为:**
- Business Line、System、Project 删除时执行软删除（`deleted_at` timestamp）
- 删除时备份当前的关系数据（System → Project → Resource 的链接关系）
- 资源（设备等）不受影响，保留在原状态

**恢复行为:**
- 可恢复已软删除的 Business Line
- 恢复时从备份数据重建之前的层级关系和资源链接

**资源生命周期:**
- 资源（device、physical_host 等）有独立生命周期
- 资源必须先下线才能删除（不能删除正在使用的资源）
- 资源删除需 super_admin 审批

### 12.5 FinOps 报表

CSV 导出 via `GET /api/org/reports/finops?period=2026-04`

```csv
Business Line,System,Project Type,Project,Resource Type,Resource ID,Weight,Count,Unit,Cost
电商事业部,订单系统,Backend,order-backend,VM,shared-server-01,0.5,1,nodes,500
电商事业部,订单系统,Backend,order-backend,VM,shared-server-01,0.3,1,nodes,300
电商事业部,订单系统,Backend,order-backend,Storage,shared-storage-01,1.0,500,GB,1000
```

**计算规则:**
- 单项目独占资源：费用 = 资源总成本
- 共享资源：费用 = 资源总成本 × 权重
- 权重范围：0.0 - 1.0（或 0 - 100%）
- 所有链接同一资源的项目权重之和应 ≤ 1.0

### 12.6 数据模型

```go
type BusinessLine struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Systems   []System  `json:"systems,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type System struct {
    ID              string    `json:"id"`
    BusinessLineID  string    `json:"business_line_id"`
    Name            string    `json:"name"`
    Projects        []Project `json:"projects,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
}

type Project struct {
    ID        string    `json:"id"`
    SystemID  string    `json:"system_id"`
    Name      string    `json:"name"`
    Type      string    `json:"type"` // frontend, backend, 及其他可选类型
    Resources []ProjectResource `json:"resources,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type ProjectResource struct {
    ID           string    `json:"id"`
    ProjectID    string    `json:"project_id"`
    ResourceID   string    `json:"resource_id"`
    ResourceType string    `json:"resource_type"` // device, pipeline, physical_host
    Weight       float64   `json:"weight"`        // 0.0-1.0，财务分摊比例
    LinkedAt     time.Time `json:"linked_at"`
}
```

### 12.7 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/org/business-lines` | GET/POST | 列出/创建业务线 |
| `/api/org/business-lines/:id` | GET/PUT/DELETE | 获取/更新/删除业务线 |
| `/api/org/business-lines/:id/systems` | GET/POST | 列出/创建系统 |
| `/api/org/systems/:id` | GET/PUT/DELETE | 获取/更新/删除系统 |
| `/api/org/systems/:id/projects` | GET/POST | 列出/创建项目 |
| `/api/org/projects/:id` | GET/PUT/DELETE | 获取/更新/删除项目 |
| `/api/org/projects/:id/resources` | GET/POST | 列出/链接资源 |
| `/api/org/projects/:id/resources/:resource_id` | DELETE | 取消链接资源 |
| `/api/org/projects/:id/permissions` | GET/POST | 列出/授予权限 |
| `/api/org/permissions/:perm_id` | DELETE | 撤销权限 |
| `/api/org/reports/finops` | GET | FinOps CSV 导出 |
| `/api/org/audit-logs` | GET | 查询审计日志 |

### 12.8 测试矩阵

#### CRUD 测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 创建 BusinessLine | 有效名称 | 创建成功 | DB 记录，ID 生成 |
| 创建 System | business_line_id + 名称 | 创建成功 | 关联正确 |
| 创建 Project | system_id + 名称 + 类型 | 创建成功 | 类型正确 |
| 更新层级 | 存在的 ID + 新数据 | 更新成功 | DB 值匹配 |
| 删除 BusinessLine | 存在的 ID | 软删除成功 + 关系备份 | deleted_at 设置，关系数据保留 |
| 业务线恢复 | 恢复已删除的 BL | 恢复成功 | 之前的关系一并恢复 |
| 资源链接 | project_id + resource + weight | 链接成功 | ProjectResource 表记录 |
| 资源取消链接 | project_id + resource_id | 取消成功 | 记录删除 |
| 共享资源 | 同一资源链接到多个 project | 链接成功 | 每个链接有权重 |
| 权重更新 | 更新资源权重 | 更新成功 | 新权重生效 |

#### 权限测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| Viewer 读取 | viewer 角色 | 允许读取 | 返回数据 |
| Editor 写入 | editor 角色 | 允许写入 | 创建成功 |
| Admin 删除 | admin 角色 | 允许删除 | 软删除成功 |
| 跨级权限继承 | BusinessLine editor | 可编辑子 System/Project | 继承权限生效 |
| 无权访问 | viewer 访问他人资源 | 拒绝 | 返回 403 |

#### FinOps 测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| CSV 导出 | period=2026-04 | 返回 CSV 数据 | 格式正确，内容完整 |
| 按期间过滤 | 无效期间 | 返回空或错误 | 错误信息明确 |
| 资源类型汇总 | - | 按类型分组统计 | 数量正确 |

#### 审计日志测试

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 创建记录 | BusinessLine 创建 | 日志记录 | action=create, entity_type=business_line |
| 更新记录 | System 更新 | 日志记录 | changes 字段包含旧值和新值 |
| 删除记录 | Project 删除 | 日志记录 | action=delete, entity_id 匹配 |
| 查询过滤 | entity_type=project | 只返回项目日志 | 结果过滤正确 |
| 资源链接 | Project 链接资源 | 日志记录 | action=link_resource, resource_id 匹配 |
| 资源取消链接 | Project 取消链接资源 | 日志记录 | action=unlink_resource, resource_id 匹配 |
| 权重更新 | 更新资源权重 | 日志记录 | changes 包含 old_weight, new_weight |

---

## 14. Test Environment Strategy

### 14.1 概述

系统使用三层测试环境策略，平衡开发效率和集成测试真实性。

### 14.2 环境层级

```
┌─────────────────────────────────────────────────────────────┐
│  开发环境 (Development)                                      │
│  - 使用 Fake* Mock 客户端                                    │
│  - 无需外部依赖，快速迭代                                     │
│  - 适用：单元测试、快速验证、CI                              │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  集成测试环境 (Containerlab)                                 │
│  - Containerlab 模拟真实网络/数据中心/主机                     │
│  - 支持 vSphere、KVM、IPMI、SNMP 模拟                        │
│  - 适用：集成测试、端到端测试、预发布验证                      │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  生产环境 (Production)                                       │
│  - 真实硬件：vSphere、KVM、IPMI、SNMP 设备                    │
│  - 适用：最终验证、监控、生产数据                              │
└─────────────────────────────────────────────────────────────┘
```

### 14.3 Containerlab 模拟环境

Containerlab 可模拟以下基础设施：

| 模拟类型 | 组件 | 说明 |
|---------|------|------|
| 网络设备 | 交换机、路由器、防火墙 | SNMP 协议通信 |
| 计算节点 | KVM 虚拟机、IPMI 物理主机 | SSH + IPMI 协议 |
| 虚拟化平台 | vSphere/ESXi 集群 | vSphere API |
| 数据中心 | DC1、DC2 双数据中心 | 网络隔离拓扑 |

**Containerlab 配置示例:**
```yaml
# containerlab-topology.yml
name: devops-toolkit-test
topology:
  nodes:
    # DC1 交换机
    dc1-core-switch:
      kind: linux
      image: containerlab/sros:latest
    # DC1 KVM 主机
    dc1-kvm-host-01:
      kind: linux
      image: quay.io/davidhuster/kvm:latest
```

### 14.4 Mock 客户端切换

```go
// 根据配置选择客户端
var client interface {
    ListVMs(ctx context.Context, hostID string) ([]*VM, error)
    GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
}

// Mock 模式（开发环境）
if cfg.UseMock {
    client = fake.NewFakeVMwareClient()
} else {
    // 真实客户端（生产/集成测试）
    client = vmware.NewClient(cfg.VcenterURL, cfg.VcenterUser, cfg.VcenterPass)
}
```

### 14.5 环境配置

```yaml
# config.yaml
devops:
  environment: mock|containerlab|production
  containerlab:
    topology_file: containerlab-topology.yml
    kubeconfig_path: ~/.kube/config-devkit
```

### 14.6 测试矩阵

| 测试场景 | 环境 | 输入 | 预期结果 | 验证 |
|---------|------|------|---------|------|
| 设备列表 | Mock | 无 | 返回 Fake 数据 | 3 VM, 2 KVM, 3 网络设备 |
| 设备发现 | Containerlab | 模拟网络扫描 | 发现真实设备 | 设备状态 ACTIVE |
| 告警触发 | Mock | 告警条件 | Mock 发送通知 | 回调函数调用 |
| 状态同步 | Containerlab | 设备状态变更 | 状态更新 | 监控数据刷新 |
| 权限检查 | Mock | 不同角色 | 权限正确 | Auditor 只能读 |
| FinOps 导出 | Mock | period=2026-04 | CSV 生成 | 权重计算正确 |

---

## 15. Discovery System

### 15.1 概述

网络发现系统，通过主动探测发现设备。

### 15.2 发现流程

```
1. 扫描触发: POST /api/discovery/scan
2. 网络扫描: 探测 172.30.30.0/24
   - TCP 22 (SSH) → 发现为 physical_host
   - UDP 161 (SNMP) → 发现为 network_device
3. 设备创建: PENDING 状态的设备
4. 用户审批: POST /api/discovery/register
5. 状态转换: PENDING → AUTHENTICATED → REGISTERED → ACTIVE
```

### 15.3 设备 ID 命名规范

| 设备 | ID | 类型 | 数据中心 |
|------|-----|------|---------|
| DC1 Web Server | `clab-dc1-web-21` | physical_host | dc1 |
| DC1 DB Server | `clab-dc1-db-22` | physical_host | dc1 |
| DC1 Core Switch | `clab-dc1-core-11` | network_device | dc1 |
| DC2 Web Server | `clab-dc2-web-41` | physical_host | dc2 |
| DC2 Core Switch | `clab-dc2-core-31` | network_device | dc2 |

### 15.4 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/discovery/scan` | POST | 触发网络发现扫描 |
| `/api/discovery/status` | GET | 获取上次扫描状态 |
| `/api/discovery/register` | POST | 注册发现的设备 |

### 15.5 测试矩阵

| 测试场景 | 输入 | 预期结果 | 验证 |
|---------|------|---------|------|
| 扫描触发 | 有效网络范围 | 扫描开始 | 状态变为 scanning |
| SSH 发现 | 真实 SSH 服务 | 发现 physical_host | IP、端口正确 |
| SNMP 发现 | 真实 SNMP 服务 | 发现 network_device | 设备类型正确 |
| 注册设备 | 发现列表 | 批量注册 | 状态转换正确 |

---

## 14. Test Environment

### 14.1 Containerlab 双数据中心拓扑

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                               Containerlab Network                                  │
│                                  172.30.30.0/24                                     │
│                                                                                     │
│  ┌─────────────────────────┐                           ┌─────────────────────────┐│
│  │        DC1 (Left)        │                           │       DC2 (Right)      ││
│  │       10.0.1.0/24        │                           │       10.0.2.0/24      ││
│  │                           │                           │                           ││
│  │      dc1-sw1 ────────────╫═══════════════════════════╫───────────── dc2-sw1   ││
│  │       (Core)             ╫══════ Dual Trunk ══════════╫══        (Core)       ││
│  │          │                ╫═══════════════════════════╫══          │          ││
│  │      dc1-sw2              ╫═══════════════════════════╫══      dc2-sw2          ││
│  │    (Distribution)          ╫═══════════════════════════╫══   (Distribution)    ││
│  │       │  │                 ╫═══════════════════════════╫══        │  │         ││
│  │    ┌──┴──┐                ╫═══════════════════════════╫══     ┌──┴──┐         ││
│  │    │     │                ╫═══════════════════════════╫══     │     │         ││
│  │    │W    │D                ╫═══════════════════════════╫══    │W    │D         ││
│  │    │eb   │b                ╫═══════════════════════════╫══    │eb   │b         ││
│  │    │1    │1                ╫═══════════════════════════╫══    │2    │2         ││
│  │    └─────┘                ╫═══════════════════════════╫══    └─────┘         ││
│  └────────────────────────────╫════════════════════════════════════════════════─┘│
│                                 │                                                        │
│                                 │         ┌───────────────────────────────────────┐  │
│                                 └─────────│        Time Series Databases          │  │
│                                            │  InfluxDB :8086  Prometheus :9090   │  │
│                                            │  Grafana  :3001                       │  │
│                                            └───────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────────────────┘

W = Web Server (Ubuntu + SSH)
D = DB Server (Ubuntu + SSH)
sw = Switch (Ubuntu + SNMP daemon)
```

### 14.2 节点清单

| 节点 | 角色 | MGMT IP | 服务 | 端口 |
|------|------|---------|------|------|
| dc1-sw1 | DC1 Core Switch | 172.30.30.11 | SNMP | UDP:161 |
| dc1-sw2 | DC1 Distribution | 172.30.30.12 | SNMP | UDP:161 |
| dc1-web | DC1 Web Server | 172.30.30.21 | SSH | TCP:22 |
| dc1-db | DC1 DB Server | 172.30.30.22 | SSH | TCP:22 |
| dc2-sw1 | DC2 Core Switch | 172.30.30.31 | SNMP | UDP:161 |
| dc2-sw2 | DC2 Distribution | 172.30.30.32 | SNMP | UDP:161 |
| dc2-web | DC2 Web Server | 172.30.30.41 | SSH | TCP:22 |
| dc2-db | DC2 DB Server | 172.30.30.42 | SSH | TCP:22 |

### 14.3 高可用特性

1. **双数据中心** - DC1 和 DC2 独立部署
2. **双 Trunk 链路** - dc1-sw1 和 dc2-sw1 之间两条 trunk 连接 (eth2, eth3)
3. **交换机架构** - 每个 DC 有核心交换机 + 分布交换机
4. **SSH 服务** - 每个服务器运行真实 SSH 守护进程

### 14.4 设备模拟矩阵

| 设备类型 | 模拟方法 | 连接 | 端口 | 用途 |
|---------|---------|------|------|------|
| PhysicalHost | Docker + openssh-server | SSH 真实连接 | TCP:22 | 指标采集、配置下发 |
| NetworkDevice | Docker + net-snmpd | SNMP 真实连接 | UDP:161 | 流量监控、端口状态 |
| Container | Docker native | Docker API | - | K8s 集群管理 |

### 14.5 目录结构

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

### 14.6 模拟指标

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

### 14.7 测试命令

```bash
# 运行所有测试
go test ./...

# 详细输出
go test ./... -v

# 集成测试
./scripts/integration-test.sh

# 前端测试
node devops-toolkit/frontend/frontend.test.js
```

### 14.8 测试覆盖率目标

| 模块 | 目标覆盖率 |
|------|---------|
| Device Management | 80% |
| CI/CD Pipeline | 75% |
| Logging System | 80% |
| Monitoring | 70% |
| Alerts | 75% |
| K8s Multi-Cluster | 80% |
| Physical Host | 75% |
| Project Management | 80% |

---

## 附录 A: Docker 开发环境

```bash
# 启动完整基础设施
docker-compose -f docker-compose.dev.yml up -d

# 服务端口
# - Elasticsearch: http://localhost:9200
# - Kibana:       http://localhost:5601
# - Loki:         http://localhost:3100
# - Grafana:      http://localhost:3001
# - Prometheus:   http://localhost:9090
# - DevOps Toolkit: http://localhost:3000
```

---

**文档状态**: Active
**Owner**: DevOps Team