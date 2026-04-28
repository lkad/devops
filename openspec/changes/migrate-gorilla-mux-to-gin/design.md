## Context

DevOps Toolkit 目前使用 `gorilla/mux` 作为 HTTP 路由框架，所有 API 路由（约 40+ 条）通过 `mux.NewRouter()` 注册。gorilla/mux 最后一次发布是 2021 年，已进入维护状态。

当前架构：
```
net/http + gorilla/mux
├── 中间件: auth.JWT Middleware, HTTP metrics
├── 路由注册: r.HandleFunc("/api/...", handler)
├── 参数获取: mux.Vars(r)["id"]
└── WebSocket: gorilla/websocket (独立使用)
```

## Goals / Non-Goals

**Goals:**
- 迁移到活跃维护的 HTTP 框架
- 利用 Gin 内置功能减少手写代码（JSON 绑定、参数验证、日志）
- 保持 API 行为完全兼容（无 breaking API changes）
- WebSocket 功能继续正常工作

**Non-Goals:**
- 不重构业务逻辑
- 不改变 API 路由路径（保持 `/api/k8s/clusters/{name}` 等）
- 不引入新的架构模式

## Decisions

### 1. 选择 Gin 而非 Chi/ Fiber

**选择**: Gin

**理由**:
- 生态最完善：Logger、Recovery、CORS、JWT、Validation 全部有官方或社区认可的实现
- 路由性能优秀（radix tree），比 mux 快 40-50 倍
- 中间件签名标准化：`func(c *gin.Context)` 
- 与 gorilla/websocket 完全兼容

**替代方案**:
- **Chi**: 保持 stdlib 兼容，但需额外引入多个库（validation、binding）
- **Fiber**: 性能最高，但基于 fasthttp 与 net/http 有差异，可能导致 prometheus client 等库不兼容

### 2. 路由注册模式

**决策**: 保持现有路由注册结构，仅更换框架

```go
// Before (gorilla/mux)
r := mux.NewRouter()
r.HandleFunc("/api/k8s/clusters", handler).Methods("GET")

// After (Gin)
r := gin.Default()
r.GET("/api/k8s/clusters", handler)
```

**理由**: 最小化变更范围，保持代码可读性

### 3. 中间件迁移

**决策**: 创建适配层而非重写所有中间件

```go
// gin middleware wrapper for net/http style
func ginMiddleware(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
    return func(c *gin.Context) {
        handler(c.Writer, c.Request)
    }
}
```

**理由**: auth middleware 逻辑复杂，适配层减少风险

### 4. WebSocket 集成

**决策**: 保持 `gorilla/websocket`，通过 Gin 路由引导

```go
r.GET("/ws", func(c *gin.Context) {
    upgrader := websocket.Upgrader{}
    conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil)
    wsHub.HandleWebSocket(conn)
})
```

**理由**: gorilla/websocket 是 WebSocket 事实标准，Gin 官方推荐保持使用

### 5. 响应格式保持一致

**决策**: Gin 的 `c.JSON()` 与当前 `json.NewEncoder(w).Encode()` 输出格式完全相同，无需修改前端

## Risks / Trade-offs

[Risk] 处理器签名变更 → **Mitigation**: 创建中间件适配层，逐步替换

[Risk] 中间件执行顺序差异 → **Mitigation**: 测试覆盖所有认证、日志、指标场景

[Risk] Panic 处理差异 → **Mitigation**: Gin 的 `Recovery()` 中间件会自动恢复 panic，比当前实现更好

[Trade-off] Gin 包体积比 mux 大（约 3MB vs 0.3MB） → 可接受，内部平台不敏感

## Migration Plan

**Phase 1: 依赖替换**
1. 添加 `github.com/gin-gonic/gin` 到 go.mod
2. 保留 `gorilla/mux` 暂不删除（便于对比测试）

**Phase 2: 核心路由重构**
1. 创建 `internal/ginadapter/` 包（中间件适配）
2. 修改 `cmd/devops-toolkit/main.go` 路由注册
3. 更新 HTTP handlers 签名

**Phase 3: 验证**
1. 运行单元测试
2. 启动服务，验证所有 API 路由
3. 测试 WebSocket 连接
4. 对比新旧实现响应格式

**Phase 4: 清理**
1. 移除 `gorilla/mux` 依赖
2. 提交代码

**Rollback**: 如有问题，恢复 `gorilla/mux` 代码分支

## Open Questions

- 是否需要迁移 `gorilla/websocket` 到 `nhooyr.io/websocket`？（当前保持不变）
- Gin 的日志格式是否需要定制以匹配现有日志规范？
