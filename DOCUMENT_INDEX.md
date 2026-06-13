# DevOps Toolkit — 文档索引

**最后更新:** 2026-06-13 (v0.3.0.0 — B 子项目鉴权+多租户硬化落地)

> 新接手代码时,从这里开始。本文档列出所有文档的**角色、权威性和阅读顺序**。本仓库就是当前实现 (~25k 行 Go 后端 + React 前端),不需要"重新开发"。

---

## 阅读顺序

1. **本索引** — 了解文档结构
2. [README.md](README.md) — 项目入口和快速开始
3. [PRD.md](PRD.md) — 产品需求(要做什么)
4. [openspec/specs/](openspec/specs/) — 形式化技术规格(怎么做)— **权威**
5. [ARCHITECTURE.md](ARCHITECTURE.md) — 系统架构总览 (含历史迁移说明)
6. **[A 子项目 spec](docs/superpowers/specs/2026-06-12-A-production-readiness-design.md)** — v0.2.1.0 后端生产化 (生产环境 /health 拆 /live+/ready, 强制 secret, Helm chart 骨架, backup 脚本, 5-6 天 14 commit)
6. [REQUIREMENTS.md](REQUIREMENTS.md) — 后端技术规格 + 功能完成状态
7. [docs/BACKEND.md](docs/BACKEND.md) — 后端代码级规范
8. [docs/FRONTEND.md](docs/FRONTEND.md) — 前端实现规格
9. [DESIGN.md](DESIGN.md) — 前端设计系统 (Active)
10. [DEPLOY.md](DEPLOY.md) — 部署指南
11. [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md) — 测试环境准备清单 (v1.0)
12. [TEST_CASES.md](TEST_CASES.md) — 测试用例总览
13. [docs/CONFLICTS.md](docs/CONFLICTS.md) — 文档间冲突历史溯源 (选择性参考)

---

## 文档角色

### 根目录

| 文档 | 角色 | 权威性 |
|------|------|--------|
| [README.md](README.md) | 项目入口 | 中（介绍用）|
| [CLAUDE.md](CLAUDE.md) | Claude Code 项目指令 | **高**（AI 工作流）|
| [CHANGELOG.md](CHANGELOG.md) | 变更日志 | 高（事实记录）|
| [ARCHITECTURE.md](ARCHITECTURE.md) | 后端架构 | 中（含过时内容）|
| [REQUIREMENTS.md](REQUIREMENTS.md) | 后端技术规格 | **高** |
| [PRD.md](PRD.md) | 产品需求 | **高**（v2.1）|
| [DESIGN.md](DESIGN.md) | 前端设计系统 | **高**（Active）|
| [DEPLOY.md](DEPLOY.md) | 部署指南 | 中（云平台相关）|
| [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md) | 本文档 | — |
| [LICENSE](LICENSE) | 许可证 | — |

### docs/

| 文档 | 角色 | 权威性 |
|------|------|--------|
| [docs/FRONTEND.md](docs/FRONTEND.md) | 前端实现规格 | 高 |
| [docs/BACKEND.md](docs/BACKEND.md) | 后端代码级规范 | 中（Draft 状态）|
| [docs/GORM-MIGRATION.md](docs/GORM-MIGRATION.md) | GORM 迁移技术记录 | 中（历史参考）|
| [docs/CONFLICTS.md](docs/CONFLICTS.md) | **冲突清单** | **必读** |
| [docs/LOG-QUERY-API.md](docs/LOG-QUERY-API.md) | **日志查询 API 兼容性设计** | **高（v1.0）** |
| [docs/TEST-ENVIRONMENT.md](docs/TEST-ENVIRONMENT.md) | **测试环境准备清单** | **高（v1.0）** |
| [docs/archive/](docs/archive/) | 历史归档 | 低（仅供追溯）|

### openspec/specs/ — 权威技术规格

22 个 capability 规格，反映 v2.1 PRD 的最新决策。

| 类别 | Spec |
|------|------|
| 架构 | `architecture-foundation`, `middleware-stack`, `api-contract` |
| 数据 | `database-schema`, `config-management` |
| 认证授权 | `ldap-authentication`, `rbac-permissions` |
| 核心模块 | `device-management`, `physical-host-monitoring`, `physical-host-project-linking` |
| 项目管理 | `project-hierarchy` |
| K8s | `k8s-cluster-management`, `k8s-pod-log-streaming`, `k8s-pod-health` |
| 运维 | `cicd-pipeline`, `log-aggregation`, `metrics-collection`, `alert-notification` |
| 实时 | `websocket-hub`, `websocket-realtime` |
| 服务目录 | `service-catalog` |
| 可观测性 | `observability-p1` |
| 测试 | `test-environment` |
| 网络 | `network-discovery` |

### docs/superpowers/specs/ — Brainstorming 产出(子项目 spec)

| Spec | 范围 | 状态 |
|------|------|------|
| [2026-04-27-frontend-reimplementation-design](docs/superpowers/specs/2026-04-27-frontend-reimplementation-design.md) | React 前端 SPA 重构 | 已实施 |
| [2026-04-28-device-management-mock-test-design](docs/superpowers/specs/2026-04-28-device-management-mock-test-design.md) | 设备管理 mock test | 已实施 |
| [2026-06-12-A-production-readiness-design](docs/superpowers/specs/2026-06-12-A-production-readiness-design.md) | **A 子项目 — 后端生产化** (v0.2.1.0) | ✅ 已实施 |
| [2026-06-12-B-auth-rbac-hardening-design](docs/superpowers/specs/2026-06-12-B-auth-rbac-hardening-design.md) | **B 子项目 — 鉴权+多租户硬化** (v0.3.0.0) | ✅ 已实施 |
| 计划: C 子项目 (Helm 完整化) / D 子项目 (UI 空白补全) / E 子项目 (性能基线) | — | pending |

