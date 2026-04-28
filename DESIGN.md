# Design System — DevOps Toolkit

## Product Context
- **What this is:** Internal SRE/DevOps dashboard for managing dual-datacenter infrastructure (Containerlab-based test environment)
- **Who it's for:** DevOps/SRE engineers managing 8+ nodes across DC1 and DC2
- **Space/industry:** Network infrastructure management, ops tooling
- **Project type:** Data-dense dashboard / internal tool

## Aesthetic Direction
- **Direction:** Industrial/Utilitarian — function-first, terminal-inspired, no decorative fluff
- **Decoration level:** Minimal — typography and color do all the work, subtle surface hierarchy
- **Mood:** Feels like an extension of the command line. Data-dense for fast scanning during incidents. Technical credibility over polish.
- **Reference sites:** Grafana, Datadog, k9s — dark ops dashboards

## Typography
- **Display/Hero:** Geist 700 — gradient text for brand moments only (header title)
- **Body:** Geist 400/500 — all UI text, labels, body copy
- **UI/Labels:** Geist 500/600 — headings, section titles, button labels
- **Data/Tables:** JetBrains Mono 400/500 — device IDs, IP addresses, SSH commands, metrics values (must support tabular-nums)
- **Code:** JetBrains Mono — log entries, config snippets
- **Loading:** Google Fonts CDN — Geist + JetBrains Mono
- **Scale:**
  - Hero: 3rem (gradient text, brand use only)
  - H1: 2rem / H2: 1.5rem / H3: 1.1rem
  - Body: 1rem / Small: 0.875rem / Caption: 0.8rem
  - Mono data: 0.85rem

## Color
- **Approach:** Restrained — cyan accent is rare and meaningful, not everywhere
- **Primary:** `#22d3ee` — cyan, used for active states, primary actions, links, key data
- **Primary hover:** `#06b6d4`
- **Primary muted:** `rgba(34, 211, 238, 0.15)` — backgrounds behind cyan elements
- **Background:** `#0c1220` — near-black blue-tinted dark
- **Surface:** `#151d2e` — card/panel backgrounds
- **Surface elevated:** `#1c2940` — hover states, elevated cards
- **Border:** `#2a3a54` — standard borders
- **Border subtle:** `#1e2d42` — inside cards, dividers
- **Text primary:** `#f1f5f9`
- **Text secondary:** `#94a3b8` — labels, descriptions
- **Text muted:** `#64748b` — timestamps, captions
- **Semantic:**
  - Success: `#22c55e`
  - Warning: `#f59e0b`
  - Error: `#ef4444`
  - Info: `#3b82f6`
- **Dark mode:** Dark-first (no light mode redesign needed yet — dark is the primary product)

## Spacing
- **Base unit:** 8px
- **Density:** Comfortable — not cramped for data scanning, not airy
- **Scale:**
  - 1: 4px — micro gaps, badge padding
  - 2: 8px — icon margins, standard gaps
  - 3: 12px — card inner padding
  - 4: 16px — section padding, component spacing
  - 5: 20px — form element spacing
  - 6: 24px — card padding, section gaps
  - 7: 32px — major section spacing
  - 8: 48px — page section spacing

## Layout
- **Approach:** Grid-disciplined — strict columns, predictable alignment, max ~1400px content width
- **Grid:** 4-column stats grid, 1-column device lists, 2-column detail grids
- **Max content width:** 1400px
- **Border radius:**
  - sm: 4px — badges, tags
  - md: 8px — buttons, inputs
  - lg: 12px — cards, panels
  - xl: 16px — modals, large cards

## Motion
- **Approach:** Minimal-functional — only transitions that aid comprehension
- **Easing:** ease-out on enter, ease-in on exit, ease-in-out on move
- **Duration:**
  - Micro: 50-100ms — hover states, button feedback
  - Short: 150-250ms — panel transitions, tab switches
  - Medium: 250-400ms — modals, drawers
- **No choreography** — no entrance animations, no staggered lists, no scroll-driven effects

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-19 | Initial design system created | Created by /design-consultation based on product context (SRE dashboard, dark theme, dual-datacenter infrastructure) |
| 2026-04-28 | Mock testing framework | Add Fake*Client implementations for device management testing without real hardware |

## Multi-Cluster Kubernetes UI

### Environment Color Coding
Clusters must be visually distinguished by environment type using the following color scheme:

