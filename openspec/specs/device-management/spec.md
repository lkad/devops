# Device Management

## Overview

设备管理模块涵盖物理机、虚拟机、网络设备三大类别的完整生命周期管理，支持发现、监控、配置管理和告警。

## Device Types

| Type | Description | Examples |
|------|-------------|----------|
| `physical_host` | 物理服务器 | Dell R740, HP DL380, Lenovo SR650 |
| `container` | 容器实例 | Docker, Podman |
| `network_device` | 网络设备 | Cisco Catalyst, Juniper MX, Huawei USG |
| `load_balancer` | 负载均衡器 | F5, Citrix ADC, AWS ALB |
| `cloud_instance` | 云主机 | AWS EC2, Azure VM, GCP Instance |
| `iot_device` | 物联网设备 | 传感器、网关 |

## Data Models

### 1. Physical Host (物理主机)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| manufacturer | string | 厂商 | Dell, HP, Lenovo |
| model | string | 型号 | PowerEdge R740 |
| serial_no | string | 序列号 | SN123456789 |
| bios_version | string | BIOS版本 | 2.5.3 |
| hardware_uuid | string | 硬件UUID | 4B4C4D4E-... |
| **计算资源** | | | |
| cpu_model | string | CPU型号 | Intel Xeon Gold 6248 |
| cpu_cores | int | 物理核心数 | 32 |
| cpu_threads | int | 线程数 | 64 |
| memory_gb | int | 内存总容量 | 128 |
| memory_slots_used | int | 已用插槽 | 8 |
| memory_slots_total | int | 总插槽 | 16 |
| **存储** | | | |
| disk_total_gb | int | 总存储 | 2000 |
| disk_config | JSON | 磁盘阵列配置 | RAID级别、硬盘详情 |
| **网络** | | | |
| mgmt_ip | string | 管理网IP | 192.168.1.101 |
| ipmi_ip | string | IPMI/BMC IP | 192.168.1.100 |
| mac_addresses | JSON | 所有网卡MAC | ["00:0c:29:...", "..."] |
| **机房信息** | | | |
| location | string | 数据中心/机房 | DC1-A |
| rack | string | 机柜编号 | A-01-20 (排-柜-U位) |
| rack_position | int | U位起始位置 | 20 |
| rack_units | int | 占用U数 | 2 |
| **电源** | | | |
| power_supply_count | int | 电源模块数 | 2 |
| power_consumption_watts | int | 实时功耗 | 450 |
| pdu_info | JSON | PDU连接信息 | {pdu: "A1", port: 12} |
| redundant_psu | bool | 是否冗余电源 | true |
| **资产** | | | |
| asset_no | string | 资产编号 | AST-2024-0001 |
| purchase_date | date | 采购日期 | 2024-01-15 |
| warranty_expire | date | 保修到期 | 2027-01-15 |
| owner_team | string | 归属团队 | infrastructure |

### 2. Virtual Machine (虚拟机)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| vm_id | string | 虚拟机唯一标识 | vm-100, instance-abc123 |
| name | string | 虚拟机名称 | web-server-01 |
| hypervisor_type | string | 虚拟化平台 | vsphere, kvm, xen, hyperv |
| hypervisor_host | string | 宿主物理机 ID | host-1 |
| resource_pool | string | 资源池 | RP-Production |
| cluster | string | 所属集群 | Cluster-01 |
| **计算资源** | | | |
| vcpu | int | 虚拟 CPU 数量 | 4 |
| vcpu_reservation | int | CPU 保留 (MHz) | 2000 |
| memory_mb | int | 内存 (MB) | 8192 |
| memory_reservation_mb | int | 内存保留 (MB) | 4096 |
| memory_limit_mb | int | 内存上限 | 16384 |
| **存储** | | | |
| disk_total_gb | int | 总磁盘容量 | 100 |
| disk_snapshot_count | int | 快照数量 | 2 |
| disk_datastore | string | 存储卷/数据存储 | datastore-1 |
| disk_path | string | VMDK/磁盘路径 | /vmfs/volumes/... |
| **网络** | | | |
| interfaces | JSON | 网卡配置列表 | [{name: "eth0", ...}] |
| ip_addresses | JSON | IP 地址列表 | ["192.168.1.10", "10.0.0.5"] |
| mac_address | string | 主 MAC 地址 | 00:0c:29:ab:cd:ef |
| port_group | string | 虚拟端口组 | VM Network |
| **状态** | | | |
| power_state | string | 电源状态 | on, off, suspended |
| guest_os | string | 操作系统 | ubuntu 22.04, windows 2019 |
| tools_status | string | VM Tools 状态 | running, outdated, not installed |
| **生命周期** | | | |
| created_time | datetime | 创建时间 | |
| template | string | 是否从模板部署 | template-name |
| template_name | string | 模板名称 | ubuntu-22.04-base |

