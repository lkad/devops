# Design System — DevOps Toolkit

**状态:** Active
**最后更新:** 2026-04-29
**基于:** PRD.md v2.1

---

## 产品背景

- **产品类型:** 内部 SRE/DevOps 仪表板
- **目标用户:** DevOps/SRE 工程师，管理 8+ 节点跨 DC1 和 DC2
- **行业:** 网络基础设施管理，运维工具
- **特点:** 数据密集，用于快速扫描和事件处理

---

## 美学方向

**方向:** 工业/实用主义 — 功能优先，终端风格，无装饰性元素

| 属性 | 说明 |
|------|------|
| 装饰级别 | 极简 — 字体和颜色完成所有工作 |
| 情绪 | 像是命令行的延伸，数据密集以便快速扫描 |
| 参考 | Grafana, Datadog, k9s — 暗色 ops 仪表板 |
| 信任来源 | 技术可信度，而非美观度 |

---

## 字体

### 字体族

| 用途 | 字体 | 特点 |
|------|------|------|
| 标题/品牌 | Geist 700 | 仅用于首屏标题，使用渐变色 |
| 正文/UI | Geist 400/500 | 所有 UI 文本、标签、正文 |
| 标题标签 | Geist 500/600 | 标题、分段标题、按钮标签 |
| 数据/表格 | JetBrains Mono 400/500 | 设备 ID、IP 地址、SSH 命令、指标值（必须支持 tabular-nums） |
| 代码 | JetBrains Mono | 日志条目、配置片段 |

### 字体加载

```html
<link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
```

### 字号

| 层级 | 尺寸 | 行高 | 用途 |
|------|------|------|------|
| Hero | 3rem (48px) | 1.1 | 品牌首屏标题，使用渐变 |
| H1 | 2rem (32px) | 1.2 | 页面标题 |
| H2 | 1.5rem (24px) | 1.3 | 区块标题 |
| H3 | 1.1rem (18px) | 1.4 | 卡片标题 |
| Body | 1rem (16px) | 1.5 | 正文 |
| Small | 0.875rem (14px) | 1.4 | 次要文本 |
| Caption | 0.8rem (13px) | 1.3 | 时间戳、说明 |
| Mono | 0.85rem (14px) | 1.4 | 数据表格、代码 |

---

## 颜色

### 颜色系统

| 用途 | 色值 | 说明 |
|------|------|------|
| Primary | `#22d3ee` | 强调色，用于活跃状态、主要操作、链接、关键数据 |
| Primary Hover | `#06b6d4` | 主色悬停 |
| Primary Muted | `rgba(34, 211, 238, 0.15)` | 主色元素背景 |
| Background | `#0c1220` | 近黑蓝底色 |
| Surface | `#151d2e` | 卡片/面板背景 |
| Surface Elevated | `#1c2940` | 悬停状态、提升卡片 |
| Border | `#2a3a54` | 标准边框 |
| Border Subtle | `#1e2d42` | 卡片内、分隔线 |
| Text Primary | `#f1f5f9` | 主文本 |
| Text Secondary | `#94a3b8` | 标签、描述 |
| Text Muted | `#64748b` | 时间戳、说明 |

### 语义色

| 用途 | 色值 | 说明 |
|------|------|------|
| Success | `#22c55e` | 成功、在线、正常 |
| Warning | `#f59e0b` | 警告、监控异常 |
| Error | `#ef4444` | 错误、离线、故障 |
| Info | `#3b82f6` | 信息、进行中 |

### 环境色（K8s 集群）

| 环境 | 背景 | 边框 | 标签 |
|------|------|------|------|
| dev | `rgba(59, 130, 246, 0.15)` | `#3b82f6` | 开发 |
| test | `rgba(245, 158, 11, 0.15)` | `#f59e0b` | 测试 |
| uat | `rgba(168, 85, 247, 0.15)` | `#a855f7` | UAT |
| prod | `rgba(239, 68, 68, 0.15)` | `#ef4444` | 生产 |

### 状态色（物理主机）

| 状态 | 色值 | 说明 |
|------|------|------|
| online | `#22c55e` (Success) | 监控正常，SSH 正常 |
| monitoring_issue | `#f59e0b` (Warning) | 监控 DOWN，SSH 正常 |
| offline | `#ef4444` (Error) | 监控 DOWN，SSH 失败 |

---

## 间距

### 基础单位

**8px** — 所有间距基于此倍数

### 间距比例

| 层级 | 像素 | 用途 |
|------|------|------|
| 1 | 4px | 微间距、徽章内边距 |
| 2 | 8px | 图标边距、标准间距 |
| 3 | 12px | 卡片内边距 |
| 4 | 16px | 分段内边距、组件间距 |
| 5 | 20px | 表单元素间距 |
| 6 | 24px | 卡片边距、分段间距 |
| 7 | 32px | 主要分段间距 |
| 8 | 48px | 页面分段间距 |

---

## 布局

### 结构

```
┌─────────────────────────────────────────────────────────────────┐
│  Header (56px)                                                  │
├────────┬────────────────────────────────────────────────────────┤
│        │                                                        │
│ SideNav│                   Main Content                         │
│ (240px)│              (max-width: 1400px)                       │
│        │                                                        │
│        │                                                        │
└────────┴────────────────────────────────────────────────────────┘
```

