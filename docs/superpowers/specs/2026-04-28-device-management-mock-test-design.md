# 设备管理 Mock 测试方案设计

## 概述

本文档描述设备管理模块（Physical Host、VM、Network Device）的 Mock 测试方案，用于在无真实硬件环境下验证代码逻辑。

## 背景

设备管理涉及多种硬件平台的集成：
- **Physical Host**: IPMI、SSH 协议
- **Virtual Machine**: vSphere API、KVM/libvirt API
- **Network Device**: SNMP、SSH

在没有真实硬件环境时，使用 Mock 可以快速验证：
- 设备发现逻辑
- 监控指标采集
- 告警触发机制
- 状态机转换

## Mock 架构

```
┌──────────────────────────────────────────────────────────────┐
│                    测试架构                                   │
├────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌─────────────────┐       ┌──────────────────────────┐    │
│   │   Go Tests       │──────▶│  Hypervisor Mock         │    │
│   │  (被测代码)      │ HTTP  │  (Fake Response)         │    │
│   └─────────────────┘       └──────────────────────────┘    │
│           │                                                 │
│           │ 替换为真实 client                                │
│           ▼                                                 │
│   ┌─────────────────┐                                       │
│   │ Real Hypervisor │ (生产/集成测试)                        │
│   │ (vSphere/KVM)   │                                       │
│   └─────────────────┘                                       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## 1. Mock Hypervisor Client

### 目录结构

```
internal/
├── discovery/
│   ├── manager.go           # 发现管理器
│   ├── hypervisor.go        # 接口定义
│   ├── vsphere.go           # 真实 vSphere 客户端
│   ├── kvm.go               # 真实 KVM 客户端
│   └── fake/
│       ├── fake_vmware.go   # Mock vSphere
│       ├── fake_kvm.go      # Mock KVM
│       └── fake_metrics.go  # Mock 监控指标
```

### 接口定义

```go
// internal/discovery/hypervisor.go
type HypervisorClient interface {
    ListVMs(ctx context.Context, hostID string) ([]*VM, error)
    GetVM(ctx context.Context, vmID string) (*VM, error)
    GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    GetHostInfo(ctx context.Context, hostID string) (*HostInfo, error)
}
```

### Fake vSphere 实现

```go
// internal/discovery/fake/fake_vmware.go
type FakeVMwareClient struct {
    VMs      []*VM
    Hosts    []*PhysicalHost
    Metrics  map[string]*VMMetrics
}

func NewFakeVMwareClient() *FakeVMwareClient {
    return &FakeVMwareClient{
        VMs: []*VM{
            {
                ID:         "vm-100",
                Name:       "web-server-01",
                VCPU:       2,
                MemoryMB:   4096,
                State:      "running",
                Hypervisor: "host-1",
                IPAddresses: []string{"192.168.1.10"},
                MACAddress: "00:0c:29:ab:cd:ef",
                DiskGB:     50,
                CreatedAt:  time.Now().Add(-720 * time.Hour),
            },
            {
                ID:         "vm-101",
                Name:       "db-server-01",
                VCPU:       4,
                MemoryMB:   16384,
                State:      "running",
                Hypervisor: "host-1",
                IPAddresses: []string{"192.168.1.11"},
                DiskGB:     200,
            },
            {
                ID:         "vm-102",
                Name:       "cache-server-01",
                VCPU:       2,
                MemoryMB:   8192,
                State:      "running",
                Hypervisor: "host-2",
                IPAddresses: []string{"192.168.1.12"},
                DiskGB:     100,
            },
        },
        Hosts: []*PhysicalHost{
            {
                ID:           "host-1",
                Name:         "esxi-host-01",
                Type:         "physical",
                Manufacturer:  "Dell",
                Model:         "PowerEdge R740",
                SerialNo:      "SN123456",
                CPUCores:      32,
                MemoryGB:      128,
                State:         "active",
                MgmtIP:        "192.168.1.101",
            },
        },
        Metrics: make(map[string]*VMMetrics),
    }
}

func (f *FakeVMwareClient) ListVMs(ctx context.Context, hostID string) ([]*VM, error) {
    if hostID != "" {
        var result []*VM
        for _, vm := range f.VMs {
            if vm.Hypervisor == hostID {
                result = append(result, vm)
            }
        }
        return result, nil
    }
    return f.VMs, nil
}

func (f *FakeVMwareClient) GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error) {
    if _, ok := f.Metrics[vmID]; !ok {
        f.Metrics[vmID] = f.generateFakeMetrics(vmID)
    }
    return f.Metrics[vmID], nil
}