### docs/superpowers/plans/ — 实施计划

| Plan | 范围 | 状态 |
|------|------|------|
| [2026-04-27-frontend-reimplementation](docs/superpowers/plans/2026-04-27-frontend-reimplementation.md) | 前端 SPA 重构 plan | ✅ Done |
| [2026-04-27-full-implementation](docs/superpowers/plans/2026-04-27-full-implementation.md) | 全量 plan (历史) | ✅ Done |
| [2026-04-30-k8s-log-viewer](docs/superpowers/plans/2026-04-30-k8s-log-viewer.md) | K8s log viewer plan | ✅ Done |
| [2026-06-12-A-production-readiness](docs/superpowers/plans/2026-06-12-A-production-readiness.md) | **A 子项目 plan** (14 task × 105 step, 5 agent 并行) | ✅ Done |
| [2026-06-12-B-auth-rbac-hardening](docs/superpowers/plans/2026-06-12-B-auth-rbac-hardening.md) | **B 子项目 plan** (23 task × 62 step, 5 phase × agent 并行) | ✅ Done |

### openspec/changes/ — 进行中的变更

10 个 change 目录，记录具体变更的 proposal/design/tasks。

---

## 文档间冲突

**重新开发前必读 [docs/CONFLICTS.md](docs/CONFLICTS.md)**

主要冲突点（已记录在 CONFLICTS.md）：

| # | 冲突 | 当前建议 |
|---|------|----------|
| 1 | HTTP 框架：gorilla/mux vs Gin | 采用 Gin |
| 2 | 配置库：手写 vs Viper | 采用 Viper |
| 3 | 项目 Type：内嵌枚举 vs 独立表 | 采用独立表 |
| 4 | 日志库：未指定 vs Zap | ✅ 已解决：采用 log/slog（stdlib）|
| 5 | 物理主机状态：6 态 vs 3 态 | ✅ 已解决：采用 4 态（+maintenance）|

---

## 关键决策摘要

| 日期 | 决策 | 来源 |
|------|------|------|
| 2026-04-24 | 项目管理 3 级层级 (BL → System → Project) | ARCHITECTURE.md |
| 2026-04-24 | 本地 RBAC + LDAP 仅认证 | ARCHITECTURE.md |
| 2026-04-25 | 前端 API 路径使用相对地址（支持反向代理）| ARCHITECTURE.md |
| 2026-04-25 | K8s 集群 Type 字段（k3d/kind/standard）| ARCHITECTURE.md |
| 2026-04-28 | Project 权限统一为 viewer/editor/admin | PRD v2.1 |
| 2026-04-28 | 物理主机状态简化为 3 态 | PRD v2.1 |
| 2026-04-28 | 告警条件 DSL 规范 | PRD v2.1 |
| 2026-05-01 | K8s 日志时间范围限制 30 天 | ARCHITECTURE.md |
| 2026-05 | database/sql 迁移到 GORM | GORM-MIGRATION.md |
| 2026-05 | gorilla/mux 迁移到 Gin | openspec spec |
| 2026-06-04 | 物理主机状态增加 maintenance（含告警抑制）| openspec spec |
| 2026-06-05 | 日志查询 API 兼容性设计（capabilities / 降级 / 错误码）| docs/LOG-QUERY-API.md |
| 2026-06-05 | 测试环境准备清单（三层环境 + 30 个文件）| docs/TEST-ENVIRONMENT.md |
| 2026-06-05 | openspec 22 个 spec 全部加上 `## Purpose` 段并通过 validate | openspec/specs/ |
| 2026-06-05 | 主设计文档（PRD/ARCHITECTURE/REQUIREMENTS）与 openspec 同步：维护模式、日志 API 兼容、三层测试环境 | PRD/ARCHITECTURE/REQUIREMENTS |

---

## 文档状态

- ✅ **权威**: PRD.md (v2.1), openspec/specs/, REQUIREMENTS.md, DESIGN.md (Active), docs/LOG-QUERY-API.md, docs/TEST-ENVIRONMENT.md
- ⚠️ **部分过时**: ARCHITECTURE.md (迁移前内容)
- 📝 **Draft**: docs/BACKEND.md, docs/FRONTEND.md
- 📦 **历史归档**: docs/archive/ (3 个 .unused 文件)
- 🗑️ **已删除**: TODOS.md（已并入 REQUIREMENTS.md）

---

## 重新开发起点

如果要快速开始实现：

1. 读 [PRD.md](PRD.md) 了解产品目标
2. 读 [openspec/specs/architecture-foundation/spec.md](openspec/specs/architecture-foundation/spec.md) 了解代码组织
3. 读 [openspec/specs/api-contract/spec.md](openspec/specs/api-contract/spec.md) 了解 API 规范
4. 按 openspec specs 中的 22 个 capability 逐个实现
5. 实现时如有疑问，参考 [docs/CONFLICTS.md](docs/CONFLICTS.md) 解决冲突
