# DevOps Toolkit

Go-based internal DevOps platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts.

> **📖 重新开发请先读 [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)**
>
> 仓库中包含完整的文档集和 OpenSpec 形式化规格。代码已清理，需要从零重建。

---

## 项目概述

- **类型:** 内部 SRE/DevOps 平台
- **目标用户:** DevOps/SRE 工程师
- **核心功能:** 设备、流水线、日志、告警、K8s、物理主机、项目管理
- **技术栈:** Go + Gin + GORM + PostgreSQL（后端）；React + TypeScript + Vite（前端）

---

## 文档入口

| 文档 | 用途 |
|------|------|
| **[DOCUMENT_INDEX.md](DOCUMENT_INDEX.md)** | **文档索引（推荐入口）** |
| [PRD.md](PRD.md) | 产品需求（要做什么）|
| [openspec/specs/](openspec/specs/) | 形式化技术规格（怎么做，权威）|
| [ARCHITECTURE.md](ARCHITECTURE.md) | 后端架构 |
| [REQUIREMENTS.md](REQUIREMENTS.md) | 后端技术规格 + 功能完成状态 |
| [docs/FRONTEND.md](docs/FRONTEND.md) | 前端实现规格 |
| [DESIGN.md](DESIGN.md) | 前端设计系统 |
| [DEPLOY.md](DEPLOY.md) | 部署指南 |
| [docs/CONFLICTS.md](docs/CONFLICTS.md) | **文档冲突清单（必读）** |
| [CHANGELOG.md](CHANGELOG.md) | 变更日志 |

---

## 核心模块

| 模块 | 用途 |
|------|------|
| 设备管理 | 物理/虚拟/网络设备统一管理（状态机）|
| 流水线 | CI/CD 编排、阶段执行、运行历史 |
| 日志 | 多后端日志（Local/ES/Loki）|
| 监控 | Prometheus 指标采集 |
| 告警 | 多通道通知（Slack/Webhook/Email/Log）|
| WebSocket | 实时事件推送 |
| K8s 多集群 | 多集群生命周期管理 |
| 物理主机 | SSH 监控主机 |
| 项目管理 | 组织层级（业务线 → 系统 → 项目）+ FinOps |
| 审计日志 | 项目管理变动记录 |

---

## 技术栈（v2.1）

| 层级 | 技术 |
|------|------|
| 后端语言 | Go 1.21+ |
| Web 框架 | Gin（从 gorilla/mux 迁移）|
| ORM | GORM（从 database/sql 迁移）|
| 数据库 | PostgreSQL 14+ |
| 配置 | Viper |
| 日志 | log/slog（Go 1.21+ 标准库）|
| 认证 | LDAP + JWT |
| 前端 | React 18 + TypeScript + Vite |
| 状态管理 | Zustand |
| 实时 | gorilla/websocket |

> 注: 早期文档（ARCHITECTURE.md）描述的是迁移前状态。重新开发时采用上表技术栈。

---

## 快速开始（重新开发后）

```bash
# 后端
go build -o devops-toolkit ./cmd/devops-toolkit
./devops-toolkit

# 前端
cd frontend
npm install
npm run dev

# 健康检查
curl http://localhost:3000/health
```

详细部署见 [DEPLOY.md](DEPLOY.md)。

---

## 文档状态

- ✅ PRD v2.1 + openspec/specs/ 为权威来源
- ⚠️ ARCHITECTURE.md 包含迁移前内容
- 📦 docs/archive/ 为历史归档
- 🗑️ 代码已清理（参见 [CHANGELOG.md](CHANGELOG.md)）

---

## 许可证

参见 [LICENSE](LICENSE)。
