# Device Management Mock Test - Implementation Tasks

## Overview

实现设备管理模块的 Mock 测试框架，包括 Fake 客户端和测试用例。

## Task List

### Phase 1: Interface Definitions

- [ ] **T1.1**: 创建 `internal/discovery/hypervisor.go` - HypervisorClient 接口定义
- [ ] **T1.2**: 创建 `internal/discovery/metrics.go` - MetricsCollector 接口定义
- [ ] **T1.3**: 创建 `internal/discovery/network.go` - NetworkDeviceClient 接口定义

### Phase 2: Fake Clients

- [ ] **T2.1**: 创建 `internal/discovery/fake/` 目录结构
- [ ] **T2.2**: 实现 `FakeVMwareClient` - vSphere API Mock
  - ListVMs / GetVM / GetVMMetrics
  - GetHostInfo / GetHostMetrics
- [ ] **T2.3**: 实现 `FakeKVMClient` - KVM/libvirt API Mock
  - ListVMs / GetVM / GetVMMetrics
- [ ] **T2.4**: 实现 `FakeMetricsCollector` - 动态指标生成
  - CollectVMMetrics
  - CollectHostMetrics
  - CollectNetworkDeviceMetrics
- [ ] **T2.5**: 实现 `FakeNetworkDeviceClient` - SNMP Mock
  - ListDevices / GetDevice
  - GetDeviceInterfaces / GetDeviceMetrics
- [ ] **T2.6**: 实现 `FakeIPMIClient` - IPMI/BMC Mock
  - GetHostPowerState / SetHostPowerState

### Phase 3: Manager Integration

- [ ] **T3.1**: 修改 `internal/discovery/manager.go` - 支持选项模式注入
- [ ] **T3.2**: 实现 `NewRealManager()` - 生产环境工厂函数
- [ ] **T3.3**: 实现 `NewFakeManager()` - 测试环境工厂函数
- [ ] **T3.4**: 添加环境变量切换逻辑 (`DISCOVERY_MODE=fake`)

### Phase 4: Test Cases

- [ ] **T4.1**: 编写 `FakeVMwareClient` 测试
  - TestVMDiscovery
  - TestVMDiscoveryByHost
  - TestVMMetricsInRange
- [ ] **T4.2**: 编写 `FakeMetricsCollector` 测试
  - TestMetricsWithinValidRange
  - TestMetricsVariation
- [ ] **T4.3**: 编写告警触发测试
  - TestHighCPUAlert
  - TestHighMemoryAlert
  - TestDiskSpaceAlert
- [ ] **T4.4**: 编写物理机发现测试
  - TestPhysicalHostDiscovery
  - TestPhysicalHostMetrics
- [ ] **T4.5**: 编写网络设备测试
  - TestNetworkDeviceDiscovery
  - TestNetworkDeviceMetrics

### Phase 5: CI/CD Integration

- [ ] **T5.1**: 配置单元测试运行 (`DISCOVERY_MODE=fake`)
- [ ] **T5.2**: 配置集成测试运行 (需要真实环境)
- [ ] **T5.3**: 添加测试覆盖率报告

## File清单

```
internal/discovery/
├── hypervisor.go           # [新增] T1.1
├── metrics.go              # [新增] T1.2
├── network.go              # [新增] T1.3
├── manager.go              # [修改] T3.1-T3.4
├── vsphere.go              # [已有] 真实客户端
├── kvm.go                  # [已有] 真实客户端
└── fake/
    ├── fake_vmware.go      # [新增] T2.2
    ├── fake_vmware_test.go # [新增] T4.1
    ├── fake_kvm.go         # [新增] T2.3
    ├── fake_kvm_test.go    # [新增] T4.1
    ├── fake_metrics.go     # [新增] T2.4
    ├── fake_metrics_test.go# [新增] T4.2
    ├── fake_network.go     # [新增] T2.5
    ├── fake_network_test.go# [新增] T4.4
    ├── fake_ipmi.go        # [新增] T2.6
    └── fake_ipmi_test.go   # [新增] T4.4
```

## Dependencies

- None (仅使用 Go 标准库 + testing)

## Priority Order

1. T1.1-T1.3: 接口定义 (基础)
2. T2.2, T2.4: FakeVMwareClient + FakeMetricsCollector (核心场景)
3. T3.1-T3.4: Manager 集成 (让测试可以运行)
4. T4.1-T4.2: 核心测试用例
5. T2.3, T2.5, T2.6: 其他 Fake 客户端
6. T4.3-T4.5: 扩展测试场景
7. T5.1-T5.3: CI/CD 配置

## Estimated Effort

| Phase | Tasks | Estimated Time |
|-------|-------|----------------|
| Phase 1 | 3 | 1h |
| Phase 2 | 6 | 4h |
| Phase 3 | 4 | 2h |
| Phase 4 | 5 | 3h |
| Phase 5 | 3 | 1h |
| **Total** | **21** | **11h** |