# Virtual Machine Spec

## Overview

虚拟机管理规格，涵盖 VM 从创建到销毁的完整生命周期，支持 vSphere、KVM、Xen、Hyper-V 等虚拟化平台。

## Data Model

### VM Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| VMID | string | 虚拟机唯一标识 (平台原生ID) | vm-100, instance-abc123 |
| Name | string | 虚拟机名称 | web-server-01 |
| HypervisorType | string | 虚拟化平台 | vsphere, kvm, xen, hyperv |
| HypervisorHost | string | 宿主物理机 ID | host-1 |
| ResourcePool | string | 资源池 | RP-Production |
| Cluster | string | 所属集群 | Cluster-01 |
| **计算资源** | | | |
| VCPU | int | 虚拟 CPU 数量 | 4 |
| VCPUReservation | int | CPU 保留 (MHz) | 2000 |
| MemoryMB | int | 内存 (MB) | 8192 |
| MemoryReservationMB | int | 内存保留 (MB) | 4096 |
| MemoryLimitMB | int | 内存上限 | 16384 |
| **存储** | | | |
| DiskTotalGB | int | 总磁盘容量 | 100 |
| DiskSnapshotCount | int | 快照数量 | 2 |
| DiskDatastore | string | 存储卷/数据存储 | datastore-1 |
| DiskPath | string | VMDK/磁盘路径 | /vmfs/volumes/... |
| **网络** | | | |
| Interfaces | JSON | 网卡配置列表 | [{name: "eth0", ...}] |
| IPAddresses | JSON | IP 地址列表 | ["192.168.1.10", "10.0.0.5"] |
| MACAddress | string | 主 MAC 地址 | 00:0c:29:ab:cd:ef |
| PortGroup | string | 虚拟端口组 | VM Network |
| **状态** | | | |
| PowerState | string | 电源状态 | on, off, suspended |
| GuestOS | string | 操作系统 | ubuntu 22.04, windows 2019 |
| ToolsStatus | string | VM Tools 状态 | running, outdated, not installed |
| **生命周期** | | | |
| CreatedTime | datetime | 创建时间 | |
| Template | string | 是否从模板部署 | template-name |
| TemplateName | string | 模板名称 | ubuntu-22.04-base |

### VM Interface Structure (JSON)

```json
{
  "interfaces": [
    {
      "name": "eth0",
      "type": "virtio",
      "mac_address": "00:0c:29:ab:cd:ef",
      "ip_addresses": ["192.168.1.10"],
      "port_group": "VM Network",
      "status": "connected",
      "speed": "10G"
    },
    {
      "name": "eth1",
      "type": "virtio",
      "mac_address": "00:0c:29:ab:cd:f0",
      "ip_addresses": ["10.0.0.5"],
      "port_group": "Management",
      "status": "connected",
      "speed": "1G"
    }
  ]
}
```

## State Machine

```
            ┌──────────┐
            │ pending  │ (VM 创建中)
            └────┬─────┘
                 ▼
            ┌──────────┐
    ┌───────│ running  │◀──────┐
    │       └────┬─────┘       │
    │            │             │
    │      ┌─────┴─────┐       │
    │      ▼           ▼       │
    │  ┌───────┐  ┌─────────┐  │
    │  │suspended│ │stopped  │  │
    │  └───┬───┘  └────┬────┘  │
    │      │          │       │
    └──────┘          │       │
         (resume)     │ (start)
                       │
                       ▼
                 ┌──────────┐
                 │ retired  │ (销毁)
                 └──────────┘
```

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| pending | 创建中/克隆中 | running, failed |
| running | 运行中 | suspended, stopped |
| suspended | 暂停 (内存挂起) | running |
| stopped | 关机 | running, retired |
| failed | 创建/操作失败 | pending (重试) |
| retired | 已销毁 | (terminal) |

## API Endpoints

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
| POST | /api/devices/vm/discover | 触发发现 (从 hypervisor) |

## Query Parameters

```
GET /api/devices/vm?hypervisor=host-1&state=running&cluster=Production&page=1&page_size=50
```

| Parameter | Type | Description |
|-----------|------|-------------|
| hypervisor | string | 宿主机 ID |
| hypervisor_type | string | 虚拟化类型: vsphere, kvm, xen, hyperv |
| cluster | string | 集群名称 |
| resource_pool | string | 资源池 |
| state | string | 电源状态 |
| labels | string | 标签过滤 |
| page | int | 页码 |
| page_size | int | 每页数量 |

## Response Format

```json
{
  "items": [
    {
      "id": "vm-100",
      "vm_id": "500a1234-5678-90ab-cdef-111111111111",
      "name": "web-server-01",
      "hypervisor_type": "vsphere",
      "hypervisor_host": "host-1",
      "cluster": "Production",
      "vcpu": 4,
      "memory_mb": 8192,
      "disk_total_gb": 100,
      "power_state": "running",
      "guest_os": "ubuntu 22.04",
      "ip_addresses": ["192.168.1.10", "10.0.0.5"],
      "created_time": "2024-01-15T10:00:00Z",
      "labels": {"env": "prod", "tier": "web"}
    }
  ],
  "total": 150,
  "page": 1,
  "page_size": 50,
  "breakdown": {
    "vsphere": 80,
    "kvm": 50,
    "hyperv": 20
  }
}
```

## Topology / Relationships

```json
// GET /api/devices/vm/:id/topology
{
  "vm": {
    "id": "vm-100",
    "name": "web-server-01"
  },
  "host": {
    "id": "host-1",
    "name": "esxi-host-01",
    "type": "physical"
  },
  "resource_pool": {
    "id": "rp-01",
    "name": "Production"
  },
  "cluster": {
    "id": "cluster-01",
    "name": "Production Cluster"
  },
  "datastore": {
    "id": "ds-01",
    "name": "SAN-DS-01"
  },
  "network": [
    {"name": "VM Network", "vlan": 100},
    {"name": "Management", "vlan": 200}
  ]
}
```

## Validation Rules

| Field | Rules |
|-------|-------|
| Name | 必填, 1-128字符 |
| VMID | 必填, 唯一 |
| HypervisorType | 必填, 枚举: vsphere, kvm, xen, hyperv |
| VCPU | > 0, <= 256 |
| MemoryMB | > 512, <= 65536 |
| DiskTotalGB | > 0, <= 65536 |

## Relationships

- VM 属于一个 PhysicalHost (宿主)
- VM 可以关联多个 Project
- VM 关联多个 Resource
- VM 可以有多个 Snapshot