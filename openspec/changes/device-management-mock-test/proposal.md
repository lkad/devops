# Device Management Mock Testing Proposal

## Summary

建立设备管理模块（Physical Host、VM、Network Device）的 Mock 测试方案，在无真实硬件环境下验证代码逻辑，实现快速迭代和持续集成。

## Problem Statement

设备管理模块依赖多种外部硬件平台：
- **vSphere/KVM**: 虚拟机管理需要商业虚拟化平台
- **IPMI/SSH**: 物理机管理需要带外管理接口
- **SNMP**: 网络设备管理需要真实设备

**痛点**：
1. 开发人员无法在没有硬件的环境中测试代码
2. CI/CD 流水线无法在无设备环境中运行
3. 测试场景覆盖困难（边界条件、故障注入）
4. 团队协作受限，无法并行开发

## Goals

1. **Mock Hypervisor**: 模拟 vSphere/KVM API 返回虚拟机列表和指标
2. **Mock 监控指标**: 生成符合业务规则的模拟指标数据
3. **Mock 网络设备**: 模拟 SNMP/SSH 网络设备响应
4. **可切换架构**: Mock 与真实环境通过接口切换，无需修改业务逻辑
5. **覆盖主要场景**: VM 发现、监控采集、告警触发、状态变更

## Scope

### In Scope

- FakeVMwareClient (vSphere API mock)
- FakeKVMClient (libvirt API mock)
- FakeMetricsCollector (动态指标生成)
- FakeNetworkDevice (SNMP mock)
- FakeIPMIClient (IPMI/BMC mock)
- 测试用例覆盖主要场景

### Out of Scope

- 真实硬件环境集成
- 性能压力测试
- 长时间稳定性测试

## Approach

采用 **接口注入** 模式，通过依赖注入切换 Mock/真实客户端：

```
Manager (业务逻辑)
    ↓
HypervisorClient (接口)
    ↓
[FakeVMwareClient | RealVMwareClient] (运行时注入)
```

## Deliverables

1. `internal/discovery/fake/` - Mock 客户端实现
2. `internal/discovery/fake/*_test.go` - 测试用例
3. 接口定义与切换机制文档