func (f *FakeVMwareClient) generateFakeMetrics(vmID string) *VMMetrics {
    rand.Seed(time.Now().UnixNano())
    return &VMMetrics{
        VMID:       vmID,
        CPUUsage:   45.5 + rand.Float64()*30,    // 45-75%
        MemUsage:   62.3 + rand.Float64()*20,    // 62-82%
        DiskIOPS:   1200 + rand.Intn(800),
        DiskMBps:   50 + rand.Intn(100),
        NetRXMbps:  100 + rand.Intn(400),
        NetTXMbps:  80 + rand.Intn(300),
        CollectedAt: time.Now(),
    }
}
```

### Fake KVM 实现

```go
// internal/discovery/fake/fake_kvm.go
type FakeKVMClient struct {
    VMs     []*VM
    Domains []*libvirt.Domain
}

func NewFakeKVMClient() *FakeKVMClient {
    return &FakeKVMClient{
        VMs: []*VM{
            {
                ID:         "kvm-vm-001",
                Name:       "kvm-web-01",
                VCPU:       4,
                MemoryMB:   8192,
                State:      "running",
                Hypervisor: "kvm-host-01",
                DiskGB:     80,
            },
        },
    }
}
```

## 2. Mock 监控指标

```go
// internal/discovery/fake/fake_metrics.go
type FakeMetricsCollector struct {
    BaseCPU    float64
    BaseMemory float64
}

func NewFakeMetricsCollector() *FakeMetricsCollector {
    return &FakeMetricsCollector{
        BaseCPU:    50.0,
        BaseMemory: 60.0,
    }
}

func (f *FakeMetricsCollector) CollectVMMetrics(vmID string) (*VMMetrics, error) {
    rand.Seed(time.Now().UnixNano())
    return &VMMetrics{
        VMID:        vmID,
        CPUUsage:    f.BaseCPU + rand.Float64()*30,
        MemUsage:    f.BaseMemory + rand.Float64()*25,
        DiskIOPS:    1000 + rand.Intn(1000),
        NetRXMbps:   50 + rand.Intn(500),
        NetTXMbps:   30 + rand.Intn(400),
        CollectedAt: time.Now(),
    }, nil
}

func (f *FakeMetricsCollector) CollectHostMetrics(hostID string) (*HostMetrics, error) {
    rand.Seed(time.Now().UnixNano())
    return &HostMetrics{
        HostID:           hostID,
        CPUUsage:         30 + rand.Float64()*40,
        MemoryUsage:      50 + rand.Float64()*35,
        MemoryTotalGB:    128,
        MemoryUsedGB:     64 + rand.Float64()*32,
        DiskUsagePercent: 45 + rand.Float64()*30,
        PowerWatts:       300 + rand.Intn(200),
        TempCelsius:      35 + rand.Intn(25),
        CollectedAt:      time.Now(),
    }, nil
}
```

## 3. Mock 网络设备

```go
// internal/discovery/fake/fake_network.go
type FakeNetworkDeviceClient struct {
    Devices []*NetworkDevice
}

func NewFakeNetworkDeviceClient() *FakeNetworkDeviceClient {
    return &FakeNetworkDeviceClient{
        Devices: []*NetworkDevice{
            {
                ID:         "sw-001",
                Name:       "core-switch-01",
                Type:       "switch",
                Vendor:     "Cisco",
                Model:      "Catalyst 9300",
                OSVersion:  "17.6.1",
                SerialNo:   "FCW2233L0AA",
                State:      "active",
                MgmtIP:     "192.168.1.1",
                Interfaces: []NetworkInterface{
                    {Name: "Gi0/0/1", Status: "up", Speed: "10G", VLAN: 100},
                    {Name: "Gi0/0/2", Status: "up", Speed: "10G", VLAN: 200},
                    {Name: "Gi0/0/3", Status: "down", Speed: "1G", VLAN: 300},
                },
                VLANS: []VLAN{
                    {ID: 100, Name: "VLAN100", Ports: 24},
                    {ID: 200, Name: "VLAN200", Ports: 12},
                },
            },
            {
                ID:         "fw-001",
                Name:       "edge-firewall-01",
                Type:       "firewall",
                Vendor:     "Huawei",
                Model:      "USG6555E",
                State:      "active",
                MgmtIP:     "192.168.1.2",
            },
        },
    }
}
```

## 4. 测试用例设计

```go
// internal/discovery/discovery_test.go
func TestVMDiscovery(t *testing.T) {
    fake := fake.NewFakeVMwareClient()
    mgr := discovery.NewManager(fake)

    // 测试 VM 发现
    vms, err := mgr.DiscoverVMs("")
    assert.NoError(t, err)
    assert.Len(t, vms, 3)

    // 验证 VM 属性
    assert.Equal(t, "web-server-01", vms[0].Name)
    assert.Equal(t, 2, vms[0].VCPU)
    assert.Equal(t, 4096, vms[0].MemoryMB)
}