### 3. Network Device (网络设备)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| device_type | string | 设备类型 | switch, router, firewall, loadbalancer |
| vendor | string | 厂商 | Cisco, Juniper, Huawei, H3C, Arista |
| model | string | 型号 | Catalyst 9300, MX240, USG6555E |
| os_version | string | 操作系统版本 | 17.6.1, 12.3R12.3 |
| serial_no | string | 序列号 | FCW2233L0AA |
| firmware_version | string | 固件版本 | 5.0.1 |
| **管理** | | | |
| mgmt_ip | string | 管理网 IP | 192.168.1.1 |
| mgmt_vrf | string | 管理 VRF | management |
| console_ip | string | Console Server IP | 192.168.1.254 |
| console_port | string | Console 端口 | 1/A/2 |
| **网络接口** | | | |
| interfaces | JSON | 端口配置列表 | (见 Interface 结构) |
| port_channels | JSON | LACP 链路聚合 | (见 PortChannel 结构) |
| vlans | JSON | VLAN 配置 | |
| spanning_tree | JSON | 生成树状态 | |
| **路由交换** | | | |
| routing_instances | JSON | VRF/路由实例 | |
| bgp_config | JSON | BGP 邻居配置 | |
| ospf_config | JSON | OSPF 配置 | |
| static_routes | JSON | 静态路由 | |
| **安全** | | | |
| acl_config | JSON | 访问控制列表 | |
| port_security | JSON | 端口安全 | |
| dot1x_config | JSON | 802.1X 配置 | |
| **配置管理** | | | |
| config_backup_last | datetime | 上次备份时间 | |
| config_backup_status | string | 备份状态 | success, failed, pending |
| config_diff_from_baseline | bool | 相比基线是否有变更 | true |
| **SNMP** | | | |
| snmp_read_community | string | SNMP 只读 community | |
| snmp_write_community | string | SNMP 写 community | |
| snmp_v3_config | JSON | SNMPv3 配置 | |
| snmp_trap_targets | JSON | Trap 目标地址 | |

## State Machines

### Physical Host State Machine

```
pending → authenticated → registered → active
                                      ↓
                              maintenance ←→ suspended
                                      ↓
                                    retired
```

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| pending | 初始录入，未验证 | authenticated, failed |
| authenticated | IPMI/SSH 验证通过 | registered, failed |
| registered | 已分配资源池 | active, retired |
| active | 正常运行 | maintenance, suspended |
| maintenance | 维护中 | active |
| suspended | 暂停使用 | active, retired |
| retired | 已退役 | (terminal) |
| failed | 验证/连接失败 | (需手动处理) |

### VM State Machine

```
pending → running ←→ suspended
                ↓
              stopped → retired
```

| State | Description |
|-------|-------------|
| pending | 创建中/克隆中 |
| running | 运行中 |
| suspended | 暂停 (内存挂起) |
| stopped | 关机 |
| retired | 已销毁 |

### Network Device State Machine

```
pending → discovered → active ←→ maintenance
                           ↓
                         failed
                           ↓
                        retired
```

## API Endpoints

### Physical Host

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/devices/physical | 列表 (分页、过滤) |
| POST | /api/devices/physical | 创建 |
| GET | /api/devices/physical/:id | 获取单个 |
| PUT | /api/devices/physical/:id | 更新 |
| DELETE | /api/devices/physical/:id | 删除 |
| POST | /api/devices/physical/:id/discover | 触发发现 |
| POST | /api/devices/physical/:id/metrics | 采集指标 |
| PUT | /api/devices/physical/:id/state | 状态变更 |

### Virtual Machine

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/devices/vm | 列表 (分页、过滤) |
| POST | /api/devices/vm | 创建/注册 |
| GET | /api/devices/vm/:id | 获取单个 |
| PUT | /api/devices/vm/:id | 更新 |
| DELETE | /api/devices/vm/:id | 删除 |
| POST | /api/devices/vm/:id/power-on | 开机 |
| POST | /api/devices/vm/:id/power-off | 关机 |
| POST | /api/devices/vm/:id/suspend | 暂停 |
| POST | /api/devices/vm/:id/resume | 恢复 |
| POST | /api/devices/vm/:id/snapshot | 创建快照 |
| GET | /api/devices/vm/:id/metrics | 获取监控指标 |
| GET | /api/devices/vm/:id/topology | 获取拓扑关系 |

### Network Device

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/devices/network | 列表 (分页、过滤) |
| POST | /api/devices/network | 创建 |
| GET | /api/devices/network/:id | 获取单个 |
| PUT | /api/devices/network/:id | 更新 |
| DELETE | /api/devices/network/:id | 删除 |
| GET | /api/devices/network/:id/interfaces | 获取端口列表 |
| GET | /api/devices/network/:id/vlans | 获取 VLAN 配置 |
| POST | /api/devices/network/:id/backup | 触发配置备份 |
| GET | /api/devices/network/:id/config-diff | 获取配置差异 |
| POST | /api/devices/network/:id/metrics | 采集指标 |

## Network Interface Structure

