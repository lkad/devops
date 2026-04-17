# Design System — DevOps Toolkit

## Product Context
- **What this is:** 统一设备管理平台，管理物理机、容器、网络设备和云实例
- **Who it's for:** DevOps 工程师、开发工程师、项目管理人员
- **Space/industry:** 运维管理 / Infrastructure Management
- **Project type:** 内部工具 / Dashboard / Web App

## Aesthetic Direction
- **Direction:** Modern Industrial — 干净的深色界面，高对比度数据展示
- **Decoration level:** Intentional — 通过层次感创造深度，不过度装饰
- **Mood:** 专业、高效、清晰，适合长时间使用
- **Reference sites:** Datadog, Grafana, Portainer, Runbook

## Typography
- **Display/Hero:** Geist Bold — 标题和重要数字
- **Body:** Geist — 界面文字、按钮、导航
- **UI/Labels:** Geist Medium — 标签、状态文字
- **Data/Tables:** JetBrains Mono — 设备ID、代码、数值数据
- **Code:** JetBrains Mono
- **Loading:** Google Fonts CDN (Geist, JetBrains Mono)
- **Scale:**
  - Hero: 28px / 1.75rem
  - H1: 24px / 1.5rem
  - H2: 20px / 1.25rem
  - Body: 16px / 1rem
  - Small: 14px / 0.875rem
  - Caption: 12px / 0.75rem

## Color
- **Approach:** Restrained — 单一主色 + 语义色，色彩稀有且有意义
- **Primary:** #22d3ee (Cyan) — 主色调，区别于 Datadog/Grafana
- **Primary Hover:** #06b6d4
- **Primary Muted:** rgba(34, 211, 238, 0.15) — 选中状态背景
- **Success:** #22c55e — 在线/正常状态
- **Warning:** #f59e0b — 待注册/警告状态
- **Error:** #ef4444 — 异常/错误状态
- **Info:** #3b82f6 — 信息提示
- **Background:** #0c1220 — 主背景（深色）
- **Surface:** #151d2e — 卡片/面板背景
- **Surface Elevated:** #1c2940 — 悬浮/激活状态
- **Border:** #2a3a54 — 边框
- **Text:** #f1f5f9 — 主文字
- **Text Secondary:** #94a3b8 — 次要文字
- **Text Muted:** #64748b — 辅助文字
- **Dark mode:** 减少饱和度 10-20%，保持同一色相

## Spacing
- **Base unit:** 4px
- **Density:** Comfortable — 不拥挤，适合长时间阅读
- **Scale:**
  - 2xs: 2px
  - xs: 4px
  - sm: 8px
  - md: 12px
  - lg: 16px
  - xl: 20px
  - 2xl: 24px
  - 3xl: 32px
  - 4xl: 40px
  - 5xl: 48px

## Layout
- **Approach:** Grid-disciplined — 严格网格，卡片布局
- **Sidebar:** 固定宽度 240px，深色背景，左侧导航
- **Content max width:** 1400px
- **Grid columns:** 12列系统
- **Border radius:**
  - sm: 4px (小标签)
  - md: 8px (按钮、输入框)
  - lg: 12px (卡片)
  - xl: 16px (大面板)
  - full: 9999px (胶囊按钮)

## Motion
- **Approach:** Minimal-functional — 仅用于辅助理解，不过度动画
- **Easing:**
  - enter: ease-out
  - exit: ease-in
  - move: ease-in-out
- **Duration:**
  - micro: 50-100ms (状态切换)
  - short: 150-250ms (悬停、展开)
  - medium: 250-400ms (页面过渡)
- **Hover states:** transform + opacity，subtle lift effect

## Component States

### Buttons
- Default: 主色背景或边框
- Hover: 背景色加深
- Active: 轻微下沉
- Disabled: 50% opacity

### Cards
- Default: surface 背景 + 细边框
- Hover: 边框变亮 + 轻微上浮 (translateY -2px)
- Active/Selected: 主色边框

### Form Inputs
- Default: 深色背景 + 细边框
- Focus: 主色边框
- Error: 错误色边框
- Disabled: 降低对比度

### Device Status Badges
- Online: 绿色背景 rgba + 绿色文字
- Pending: 橙色背景 rgba + 橙色文字
- Offline: 灰色背景 rgba + 灰色文字
- Error: 红色背景 rgba + 红色文字

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-17 | Initial design system created | 基于 Datadog/Grafana/Portainer/Runbook 研究，为 DevOps 多角色用户设计 |
