## Why

当前项目使用 `database/sql` + 原生 SQL，手写 SQL 字符串、manual `rows.Scan()`、手动 JSON 处理。这种方式存在以下问题：

- **SQL 注入风险**：部分代码使用字符串拼接（如 `fmt.Sprintf(" AND labels->>'%s' = $%d", k, argIdx)`）
- **类型不安全**：手写 Scan 容易出错，15+ 字段的表容易遗漏
- **无自动迁移**：Schema 版本管理缺失
- **代码冗余**：每个 Repository 都有大量重复的 CRUD 模式

GORM 提供类型安全的 ORM，自动迁移，事务支持，生命周期钩子，能显著提升开发效率和代码质量。

## What Changes

- 移除 `database/sql` + `lib/pq`，引入 `gorm.io/gorm` + `gorm.io/driver/postgres`
- 创建 GORM Model 定义（替换现有的手写 schema）
- 重写 Repository 层使用 GORM API
- 利用 GORM AutoMigrate 自动创建/更新表结构
- 移除手写 JSON Marshal/Unmarshal，利用 GORM 的 JSON 字段支持

**BREAKING**: 移除所有手写 SQL 字符串，替换为 GORM Chainable API

## Status

**IMPLEMENTED** - All tasks completed ✅

### Implementation Summary

| Module | Status | Files Changed |
|--------|--------|---------------|
| Device GORM Model | ✅ Complete | internal/device/models.go |
| Device Repository | ✅ Complete | internal/device/repository.go |
| Device Manager | ✅ Complete | internal/device/manager.go |
| Project GORM Models | ✅ Complete | internal/project/models.go |
| Project Repository | ✅ Complete | internal/project/repository.go |
| Project Manager | ✅ Complete | internal/project/manager.go |
| Database Init | ✅ Complete | pkg/database/gorm.go |
| Main Integration | ✅ Complete | cmd/devops-toolkit/main.go |

### Build & Test Status

```
go build ./...  # ✅ Passed
go test ./...   # ✅ All tests passed
```

## Capabilities

### New Capabilities
- `database-orm`: GORM ORM 层，定义所有数据模型和 Repository

### Modified Capabilities
- 无 spec 级行为变更 - 此为纯实现层迁移

## Impact

**代码变更范围**:
- `internal/device/repository.go` → GORM 模式 ✅
- `internal/project/repository.go` → GORM 模式 ✅
- `pkg/database/postgres.go` → 已删除 ✅
- 新增 GORM Model 文件 ✅

**数据模型清单**:
```
devices (device)
├── business_lines
├── project_types
├── systems
├── projects
├── project_resources
├── project_permissions
└── audit_logs
```

**依赖变更**:
- 移除: `github.com/lib/pq` ✅
- 添加: `gorm.io/gorm v1.31.1` ✅
- 添加: `gorm.io/driver/postgres v1.6.0` ✅
