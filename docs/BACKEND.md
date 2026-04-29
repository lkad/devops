# Backend Technical Requirements — DevOps Toolkit

**状态:** Draft
**最后更新:** 2026-04-29
**基于:** PRD.md v2.1

---

## 1. 概述

### 1.1 项目背景

Go 后端 API 服务，为前端和第三方集成提供统一的 REST API。

### 1.2 技术栈

| 类别 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.21+ | 主要开发语言 |
| Web 框架 | Gin | HTTP 路由和中间件 |
| ORM | GORM | 数据库操作 |
| 数据库 | PostgreSQL 15+ | 主数据存储 |
| 认证 | LDAP + JWT | 用户认证 |
| 配置 | Viper | 配置管理 |
| 日志 | Zap | 结构化日志 |
| WebSocket | gorilla/websocket | 实时通信 |

---

## 2. 项目结构

```
cmd/
└── devops-toolkit/
    └── main.go              # 应用入口

internal/
├── auth/                   # 认证模块
│   ├── handler.go          # HTTP 处理
│   ├── middleware.go       # 中间件
│   ├── ldap/               # LDAP 客户端
│   └── jwt.go              # JWT 工具
├── device/                 # 设备管理
│   ├── handler.go          # HTTP 处理
│   ├── manager.go          # 业务逻辑
│   ├── repository.go       # 数据访问
│   ├── models.go           # 数据模型
│   └── fake/               # Mock 客户端（测试用）
├── physicalhost/           # 物理主机
│   ├── handler.go
│   ├── manager.go
│   ├── repository.go
│   └── models.go
├── pipeline/               # CI/CD 流水线
│   ├── handler.go
│   ├── manager.go
│   └── models.go
├── logs/                   # 日志系统
│   ├── handler.go
│   ├── manager.go
│   └── models.go
├── alerts/                 # 告警系统
│   ├── handler.go
│   ├── manager.go
│   └── models.go
├── k8s/                    # Kubernetes 多集群
│   ├── handler.go
│   ├── manager.go
│   └── models.go
├── project/                # 项目管理
│   ├── handler.go
│   ├── manager.go
│   ├── repository.go
│   └── models.go
├── discovery/              # 网络发现
│   ├── handler.go
│   ├── manager.go
│   └── scanner.go
├── metrics/                # 指标采集
│   └── collector.go
├── websocket/              # WebSocket Hub
│   └── hub.go
├── config/                 # 配置加载
│   └── config.go
└── ginadapter/           # Gin 适配器

pkg/
└── database/
    └── gorm.go             # 数据库连接

config/
├── config.yaml             # 配置文件
├── ldap.yaml               # LDAP 配置
└── permissions.yaml        # 权限配置
```

---

## 3. API 设计规范

### 3.1 REST 约定

| 方法 | 用途 | 约定 |
|------|------|------|
| GET | 查询资源 | 列表返回分页，详情返回单个 |
| POST | 创建资源 | 201 Created |
| PUT | 更新资源 | 200 OK |
| DELETE | 删除资源 | 204 No Content |

### 3.2 请求格式

**Headers:**
```
Content-Type: application/json
Authorization: Bearer <jwt_token>
```

**分页参数:**
```
?page=1&pageSize=20
```

**过滤参数:**
```
?field=value&field2=value2
```

### 3.3 响应格式

**成功响应:**
```json
// 单个资源
{
  "id": "abc123",
  "name": "device-01",
  "status": "active"
}

// 列表资源（带分页）
{
  "data": [...],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 100,
    "totalPages": 5
  }
}
```

**错误响应:**
```json
{
  "error": "validation_error",
  "message": "Invalid input: name is required",
  "details": {
    "field": "name",
    "reason": "required"
  }
}
```

### 3.4 HTTP 状态码

| 状态码 | 用途 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 204 | 删除成功（无返回体） |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 500 | 服务器错误 |

---

## 4. 数据模型

### 4.1 GORM 模型规范

```go
// 所有模型必须使用 gorm.Model（包含 ID, CreatedAt, UpdatedAt, DeletedAt）
type Device struct {
    gorm.Model
    Name     string `json:"name"`
    Status   string `json:"status"`
    // ...
}
```

### 4.2 软删除

- 默认使用软删除（gorm.Model.DeletedAt）
- 硬删除需要显式使用 `db.Unscoped().Delete()`
- 查询时自动过滤已删除记录

