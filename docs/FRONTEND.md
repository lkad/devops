# Frontend Requirements — DevOps Toolkit

**状态:** Draft
**最后更新:** 2026-04-29
**基于:** PRD.md v2.1, DESIGN.md

---

## 1. 概述

### 1.1 项目背景

前端重新开发，需覆盖 PRD 中所有功能模块的 UI 界面。

### 1.2 技术栈

| 类别 | 技术 | 说明 |
|------|------|------|
| 框架 | React 18 + TypeScript | SPA 应用 |
| 构建 | Vite | 快速开发构建 |
| 路由 | React Router v6 | 页面路由 |
| 状态管理 | Zustand | 轻量级状态管理 |
| 样式 | CSS Modules + CSS Variables | 遵循 DESIGN.md |
| 图表 | Recharts | 数据可视化 |
| 表格 | TanStack Table | 数据表格 |
| HTTP | Fetch API | 与后端通信 |
| WebSocket | Native WebSocket | 实时事件 |

### 1.3 设计系统

所有 UI 必须遵循 [DESIGN.md](../DESIGN.md) 规范：

- **字体:** Geist (UI), JetBrains Mono (数据/代码)
- **颜色:** 暗色主题，cyan 作为主色调
- **间距:** 8px 基础单位
- **圆角:** sm:4px, md:8px, lg:12px, xl:16px

---

## 2. 页面与路由

### 2.1 路由结构

```
/                           → Dashboard (首页，需登录)
/login                      → 登录页面
/devices                    → 设备列表
/devices/:id                → 设备详情
/physical-hosts             → 物理主机列表
/physical-hosts/:id         → 主机详情
/physical-hosts/:id/services → 主机服务列表
/physical-hosts/:id/config  → 配置推送
/pipelines                  → 流水线列表
/pipelines/:id              → 流水线详情
/pipelines/:id/run          → 流水线运行
/logs                       → 日志查询
/logs/alerts                → 告警规则
/alerts                     → 告警通道
/alerts/history             → 告警历史
/k8s                        → K8s 集群列表
/k8s/:cluster               → 集群详情（含 tabs）
/k8s/:cluster/nodes         → 节点列表
/k8s/:cluster/namespaces    → 命名空间列表
/k8s/:cluster/pods          → Pod 列表
/k8s/:cluster/pods/:pod/logs → Pod 日志
/k8s/:cluster/pods/:pod/exec → Pod exec
/projects                   → 项目列表（BL → System → Project）
/projects/:id               → 项目详情
/projects/:id/resources     → 项目资源
/projects/:id/permissions   → 项目权限
/reports/finops             → FinOps 报表
/audit-logs                 → 审计日志
/settings                   → 设置
```

### 2.2 页面布局

```
┌─────────────────────────────────────────────────────────────────┐
│  Header (Logo + Nav + User Menu)                                │
├────────┬────────────────────────────────────────────────────────┤
│        │                                                        │
│  Side  │                    Main Content                       │
│  Nav   │                                                        │
│        │                                                        │
│        │                                                        │
└────────┴────────────────────────────────────────────────────────┘
```

**Header:** 高度 56px，包含 Logo、顶部导航、用户信息
**SideNav:** 宽度 240px，可折叠，包含模块导航
**Main:** 最大宽度 1400px，居中显示

---

## 3. 核心功能模块

### 3.1 Dashboard (首页)

**功能:**
- 系统状态概览卡片（设备数量、流水线状态、告警数量）
- 最近活动时间线
- 快捷入口链接

**组件:**
- `StatsCard` — 统计卡片（图标 + 数字 + 趋势）
- `RecentActivity` — 最近活动时间线
- `QuickActions` — 快捷操作入口

### 3.2 设备管理

**功能:**
- 设备列表（分页、搜索、筛选）
- 设备状态一览（ACTIVE/MAINTENANCE/SUSPENDED/RETIRED）
- 设备详情页面
- 设备状态转换操作

**组件:**
- `DeviceTable` — 设备列表表格（TanStack Table）
- `DeviceFilters` — 筛选器（类型、状态、数据中心）
- `DeviceStatusBadge` — 状态标签
- `DeviceStateMachine` — 状态转换图
- `DeviceDetailPanel` — 设备详情面板

**用户流程:**
```
设备列表 → 点击设备 → 设备详情 → 执行操作（维护/暂停/退役）
```

### 3.3 物理主机管理

**功能:**
- 主机列表（含状态监控）
- SSH 指标展示（CPU/内存/磁盘）
- 服务状态监控
- 配置推送

