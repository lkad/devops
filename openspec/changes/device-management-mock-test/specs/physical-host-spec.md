# Physical Host Spec

## Overview

物理主机管理规格，涵盖服务器从上架到退役的完整生命周期管理。

## Data Model

### GORMModel (Common)

所有设备继承的公共字段：

| Field | Type | Description |
|-------|------|-------------|
| ID | uint | 主键 |
| UUID | string | 唯一标识 (UUID) |
| Name | string | 设备名称 |
| Type | string | 设备类型: `physical`, `vm`, `network` |
| State | string | 状态 |
| Labels | JSON | 标签 key-value |
| CreatedAt | datetime | 创建时间 |
| UpdatedAt | datetime | 更新时间 |

### PhysicalHost Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| **基础信息** | | | |
| Manufacturer | string | 厂商 | Dell, HP, Lenovo |
| Model | string | 型号 | PowerEdge R740 |
| SerialNo | string | 序列号 | SN123456789 |
| BIOSVersion | string | BIOS版本 | 2.5.3 |
| HardwareUUID | string | 硬件UUID | 4B4C4D4E-... |
| **计算资源** | | | |
| CPUModel | string | CPU型号 | Intel Xeon Gold 6248 |
| CPUCores | int | 物理核心数 | 32 |
| CPUThreads | int | 线程数 | 64 |
| MemoryGB | int | 内存总容量 | 128 |
| MemorySlotsUsed | int | 已用插槽 | 8 |
| MemorySlotsTotal | int | 总插槽 | 16 |
| **存储** | | | |
| DiskTotalGB | int | 总存储 | 2000 |
| DiskConfig | JSON | 磁盘阵列配置 | RAID, 硬盘详情 |
| **网络** | | | |
| MgmtIP | string | 管理网IP | 192.168.1.101 |
| IPMIIP | string | IPMI/BMC IP | 192.168.1.100 |
| MACAddresses | JSON | 所有网卡MAC | ["00:0c:29:...", "..."] |
| **机房信息** | | | |
| Location | string | 数据中心/机房 | DC1-A |
| Rack | string | 机柜编号 | A-01-20 (排-柜-U位) |
| RackPosition | int | U位起始位置 | 20 |
| RackUnits | int | 占用U数 | 2 |
| **电源** | | | |
| PowerSupplyCount | int | 电源模块数 | 2 |
| PowerConsumptionWatts | int | 实时功耗 | 450 |
| PDUInfo | JSON | PDU连接信息 | {pdu: "A1", port: 12} |
| RedundantPSU | bool | 是否冗余电源 | true |
| **资产** | | | |
| AssetNo | string | 资产编号 | AST-2024-0001 |
| PurchaseDate | date | 采购日期 | 2024-01-15 |
| WarrantyExpire | date | 保修到期 | 2027-01-15 |
| OwnerTeam | string | 归属团队 | infrastructure |

## State Machine

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

## API Endpoints

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

## Query Parameters

```
GET /api/devices/physical?state=active&location=DC1&manufacturer=Dell&page=1&page_size=20
```

| Parameter | Type | Description |
|-----------|------|-------------|
| state | string | 状态过滤 |
| location | string | 机房位置 |
| manufacturer | string | 厂商 |
| model | string | 型号 |
| rack | string | 机柜 |
| labels | string | 标签过滤 (key:value,key:value) |
| page | int | 页码 |
| page_size | int | 每页数量 |

## Response Format

```json
{
  "items": [
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "name": "dell-r740-01",
      "type": "physical",
      "state": "active",
      "manufacturer": "Dell",
      "model": "PowerEdge R740",
      "serial_no": "SN123456789",
      "cpu_cores": 32,
      "memory_gb": 128,
      "disk_total_gb": 2000,
      "location": "DC1-A",
      "rack": "A-01-20",
      "mgmt_ip": "192.168.1.101",
      "ipmi_ip": "192.168.1.100",
      "labels": {"env": "prod", "team": "infra"},
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 50,
  "page": 1,
  "page_size": 20
}
```

## Validation Rules

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

## Relationships

- PhysicalHost 可以是多个 VM 的宿主 (1:N)
- PhysicalHost 属于一个 BusinessLine
- PhysicalHost 关联多个 Resource