func TestVMMetricsCollection(t *testing.T) {
    fake := fake.NewFakeVMwareClient()
    fakeMetrics := fake.NewFakeMetricsCollector()
    mgr := discovery.NewManager(fake, discovery.WithMetricsCollector(fakeMetrics))

    // 验证指标在合理范围
    metrics, err := mgr.CollectVMMetrics("vm-100")
    assert.NoError(t, err)
    assert.Greater(t, metrics.CPUUsage, 0.0)
    assert.Less(t, metrics.CPUUsage, 100.0)
    assert.Greater(t, metrics.MemUsage, 0.0)
    assert.Less(t, metrics.MemUsage, 100.0)
}

func TestVMAlerting(t *testing.T) {
    fake := &FakeVMwareClient{
        VMs: []*VM{
            {ID: "vm-100", Name: "high-cpu-vm", VCPU: 2, State: "running"},
        },
    }
    fakeMetrics := &FakeHighCPUCollector{} // 模拟 CPU > 80%
    alertsMgr := alerts.NewManager(metrics.NewCollector())

    // 触发告警检查
    hasAlert := alertsMgr.CheckVMThreshold(fakeMetrics, "vm-100", "cpu", 80.0)
    assert.True(t, hasAlert)
}

func TestPhysicalHostDiscovery(t *testing.T) {
    fake := &FakeIPMIClient{
        Hosts: []*PhysicalHost{
            {
                ID:           "host-001",
                Name:         "dell-r740-01",
                Manufacturer: "Dell",
                Model:        "PowerEdge R740",
                SerialNo:     "SNABC123",
                CPUCores:     32,
                MemoryGB:     128,
                State:        "active",
                MgmtIP:       "192.168.1.101",
                IPMIIP:       "192.168.1.100",
            },
        },
    }
    mgr := discovery.NewManager(fake)

    hosts, err := mgr.DiscoverHosts()
    assert.NoError(t, err)
    assert.Len(t, hosts, 1)
    assert.Equal(t, "Dell", hosts[0].Manufacturer)
}
```

## 5. 测试场景覆盖矩阵

| 场景 | Mock 数据 | 验证点 |
|------|-----------|--------|
| VM 发现 | 3 台不同规格 VM | 数量、属性正确 |
| VM 监控采集 | 动态指标 (45-75%) | 指标在合理范围 |
| VM 告警触发 | CPU > 80% | 告警正确触发 |
| VM 状态变更 | running → stopped | 状态机转换 |
| 物理机发现 | IPMI 返回信息 | 厂商、型号正确 |
| 物理机监控 | 温度、功耗、风扇 | 指标采集正确 |
| 网络设备发现 | Cisco/Juniper 设备 | 设备类型识别 |
| 配置变更告警 | 配置与基线不同 | diff 检测正确 |

## 6. 与真实环境切换

```go
// internal/discovery/manager.go
type Manager struct {
    hypervisor HypervisorClient  // 接口，运行时注入
    metrics    MetricsCollector
    network    NetworkDeviceClient
}

func NewManager(h HypervisorClient, opts ...Option) *Manager {
    m := &Manager{hypervisor: h}
    for _, opt := range opts {
        opt(m)
    }
    return m
}

// 生产环境
func NewRealManager() *Manager {
    return NewManager(
        vsphere.NewClient(vCenterAddr, creds),
        metrics.NewRealCollector(),
        snmp.NewClient(community),
    )
}

// 测试环境
func NewFakeManager() *Manager {
    return NewManager(
        fake.NewFakeVMwareClient(),
        fake.NewFakeMetricsCollector(),
        fake.NewFakeNetworkDeviceClient(),
    )
}
```

## 7. 实现优先级

| 优先级 | 组件 | 工作量 | 说明 |
|--------|------|--------|------|
| P0 | FakeVMwareClient | 2h | 覆盖 80% 测试场景 |
| P0 | FakeMetricsCollector | 1h | 指标采集验证 |
| P1 | FakeKVMClient | 2h | KVM 环境支持 |
| P1 | FakeNetworkDevice | 2h | SNMP 设备模拟 |
| P2 | FakeIPMIClient | 1h | 物理机 BMC 模拟 |
| P2 | 测试用例编写 | 4h | 覆盖主要场景 |

## 8. 文件清单

```
internal/discovery/
├── manager.go              # [修改] 支持选项模式注入
├── hypervisor.go           # [新增] 接口定义
├── vsphere.go              # [已有] 真实客户端
├── kvm.go                   # [已有] 真实客户端
└── fake/
    ├── fake_vmware.go      # [新增] Mock vSphere
    ├── fake_kvm.go         # [新增] Mock KVM
    ├── fake_metrics.go     # [新增] Mock 指标
    ├── fake_network.go     # [新增] Mock 网络设备
    └── fake_test.go        # [新增] 测试用例
```

## 总结

使用 Mock 方案可以：
- **快速验证** 代码逻辑，不依赖真实硬件
- **覆盖边界** CPU 100%、内存耗尽等极端场景
- **持续集成** 每次提交自动跑测试
- **团队协作** 多人同时开发，不抢硬件资源

后续接真实环境时，只需替换 `HypervisorClient` 实现类，业务逻辑无需修改。
