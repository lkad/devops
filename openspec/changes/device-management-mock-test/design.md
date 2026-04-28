# Device Management Mock Testing Design

## Overview

设计一套 Mock 测试框架，用于设备管理模块的单元测试和集成测试，支持物理机、虚拟机、网络设备的模拟测试。

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Mock Test Architecture                     │
├────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌─────────────────────────────────────────────────────┐   │
│   │                  Test Suite                          │   │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│   │  │ Unit Tests  │  │ Integration │  │  E2E Tests  │  │   │
│   │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │   │
│   └─────────┼───────────────┼───────────────┼─────────────┘   │
│             │               │               │                 │
│             ▼               ▼               ▼                 │
│   ┌─────────────────────────────────────────────────────┐   │
│   │              Discovery Manager                       │   │
│   │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│   │   │ VM Manager  │  │ Host Manager│  │Net Manager  │  │   │
│   │   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │   │
│   └─────────┼───────────────┼───────────────┼─────────────┘   │
│             │               │               │                 │
│             ▼               ▼               ▼                 │
│   ┌─────────────────────────────────────────────────────┐   │
│   │              Client Interface Layer                  │   │
│   │   ┌─────────────────────────────────────────────┐    │   │
│   │   │         HypervisorClient (interface)        │    │   │
│   │   └─────────────────────────────────────────────┘    │   │
│   └─────────────────────┬───────────────────────────────┘   │
│                         │                                    │
│         ┌───────────────┼───────────────┐                    │
│         ▼               ▼               ▼                    │
│   ┌───────────┐   ┌───────────┐   ┌───────────┐             │
│   │   Fake    │   │  Real     │   │  Fake     │             │
│   │ VMware   │   │ vSphere   │   │   KVM     │             │
│   └───────────┘   └───────────┘   └───────────┘             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## 1. Interface Definitions

### HypervisorClient Interface

```go
// internal/discovery/hypervisor.go
package discovery

type HypervisorClient interface {
    // VM Operations
    ListVMs(ctx context.Context, hostID string) ([]*VM, error)
    GetVM(ctx context.Context, vmID string) (*VM, error)
    GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)

    // Host Operations
    GetHostInfo(ctx context.Context, hostID string) (*PhysicalHost, error)
    GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)

    // Power Operations
    GetHostPowerState(ctx context.Context, hostID string) (string, error)
    SetHostPowerState(ctx context.Context, hostID string, state string) error
}
```

### MetricsCollector Interface

```go
// internal/discovery/metrics.go
type MetricsCollector interface {
    CollectVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    CollectHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
    CollectNetworkDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
}
```

### NetworkDeviceClient Interface

```go
// internal/discovery/network.go
type NetworkDeviceClient interface {
    ListDevices(ctx context.Context) ([]*NetworkDevice, error)
    GetDevice(ctx context.Context, deviceID string) (*NetworkDevice, error)
    GetDeviceInterfaces(ctx context.Context, deviceID string) ([]*NetworkInterface, error)
    GetDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
    BackupConfig(ctx context.Context, deviceID string) (string, error)
}
```

## 2. Fake Implementations

### 2.1 FakeVMwareClient