### 4.3 迁移策略

```go
func AutoMigrate() error {
    // 先添加缺失的 deleted_at 列（兼容旧表）
    addMissingDeletedAtColumns()
    // 再执行 AutoMigrate
    return db.AutoMigrate(
        &Device{},
        &Project{},
        // ...
    )
}
```

---

## 5. 中间件

### 5.1 认证中间件

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        // 验证 JWT
        user, err := validateToken(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid_token"})
            c.Abort()
            return
        }
        c.Set("user", user)
        c.Next()
    }
}
```

### 5.2 日志中间件

```go
func LoggerMiddleware() gin.HandlerFunc {
    return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
        return fmt.Sprintf("%s %s %d %s",
            param.Method,
            param.Path,
            param.StatusCode,
            param.Latency)
    })
}
```

### 5.3 恢复中间件

```go
func RecoveryMiddleware() gin.HandlerFunc {
    return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
        log.Error("panic recovered:", recovered)
        c.JSON(500, gin.H{"error": "internal_error"})
    })
}
```

### 5.4 CORS 中间件

```go
func CORSMiddleware(cfg *CORSConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        // 检查 origin 是否在允许列表中
        if isOriginAllowed(origin, cfg.AllowedOrigins) {
            c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
            c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        }
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}

func isOriginAllowed(origin string, allowed []string) bool {
    for _, o := range allowed {
        if o == "*" || o == origin {
            return true
        }
    }
    return false
}
```

**配置:**
```yaml
cors:
  allowed_origins:
    - "http://localhost:3000"
    - "https://devops.internal.com"
  # 生产环境不使用 *，严格限制来源
```

---

## 6. 配置管理

### 6.1 配置结构

```yaml
server:
  host: "0.0.0.0"
  port: 3000
  base_path: ""  # 如 /proxy/3000

database:
  host: "localhost"
  port: 5432
  user: "devops"
  password: "password"
  name: "devops_toolkit"
  max_connections: 25
  ssl_mode: "disable"
  conn_max_lifetime: 300
  # 也支持 DATABASE_URL 环境变量（优先于上述配置）
  # DATABASE_URL="postgres://devops:password@localhost:5432/devops_toolkit?sslmode=disable"

auth:
  jwt_secret: "your-secret-key"
  jwt_expiry: 86400  # 24小时
  dev_bypass: false  # 开发模式跳过认证

ldap:
  url: "ldap://localhost:389"
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_password: "admin"
  user_filter: "(uid=%s)"
  group_filter: "(memberUid=%s)"

logs:
  backend: "local"  # local, elasticsearch, loki
  retention_days: 30

environment: "development"  # mock, containerlab, production

cors:
  allowed_origins:
    - "http://localhost:3000"
    - "https://devops.internal.com"

tracing:
  enabled: false
  endpoint: "http://jaeger:14268/api/traces"

rate_limit:
  auth_login: 10/minute
  api_general: 100/minute
  metrics: 60/minute