**组件:**
- `PhysicalHostTable` — 主机列表
- `HostMetricsPanel` — 指标面板（仪表盘样式）
- `ServiceStatusList` — 服务状态列表
- `ConfigPushForm` — 配置推送表单

**状态显示:**
| 状态 | 颜色 | 说明 |
|------|------|------|
| online | success (绿) | 监控正常，SSH 正常 |
| monitoring_issue | warning (黄) | 监控 DOWN，SSH 正常 |
| offline | error (红) | 监控 DOWN，SSH 失败 |

### 3.4 CI/CD 流水线

**功能:**
- 流水线列表与状态
- 流水线编辑器视图（阶段可视化）
- 运行历史与日志
- 手动执行/取消

**组件:**
- `PipelineList` — 流水线列表
- `PipelineEditor` — 流水线阶段编辑器（可视化）
- `PipelineRunHistory` — 运行历史表格
- `PipelineRunLogs` — 运行日志查看器
- `StageStatusIndicator` — 阶段状态指示器

**流水线阶段显示:**
```
[✓ Source] → [✓ Build] → [✓ Test] → [✓ Deploy] → [✓ Smoke Test]
```

### 3.5 日志系统

**功能:**
- 日志查询界面（时间范围、级别、来源）
- 实时日志流（WebSocket）
- 日志统计图表
- 告警规则管理

**组件:**
- `LogQueryForm` — 查询表单（时间范围 + 过滤条件）
- `LogTable` — 日志列表（虚拟滚动）
- `LogStream` — 实时日志流（WebSocket）
- `LogChart` — 统计图表（Recharts）
- `AlertRuleList` — 告警规则列表
- `AlertRuleEditor` — 告警规则编辑器

### 3.6 告警系统

**功能:**
- 告警通道管理（创建/编辑/删除）
- 告警历史查询
- 告警统计

**组件:**
- `AlertChannelList` — 通道列表
- `AlertChannelForm` — 通道表单（Slack/Webhook/Email/Log）
- `AlertHistoryTable` — 历史记录
- `AlertStatsChart` — 统计图表

### 3.7 K8s 多集群管理

**功能:**
- 集群列表（含环境标签）
- 集群健康状态
- 节点与 Pod 管理
- Pod 日志查看
- Pod exec 命令

**组件:**
- `ClusterCard` — 集群卡片（含环境颜色）
- `ClusterFilters` — 搜索 + 环境筛选
- `NodeTable` — 节点列表
- `PodTable` — Pod 列表（按命名空间分组）
- `PodLogsViewer` — Pod 日志查看器
- `PodExecTerminal` — 命令执行终端

**环境颜色:**
| 环境 | 背景 | 边框 |
|------|------|------|
| dev | rgba(59,130,246,0.15) | #3b82f6 |
| test | rgba(245,158,11,0.15) | #f59e0b |
| uat | rgba(168,85,247,0.15) | #a855f7 |
| prod | rgba(239,68,68,0.15) | #ef4444 |

### 3.8 项目管理

**功能:**
- 层级结构：Business Line → System → Project
- 项目资源链接与管理
- FinOps 报表导出

**组件:**
- `BusinessLineTree` — 业务线树形结构
- `ProjectResourceManager` — 资源链接管理（含权重设置）
- `FinOpsExport` — FinOps CSV 导出
- `ProjectTypeTag` — 项目类型标签

**资源权重编辑:**
```
设备: shared-server-01
├── Project A: [====--------] 50%
├── Project B: [===---------] 30%
└── Project C: [==----------] 20%
```

### 3.9 审计日志

**功能:**
- 审计日志查询
- 按实体类型、操作类型过滤

**组件:**
- `AuditLogTable` — 审计日志表格
- `AuditLogFilters` — 过滤条件

---

## 4. 通用组件

### 4.1 布局组件

| 组件 | 说明 |
|------|------|
| `AppShell` | 整体布局容器（Header + SideNav + Main） |
| `Header` | 顶部导航栏 |
| `SideNav` | 侧边导航菜单 |
| `PageContainer` | 页面内容容器 |

### 4.2 数据展示

| 组件 | 说明 |
|------|------|
| `DataTable` | 通用数据表格（基于 TanStack Table） |
| `StatsCard` | 统计卡片 |
| `Badge` | 状态标签 |
| `EmptyState` | 空状态占位 |
| `LoadingSpinner` | 加载状态 |

### 4.3 表单组件

| 组件 | 说明 |
|------|------|
| `Input` | 文本输入 |
| `Select` | 下拉选择 |
| `Button` | 按钮（primary/secondary/danger） |
| `FormField` | 表单字段包装 |