```go
// internal/discovery/fake/fake_vmware.go
package fake

import (
    "context"
    "math/rand"
    "sync"
    "time"
)

type FakeVMwareClient struct {
    mu     sync.RWMutex
    VMs    []*VM
    Hosts  []*PhysicalHost
    Metrics map[string]*VMMetrics

    // Configuration
    Latency time.Duration // Simulate network latency
    ErrorRate float64    // Simulate random errors
}

func NewFakeVMwareClient() *FakeVMwareClient {
    return &FakeVMwareClient{
        VMs: []*VM{
            {
                ID: "vm-100", Name: "web-server-01",
                VCPU: 2, MemoryMB: 4096, State: "running",
                Hypervisor: "host-1", DiskGB: 50,
                IPAddresses: []string{"192.168.1.10"},
                MACAddress: "00:0c:29:ab:cd:ef",
                CreatedAt: time.Now().Add(-720 * time.Hour),
            },
            {
                ID: "vm-101", Name: "db-server-01",
                VCPU: 4, MemoryMB: 16384, State: "running",
                Hypervisor: "host-1", DiskGB: 200,
                IPAddresses: []string{"192.168.1.11"},
            },
            {
                ID: "vm-102", Name: "cache-server-01",
                VCPU: 2, MemoryMB: 8192, State: "running",
                Hypervisor: "host-2", DiskGB: 100,
            },
        },
        Hosts: []*PhysicalHost{
            {
                ID: "host-1", Name: "esxi-host-01",
                Type: "physical", Manufacturer: "Dell",
                Model: "PowerEdge R740", SerialNo: "SN123456",
                CPUCores: 32, MemoryGB: 128, State: "active",
                MgmtIP: "192.168.1.101", IPMIIP: "192.168.1.100",
            },
        },
        Metrics: make(map[string]*VMMetrics),
        Latency: 50 * time.Millisecond,
        ErrorRate: 0.0,
    }
}

func (f *FakeVMwareClient) ListVMs(ctx context.Context, hostID string) ([]*VM, error) {
    if f.Latency > 0 {
        time.Sleep(f.Latency)
    }

    f.mu.RLock()
    defer f.mu.RUnlock()

    if f.ErrorRate > 0 && rand.Float64() < f.ErrorRate {
        return nil, errors.New("simulated vSphere API error")
    }

    if hostID == "" {
        return f.VMs, nil
    }

    var result []*VM
    for _, vm := range f.VMs {
        if vm.Hypervisor == hostID {
            result = append(result, vm)
        }
    }
    return result, nil
}

func (f *FakeVMwareClient) GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error) {
    f.mu.Lock()
    defer f.mu.Unlock()

    if _, ok := f.Metrics[vmID]; !ok {
        f.Metrics[vmID] = f.generateMetrics(vmID)
    }
    return f.Metrics[vmID], nil
}

func (f *FakeVMwareClient) generateMetrics(vmID string) *VMMetrics {
    rand.Seed(time.Now().UnixNano() + int64(len(vmID)))
    return &VMMetrics{
        VMID:       vmID,
        CPUUsage:   45.5 + rand.Float64()*30,
        MemUsage:   62.3 + rand.Float64()*20,
        DiskIOPS:   1200 + rand.Intn(800),
        DiskMBps:   50 + rand.Intn(100),
        NetRXMbps:  100 + rand.Intn(400),
        NetTXMbps:  80 + rand.Intn(300),
        CollectedAt: time.Now(),
    }
}
```

### 2.2 FakeMetricsCollector

```go
// internal/discovery/fake/fake_metrics.go
type FakeMetricsCollector struct {
    BaseCPU    float64
    BaseMemory float64
    BaseDisk   float64
    BaseNet    float64

    mu sync.Mutex
}

func NewFakeMetricsCollector() *FakeMetricsCollector {
    return &FakeMetricsCollector{
        BaseCPU:  50.0,
        BaseMemory: 60.0,
        BaseDisk: 40.0,
        BaseNet: 100.0,
    }
}

func (f *FakeMetricsCollector) CollectVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error) {
    rand.Seed(time.Now().UnixNano() + int64(len(vmID))*1000)

    f.mu.Lock()
    cpu := f.BaseCPU + rand.Float64()*30
    mem := f.BaseMemory + rand.Float64()*25
    f.mu.Unlock()

    return &VMMetrics{
        VMID:       vmID,
        CPUUsage:   cpu,
        MemUsage:   mem,
        DiskIOPS:   1000 + rand.Intn(1000),
        NetRXMbps:  50 + rand.Intn(500),
        NetTXMbps:  30 + rand.Intn(400),
        CollectedAt: time.Now(),
    }, nil
}

func (f *FakeMetricsCollector) CollectHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error) {
    rand.Seed(time.Now().UnixNano() + int64(len(hostID))*2000)

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

### 2.3 FakeNetworkDevice

```go
// internal/discovery/fake/fake_network.go
type FakeNetworkDeviceClient struct {
    Devices []*NetworkDevice
    mu      sync.RWMutex
}

