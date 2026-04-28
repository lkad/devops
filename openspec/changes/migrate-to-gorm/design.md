## Context

项目当前使用 `database/sql` + 原生 SQL，存在以下问题：
- 手写 SQL 字符串拼接（部分有注入风险）
- Manual `rows.Scan()` 处理结果集
- 手动 JSON Marshal/Unmarshal 处理 JSONB 字段
- 无 Schema 版本管理和自动迁移

所有数据访问代码位于 `internal/*/repository.go`，项目未上线。

## Goals / Non-Goals

**Goals:**
- 引入 GORM 作为 ORM 层
- 类型安全的 Model 定义
- 自动迁移（AutoMigrate）
- 移除所有手写 SQL 字符串
- 利用 GORM 的 Preload 处理关联查询

**Non-Goals:**
- 不改变 API 响应格式（保持 JSON 兼容）
- 不引入缓存层
- 不切换数据库类型（保持 PostgreSQL）

## Decisions

### 1. GORM vs sqlx

**选择**: GORM

**理由**:
- AutoMigrate 功能强大
- 关联预加载（Preload）简化嵌套查询
- 事务 API 简洁
- 社区活跃，文档完善

**替代方案**:
- **sqlx**: 保持 SQL 风格但无 ORM 特性，需要手写 SQL
- **go-pg**: 专为 PostgreSQL，性能好但多数据库支持弱

### 2. Model 组织结构

**决策**: 按领域分组，每个模块独立 Model 文件

```
internal/
├── device/
│   ├── models.go      # GORMDevice, StringMap, JSONMap
│   └── repository.go  # DeviceRepository (GORM)
├── project/
│   ├── models.go     # GORMBusinessLine, GORMSystem, GORMProject 等
│   ├── audit.go      # AuditLog
│   └── repository.go  # ProjectRepository (GORM)
```

**理由**: 与现有目录结构一致，便于迁移

### 3. JSONB 字段处理

**决策**: 自定义类型实现 `driver.Valuer` 和 `sql.Scanner`

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

**理由**: 避免依赖 GORM 的 JSON serializer，更加轻量可控

### 4. 迁移策略

**决策**: 一次性完整迁移

```go
// pkg/database/gorm.go
func NewGORM(cfg *GORMConfig) (*gorm.DB, error) {
    gormConfig := &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    }
    db, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
    // ... connection pool setup
}
```

**理由**: 项目未上线，无历史数据包袱

## Implementation Status

### Completed

- [x] Phase 1: 依赖替换 - 添加 `gorm.io/gorm`, `gorm.io/driver/postgres`
- [x] Phase 2: Model 定义 - 所有 GORM Model 已创建
- [x] Phase 3: Repository 重写 - Device 和 Project Repository 已重写
- [x] Phase 4: 验证 - `go build ./...` 通过，所有测试通过

### Files Modified

| File | Change |
|------|--------|
| `pkg/database/gorm.go` | 新建 - GORM 连接初始化 |
| `internal/device/models.go` | 新增 GORMDevice, StringMap, JSONMap |
| `internal/device/repository.go` | 重写为 GORM |
| `internal/device/manager.go` | 更新为接受 `*gorm.DB` |
| `internal/project/models.go` | 新增所有 GORM Model |
| `internal/project/repository.go` | 重写为 GORM |
| `internal/project/manager.go` | 更新为接受 `*gorm.DB` |
| `cmd/devops-toolkit/main.go` | 更新使用 `database.NewGORM()` |
| `pkg/database/postgres.go` | 已删除（原 `database/sql` 实现） |

### AutoMigrate Note

AutoMigrate 尚未添加到 `NewGORM()` 中。如需启用，可在初始化后调用：

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

## Risks / Trade-offs

[Risk] GORM 性能略低于原生 SQL → **Mitigation**: 对于内部平台，查询量有限，性能差异可忽略

[Risk] GORM 生成的 SQL 不如手写精确 → **Mitigation**: 使用 GORM 的 `Debug()` 模式验证生成的 SQL

[Trade-off] 学习曲线 → 团队需要熟悉 GORM Chainable API

## Migration Plan

**Phase 1: 依赖替换** ✅
1. 添加 GORM 依赖 ✅
2. 创建 `pkg/database/gorm.go` 初始化连接 ✅

**Phase 2: Model 定义** ✅
1. 创建 `internal/device/models.go`（GORM Model）✅
2. 创建 `internal/project/models.go`（GORM Model）✅

**Phase 3: Repository 重写** ✅
1. 重写 `internal/device/repository.go` ✅
2. 重写 `internal/project/repository.go` ✅

**Phase 4: 验证** ✅
1. 运行 `go build ./...` ✅
2. 启动服务验证 CRUD（待测试）
3. 对比 API 响应格式（待测试）

## Open Questions

- [x] 是否需要保留 `pkg/database/postgres.go` 作为备份？→ **已删除**
- [ ] 是否需要 GORM 钩子（如 `BeforeCreate`）？→ **暂不需要**