```json
{
  "name": "GigabitEthernet0/0/1",
  "description": "To-Core-SW-01",
  "admin_status": "up",
  "oper_status": "up",
  "speed": "1000M",
  "duplex": "full",
  "mtu": 1500,
  "vlan_access": 100,
  "vlan_trunk": [10, 20, 30],
  "layer": 3,
  "ip_address": "10.0.1.2/24",
  "pvid": 100,
  "poe_status": "enabled",
  "mac_address": "aabb.ccdd.eeff",
  "counters": {
    "in_bytes": 1234567890,
    "out_bytes": 9876543210,
    "in_errors": 0,
    "out_errors": 0
  }
}
```

## Requirements

### Requirement: Device State Machine
The system SHALL enforce a strict device state machine with validated transitions.

#### Scenario: Valid state transition
- **WHEN** device in AUTHENTICATED state receives register action
- **THEN** system transitions device to REGISTERED state

#### Scenario: Invalid state transition
- **WHEN** device in PENDING state receives activate action
- **THEN** system rejects transition with 400 and error message

### Requirement: Device Types
The system SHALL support multiple device types: PhysicalHost, Container, NetworkDevice, LoadBalancer, CloudInstance, IoT_Device.

### Requirement: Device Hierarchy
The system SHALL support parent-child relationships between devices.

#### Scenario: Create device hierarchy
- **WHEN** user creates PhysicalHost, then adds Container as child
- **THEN** system establishes parent-child relationship

### Requirement: Device Groups
The system SHALL support flat, hierarchical, and dynamic device grouping.

### Requirement: Configuration Templates
The system SHALL support Jinja2-style configuration templates with inheritance.

### Requirement: Device Search
The system SHALL support searching devices by tags via GET /api/devices/search?tag=label=value.

### Requirement: Device Actions
The system SHALL support executing actions on devices via POST /api/devices/:id/actions.

## Validation Rules

### Physical Host

| Field | Rules |
|-------|-------|
| Name | 必填, 1-64字符, 唯一 |
| SerialNo | 必填, 唯一 |
| Manufacturer | 必填, 枚举: Dell, HP, Lenovo, Huawei, Inspur, SuperMicro |
| MgmtIP | 有效的 IPv4 地址 |
| IPMIIP | 有效的 IPv4 地址 |
| Rack | 格式: `^[A-Z]-\d{2}-\d{2}$` |
| MemoryGB | > 0, <= 8192 |
| CPUCores | > 0, <= 256 |

### Virtual Machine

| Field | Rules |
|-------|-------|
| Name | 必填, 1-128字符 |
| VMID | 必填, 唯一 |
| HypervisorType | 必填, 枚举: vsphere, kvm, xen, hyperv |
| VCPU | > 0, <= 256 |
| MemoryMB | > 512, <= 65536 |
| DiskTotalGB | > 0, <= 65536 |

### Network Device

| Field | Rules |
|-------|-------|
| Name | 必填, 1-64字符, 唯一 |
| DeviceType | 必填, 枚举: switch, router, firewall, loadbalancer, wireless, storage, other |
| Vendor | 必填, 枚举: Cisco, Juniper, Huawei, H3C, Arista, Fortinet, PaloAlto, other |
| MgmtIP | 有效的 IPv4 地址 |
| SerialNo | 唯一 |

## Monitoring Metrics

### VM Metrics

| Metric | Type | Description |
|--------|------|-------------|
| cpu_usage | float | CPU 使用率 (%) |
| mem_usage | float | 内存使用率 (%) |
| disk_iops | int | 磁盘 IOPS |
| disk_mbps | int | 磁盘吞吐量 (MB/s) |
| net_rx_mbps | float | 网络接收速率 (Mbps) |
| net_tx_mbps | float | 网络发送速率 (Mbps) |

### Host Metrics

| Metric | Type | Description |
|--------|------|-------------|
| cpu_usage | float | CPU 使用率 (%) |
| memory_usage | float | 内存使用率 (%) |
| memory_total_gb | int | 总内存 (GB) |
| memory_used_gb | int | 已用内存 (GB) |
| disk_usage_percent | float | 磁盘使用率 (%) |
| power_watts | int | 功耗 (W) |
| temp_celsius | int | 温度 (°C) |

### Network Device Metrics

| Metric | Type | Description |
|--------|------|-------------|
| cpu_usage | float | CPU 使用率 (%) |
| memory_usage | float | 内存使用率 (%) |
| temperature | int | 温度 (°C) |
| interface_stats | map | 各端口统计 |

## Relationships

- PhysicalHost 可以是多个 VM 的宿主 (1:N)
- VM 可以有一个 PhysicalHost 作为宿主
- PhysicalHost 属于一个 BusinessLine
- NetworkDevice 可以连接其他 NetworkDevice
- NetworkDevice 可以连接 PhysicalHost (管理网络)
- 所有设备类型可以关联多个 Project
- 所有设备类型支持 Labels 标签分类