| Environment | Background | Border | Label |
|------------|------------|--------|-------|
| dev | rgba(59, 130, 246, 0.15) | #3b82f6 | 开发 |
| test | rgba(245, 158, 11, 0.15) | #f59e0b | 测试 |
| uat | rgba(168, 85, 247, 0.15) | #a855f7 | UAT |
| prod | rgba(239, 68, 68, 0.15) | #ef4444 | 生产 |
| default | rgba(148, 163, 184, 0.1) | #94a3b8 | 标准 |

### Filtering Requirements
- **Search input**: Filter clusters by name (case-insensitive substring match)
- **Environment dropdown**: Filter by environment type (dev/test/uat/prod)
- Both filters work together (AND logic)
- Filtered results update in real-time as user types/selects

### Selected Cluster Highlighting
- Selected cluster has a cyan border (3px solid #22d3ee)
- Selected cluster has a cyan glow shadow: `box-shadow: 0 0 20px rgba(34, 211, 238, 0.3)`
- Selected cluster has a cyan dot indicator in top-right corner

### Implementation Notes
- Cluster type is determined by parsing the `type` field (case-insensitive match for keywords: dev, test, uat, prod)
- All clusters stored in `allK8sClusters` global variable for client-side filtering
- `filterK8sClusters()` function handles both search and environment filtering
- `getEnvStyle()` returns appropriate color scheme based on cluster type

## Mock Testing Framework

### Purpose
在没有真实硬件的环境下进行开发和测试，使用模拟客户端替代真实的 vSphere、KVM、IPMI、SNMP 等连接。

### Mock Client 文件位置
`internal/device/fake/` 目录下包含所有模拟客户端实现：

| 文件 | 用途 |
|------|------|
| `fake_vmware.go` | 模拟 vSphere/ESXi 虚拟化平台 |
| `fake_kvm.go` | 模拟 KVM 虚拟化平台 |
| `fake_ipmi.go` | 模拟 IPMI 智能平台管理接口 |
| `fake_network.go` | 模拟 SNMP 网络设备 (交换机、路由器、防火墙) |
| `fake_metrics.go` | 模拟指标采集 (CPU、内存、磁盘、网络) |
| `fake_test.go` | 测试辅助工具 |

### 使用方式

```go
import "github.com/devops-toolkit/internal/device/fake"

// 创建模拟客户端
vmClient := fake.NewFakeVMwareClient()
kvmClient := fake.NewFakeKVMClient()
ipmiClient := fake.NewFakeIPMIClient()
networkClient := fake.NewFakeNetworkDeviceClient()
metricsCollector := fake.NewFakeMetricsCollector()

// 创建带模拟客户端的设备管理器
manager := device.NewManagerWithClients(db, vmClient, metricsCollector, networkClient)

// 使用 manager 进行操作
vms, err := manager.DiscoverVMsFromHost(ctx, "host-1")
```

### 模拟数据

**FakeVMwareClient:** 3 台 VM (web-server-01, db-server-01, app-server-01)
**FakeKVMClient:** 2 台 VM (kvm-vm-01, kvm-vm-02)
**FakeNetworkDeviceClient:** 3 台网络设备 (core-switch-01, access-switch-01, edge-firewall-01)

### 接口对齐

所有 Fake 客户端实现真实客户端的相同接口，确保测试环境与生产环境代码路径一致。

**HypervisorClient 接口 (虚拟化平台):**
```go
type HypervisorClient interface {
    ListVMs(ctx context.Context, hostID string) ([]*VM, error)
    GetVM(ctx context.Context, vmID string) (*VM, error)
    GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    GetHostInfo(ctx context.Context, hostID string) (*GORMDevice, error)
    GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
    GetHostPowerState(ctx context.Context, hostID string) (string, error)
    SetHostPowerState(ctx context.Context, hostID string, state string) error
}
```

**NetworkDeviceClient 接口 (网络设备):**
```go
type NetworkDeviceClient interface {
    ListDevices(ctx context.Context) ([]*GORMDevice, error)
    GetDevice(ctx context.Context, deviceID string) (*GORMDevice, error)
    GetDeviceInterfaces(ctx context.Context, deviceID string) ([]*NetworkInterface, error)
    GetDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
    BackupConfig(ctx context.Context, deviceID string) (string, error)
}
```

**MetricsCollector 接口 (指标采集):**
```go
type MetricsCollector interface {
    CollectVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
    CollectHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
    CollectNetworkDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
}
```