### 4.4 反馈组件

| 组件 | 说明 |
|------|------|
| `Toast` | 操作反馈提示 |
| `Modal` | 模态框 |
| `ConfirmDialog` | 确认对话框 |
| `ErrorBoundary` | 错误边界 |

---

## 5. API 约定

### 5.1 通用规则

- 所有 API 前缀 `/api`
- 认证通过 JWT Token（Header: `Authorization: Bearer <token>`）
- 列表返回分页格式：`{ data: [...], pagination: { page, pageSize, total } }`
- 错误格式：`{ error: string, message: string }`

### 5.2 主要 API 端点

| 模块 | 端点 | 方法 |
|------|------|------|
| 设备 | `/api/devices` | GET, POST |
| 设备 | `/api/devices/:id` | GET, PUT, DELETE |
| 设备 | `/api/devices/search` | GET |
| 物理主机 | `/api/physical-hosts` | GET, POST |
| 物理主机 | `/api/physical-hosts/:id` | GET, DELETE |
| 物理主机 | `/api/physical-hosts/:id/services` | GET |
| 物理主机 | `/api/physical-hosts/:id/config` | POST |
| 流水线 | `/api/pipelines` | GET, POST |
| 流水线 | `/api/pipelines/:id` | GET, DELETE |
| 流水线 | `/api/pipelines/:id/execute` | POST |
| 日志 | `/api/logs` | GET, POST |
| 日志 | `/api/logs/stats` | GET |
| 日志 | `/api/logs/alerts` | GET, POST |
| 日志 | `/api/logs/filters` | GET, POST |
| 告警 | `/api/alerts/channels` | GET, POST |
| 告警 | `/api/alerts/history` | GET |
| K8s | `/api/k8s/clusters` | GET, POST |
| K8s | `/api/k8s/clusters/:name` | DELETE |
| K8s | `/api/k8s/clusters/:name/health` | GET |
| K8s | `/api/k8s/clusters/:name/nodes` | GET |
| K8s | `/api/k8s/clusters/:name/namespaces` | GET |
| K8s | `/api/k8s/clusters/:name/pods` | GET |
| K8s | `/api/k8s/clusters/:name/pods/:pod/logs` | GET |
| K8s | `/api/k8s/clusters/:name/metrics` | GET |
| 认证 | `/api/auth/login` | POST |
| 认证 | `/api/auth/logout` | POST |
| 认证 | `/api/auth/me` | GET |
| 项目 | `/api/org/business-lines` | GET, POST |
| 项目 | `/api/org/business-lines/:id` | GET, PUT, DELETE |
| 项目 | `/api/org/business-lines/:id/systems` | GET, POST |
| 项目 | `/api/org/systems/:id` | GET, PUT, DELETE |
| 项目 | `/api/org/systems/:id/projects` | GET, POST |
| 项目 | `/api/org/projects/:id` | GET, PUT, DELETE |
| 项目 | `/api/org/projects/:id/resources` | GET, POST, DELETE |
| 项目 | `/api/org/projects/:id/permissions` | GET, POST |
| 项目 | `/api/org/permissions/:perm_id` | DELETE |
| FinOps | `/api/org/reports/finops?period=` | GET |
| 审计 | `/api/org/audit-logs` | GET |
| 发现 | `/api/discovery/scan` | POST |
| 发现 | `/api/discovery/status` | GET |

### 5.3 WebSocket

- 端点: `/ws`
- 订阅通道: `log`, `metric`, `device_event`, `pipeline_update`, `alert`
- 消息格式:
```json
{
  "channel": "log",
  "type": "log",
  "data": { ... },
  "timestamp": "2026-04-29T00:00:00.000Z"
}
```

---

## 6. 状态管理

### 6.1 Store 结构

| Store | 用途 |
|-------|------|
| `authStore` | 用户认证状态 |
| `deviceStore` | 设备列表与详情 |
| `pipelineStore` | 流水线状态 |
| `logStore` | 日志查询与实时流 |
| `k8sStore` | K8s 集群与资源 |
| `projectStore` | 项目层级与资源 |
| `uiStore` | UI 状态（侧边栏折叠、Toast 等） |

### 6.2 数据获取策略

- 列表页: 首次加载获取完整列表，前端分页
- 详情页: 按需加载，缓存结果
- 实时数据: WebSocket 订阅，按 channel 区分

### 6.3 错误处理

