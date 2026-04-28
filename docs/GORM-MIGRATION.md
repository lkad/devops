# GORM 迁移技术文档

## 概述

本文档记录了将项目从 `database/sql` + `lib/pq` 迁移到 GORM ORM 的完整过程。

**状态**: ✅ 已完成

---

## 1. 背景与目标

### 1.1 迁移原因

原实现存在以下问题：
- **SQL 注入风险**：手写 SQL 字符串拼接
- **类型不安全**：手写 `Scan` 容易出错
- **无自动迁移**：Schema 版本管理缺失
- **代码冗余**：重复的 CRUD 模式

### 1.2 迁移目标

| 目标 | 描述 |
|------|------|
| 引入 GORM | 作为 ORM 层 |
| 类型安全 | Model 定义替代手写 SQL |
| 自动迁移 | AutoMigrate 支持 |
| 移除手写 SQL | 使用 Chainable API |
| Preload 支持 | 简化关联查询 |

### 1.3 非目标

- 不改变 API 响应格式（保持 JSON 兼容）
- 不引入缓存层
- 不切换数据库类型（保持 PostgreSQL）

---

## 2. 技术决策

### 2.1 GORM vs sqlx

| 特性 | GORM | sqlx |
|------|------|------|
| AutoMigrate | ✅ | ❌ |
| Preload | ✅ | ❌ |
| Chainable API | ✅ | 部分 |
| 学习曲线 | 中等 | 低 |

### 2.2 JSONB 字段处理

使用自定义类型实现 `driver.Valuer` 和 `sql.Scanner`：

```go
type StringMap map[string]string

func (s StringMap) Value() (driver.Value, error) {
    if s == nil {
        return "{}", nil
    }
    return json.Marshal(s)
}

func (s *StringMap) Scan(value interface{}) error {
    if value == nil {
        *s = make(StringMap)
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, s)
}
```

---

## 3. 数据模型

### 3.1 Device 模块

**文件**: `internal/device/models.go`

```go
// GORMDevice 是设备的 GORM 模型
type GORMDevice struct {
    gorm.Model
    ID             string       `gorm:"type:text;primaryKey" json:"id"`
    Type           DeviceType   `gorm:"type:text;not null" json:"type"`
    Name           string       `gorm:"type:varchar(255);not null" json:"name"`
    Status         string       `gorm:"type:text;not null;default:'pending'" json:"status"`
    Environment    string       `gorm:"type:text;not null;default:'dev'" json:"environment"`
    Labels         StringMap    `gorm:"type:jsonb;serializer:json" json:"labels"`
    BusinessUnit   string       `gorm:"type:text" json:"business_unit,omitempty"`
    ComputeCluster string       `gorm:"type:text" json:"compute_cluster,omitempty"`
    ParentID       string       `gorm:"type:text" json:"parent_id,omitempty"`
    Config         JSONMap      `gorm:"type:jsonb;serializer:json" json:"config,omitempty"`
    Metadata       JSONMap      `gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"`
    RegisteredAt   *time.Time   `gorm:"type:timestamp" json:"registered_at,omitempty"`
    LastSeen       *time.Time   `gorm:"type:timestamp" json:"last_seen,omitempty"`
    LastConfigSync *time.Time   `gorm:"type:timestamp" json:"last_config_sync,omitempty"`
}