### 网格

| 场景 | 列数 | 说明 |
|------|------|------|
| 统计卡片 | 4 列 | 指标概览 |
| 设备列表 | 1 列 | 单列显示更多数据 |
| 详情面板 | 2 列 | 左右布局 |

### 最大宽度

**1400px** — 内容区域最大宽度，居中显示

### 圆角

| 层级 | 像素 | 用途 |
|------|------|------|
| sm | 4px | 徽章、标签 |
| md | 8px | 按钮、输入框 |
| lg | 12px | 卡片、面板 |
| xl | 16px | 模态框、大型卡片 |

---

## 动效

### 原则

功能性动效 — 仅用于帮助理解的过渡，不做装饰

### 时长

| 层级 | 范围 | 用途 |
|------|------|------|
| Micro | 50-100ms | 悬停状态、按钮反馈 |
| Short | 150-250ms | 面板切换、Tab 切换 |
| Medium | 250-400ms | 模态框、抽屉 |

### 缓动

| 类型 | 曲线 | 用途 |
|------|------|------|
| Enter | ease-out | 元素进入 |
| Exit | ease-in | 元素离开 |
| Move | ease-in-out | 元素移动 |

### 禁止

- 无入场动画
- 无交错列表
- 无滚动驱动效果

---

## 组件

### 按钮

```css
.btn {
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 100ms ease-out;
}

/* 变体 */
.btn-primary { background: #22d3ee; color: #0c1220; }
.btn-primary:hover { background: #06b6d4; }
.btn-secondary { background: transparent; border: 1px solid #2a3a54; color: #f1f5f9; }
.btn-secondary:hover { background: #1c2940; }
.btn-danger { background: #ef4444; color: white; }
```

### 卡片

```css
.card {
  background: #151d2e;
  border: 1px solid #2a3a54;
  border-radius: 12px;
  padding: 16px;
}

.card-elevated {
  background: #1c2940;
}
```

### 输入框

```css
.input {
  background: #151d2e;
  border: 1px solid #2a3a54;
  border-radius: 8px;
  padding: 8px 12px;
  color: #f1f5f9;
  font-size: 0.875rem;
}

.input:focus {
  border-color: #22d3ee;
  outline: none;
}
```

### 表格

```css
.table {
  width: 100%;
  border-collapse: collapse;
}

.table-header {
  background: #1c2940;
  font-weight: 600;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #94a3b8;
}

.table-row {
  border-bottom: 1px solid #1e2d42;
}

.table-row:hover {
  background: #1c2940;
}

/* 表格数据使用等宽字体 */
.table-cell-mono {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.85rem;
}
```

### 徽章

```css
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

/* 状态徽章 */
.badge-success { background: rgba(34, 197, 94, 0.15); color: #22c55e; }
.badge-warning { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
.badge-error { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
.badge-info { background: rgba(59, 130, 246, 0.15); color: #3b82f6; }
```

### 模态框

```css
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal {
  background: #151d2e;
  border: 1px solid #2a3a54;
  border-radius: 16px;
  padding: 24px;
  max-width: 500px;
  width: 90%;
}
```

---

## 交互状态

### 空状态

每个空状态需要包含：
- 图标或插图（简单线条风格）
- 主文案（说明当前状态）
- 副文案（解释原因）
- 操作按钮（如果有）

```html
<div class="empty-state">
  <svg class="empty-icon">...</svg>
  <h3 class="empty-title">No devices found</h3>
  <p class="empty-description">No devices match your current filters.</p>
  <button class="btn-primary">Add Device</button>
</div>
```

### 加载状态

- 骨架屏：与实际布局一致，使用 `#1c2940` 闪烁
- 按钮加载：按钮内显示 spinner，禁用交互
- 表格加载：显示骨架行而非 spinner

### 错误状态

- Inline 错误：在表单字段下方显示红色文本
- Toast 错误：3秒自动消失，带重试按钮
- 全页错误：显示错误插图、操作按钮、错误ID

### 成功状态

- Toast 成功：绿色边框，3秒自动消失
- 操作确认：对于不可逆操作，显示确认对话框

---

## 响应式断点

| 断点 | 宽度 | 行为 |
|------|------|------|
| Desktop | ≥ 1024px | 完整布局（SideNav + Main） |
| Tablet | 768-1023px | SideNav 收起为 64px 图标模式 |
| Mobile | < 768px | SideNav 隐藏，汉堡菜单触发 |

### 响应式行为

- SideNav 收起时只显示图标，不显示文字
- DataTable 在窄屏下水平滚动
- 统计卡片在移动端变为单列
- 表单字段在移动端全宽

---

## 无障碍

- 键盘导航：所有交互元素可 Tab 访问
- ARIA 标记：模态框、折叠面板、标签页
- 对比度：文本对背景至少 4.5:1
- 触摸目标：最小 44x44px

---

## 设计决策记录

| 日期 | 决策 | 原因 |
|------|------|------|
| 2026-04-29 | 创建新的设计系统 | 旧的 DESIGN.md 已废弃 |

---

**文档状态:** Active