func NewFakeNetworkDeviceClient() *FakeNetworkDeviceClient {
    return &FakeNetworkDeviceClient{
        Devices: []*NetworkDevice{
            {
                ID: "sw-001", Name: "core-switch-01",
                Type: "switch", Vendor: "Cisco",
                Model: "Catalyst 9300", OSVersion: "17.6.1",
                SerialNo: "FCW2233L0AA", State: "active",
                MgmtIP: "192.168.1.1",
                Interfaces: []NetworkInterface{
                    {Name: "Gi0/0/1", Status: "up", Speed: "10G", VLAN: 100},
                    {Name: "Gi0/0/2", Status: "up", Speed: "10G", VLAN: 200},
                    {Name: "Gi0/0/3", Status: "down", Speed: "1G", VLAN: 300},
                },
                VLANS: []VLAN{
                    {ID: 100, Name: "VLAN100"},
                    {ID: 200, Name: "VLAN200"},
                },
            },
            {
                ID: "fw-001", Name: "edge-firewall-01",
                Type: "firewall", Vendor: "Huawei",
                Model: "USG6555E", State: "active",
                MgmtIP: "192.168.1.2",
            },
        },
    }
}

func (f *FakeNetworkDeviceClient) ListDevices(ctx context.Context) ([]*NetworkDevice, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    return f.Devices, nil
}

func (f *FakeNetworkDeviceClient) GetDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error) {
    rand.Seed(time.Now().UnixNano())
    return &NetworkMetrics{
        DeviceID:     deviceID,
        CPUUsage:     30 + rand.Float64()*40,
        MemoryUsage:  45 + rand.Float64()*30,
        Temperature:  35 + rand.Intn(20),
        InterfaceStats: map[string]*InterfaceStats{
            "Gi0/0/1": {InBytes: 1000000, OutBytes: 800000, InErrors: 0, OutErrors: 0},
            "Gi0/0/2": {InBytes: 800000, OutBytes: 600000, InErrors: 0, OutErrors: 0},
        },
        CollectedAt: time.Now(),
    }, nil
}
```

## 3. Manager with Dependency Injection

```go
// internal/discovery/manager.go
type Manager struct {
    hypervisor HypervisorClient
    metrics    MetricsCollector
    network    NetworkDeviceClient
}

type Option func(*Manager)

func WithHypervisor(h HypervisorClient) Option {
    return func(m *Manager) { m.hypervisor = h }
}

func WithMetrics(c MetricsCollector) Option {
    return func(m *Manager) { m.metrics = c }
}

func WithNetwork(n NetworkDeviceClient) Option {
    return func(m *Manager) { m.network = n }
}

func NewManager(opts ...Option) *Manager {
    m := &Manager{}
    for _, opt := range opts {
        opt(m)
    }
    return m
}

// Production factory
func NewRealManager(vCenter, username, password string) *Manager {
    return NewManager(
        WithHypervisor(vsphere.NewClient(vCenter, username, password)),
        WithMetrics(NewRealMetricsCollector()),
        WithNetwork(snmp.NewClient()),
    )
}

// Test factory
func NewFakeManager() *Manager {
    return NewManager(
        WithHypervisor(fake.NewFakeVMwareClient()),
        WithMetrics(fake.NewFakeMetricsCollector()),
        WithNetwork(fake.NewFakeNetworkDeviceClient()),
    )
}
```

## 4. Test Cases

```go
// internal/discovery/fake/fake_vmware_test.go
func TestVMDiscovery(t *testing.T) {
    fake := fake.NewFakeVMwareClient()
    mgr := discovery.NewManager(discovery.WithHypervisor(fake))

    vms, err := mgr.DiscoverVMs("")
    assert.NoError(t, err)
    assert.Len(t, vms, 3)

    assert.Equal(t, "web-server-01", vms[0].Name)
    assert.Equal(t, 2, vms[0].VCPU)
    assert.Equal(t, 4096, vms[0].MemoryMB)
}

func TestVMMetricsInRange(t *testing.T) {
    fake := fake.NewFakeVMwareClient()
    metrics := fake.NewFakeMetricsCollector()
    mgr := discovery.NewManager(
        discovery.WithHypervisor(fake),
        discovery.WithMetrics(metrics),
    )

    m, err := mgr.CollectVMMetrics("vm-100")
    assert.NoError(t, err)
    assert.Greater(t, m.CPUUsage, 0.0)
    assert.Less(t, m.CPUUsage, 100.0)
    assert.Greater(t, m.MemUsage, 0.0)
    assert.Less(t, m.MemUsage, 100.0)
}

