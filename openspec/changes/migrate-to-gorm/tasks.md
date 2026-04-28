## 1. 依赖更新

- [x] 1.1 添加 GORM 依赖：`go get gorm.io/gorm gorm.io/driver/postgres`
- [x] 1.2 运行 `go mod tidy` 确保依赖完整
- [x] 1.3 移除 `github.com/lib/pq` 依赖

## 2. 创建 GORM 数据库连接

- [x] 2.1 创建 `pkg/database/gorm.go` 初始化函数
- [x] 2.2 实现 `NewGORM()` 返回 `*gorm.DB`
- [x] 2.3 配置连接池参数（MaxOpenConns, MaxIdleConns）
- [ ] 2.4 添加 AutoMigrate 调用

## 3. 创建 Device Model

- [x] 3.1 在 `internal/device/models.go` 定义 GORM Model
- [x] 3.2 添加 TableName, CreatedAt, UpdatedAt 标签
- [x] 3.3 定义 State, Environment 类型
- [x] 3.4 添加 JSONB 字段（Labels, Config, Metadata）

## 4. 重写 Device Repository

- [x] 4.1 将 `Repository` 结构体从 `*sql.DB` 改为 `*gorm.DB`
- [x] 4.2 实现 `Create()` 使用 `db.Create()`
- [x] 4.3 实现 `GetByID()` 使用 `db.First()`
- [x] 4.4 实现 `List()` 使用 `db.Find()`
- [x] 4.5 实现 `ListPaginated()` 使用 `db.Limit().Offset()`
- [x] 4.6 实现 `Update()` 使用 `db.Save()`
- [x] 4.7 实现 `Delete()` 使用 `db.Delete()`
- [x] 4.8 实现 `SearchByLabels()` 使用 `db.Where()` 动态查询

## 5. 创建 Project Model

- [x] 5.1 在 `internal/project/models.go` 定义 BusinessLine GORM Model
- [x] 5.2 定义 System GORM Model（含外键关联）
- [x] 5.3 定义 Project GORM Model（含外键关联）
- [x] 5.4 定义 ProjectType, ProjectResource, ProjectPermission GORM Model
- [x] 5.5 定义 AuditLog GORM Model

## 6. 重写 Project Repository

- [x] 6.1 将 `Repository` 结构体从 `*sql.DB` 改为 `*gorm.DB`
- [x] 6.2 实现 BusinessLine CRUD（Create, Get, List, Update, Delete）
- [x] 6.3 实现 System CRUD（Create, Get, List, Update, Delete）
- [x] 6.4 实现 Project CRUD（Create, Get, List, Update, Delete）
- [x] 6.5 实现 ProjectType CRUD
- [x] 6.6 实现 ProjectResource CRUD
- [x] 6.7 实现 Permission CRUD
- [x] 6.8 实现 AuditLog CRUD
- [x] 6.9 实现权限检查逻辑（CheckPermission）
- [x] 6.10 实现 FinOps 报表查询

## 7. 更新 Manager 层

- [x] 7.1 更新 `internal/device/manager.go` 使用新 Repository
- [x] 7.2 更新 `internal/project/manager.go` 使用新 Repository

## 8. 验证测试

- [x] 8.1 运行 `go build ./...` 确保编译通过
- [ ] 8.2 启动服务验证数据库连接
- [ ] 8.3 测试 Device CRUD API
- [ ] 8.4 测试 Project hierarchy API
- [ ] 8.5 验证 JSON 响应格式一致