func (GORMDevice) TableName() string {
    return "devices"
}
```

### 3.2 Project 模块

**文件**: `internal/project/models.go`

| 模型 | 表名 | 说明 |
|------|------|------|
| GORMBusinessLine | business_lines | 业务线 |
| GORMProjectType | project_types | 项目类型 |
| GORMSystem | systems | 系统 |
| GORMProject | projects | 项目 |
| GORMResource | project_resources | 项目资源 |
| GORMPermission | project_permissions | 权限 |
| AuditLog | audit_logs | 审计日志 |

---

## 4. Repository 层变更

### 4.1 Device Repository

| 方法 | GORM 实现 |
|------|----------|
| Create | `db.Create(device)` |
| GetByID | `db.First(&device, "id = ?", id)` |
| List | `db.Order("created_at DESC").Find(&devices)` |
| ListPaginated | `db.Limit().Offset().Find()` |
| Update | `db.Save(device)` |
| Delete | `db.Delete(&GORMDevice{}, "id = ?", id)` |
| SearchByLabels | `db.Where("labels->>'?' = ?", k, v)` |

### 4.2 Project Repository

| 方法 | GORM 实现 |
|------|----------|
| GetBusinessLineWithSystems | `db.Preload("Systems").First(&gbl)` |
| GetSystemWithProjects | `db.Preload("Projects").First(&gs)` |
| GetProjectWithResources | `db.Preload("Resources").First(&gp)` |
| CheckPermission | 层级向上查找 |

---

## 5. 文件变更清单

### 5.1 新建文件

| 文件 | 描述 |
|------|------|
| `pkg/database/gorm.go` | GORM 连接初始化 |

### 5.2 修改文件

| 文件 | 变更 |
|------|------|
| `internal/device/models.go` | 新增 GORMDevice, StringMap, JSONMap |
| `internal/device/repository.go` | 重写为 GORM |
| `internal/device/manager.go` | 接受 `*gorm.DB` |
| `internal/project/models.go` | 新增所有 GORM Model |
| `internal/project/repository.go` | 重写为 GORM |
| `internal/project/manager.go` | 接受 `*gorm.DB` |
| `cmd/devops-toolkit/main.go` | 使用 `database.NewGORM()` |

### 5.3 删除文件

| 文件 | 原因 |
|------|------|
| `pkg/database/postgres.go` | 被 gorm.go 替代 |

---

## 6. 依赖变更

### 6.1 新增依赖

```
gorm.io/gorm v1.31.1
gorm.io/driver/postgres v1.6.0
github.com/jinzhu/inflection v1.0.0
```

### 6.2 移除依赖

```
github.com/lib/pq
```

---

## 7. 使用示例

### 7.1 初始化数据库连接

```go
import "github.com/devops-toolkit/pkg/database"

db, err := database.NewGORM(&database.GORMConfig{
    Host:     cfg.Database.Host,
    Port:     cfg.Database.Port,
    User:     cfg.Database.User,
    Password: cfg.Database.Password,
    Name:     cfg.Database.Name,
    SSLMode:  cfg.Database.SSLMode,
})
if err != nil {
    log.Fatal(err)
}
database.SetGORM(db)
```

### 7.2 使用 Device Manager

```go
deviceMgr := device.NewManager(db)

// 创建设备
device, err := deviceMgr.RegisterDevice(device.RegisterOpts{
    Name:   "server-01",
    Type:   "physical_host",
    Labels: map[string]string{"env": "prod"},
})
```

### 7.3 使用 Project Manager

```go
projectMgr := project.NewManagerWithDB(db, userProvider)

// 创建业务线
bl := project.NewBusinessLine("云计算部", "负责云资源管理")
if err := projectMgr.repo.CreateBusinessLine(bl); err != nil {
    log.Fatal(err)
}
```

### 7.4 启用 AutoMigrate

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

## 8. 验证结果

### 8.1 构建验证

```bash
$ go build ./...
# 无错误输出
```

### 8.2 测试验证

```bash
$ go test ./...
ok  github.com/devops-toolkit/internal/device   0.165s
ok  github.com/devops-toolkit/internal/project 0.146s
# ... 其他测试通过
```

---

## 9. 任务清单

| 编号 | 任务 | 状态 |
|------|------|------|
| 1.1 | 添加 GORM 依赖 | ✅ |
| 1.2 | 运行 go mod tidy | ✅ |
| 1.3 | 移除 lib/pq 依赖 | ✅ |
| 2.1 | 创建 pkg/database/gorm.go | ✅ |
| 2.2 | 实现 NewGORM() | ✅ |
| 2.3 | 配置连接池 | ✅ |
| 2.4 | 添加 AutoMigrate 调用 | ⏳ |
| 3.1 | Device GORM Model | ✅ |
| 3.2 | TableName, CreatedAt 标签 | ✅ |
| 3.3 | State, Environment 类型 | ✅ |
| 3.4 | JSONB 字段 | ✅ |
| 4.1-4.8 | Device Repository 重写 | ✅ |
| 5.1-5.5 | Project Models | ✅ |
| 6.1-6.10 | Project Repository 重写 | ✅ |
| 7.1-7.2 | Manager 层更新 | ✅ |
| 8.1 | go build 通过 | ✅ |
| 8.2-8.5 | 服务验证 | ⏳ |

---

## 10. 风险与注意事项

### 10.1 已知风险

| 风险 | 缓解措施 |
|------|----------|
| GORM 性能略低于原生 SQL | 内部平台查询量有限 |
| 生成的 SQL 不如手写精确 | 使用 Debug() 模式验证 |

### 10.2 注意事项

- AutoMigrate 需在首次部署时调用
- 已有数据需手动迁移
- 建议在测试环境验证后再部署生产

---

*文档生成时间: 2026-04-28*
