## Context

当前状态：
- `internal/physicalhost/` - 独立管理物理主机，有自己的SSH连接、监控、状态管理
- `internal/project/` - 项目层级管理，支持通过 `project_resources` 表链接资源
- `project_resources` 表已支持 `physical_host` 作为 ResourceType
- 数据库关联已存在，但UI层面没有实际使用

问题：
1. 物理主机详情页 (`HostDetail.tsx`) 没有显示该主机关联了哪些项目
2. 项目详情页的资源标签页不能很好地显示物理主机类型资源
3. 无法从物理主机快速跳转到项目，反之亦然

## Goals / Non-Goals

**Goals:**
- 物理主机详情页添加"关联项目"标签页，显示所有关联的项目
- 项目详情页资源标签页支持物理主机类型，显示主机名、状态
- 支持在任一端进行关联/取消关联操作

**Non-Goals:**
- 不修改物理主机的监控逻辑
- 不修改项目的权限管理逻辑
- 不添加新的API端点（复用现有 LinkResource/UnlinkResource）

## Decisions

### Decision 1: 复用现有资源链接API

**选择：** 不新建API，利用已有的 `/api/org/projects/:id/resources` 端点

**原因：**
- `project_resources` 表已支持 `physical_host` 类型
- LinkResource/UnlinkResource 逻辑已完整
- 避免重复代码

**备选：** 新建 `/api/physical-hosts/:id/projects` 端点
- 优点：语义更清晰
- 缺点：增加复杂度，需要同步两套API

### Decision 2: 物理主机端关联 vs 项目端关联

**选择：** 两端都支持关联操作

**原因：**
- 用户可能从物理主机角度出发："这台主机应该归项目X用"
- 用户也可能从项目角度出发："项目X需要添加一台物理主机"
- 两端操作最终调用同一个 API

### Decision 3: 使用现有项目资源列表查询

**选择：** 扩展 `ListProjectResources` 返回物理主机的详细信息

**备选：** 新建 `GetHostProjects(hostID)` 专门查询
- 优点：API语义更精确
- 缺点：需要维护两个查询路径

当前选择是直接扩展现有查询，通过 `resource_type = 'physical_host'` 过滤，这样前端代码改动最小。

## Risks / Trade-offs

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 物理主机被删除但资源链接残留 | FinOps报表出现孤立记录 | 添加外键约束或软删除时清理关联 |
| 项目删除时未清理物理主机关联 | 主机仍显示关联已删除项目 | 项目删除时级联清理资源链接 |
| 大量物理主机关联导致查询慢 | 项目详情页加载慢 | 添加分页或懒加载 |

## Open Questions

1. 物理主机是否需要显示"主项目"概念（主属项目 vs 共享项目）？
   - 当前设计：一台主机可以关联多个项目（共享资源）
   - 如需主项目，需要在 `project_resources` 添加 `is_primary` 字段

2. 物理主机取消关联时是否需要二次确认？
   - 当前设计：直接删除链接
   - 建议：对于已分配到项目的资源，取消关联需要确认