| 场景 | 处理方式 |
|------|---------|
| API 4xx 错误 | Toast 显示错误信息（3秒后自动消失） |
| API 5xx 错误 | Toast 显示"服务器错误，请稍后重试" |
| 网络断开 | 顶部 Banner 显示"网络已断开"，自动重试 |
| 请求超时 | 30秒超时，显示"请求超时" |
| WebSocket 断开 | 自动重连，最多重试 5 次 |

### 6.4 离线支持

- 静态资源（HTML/JS/CSS）可缓存
- 关键数据（设备列表）可使用 localStorage 缓存
- 离线时显示缓存数据，Banner 提示"离线模式"

---

## 7. 响应式设计

### 7.1 断点

| 断点 | 宽度 | 布局变化 |
|------|------|---------|
| Desktop | ≥ 1024px | 完整布局（SideNav + Main） |
| Tablet | 768px - 1023px | SideNav 收起为图标模式 |
| Mobile | < 768px | SideNav 隐藏，汉堡菜单触发 |

### 7.2 响应式行为

- SideNav 在 Tablet 宽度下收起为 64px 图标模式
- SideNav 在 Mobile 宽度下隐藏，由汉堡菜单触发
- DataTable 在窄屏下支持水平滚动
- 卡片在窄屏下变为单列布局

---

## 8. 目录结构

```
frontend/
├── src/
│   ├── main.tsx                 # 入口
│   ├── App.tsx                  # 根组件 + 路由
│   ├── components/
│   │   ├── layout/              # 布局组件
│   │   │   ├── AppShell.tsx
│   │   │   ├── Header.tsx
│   │   │   ├── SideNav.tsx
│   │   │   └── PageContainer.tsx
│   │   ├── common/              # 通用组件
│   │   │   ├── DataTable.tsx
│   │   │   ├── Badge.tsx
│   │   │   ├── Button.tsx
│   │   │   ├── Modal.tsx
│   │   │   └── ...
│   │   ├── devices/             # 设备模块
│   │   ├── physical-hosts/      # 物理主机模块
│   │   ├── pipelines/           # 流水线模块
│   │   ├── logs/               # 日志模块
│   │   ├── alerts/             # 告警模块
│   │   ├── k8s/                # K8s 模块
│   │   ├── projects/           # 项目管理模块
│   │   └── reports/            # 报表模块
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── DeviceList.tsx
│   │   ├── DeviceDetail.tsx
│   │   └── ...
│   ├── stores/                  # Zustand stores
│   │   ├── authStore.ts
│   │   ├── deviceStore.ts
│   │   └── ...
│   ├── hooks/                  # 自定义 hooks
│   │   ├── useApi.ts
│   │   ├── useWebSocket.ts
│   │   └── useAuth.ts
│   ├── utils/                  # 工具函数
│   ├── types/                  # TypeScript 类型
│   └── styles/                 # 全局样式
│       ├── variables.css       # CSS Variables
│       └── global.css
├── index.html
├── vite.config.ts
└── package.json
```

---

## 9. 测试需求

### 8.1 单元测试

| 模块 | 测试内容 |
|------|---------|
| Store | 状态更新逻辑 |
| Hooks | 数据获取、缓存逻辑 |
| 组件 | 渲染、交互逻辑 |
| Utils | 工具函数 |

### 8.2 集成测试

| 测试场景 | 验证 |
|---------|------|
| 设备 CRUD | 创建→查看→更新→删除 |
| 流水线执行 | 创建→执行→查看日志→完成 |
| 实时日志流 | 连接→订阅→接收消息 |
| 项目资源链接 | 链接→查看→调整权重→取消链接 |

### 8.3 E2E 测试（可选）

| 测试场景 | 工具 |
|---------|------|
| 完整用户流程 | Playwright |
| 视觉回归测试 | Playwright + screenshot diff |

---

## 10. 性能要求

| 指标 | 目标 |
|------|------|
| 首屏加载 | < 2s |
| API 响应 | < 500ms |
| 列表滚动 | 60fps |
| WebSocket 延迟 | < 100ms |

---

## 11. 待办事项

- [ ] 确定前端技术栈版本
- [ ] 创建项目骨架
- [ ] 实现布局组件（AppShell, Header, SideNav）
- [ ] 实现 Dashboard 页面
- [ ] 实现设备管理模块
- [ ] 实现物理主机模块
- [ ] 实现流水线模块
- [ ] 实现日志模块
- [ ] 实现告警模块
- [ ] 实现 K8s 多集群模块
- [ ] 实现项目管理模块
- [ ] 实现报表模块
- [ ] 单元测试覆盖
- [ ] 集成测试

---

**文档状态:** Draft
**需要评审后实施**