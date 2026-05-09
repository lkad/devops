## Why

当前项目资源只能属于一个项目，无法支持物理机或设备被多个项目共享使用的场景。按项目百分比计算财务成本是一个常见需求（例如：一台服务器 40% 用于项目A，60% 用于项目B）。现有的 `project_resources` 表虽然有 `weight` 字段，但没有在前端和财务报表中实际使用。

## What Changes

- 在项目详情页和主机详情页支持设置资源权重百分比
- FinOps 报表按权重分配资源成本到各项目
- 前端界面支持拖拽或输入方式设置权重
- 权重验证：同一资源的所有项目权重之和 ≤ 100%
- 支持从任一端（项目或资源）设置权重

## Capabilities

### New Capabilities

- `resource-weight-allocation`: 资源配置权重管理，支持设置和验证多项目权重分配
- `weighted-finops-report`: 加权财务报表，按项目权重分配资源成本

### Modified Capabilities

- `physical-host-project-linking`: 扩展现有物理主机项目关联，支持权重设置

## Impact

- **Backend**: `project_resources` 表已有 `weight` 字段，只需增强 API 和业务逻辑
- **Frontend**: 项目详情页和主机详情页需要添加权重设置 UI
- **FinOps**: 报表逻辑需要按权重分配成本
- **Database**: 可能需要为已有数据设置默认权重值（1.0）