```

### 6.2 配置加载

```go
func Load(configPath string) (*Config, error) {
    viper.SetConfigFile(configPath)
    viper.AutomaticEnv()
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

---

## 7. 错误处理

### 7.1 错误类型

```go
type AppError struct {
    Code    string `json:"error"`
    Message string `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func NewValidationError(field, reason string) *AppError {
    return &AppError{
        Code:    "validation_error",
        Message: fmt.Sprintf("Invalid input: %s", field),
        Details: map[string]string{"field": field, "reason": reason},
    }
}

func NewNotFoundError(resource string) *AppError {
    return &AppError{
        Code:    "not_found",
        Message: fmt.Sprintf("%s not found", resource),
    }
}
```

### 7.2 错误响应

```go
func handleError(c *gin.Context, err error) {
    if appErr, ok := err.(*AppError); ok {
        c.JSON(getHTTPStatus(appErr.Code), appErr)
        return
    }
    log.Error("unexpected error:", err)
    c.JSON(500, &AppError{Code: "internal_error", Message: "An unexpected error occurred"})
}

func getHTTPStatus(code string) int {
    switch code {
    case "validation_error": return 400
    case "unauthorized": return 401
    case "forbidden": return 403
    case "not_found": return 404
    case "conflict": return 409
    default: return 500
    }
}
```

---

## 8. 日志规范

### 8.1 日志级别

| 级别 | 用途 |
|------|------|
| Debug | 开发调试 |
| Info | 常规操作 |
| Warn | 警告（但不影响功能） |
| Error | 错误（需要调查） |

### 8.2 结构化日志

```go
log.Info("request processed",
    "method", c.Request.Method,
    "path", c.Request.URL.Path,
    "status", 200,
    "latency", latency,
    "user", userID,
)
```

### 8.3 日志输出

- 开发环境: 标准输出 + 颜色
- 生产环境: JSON 格式到文件
- 错误日志: 单独的错误日志文件

---

## 9. 数据库

### 9.1 连接池配置

```go
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(12)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

### 9.2 查询规范

- 使用 GORM 的链式 API
- 避免手写 SQL（使用 gorm.DB.Raw 除外）
- 批量操作使用 BatchInsert
- 分页使用 Limit 和 Offset

### 9.3 事务

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&project).Error; err != nil {
        return err
    }
    if err := tx.Create(&resource).Error; err != nil {
        return err
    }
    return nil
})
```

---

## 10. 测试

### 10.1 单元测试

```go
func TestDeviceManager_Create(t *testing.T) {
    db := setupTestDB()
    mgr := NewManager(db)
    device, err := mgr.CreateDevice(ctx, &CreateDeviceRequest{Name: "test"})
    assert.NoError(t, err)
    assert.Equal(t, "test", device.Name)
}
```

### 10.2 Mock 客户端

```go
// 使用 fake client 进行测试
func TestDiscoverVMs(t *testing.T) {
    vmClient := fake.NewFakeVMwareClient()
    mgr := NewManager(db, vmClient, nil, nil)
    vms, err := mgr.DiscoverVMsFromHost(ctx, "host-1")
    assert.NoError(t, err)
    assert.Len(t, vms, 3)
}
```

### 10.3 测试覆盖率目标

| 模块 | 目标覆盖率 |
|------|---------|
| device | 80% |
| project | 80% |
| pipeline | 75% |
| logs | 80% |
| k8s | 75% |

---

## 11. 部署

### 11.1 构建

```bash
# 编译
go build -o devops-toolkit ./cmd/devops-toolkit

# Docker 构建
docker build -t devops-toolkit:latest .
```

### 11.2 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| CONFIG_PATH | 配置文件路径 | config.yaml |
| LOG_LEVEL | 日志级别 | info |
| ENV | 环境模式 | development |

### 11.3 健康检查

```
GET /health
Response: {"status": "healthy"}
```

### 11.4 Graceful Shutdown

服务器在收到 SIGINT/SIGTERM 信号时执行优雅关闭：

```go
// 创建带超时 context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// 停止接收新请求
srv.Shutdown(ctx)

// 关闭数据库连接
db.Close()
```

**关闭流程:**
1. 停止接收新请求
2. 等待现有请求完成（最多 5 秒）
3. 关闭 WebSocket 连接
4. 关闭数据库连接池
5. 退出

### 11.5 Rate Limiting

限流中间件保护 API 免受滥用：

```go
func RateLimitMiddleware(requests int, window time.Duration) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(float64(requests)/window.Seconds()), requests)
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "too_many_requests"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**限流规则:**

| 端点 | 限制 |
|------|------|
| `/api/auth/login` | 10 次/分钟 |
| `/api/*` (其他) | 100 次/分钟 |
| `/metrics` | 60 次/分钟 |

### 11.6 链路追踪

OpenTelemetry 集成用于分布式追踪：

```go
// 初始化 Tracer
func initTracer(endpoint string) (*otlp.Exporter, error) {
    exporter, err := otlp.NewExporter(
        otlp.WithEndpoint(endpoint),
        otlp.WithInsecure(),
    )
    return exporter, err
}
```

**追踪内容:**
- HTTP 请求/响应
- 数据库查询
- 外部 API 调用
- WebSocket 消息

---

## 12. API 端点汇总

### 12.1 认证

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/auth/login` | POST | LDAP 登录 |
| `/api/auth/logout` | POST | 登出 |
| `/api/auth/me` | GET | 当前用户 |

### 12.2 设备

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/devices` | GET | 设备列表 |
| `/api/devices` | POST | 创建设备 |
| `/api/devices/:id` | GET | 设备详情 |
| `/api/devices/:id` | PUT | 更新设备 |
| `/api/devices/:id` | DELETE | 删除设备 |
| `/api/devices/search` | GET | 搜索设备 |

