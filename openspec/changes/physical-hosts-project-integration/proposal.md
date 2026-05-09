## Why

当前物理主机管理和项目层级管理是两个独立的系统。物理主机可以在项目中被链接（通过 `project_resources` 表），但物理主机详情页和项目之间没有实际的关联操作。用户无法在项目UI中看到所关联的物理主机，也无法从物理主机详情页看到它属于哪个项目。

FinOps报表需要物理主机作为资源类型参与成本分摊，但目前物理主机与项目的关联是手动操作，缺乏自动化。

## What Changes

- 物理主机详情页添加"关联项目"标签页，显示/管理该主机所属的项目
- 项目详情页的"资源"标签页支持查看和管理关联的物理主机
- 将物理主机注册/取消注册自动记录到审计日志
- 支持通过API将物理主机批量关联到项目

## Capabilities

### New Capabilities

- `physical-host-project-linking`: 物理主机与项目的关联管理
  - 在物理主机详情页显示关联的项目列表
  - 在项目详情页显示关联的物理主机列表
  - 支持从物理主机或项目两侧进行关联/取消关联操作

### Modified Capabilities

- `physical-host-monitoring`: 添加项目关联显示
  - 物理主机详情页添加"关联项目"标签页
  - 无需修改现有监控功能，只扩展UI和关联查询

- `project-hierarchy`: 扩展资源管理支持物理主机
  - 项目资源列表明确显示物理主机类型资源
  - 添加快捷操作：从项目直接跳转到物理主机详情

## Impact

- `internal/physicalhost/manager.go` - 添加项目关联查询方法
- `internal/project/manager.go` - 已有 LinkResource/UnlinkResource，支持 physical_host 类型
- `frontend/src/pages/physical-hosts/HostDetail.tsx` - 添加关联项目标签页
- `frontend/src/pages/projects/ProjectResources.tsx` - 支持物理主机资源显示
- 数据库：`project_resources` 表已支持 `physical_host` 类型，无需迁移
