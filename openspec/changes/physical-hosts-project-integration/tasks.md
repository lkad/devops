## 1. Backend - 物理主机项目关联查询

- [x] 1.1 在 physicalhost Manager 添加 GetLinkedProjects(hostID) 方法
- [x] 1.2 添加 HTTP handler: GET /api/physical-hosts/:id/projects
- [x] 1.3 注册新路由到 main.go

## 2. Frontend - 物理主机详情页关联项目标签页

- [x] 2.1 在 HostDetail.tsx 添加"关联项目"标签页
- [x] 2.2 调用 GET /api/physical-hosts/:id/projects 获取数据
- [x] 2.3 显示项目列表（名称、系统、业务线、关联时间）
- [x] 2.4 添加取消关联按钮

## 3. Frontend - 项目详情页物理主机资源显示

- [x] 3.1 在 ProjectResources.tsx 添加物理主机类型过滤
- [x] 3.2 物理主机资源显示：名称、IP、状态
- [x] 3.3 添加跳转到主机详情的链接

## 4. 功能测试

- [x] 4.1 测试：主机详情页显示关联项目
- [x] 4.2 测试：从主机详情页取消关联项目
- [x] 4.3 测试：项目详情页显示关联的物理主机
- [x] 4.4 测试：从项目详情页添加物理主机关联