### 12.3 物理主机

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/physical-hosts` | GET | 主机列表 |
| `/api/physical-hosts` | POST | 注册主机 |
| `/api/physical-hosts/:id` | GET | 主机详情 |
| `/api/physical-hosts/:id` | DELETE | 删除主机 |
| `/api/physical-hosts/:id/services` | GET | 服务列表 |
| `/api/physical-hosts/:id/config` | POST | 推送配置 |

### 12.4 流水线

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/pipelines` | GET | 流水线列表 |
| `/api/pipelines` | POST | 创建流水线 |
| `/api/pipelines/:id` | GET | 流水线详情 |
| `/api/pipelines/:id` | DELETE | 删除流水线 |
| `/api/pipelines/:id/execute` | POST | 执行流水线 |

### 12.5 日志

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/logs` | GET | 查询日志 |
| `/api/logs` | POST | 创建日志 |
| `/api/logs/stats` | GET | 日志统计 |
| `/api/logs/alerts` | GET | 告警规则 |
| `/api/logs/alerts` | POST | 创建规则 |
| `/api/logs/filters` | GET | 保存的过滤器 |
| `/api/logs/filters` | POST | 创建过滤器 |

### 12.6 告警

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/alerts/channels` | GET | 通道列表 |
| `/api/alerts/channels` | POST | 创建通道 |
| `/api/alerts/history` | GET | 历史记录 |

### 12.7 K8s

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/k8s/clusters` | GET | 集群列表 |
| `/api/k8s/clusters` | POST | 创建集群 |
| `/api/k8s/clusters/:name` | DELETE | 删除集群 |
| `/api/k8s/clusters/:name/health` | GET | 健康检查 |
| `/api/k8s/clusters/:name/nodes` | GET | 节点列表 |
| `/api/k8s/clusters/:name/namespaces` | GET | 命名空间列表 |
| `/api/k8s/clusters/:name/pods` | GET | Pod 列表 |
| `/api/k8s/clusters/:name/pods/:pod/logs` | GET | Pod 日志 |
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs` | GET | 带命名空间的 Pod 日志 |
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/exec` | POST | Pod exec |
| `/api/k8s/clusters/:name/metrics` | GET | 集群指标 |
| `/api/k8s/maintenance` | POST | 维护操作 |

### 12.8 项目

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/org/business-lines` | GET/POST | 业务线 |
| `/api/org/business-lines/:id` | GET/PUT/DELETE | 业务线详情 |
| `/api/org/business-lines/:id/systems` | GET/POST | 系统 |
| `/api/org/systems/:id` | GET/PUT/DELETE | 系统详情 |
| `/api/org/systems/:id/projects` | GET/POST | 项目 |
| `/api/org/projects/:id` | GET/PUT/DELETE | 项目详情 |
| `/api/org/projects/:id/resources` | GET/POST/DELETE | 资源链接 |
| `/api/org/projects/:id/permissions` | GET/POST | 权限 |
| `/api/org/permissions/:perm_id` | DELETE | 撤销权限 |
| `/api/org/reports/finops` | GET | FinOps 报表 |
| `/api/org/audit-logs` | GET | 审计日志 |

### 12.9 发现

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/discovery/scan` | POST | 触发扫描 |
| `/api/discovery/status` | GET | 扫描状态 |
| `/api/discovery/register` | POST | 注册设备 |

### 12.10 其他

| 端点 | 方法 | 说明 |
|------|------|------|
| `/metrics` | GET | Prometheus 指标 |
| `/ws` | GET | WebSocket |
| `/health` | GET | 健康检查 |

---

## 13. 性能要求

| 指标 | 目标 |
|------|------|
| API 响应时间 | < 100ms (P95) |
| 并发连接数 | 100+ |
| 数据库连接 | 25 max |
| WebSocket 并发 | 500+ |

---

## 14. 安全要求

- JWT Token 24小时过期
- LDAP 认证失败重试3次
- 敏感操作记录审计日志
- 配置密码加密存储（不暴露明文密码）
- CORS 严格限制来源（不使用 *）
- Rate Limiting 防止滥用
- Graceful Shutdown 防止请求中断
- 请求超时限制（30秒）

---

**文档状态:** Draft
**需要评审后实施**