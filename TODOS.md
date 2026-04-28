# DevOps Toolkit - 待办事项

**最后更新:** 2026-04-28

## 状态总览

| 组件 | 状态 | 说明 |
|-----------|--------|-------|
| 日志系统 | ✅ 完成 | Local/ES/Loki 后端，查询委托 |
| Prometheus 指标 | ✅ 完成 | /metrics 端点，计数器/仪表/直方图 |
| 告警通知 | ✅ 完成 | Slack/webhook/email/log 通道，限流 |
| WebSocket | ✅ 完成 | 实时事件广播 |
| CI/CD 流水线 | ✅ 完成 | 执行引擎，阶段模拟，运行历史 |
| 设备管理 | ✅ 完成 | 状态机，设备组，层级关系，配置模板 |
| LDAP 认证 | ✅ 完成 | LDAP 认证，组角色映射 |
| 权限模型 | ✅ 完成 | 中间件强制执行，基于标签的访问控制 |
| K8s 多集群 | ✅ 完成 | k3d集群管理，多集群健康检查 |
| 物理主机管理 | ✅ 完成 | SSH连接管理，状态监控，指标采集 |
| 项目管理 | ✅ 完成 | 事业群 → 系统 → 项目层级，FinOps 报表 |
| 审计日志 | ✅ 完成 | 项目管理变动记录，审计界面 |
| GORM ORM | ✅ 完成 | database/sql 迁移到 GORM |

---

## ✅ 已完成功能

### 项目管理模块
- [x] 事业群 CRUD (`/api/org/business-lines`)
- [x] 系统 CRUD (`/api/org/business-lines/:id/systems`)
- [x] 项目 CRUD (`/api/org/systems/:id/projects`)
- [x] 资源链接 (`/api/org/projects/:id/resources`)
- [x] 级别RBAC权限 (viewer, editor, admin)
- [x] FinOps CSV 导出 (`/api/org/reports/finops?period=YYYY-MM`)
- [x] 审计日志 (`/api/org/audit-logs`) - 记录所有 CRUD 变动
- [x] PostgreSQL 迁移

**数据模型:**
- BusinessLine → System → Project (3级层级)
- ProjectResource (链接表，关联日志、告警、设备、流水线、主机)
- ProjectPermission (本地RBAC，LDAP仅用于认证)

### 日志系统
- [x] 本地存储后端 (默认)
- [x] Elasticsearch 后端
- [x] Loki 后端
- [x] 基于存储类型的查询委托
- [x] 日志保留策略
- [x] 日志统计和过滤
- [x] Filebeat 集成脚本

### Prometheus 指标
- [x] /metrics 端点 (Prometheus 格式)
- [x] /api/metrics 端点 (JSON)
- [x] Counter、Gauge、Histogram 支持
- [x] HTTP 请求指标中间件
- [x] 设备事件指标

### 告警通知
- [x] 通道管理 (slack, webhook, email, log)
- [x] 告警触发 API
- [x] 限流 (每名称 10条/分钟)
- [x] 告警历史和统计

### WebSocket
- [x] /ws 端点
- [x] 通道订阅 (log, metric, device_event, pipeline_update, alert)
- [x] 实时事件广播
- [x] 日志/告警管理器的回调集成

### CI/CD 流水线
- [x] 流水线 CRUD 操作
- [x] 阶段执行引擎 (模拟)
- [x] 运行历史追踪
- [x] 流水线统计
- [x] 取消运行支持

### 设备管理
- [x] 设备状态机 (PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE)
- [x] 状态转换验证
- [x] 设备注册和认证
- [x] 父子关系 (层级)
- [x] 设备组 (层级/动态)
- [x] 按标签批量操作
- [x] 配置模板 (基础)

### LDAP 认证
- [x] LDAP 服务器用户认证
- [x] 组 membership 获取
- [x] 组到角色映射
- [x] 连接池
- [x] 健康检查

### 权限模型
- [x] 基于角色的访问控制中间件
- [x] 设备权限检查
- [x] 基于标签的访问控制
- [x] 环境限制 (prod/dev/test)
- [x] 业务层级继承
- [x] SuperAdmin/Operator/Developer/Auditor 角色

### K8s 多集群管理
- [x] 通过 k3d 实现多集群 Kubernetes 管理
- [x] 集群健康检查 (节点, 部署)
- [x] 工作负载部署和扩缩容
- [x] 指标采集 (CPU/内存)
- [x] Pod 日志获取
- [x] 跨集群操作
- [x] k3d-setup.sh 环境脚本

