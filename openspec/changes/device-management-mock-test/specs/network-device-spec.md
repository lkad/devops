# Network Device Spec

## Overview

网络设备管理规格，涵盖交换机、路由器、防火墙、负载均衡器等网络设备的管理、监控和配置管理。

## Data Model

### NetworkDevice Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| DeviceType | string | 设备类型 | switch, router, firewall, loadbalancer |
| Vendor | string | 厂商 | Cisco, Juniper, Huawei, H3C, Arista |
| Model | string | 型号 | Catalyst 9300, MX240, USG6555E |
| OSVersion | string | 操作系统版本 | 17.6.1, 12.3R12.3 |
| SerialNo | string | 序列号 | FCW2233L0AA |
| FirmwareVersion | string | 固件版本 | 5.0.1 |
| **管理** | | | |
| MgmtIP | string | 管理网 IP | 192.168.1.1 |
| MgmtVRF | string | 管理 VRF | management |
| ConsoleIP | string | Console Server IP | 192.168.1.254 |
| ConsolePort | string | Console 端口 | 1/A/2 |
| **网络接口** | | | |
| Interfaces | JSON | 端口配置列表 | (见下方) |
| PortChannels | JSON | LACP 链路聚合 | (见下方) |
| VLANS | JSON | VLAN 配置 | (见下方) |
| SpanningTree | JSON | 生成树状态 | |
| **路由交换** | | | |
| RoutingInstances | JSON | VRF/路由实例 | |
| BGPConfig | JSON | BGP 邻居配置 | |
| OSPFConfig | JSON | OSPF 配置 | |
| StaticRoutes | JSON | 静态路由 | |
| **安全** | | | |
| ACLConfig | JSON | 访问控制列表 | |
| PortSecurity | JSON | 端口安全 | |
| Dot1xConfig | JSON | 802.1X 配置 | |
| **配置管理** | | | |
| ConfigBackupLast | datetime | 上次备份时间 | |
| ConfigBackupStatus | string | 备份状态 | success, failed, pending |
| ConfigDiffFromBaseline | bool | 相比基线是否有变更 | true |
| **SNMP** | | | |
| SNMPReadCommunity | string | SNMP 只读 community | |
| SNMPWriteCommunity | string | SNMP 写 community | |
| SNMPv3Config | JSON | SNMPv3 配置 | |
| SNMPTrapTargets | JSON | Trap 目标地址 | |

### NetworkInterface Structure

```json
{
  "interfaces": [
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
    },
    {
      "name": "GigabitEthernet0/0/2",
      "description": "To-Access-SW-01",
      "admin_status": "up",
      "oper_status": "up",
      "speed": "1000M",
      "layer": 2,
      "vlan_access": 200,
      "trunk_vlans": [10, 20]
    }
  ]
}
```

### PortChannel Structure

```json
{
  "port_channels": [
    {
      "name": "Port-Channel1",
      "members": ["Gi0/0/1", "Gi0/0/2"],
      "mode": "active",
      "status": "up",
      "speed": "20G"
    }
  ]
}
```

## State Machine

```
  ┌──────────┐
  │ pending  │ (新设备注册)
  └────┬─────┘
       ▼
  ┌──────────┐     ┌────────────┐
  │discovered│────▶│   active   │
  └────┬─────┘     └─────┬──────┘
       │ IPMI/SSH失败      │ 配置变更/故障
       ▼                  ▼
  ┌──────────┐      ┌──────────┐
  │ failed   │      │maintenance│
  └──────────┘      └─────┬──────┘
                          │
                     维护完成
                          │
                          ▼
                     ┌──────────┐
                     │  retired │
                     └──────────┘
```

| State | Description |
|-------|-------------|
| pending | 初始录入 |
| discovered | 发现完成，配置收集中 |
| active | 正常运行 |
| maintenance | 维护中 |
| failed | 连接/认证失败 |
| retired | 已退役 |

## Device Types

| Type | Description | Examples |
|------|-------------|----------|
| switch | 二层/三层交换机 | Cisco Catalyst, Juniper EX, Huawei S |
| router | 路由器 | Cisco ASR, Juniper MX, Huawei AR |
| firewall | 防火墙 | USG, ASA, Fortinet, PanOS |
| loadbalancer | 负载均衡器 | F5, A10, Citrix ADC |
| wireless | 无线控制器 | Cisco WLC, Aruba AirWave |
| storage | 存储网络设备 | EMC Connectrix, IBM SAN |
| other | 其他网络设备 | Console Server, PDU, KVM |

## API Endpoints

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
| POST | /api/devices/network/:id/discover | 触发发现 |

## Query Parameters

```
GET /api/devices/network?type=switch&vendor=Cisco&state=active&location=DC1&page=1&page_size=20
```

| Parameter | Type | Description |
|-----------|------|-------------|
| type | string | 设备类型 |
| vendor | string | 厂商 |
| model | string | 型号 |
| state | string | 状态 |
| location | string | 机房位置 |
| mgmt_ip | string | 管理 IP (精确匹配) |
| labels | string | 标签过滤 |
| page | int | 页码 |
| page_size | int | 每页数量 |

## Response Format

```json
{
  "items": [
    {
      "id": "sw-001",
      "name": "core-switch-01",
      "device_type": "switch",
      "vendor": "Cisco",
      "model": "Catalyst 9300",
      "os_version": "17.6.1",
      "serial_no": "FCW2233L0AA",
      "state": "active",
      "mgmt_ip": "192.168.1.1",
      "location": "DC1-A",
      "interfaces_count": 48,
      "vlans_count": 20,
      "up_ports": 45,
      "down_ports": 3,
      "last_backup": "2024-01-20T02:00:00Z",
      "backup_status": "success",
      "labels": {"env": "prod", "tier": "core"}
    }
  ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

## Validation Rules

| Field | Rules |
|-------|-------|
| Name | 必填, 1-64字符, 唯一 |
| DeviceType | 必填, 枚举: switch, router, firewall, loadbalancer, wireless, storage, other |
| Vendor | 必填, 枚举: Cisco, Juniper, Huawei, H3C, Arista, Fortinet, PaloAlto, other |
| MgmtIP | 有效的 IPv4 地址 |
| SerialNo | 唯一 |

## SNMP Configuration

```json
{
  "snmp_v2c": {
    "read_community": "public",
    "write_community": "private",
    "trap_community": "trap Community"
  },
  "snmp_v3": {
    "security_level": "authPriv",
    "auth_protocol": "SHA256",
    "auth_key": "********",
    "priv_protocol": "AES256",
    "priv_key": "********"
  },
  "trap_targets": [
    {"host": "192.168.1.50", "port": 162, "vrf": "management"}
  ]
}
```

## Relationships

- NetworkDevice 可以连接其他 NetworkDevice
- NetworkDevice 可以连接 PhysicalHost (管理网络)
- NetworkDevice 关联多个 VLAN
- NetworkDevice 可以属于多个 Project