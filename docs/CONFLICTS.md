# Documentation Conflicts Log

本文档记录文档中**未解决的冲突**。重新开发时，需要先决定采用哪个版本。

**权威优先级（按从高到低）：**
1. [openspec/specs/](../openspec/specs/) — 形式化规格，反映 v2.1 PRD 最新决策
2. [REQUIREMENTS.md](../REQUIREMENTS.md) — 当前技术规格
3. [docs/BACKEND.md](BACKEND.md) — 后端技术规格 (Draft)
4. [ARCHITECTURE.md](../ARCHITECTURE.md) — 早期架构描述
5. [docs/archive/](archive/) — 历史归档

---

## 冲突 #1: HTTP Web 框架

| 文档 | 描述 |
|------|------|
| [ARCHITECTURE.md](../ARCHITECTURE.md) | `gorilla/mux` + `net/http` |
| [REQUIREMENTS.md](../REQUIREMENTS.md) | **Gin**（从 gorilla/mux 迁移完成）|
| [docs/BACKEND.md](BACKEND.md) | **Gin** |

**建议:** 采用 Gin。REQUIREMENTS.md 和 BACKEND.md 一致，ARCHITECTURE.md 已过时。

---

## 冲突 #2: 配置库

| 文档 | 描述 |
|------|------|
| [ARCHITECTURE.md](../ARCHITECTURE.md) | YAML + 环境变量覆盖（无具体库）|
| [docs/BACKEND.md](BACKEND.md) | **Viper** |

**建议:** 采用 Viper。BACKEND.md 更详细。

---

## 冲突 #3: 项目 Type 字段

| 文档 | 描述 |
|------|------|
| [ARCHITECTURE.md](../ARCHITECTURE.md) | `Project.type` 内嵌枚举 (frontend/backend) |
| [REQUIREMENTS.md](../REQUIREMENTS.md) | **独立 `project_types` 表 + `GORMProjectType` 模型** |

**建议:** 采用独立表设计。REQUIREMENTS.md 是 v2.1 设计，扩展性更好。

---

## 冲突 #4: 日志库（已解决）

| 文档 | 原描述 | 当前 |
|------|------|------|
| [docs/BACKEND.md](BACKEND.md) | Zap | log/slog（已更新）|
| [README.md](../README.md) | — | log/slog |
| 其他文档 | 未指定 | — |

**决议:** 采用 **log/slog**（stdlib）。理由：
- 零依赖，Go 1.21+ 自带
- 官方长期支持（不会像 logrus 一样被弃用）
- 性能与 Zap 相当（内部平台场景差距可忽略）
- Gin 1.9+ / GORM 1.25+ 原生支持

何时仍选 Zap：极端高 QPS，或需要 Sugar API（printf 风格）。

---

## 冲突 #5: 物理主机状态机（已解决）

| 文档 | 原描述 | 当前 |
|------|------|------|
| [PRD.md](../PRD.md) v2.0 | 6 态 | — |
| [DESIGN.md](../DESIGN.md) | 3 态：online / monitoring_issue / offline | **4 态：+ maintenance** |
| [openspec/specs/physical-host-monitoring/spec.md](../openspec/specs/physical-host-monitoring/spec.md) | — | **4 态：online / monitoring_issue / offline / maintenance** |

**决议:** 采用 **4 态模型**：
| 状态 | 颜色 | 说明 |
|------|------|------|
| online | `#22c55e` (绿) | 监控正常，SSH 正常 |
| monitoring_issue | `#f59e0b` (黄) | 监控 DOWN，SSH 正常 |
| offline | `#ef4444` (红) | 监控 DOWN，SSH 失败 |
| **maintenance** | `#a855f7` (紫) | 手动维护，抑制外部告警 |

**维护模式行为：**
- 抑制外部告警（Slack/Webhook/Email）
- 内部日志和 WebSocket 仍记录
- 自动状态转换被阻止（不会变 offline）
- 监控健康检查继续运行
- 审计日志记录所有进入/退出
- 退出后不补发抑制期间的告警

详见 [openspec/specs/physical-host-monitoring/spec.md](../openspec/specs/physical-host-monitoring/spec.md) 的 Maintenance Mode 章节和 [openspec/specs/alert-notification/spec.md](../openspec/specs/alert-notification/spec.md) 的 Maintenance Mode Alert Suppression 章节。

---

## 冲突 #6: 设备状态机

| 文档 | 描述 |
|------|------|
| [ARCHITECTURE.md](../ARCHITECTURE.md) | 6 态：PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE |
| [TODOS.md](../TODOS.md)（已并入 REQUIREMENTS）| 同上 6 态 |

**建议:** 两边一致，保留 6 态。

---

## 已澄清（非冲突，是双层设计）

### 角色模型

| 层级 | 模型 | 文档 |
|------|------|------|
| **全局** | SuperAdmin / Operator / Developer / Auditor | [REQUIREMENTS.md](../REQUIREMENTS.md) |
| **项目级** | viewer / editor / admin | [ARCHITECTURE.md](../ARCHITECTURE.md) |

**说明:** 这是有意的双层设计，不是冲突。全局角色来自 LDAP 组映射，项目级角色是项目本地 RBAC。

### 状态色 vs 状态机

| 概念 | 含义 |
|------|------|
| **状态机** | 设备的生命周期阶段（6 态）|
| **状态色** | 物理主机监控徽章颜色（3 色）|

**说明:** 不同模块的状态定义不同，不是冲突。

---

## 重新开发时建议

1. **先读 openspec/specs/** — 22 个 capability 规格是单一真相源
2. **本文档列出的冲突** — 重新开发时按"建议"列决策
3. **历史归档** — `docs/archive/` 内容仅供决策追溯，不作实现依据