### 物理主机管理
- [x] SSH 连接管理和连接池
- [x] 主机注册和移除
- [x] 心跳机制状态监控
- [x] 指标采集 (CPU, 内存, 磁盘, 运行时间)
- [x] 服务监控 (systemctl/service)
- [x] 通过 SSH 推送配置
- [x] 状态变化事件发送

### GORM ORM 迁移
- [x] `database/sql` + `lib/pq` → `gorm.io/gorm` + `gorm.io/driver/postgres`
- [x] Device 模块 GORM Model (GORMDevice, StringMap, JSONMap)
- [x] Project 模块 GORM Model (BusinessLine, System, Project, Resource, Permission, AuditLog)
- [x] Repository 层重写 (Device, Project)
- [x] Manager 层更新接受 `*gorm.DB`
- [x] AutoMigrate 支持 (需首次部署时调用)
- [x] 移除手写 SQL，修复 SQL 注入风险

---

## 测试脚本

| 脚本 | 说明 |
|--------|-------------|
| `scripts/run-tests.sh` | 运行单元测试 |
| `scripts/integration-test.sh` | 对实时服务器运行集成测试 |
| `scripts/run-ci-tests.sh` | CI 流水线测试运行器 |
| `scripts/test-logs.sh` | 日志系统测试 |
| `scripts/k3d-setup.sh` | 设置 k3d 多集群环境 |

### 测试命令
```bash
# Go 单元测试
go test ./...

# Go 单元测试 (详细)
go test ./... -v

# 前端测试
node devops-toolkit/frontend/frontend.test.js

# Go 编译检查
go build ./...
```

---

## 测试覆盖率

**当前状态:** 450+ tests passing (含K8s集成测试)

### Go 测试文件 (internal/)
- `internal/device/` - 8 tests (设备状态机, CRUD)
- `internal/pipeline/` - tests (CI/CD 阶段)
- `internal/logs/` - tests (日志存储, 查询, 告警, 保留)
- `internal/metrics/` - tests (Prometheus 指标)
- `internal/alerts/` - tests (告警通道, 限流)
- `internal/websocket/` - tests (WebSocket 事件)
- `internal/k8s/` - 16 tests (K8s 集群管理，包含真实k3d集群集成测试)
- `internal/discovery/` - tests (网络发现)
- `internal/physicalhost/` - tests (SSH 主机管理)
- `internal/auth/ldap/` - tests (LDAP 认证)
- `internal/project/` - 24 tests (项目管理)

### 前端测试文件
- `devops-toolkit/frontend/frontend.test.js` - 35 tests (UI 逻辑测试)

### 测试原则
**集成测试使用真实环境数据:**
- K8s测试: 连接真实的k3d集群 (dev-cluster-1, dev-cluster-2)
- 测试节点、Pod、命名空间、日志获取等操作
- Cordon/Uncordon 操作使用专门的测试节点
- 不使用 mock 数据，确保测试反映真实使用场景

### PRD 需求测试
- [x] CI/CD smoke_test 阶段测试
- [x] 部署后验证阶段测试
- [x] LDAP 连接重试逻辑测试
- [x] 健康检查验证测试
- [x] 指标管理器测试 (计数器, 仪表, 直方图, Prometheus 导出)
- [x] 告警通知管理器测试 (通道, 限流, 历史)
- [x] WebSocket 管理器测试 (订阅, 广播, 通道)
- [x] 设备代理测试 (状态, 连接, 配置)
- [x] 存储后端测试 (Local, Elasticsearch, Loki)
- [x] 网络发现测试 (SNMP/SSH 设备扫描)
- [x] 日志管理器测试 (查询, 告警规则, 保留, 统计)

---

## 新建/修改文件

### 项目管理 (Go)
- `internal/project/models.go` - 数据模型 (BusinessLine, System, Project, ProjectResource, ProjectPermission)
- `internal/project/repository.go` - PostgreSQL 仓库和迁移
- `internal/project/manager.go` - CRUD 处理器和 HTTP 端点
- `internal/project/manager_test.go` - Go 测试 (24 tests)

### 前端
- `devops-toolkit/frontend/index.html` - 项目管理页面 UI
- `devops-toolkit/frontend/frontend.test.js` - 前端测试 (35 tests)

### 修改的文件
- `cmd/devops-toolkit/main.go` - 添加项目管理路由
- `TODOS.md` - 本文档

---

## 配置文件

| 文件 | 用途 |
|------|------|
| `config/ldap.yaml` | LDAP 服务器连接和角色映射 |
| `config/permissions.yaml` | 角色到权限的映射 |
| `config/pipelines.json` | 流水线定义和运行历史 |
| `config/devices/devices.json` | 设备注册表 |
| `config/devices/groups.json` | 设备组 (自动创建) |
| `config/logs.json` | 日志存储配置 |
