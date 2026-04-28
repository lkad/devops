## Why

gorilla/mux 已进入维护模式（最后更新 2021 年），不再积极开发新功能。DevOps Toolkit 依赖该框架实现所有 HTTP 路由，但缺乏活跃维护带来以下风险：
- 安全漏洞修复延迟
- 与 Go 新版本兼容性问题
- 无法获得性能优化

Gin 是目前最活跃的 Go HTTP 框架社区，拥有完善的中间件生态（Logger、Recovery、CORS、JWT、Validation），能显著减少手写代码量并提升开发效率。

## What Changes

- 将 `github.com/gorilla/mux` 替换为 `github.com/gin-gonic/gin`
- 路由注册方式从 `mux.Router` 迁移到 `gin.Engine`
- 路径参数获取从 `mux.Vars(r)["param"]` 改为 `c.Param("param")`
- 中间件模式从 `r.Use(middleware)` 迁移到 `r.Use(middleware())`
- WebSocket 处理保持使用 gorilla/websocket（与 Gin 兼容）
- 响应写入从 `json.NewEncoder(w).Encode()` 改为 `c.JSON()`
- 请求绑定从手写解析改为 Gin 内置 `c.ShouldBindJSON()`

**BREAKING**: 需要同步更新所有调用方代码（API 路由注册、中间件链、处理器签名）

## Capabilities

### New Capabilities
- `http-router-framework`: 定义 HTTP 路由层框架选型标准和迁移规范

### Modified Capabilities
- 无 spec 级行为变更 - 此为纯实现层迁移

## Impact

**代码变更范围**:
- `cmd/devops-toolkit/main.go` - 路由注册重构
- `internal/*/handler.go` - 各模块 HTTP 处理器适配 Gin Context
- `internal/auth/middleware.go` - 中间件签名调整
- `internal/websocket/hub.go` - 保持 gorilla/websocket，需适配 Gin 集成

**依赖变更**:
- 移除: `github.com/gorilla/mux`
- 添加: `github.com/gin-gonic/gin`
- 保持: `github.com/gorilla/websocket`
