## 1. 依赖更新

- [ ] 1.1 添加 `github.com/gin-gonic/gin` 到 go.mod
- [ ] 1.2 运行 `go mod tidy` 确保依赖完整
- [ ] 1.3 保留 gorilla/mux 依赖暂不删除（便于对比测试）

## 2. 创建 Gin 适配层

- [ ] 2.1 创建 `internal/ginadapter/` 包
- [ ] 2.2 实现 ` ginMiddleware()` 适配器（net/http → gin.HandlerFunc）
- [ ] 2.3 创建 `GinContextToRequestMiddleware()` 将 gin.Context 转换

## 3. 更新 main.go 路由

- [ ] 3.1 修改 `cmd/devops-toolkit/main.go` 引入 Gin
- [ ] 3.2 将 `mux.NewRouter()` 替换为 `gin.Default()`
- [ ] 3.3 更新路由注册语句（`r.HandleFunc` → `r.GET/POST/PUT/DELETE`）
- [ ] 3.4 更新路径参数获取（`mux.Vars(r)["id"]` → `c.Param("id")`）
- [ ] 3.5 更新响应写入（`json.NewEncoder(w).Encode()` → `c.JSON()`）

## 4. 更新中间件

- [ ] 4.1 修改 `internal/auth/middleware.go` 支持 gin.Context
- [ ] 4.2 更新 `auth.Middleware()` 返回 gin.HandlerFunc
- [ ] 4.3 测试 JWT 认证流程

## 5. 更新 HTTP Handlers

- [ ] 5.1 更新 `internal/k8s/handler.go` 适配 Gin
- [ ] 5.2 更新 `internal/device/handler.go` 适配 Gin
- [ ] 5.3 更新 `internal/project/handler.go` 适配 Gin
- [ ] 5.4 更新其他模块 handler（如有）

## 6. WebSocket 集成

- [ ] 6.1 在 Gin 路由中注册 `/ws` 端点
- [ ] 6.2 实现 WebSocket 升级处理
- [ ] 6.3 测试 WebSocket 连接

## 7. 验证测试

- [ ] 7.1 运行单元测试 `go test ./...`
- [ ] 7.2 启动服务验证所有 API 路由
- [ ] 7.3 测试 health、metrics 等公开端点
- [ ] 7.4 对比新旧实现响应格式一致性

## 8. 清理

- [ ] 8.1 移除 `github.com/gorilla/mux` 依赖
- [ ] 8.2 运行 `go mod tidy`
- [ ] 8.3 提交代码
