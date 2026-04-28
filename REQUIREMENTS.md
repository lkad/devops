# DevOps Toolkit - 技术需求文档

**版本:** 1.0
**最后更新:** 2026-04-28
**状态:** 进行中

---

## 目录

1. [技术栈](#1-技术栈)
2. [数据库层](#2-数据库层)
3. [API 层](#3-api-层)
4. [认证与授权](#4-认证与授权)
5. [核心功能模块](#5-核心功能模块)
6. [基础设施集成](#6-基础设施集成)
7. [前端](#7-前端)
8. [部署](#8-部署)

---

## 1. 技术栈

### 1.1 后端技术栈

| 组件 | 技术 | 状态 |
|------|------|------|
| 语言 | Go 1.21+ | ✅ |
| Web 框架 | Gin | ✅ 从 net/http + gorilla/mux 迁移完成 |
| ORM | GORM | ✅ 已从 database/sql 迁移 |
| 数据库 | PostgreSQL | ✅ |
| 配置 | YAML | ✅ |

### 1.2 依赖清单

```
# Web 框架
github.com/gin-gonic/gin

# ORM
gorm.io/gorm v1.31.1
gorm.io/driver/postgres v1.6.0

# 认证
github.com/golang-jwt/jwt/v5
github.com/go-ldap/ldap

# Kubernetes
k8s.io/client-go

# 配置
gopkg.in/yaml.v3
```

---

## 2. 数据库层

### 2.1 GORM ORM

**状态:** ✅ 已实现

**迁移前:**
- `database/sql` + `lib/pq`
- 手写 SQL 字符串拼接
- Manual `rows.Scan()` 处理
- 手动 JSON Marshal/Unmarshal

**迁移后:**
- `gorm.io/gorm` + `gorm.io/driver/postgres`
- Chainable API
- 类型安全 Model
- 自动 JSONB 序列化

### 2.2 数据模型

#### Device 模块 (`internal/device/`)

| 模型 | 表名 | 说明 |
|------|------|------|
| GORMDevice | devices | 设备主表 |
| DeviceStateTransition | - | 状态转换历史 |
| DeviceGroup | - | 设备分组 |

**JSONB 字段:**
- `Labels` - `StringMap` (map[string]string)
- `Config` - `JSONMap` (map[string]interface{})
- `Metadata` - `JSONMap` (map[string]interface{})

#### Project 模块 (`internal/project/`)

| 模型 | 表名 | 说明 |
|------|------|------|
| GORMBusinessLine | business_lines | 事业群 |
| GORMProjectType | project_types | 项目类型 |
| GORMSystem | systems | 系统 |
| GORMProject | projects | 项目 |
| GORMResource | project_resources | 资源链接 |
| GORMPermission | project_permissions | 权限 |
| AuditLog | audit_logs | 审计日志 |

**层级关系:**
```
BusinessLine (事业群)
  └── System (系统)
        └── Project (项目)
              └── Resource (资源)
```

### 2.3 Repository 接口

#### Device Repository

```go
type DeviceRepository interface {
    Create(d *Device) error
    GetByID(id string) (*Device, error)
    List() ([]*Device, error)
    ListPaginated(limit, offset int) ([]*Device, int, error)
    Update(d *Device) error
    Delete(id string) error
    UpdateStatus(id string, status State) error
    SearchByLabels(labels map[string]string) ([]*Device, error)
}
```

#### Project Repository

```go
type ProjectRepository interface {
    // ProjectType CRUD
    ListProjectTypes() ([]*ProjectTypeDef, error)
    GetProjectType(id string) (*ProjectTypeDef, error)
    CreateProjectType(pt *ProjectTypeDef) error
    UpdateProjectType(pt *ProjectTypeDef) error
    DeleteProjectType(id string) error

    // BusinessLine CRUD
    CreateBusinessLine(bl *BusinessLine) error
    GetBusinessLine(id string) (*BusinessLine, error)
    ListBusinessLines(page, perPage int) ([]*BusinessLine, int, error)
    GetBusinessLineWithSystems(id string) (*BusinessLine, error)
    UpdateBusinessLine(bl *BusinessLine) error
    DeleteBusinessLine(id string) error

    // System CRUD
    CreateSystem(s *System) error
    GetSystem(id string) (*System, error)
    ListSystems(page, perPage int) ([]*System, int, error)
    ListSystemsByBusinessLine(blID string) ([]*System, error)
    GetSystemWithProjects(id string) (*System, error)
    UpdateSystem(s *System) error
    DeleteSystem(id string) error

    // Project CRUD
    CreateProject(p *Project) error
    GetProject(id string) (*Project, error)
    ListProjects(page, perPage int) ([]*Project, int, error)
    ListProjectsBySystem(sysID string) ([]*Project, error)
    GetProjectWithResources(id string) (*Project, error)
    UpdateProject(p *Project) error
    DeleteProject(id string) error

    // Resource CRUD
    CreateProjectResource(pr *ProjectResource) error
    ListProjectResources(projectID string) ([]*Resource, error)
    DeleteProjectResource(projectID, resourceID string) error

    // Permission CRUD
    CreatePermission(p *ProjectPermission) error
    ListPermissionsByProject(projectID string) ([]*ProjectPermission, error)
    ListPermissionsBySubject(subject string) ([]*ProjectPermission, error)
    DeletePermission(id string) error
    CheckPermission(subject string, projectID string) (Role, error)

    // AuditLog CRUD
    CreateAuditLog(log *AuditLog) error
    ListAuditLogs(entityType, entityID, username string, limit, offset int) ([]*AuditLog, int, error)
    GetAuditLog(id string) (*AuditLog, error)

    // FinOps
    GetFinOpsData(period string) ([]FinOpsRow, error)
}
```

### 2.4 AutoMigrate

首次部署时需调用:

```go
db.AutoMigrate(
    &device.GORMDevice{},
    &project.GORMBusinessLine{},
    &project.GORMProjectType{},
    &project.GORMSystem{},
    &project.GORMProject{},
    &project.GORMResource{},
    &project.GORMPermission{},
    &project.AuditLog{},
)
```

---

## 3. API 层

### 3.1 路由结构

```
/api/
├── auth/
│   ├── POST /login
│   ├── POST /logout
│   └── GET /me
├── devices/
│   ├── GET /devices
│   ├── POST /devices
│   ├── GET /devices/:id
│   ├── PUT /devices/:id
│   ├── DELETE /devices/:id
│   └── GET /devices/search
├── pipelines/
│   ├── GET /pipelines
│   ├── POST /pipelines
│   ├── GET /pipelines/:id
│   ├── DELETE /pipelines/:id
│   └── POST /pipelines/:id/execute
├── logs/
│   ├── GET /logs
│   ├── POST /logs
│   ├── GET /logs/stats
│   ├── GET /logs/alerts
│   ├── POST /logs/alerts
│   └── ...
├── metrics/
│   └── GET /metrics
├── alerts/
│   ├── GET /channels
│   ├── POST /channels
│   └── GET /history
├── k8s/
│   ├── GET /clusters
│   ├── POST /clusters
│   ├── DELETE /clusters/:name
│   └── ...
├── physical-hosts/
│   ├── GET /physical-hosts
│   ├── POST /physical-hosts
│   └── ...
├── discovery/
│   ├── GET /discovery/status
│   └── POST /discovery/scan
└── org/           # 项目管理
    ├── GET /project-types
    ├── POST /project-types
    ├── GET /business-lines
    ├── POST /business-lines
    ├── GET /business-lines/:id
    ├── GET /business-lines/:bl_id/systems
    ├── POST /business-lines/:bl_id/systems
    ├── GET /systems/:id
    ├── GET /systems/:sys_id/projects
    ├── POST /systems/:sys_id/projects
    ├── GET /projects/:id
    ├── GET /projects/:id/resources
    ├── POST /projects/:id/resources
    ├── GET /projects/:id/permissions
    ├── POST /projects/:id/permissions
    ├── GET /reports/finops
    └── GET /audit-logs
```

### 3.2 响应格式

**分页响应:**
```json
{
  "data": [...],
  "pagination": {
    "total": 100,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

**错误响应:**
```json
{
  "error": "error message",
  "code": "ERROR_CODE"
}
```

---

## 4. 认证与授权

### 4.1 认证方式

| 方式 | 说明 |
|------|------|
| LDAP | 企业用户认证 |
| JWT | API 认证令牌 |

### 4.2 角色模型

| 角色 | 说明 |
|------|------|
| SuperAdmin | 全部权限 |
| Operator | 运维权限 |
| Developer | 开发权限 |
| Auditor | 审计权限 |

### 4.3 权限继承

```
BusinessLine (事业群级权限)
  └── System (系统级权限)
        └── Project (项目级权限)
```

---

## 5. 核心功能模块

### 5.1 设备管理

**状态:** ✅ 完成

**功能:**
- 设备状态机 (PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE)
- 设备注册和认证
- 父子层级关系
- 设备分组
- 标签批量操作
- 配置模板

### 5.2 项目管理

**状态:** ✅ 完成

**功能:**
- 事业群 → 系统 → 项目 3级层级
- 资源链接
- RBAC 权限控制
- FinOps 报表导出
- 审计日志

### 5.3 日志系统

**状态:** ✅ 完成

**后端支持:**
- Local (默认)
- Elasticsearch
- Loki

**功能:**
- 日志查询
- 日志统计
- 告警规则
- 保留策略

### 5.4 CI/CD 流水线

**状态:** ✅ 完成

**功能:**
- 流水线 CRUD
- 阶段执行引擎
- 运行历史
- 取消运行

### 5.5 告警通知

**状态:** ✅ 完成

**通道:**
- Slack
- Webhook
- Email
- Log

---

## 6. 基础设施集成

### 6.1 Kubernetes

**状态:** ✅ 完成

**功能:**
- 多集群管理 (k3d)
- 健康检查
- 工作负载部署
- Pod 日志
- 指标采集

### 6.2 物理主机

**状态:** ✅ 完成

**功能:**
- SSH 连接管理
- 心跳监控
- 指标采集
- 配置推送

### 6.3 网络发现

**状态:** ✅ 完成

**协议:**
- SNMP v2c
- SSH

---

## 7. 前端

### 7.1 技术栈

| 组件 | 技术 |
|------|------|
| 框架 | React SPA |
| 构建 | Vite |
| 路由 | React Router |
| 状态 | React Context |
| HTTP | Fetch API |

### 7.2 页面结构

```
/                   # 首页仪表盘
/login              # 登录页
/devices            # 设备列表
/devices/:id        # 设备详情
/pipelines          # 流水线列表
/pipelines/:id      # 流水线详情
/logs               # 日志查询
/metrics            # 指标展示
/alerts             # 告警管理
/k8s                # K8s 集群管理
/physical-hosts     # 物理主机
/projects           # 项目管理
```

---

## 8. 部署

### 8.1 环境要求

- Go 1.21+
- PostgreSQL 14+
- Kubernetes 1.26+ (可选)

### 8.2 配置项

```yaml
server:
  host: "0.0.0.0"
  port: 3000

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: ""
  name: "devops"
  sslmode: "disable"

auth:
  ldap_url: ""
  dev_bypass: false
```

---

## 附录 A: 文件结构

```
internal/
├── device/
│   ├── models.go      # GORMDevice, StringMap, JSONMap
│   ├── repository.go  # GORM Repository
│   ├── manager.go    # 业务逻辑
│   └── manager_test.go
├── project/
│   ├── models.go     # GORM Models
│   ├── repository.go # GORM Repository
│   ├── manager.go    # 业务逻辑
│   ├── manager_test.go
│   └── audit.go      # AuditLog
├── auth/
├── logs/
├── metrics/
├── alerts/
├── k8s/
├── pipeline/
├── physicalhost/
└── discovery/

pkg/
└── database/
    └── gorm.go       # GORM 连接初始化
```

---

## 附录 B: 测试覆盖

| 模块 | 测试数 |
|------|--------|
| device | 8+ |
| project | 24+ |
| logs | + |
| metrics | + |
| alerts | + |
| k8s | 16+ |
| auth/ldap | + |
| websocket | + |
| pipeline | + |
| physicalhost | + |
| discovery | + |

**总计:** 450+ 测试

---

*文档版本: 1.0*