func TestHighCPUAlert(t *testing.T) {
    fake := &fake.FakeVMwareClient{
        VMs: []*fake.VM{
            {ID: "vm-100", Name: "high-cpu-vm", VCPU: 2, State: "running"},
        },
    }
    highCPU := &fake.FakeHighCPUCollector{BaseCPU: 85.0}
    mgr := discovery.NewManager(
        discovery.WithHypervisor(fake),
        discovery.WithMetrics(highCPU),
    )

    alert := mgr.CheckThreshold("vm-100", "cpu", 80.0)
    assert.True(t, alert)
}

func TestPhysicalHostDiscovery(t *testing.T) {
    fake := &fake.FakeIPMIClient{
        Hosts: []*fake.PhysicalHost{
            {
                ID: "host-001", Name: "dell-r740-01",
                Manufacturer: "Dell", Model: "PowerEdge R740",
                SerialNo: "SNABC123", CPUCores: 32,
                MemoryGB: 128, State: "active",
                MgmtIP: "192.168.1.101", IPMIIP: "192.168.1.100",
            },
        },
    }
    mgr := discovery.NewManager(discovery.WithHypervisor(fake))

    hosts, err := mgr.DiscoverHosts()
    assert.NoError(t, err)
    assert.Len(t, hosts, 1)
    assert.Equal(t, "Dell", hosts[0].Manufacturer)
}
```

## 5. File Structure

```
internal/discovery/
├── manager.go              # [修改] 支持选项模式注入
├── hypervisor.go           # [新增] HypervisorClient 接口
├── metrics.go              # [新增] MetricsCollector 接口
├── network.go              # [新增] NetworkDeviceClient 接口
├── vsphere.go              # [已有] 真实 vSphere 客户端
├── kvm.go                  # [已有] 真实 KVM 客户端
└── fake/
    ├── fake_vmware.go      # [新增] Mock vSphere
    ├── fake_vmware_test.go # [新增] VM 测试用例
    ├── fake_kvm.go         # [新增] Mock KVM
    ├── fake_kvm_test.go    # [新增] KVM 测试用例
    ├── fake_metrics.go     # [新增] Mock 指标
    ├── fake_metrics_test.go# [新增] 指标测试用例
    ├── fake_network.go     # [新增] Mock 网络设备
    ├── fake_network_test.go# [新增] 网络设备测试用例
    └── fake_ipmi.go        # [新增] Mock IPMI
```

## 6. Implementation Tasks

| Task | Description | Priority |
|------|-------------|----------|
| 1 | 定义 HypervisorClient/MetricsCollector 接口 | P0 |
| 2 | 实现 FakeVMwareClient + 基本测试 | P0 |
| 3 | 实现 FakeMetricsCollector | P0 |
| 4 | 实现 FakeNetworkDeviceClient | P1 |
| 5 | 修改 Manager 支持依赖注入 | P1 |
| 6 | 实现 FakeKVMClient + 测试 | P2 |
| 7 | 实现 FakeIPMIClient + 测试 | P2 |
| 8 | 编写完整测试用例覆盖矩阵 | P2 |

## 7. Switching Between Mock and Real

```go
// 运行时根据环境变量决定使用 Mock 还是真实客户端
func NewDiscoveryManager() *Manager {
    if os.Getenv("DISCOVERY_MODE") == "fake" {
        return NewFakeManager()
    }
    return NewRealManager(
        os.Getenv("VCENTER_URL"),
        os.Getenv("VCENTER_USER"),
        os.Getenv("VCENTER_PASS"),
    )
}
```

## 8. CI/CD Integration

```yaml
# .github/workflows/test.yml
- name: Run unit tests
  run: |
    DISCOVERY_MODE=fake go test ./internal/discovery/... -v

- name: Run integration tests
  if: env.VCENTER_URL != ''
  run: |
    go test ./internal/discovery/... -v -tags=integration
```

## Summary

此设计实现：
- **接口清晰**: HypervisorClient/MetricsCollector/NetworkDeviceClient 三个核心接口
- **Mock 完整**: FakeVMwareClient, FakeKVMClient, FakeMetricsCollector, FakeNetworkDevice
- **注入灵活**: 通过 Option 模式或环境变量切换 Mock/Real
- **测试覆盖**: 覆盖 VM 发现、监控采集、告警触发、物理机发现等主要场景