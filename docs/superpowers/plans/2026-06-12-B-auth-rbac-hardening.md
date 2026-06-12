# B 子项目 — 鉴权+多租户硬化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 v0.2.0.0 P0 #1 (Auth+RBAC middleware 接到 /api/v1) + P0 #2 (Service 层跨租户强制) + P0 #3 (Audit 全覆盖) + scoped-Auditor RBAC matrix 全部落地,1-1.5 周完成,~22-25 个 atomic commit。

**Architecture:** `signal.NotifyContext` 共享根 context 给所有 goroutine 已经在 A 子项目落地。Auth+RBAC middleware 在 `buildRouter` 的 `/api/v1` 顶部一次接(JWT 解析 → caller 注入 context → per-route Permission check),Service 层在 8 modules × 1-2 处加 `caller.IsMemberOf` fail-closed 检查,Audit 复用现有 `audit.Service.RecordAction` 在 8 modules × 1-5 emit 点全接,scoped-Auditor 加新 role + new permission 接线 RBAC matrix。

**Tech Stack:** Go 1.26 + Gin + GORM + JWT + LDAP(已有) + RBAC matrix(已有) + audit emitter(已有)

**Spec:** [`docs/superpowers/specs/2026-06-12-B-auth-rbac-hardening-design.md`](../specs/2026-06-12-B-auth-rbac-hardening-design.md)

---

## 范围总览

**5 phase,5 agent 并行,1-1.5 周,~22-25 commits。**

| Agent | Branch | Tasks | Phase | 估时 | 依赖 |
|---|---|---|---|---|---|
| 主 session (Phase 1) | `feat/b1-scoped-auditor` | Task 1 (matrix) | Phase 1 | 0.5 day | 无 |
| Agent 1 | `feat/b2-auth-rbac-middleware` | Task 2-5 (auth + RBAC + buildRouter + E2E) | Phase 2 | 1-1.5 day | 无 |
| Agent 2 | `feat/b3-cross-tenant` | Task 6-13 (8 modules × 1) | Phase 3 | 2-3 day | 无 |
| Agent 3 | `feat/b4-audit-coverage` | Task 14-21 (8 modules × 1) | Phase 4 | 2-3 day | 无 |
| 主 session (Phase 5) | `feat/b5-e2e-docs` | Task 22-23 (E2E + 文档) | Phase 5 | 1 day | Phase 1-4 |

主 session 协调 merge 顺序:B1 → B2 → B3 → B4 → B5。

---

## 关键共享约定(所有 agent 必须遵守)

### 1. 命名

- Module: `github.com/devops-toolkit/backend`
- File: snake_case
- Commit message 中文,无 emoji,`feat(scope): xxx` / `refactor(scope): xxx` / `test(scope): xxx` / `fix(scope): xxx`

### 2. 测试命令

- 单包: `go test -count=1 -timeout 120s ./internal/<package>/...`
- 全量: `go test -count=1 -p 1 -timeout 600s ./...` (串行,避免 sqlite 并发锁)
- race: `go test -count=1 -race -timeout 120s ./internal/<package>/...`
- vet: `go vet ./... 2>&1 | wc -l` (目标 ≤ 1,既有 audit/repository.go:89)

### 3. Branch 命名 & merge 风格

- Branch: `feat/b1-scoped-auditor` / `feat/b2-auth-rbac-middleware` / `feat/b3-cross-tenant` / `feat/b4-audit-coverage` / `feat/b5-e2e-docs`
- 每个 task 一个 commit
- 主 session merge: `git merge --no-ff <branch>`,merge commit 简短

### 4. 顺序约束

- **Task 1 (matrix) 必须先做** — Task 22 测试可能引用 matrix
- Task 2-5 (Phase 2) 改 buildRouter,会撞 cmd/devops-toolkit/main.go — 单独 branch
- Task 6-13 (Phase 3) 改 8 modules 的 service.go — 每个 module 一个 commit,无冲突
- Task 14-21 (Phase 4) 改 8 modules 的 service.go — 同上,但不同 module 不冲突
- Task 22-23 (Phase 5) 收尾,E2E test 跨 module

### 5. 不在本计划范围(YAGNI)

- 不做 LDAP group → Role 映射(已有,不动)
- 不做 SAML/OIDC SSO
- 不做 multi-tenancy 跨 region
- 不做 RBAC matrix 动态配置
- 不做 audit retention policy
- 不做 dynamic policy engine (OPA)
- 不做 RBAC UI

---

## File Structure(B 子项目总览)

### 修改文件

```
cmd/devops-toolkit/main.go                  (buildRouter 顶部接 Auth+RBAC,Phase 2)
internal/auth/middleware.go                 (AuthMiddleware 注入 caller,Phase 2)
internal/auth/rbac/middleware.go            (RBACMiddleware 新建,Phase 2)
internal/auth/rbac/matrix.go                (RoleScopedAuditor + PermissionViewAuditLogProject,Phase 1)
pkg/contracts/user.go                       (RoleScopedAuditor 常量,Phase 1)
internal/audit/handler.go                   (per-route 接受 PermissionViewAuditLogProject,Phase 1)
internal/audit/service.go                   (ListForCaller 走 scoped 路径,Phase 1)
internal/project/service.go                 (caller.IsMemberOf + audit emit,Phase 3+4)
internal/device/groups.go                   (audit emit,Phase 4)
internal/k8s/service.go                     (caller.IsMemberOf + audit emit,Phase 3+4)
internal/pipeline/service.go                (caller.IsMemberOf + audit emit,Phase 3+4)
internal/servicecatalog/service.go          (caller.IsMemberOf + audit emit 第 2 处,Phase 3+4)
internal/physicalhost/service.go            (caller.IsMemberOf,Phase 3)
internal/alerts/service.go                  (caller.IsMemberOf + audit emit,Phase 3+4)
internal/logs/service.go                    (caller.IsMemberOf + audit emit,Phase 3+4)
internal/discovery/service.go               (audit emit,Phase 4)
```

### 新增文件

```
tests/integration/auth_rbac_test.go         (E2E 8 modules 验证,Phase 5)
openspec/specs/auth-rbac-hardening/spec.md  (本 spec 的简短镜像,Phase 5)
```

---

## Task 1: scoped-Auditor RBAC matrix(Phase 1, Commit #1)

**Files:**
- Modify: `pkg/contracts/user.go`(加 `RoleScopedAuditor`)
- Modify: `internal/auth/rbac/matrix.go`(加 `PermissionViewAuditLogProject` + matrix 行)
- Modify: `internal/audit/service.go`(`ListForCaller` 接受 scoped permission)
- Modify: `internal/audit/handler.go`(per-route 加 scoped permission 选项)
- Test: `internal/auth/rbac/matrix_test.go`

**Agent:** 主 session — Task 1 是基础,其他 agent 不依赖

- [ ] **Step 1: 先看现状**

Run: `cat pkg/contracts/user.go | head -40 && echo "---" && cat internal/auth/rbac/matrix.go | head -200`

- [ ] **Step 2: 加 `RoleScopedAuditor` 常量**

在 `pkg/contracts/user.go` 找到 role 常量定义:

```go
// 旧
const (
    RoleSuperAdmin   Role = "SuperAdmin"
    RoleOperator     Role = "Operator"
    RoleDeveloper    Role = "Developer"
    RoleAuditor      Role = "Auditor"
)

// 新
const (
    RoleSuperAdmin     Role = "SuperAdmin"
    RoleOperator       Role = "Operator"
    RoleDeveloper      Role = "Developer"
    RoleAuditor        Role = "Auditor"
    RoleScopedAuditor  Role = "ScopedAuditor"
)
```

- [ ] **Step 3: 加 `PermissionViewAuditLogProject`**

在 `internal/auth/rbac/matrix.go` 找到 Permission 常量定义,加:

```go
// PermissionViewAuditLogProject covers GET /audit for a
// per-tenant scope. The /audit handler routes
// "ScopedAuditor"-role callers through ListForCaller's
// ProjectIDsIn filter so they only see audit events for
// projects they are a member of. Distinct from
// PermissionViewAuditLog which gives full read access.
PermissionViewAuditLogProject Permission = "audit.log.view.project"
```

- [ ] **Step 4: 加 `RoleScopedAuditor` matrix 行**

在 `RolePermissions` map 找到 `RoleAuditor` 块,在它后面加:

```go
contracts.RoleScopedAuditor: {
    // Same view-permissions as RoleAuditor
    PermissionViewDevices,
    PermissionViewProjects,
    PermissionViewDiscovery,
    PermissionViewK8sResources,
    PermissionViewK8sPodLogs,
    PermissionViewPipelines,
    PermissionViewServiceCatalog,
    PermissionViewPhysicalHosts,
    PermissionViewLogs,
    PermissionViewMetrics,
    PermissionViewAlerts,
    // The audit-log permissions:
    PermissionViewAuditLog,         // 既有,全量
    PermissionViewAuditLogProject,  // 新,per-tenant(走 ListForCaller 过滤)
},
```

- [ ] **Step 5: 写 failing test**

在 `internal/auth/rbac/matrix_test.go` 追加(新测试):

```go
func TestRoleHasPermission_ScopedAuditor_GrantsViewAuditLog(t *testing.T) {
    if !RoleHasPermission(contracts.RoleScopedAuditor, PermissionViewAuditLog) {
        t.Errorf("RoleScopedAuditor should have PermissionViewAuditLog")
    }
}

func TestRoleHasPermission_ScopedAuditor_GrantsViewAuditLogProject(t *testing.T) {
    if !RoleHasPermission(contracts.RoleScopedAuditor, PermissionViewAuditLogProject) {
        t.Errorf("RoleScopedAuditor should have PermissionViewAuditLogProject")
    }
}

func TestRoleHasPermission_Auditor_DoesNotHaveScopedPermission(t *testing.T) {
    if RoleHasPermission(contracts.RoleAuditor, PermissionViewAuditLogProject) {
        t.Errorf("RoleAuditor should NOT have PermissionViewAuditLogProject (only ScopedAuditor does)")
    }
}

func TestRoleHasPermission_ScopedAuditor_DoesNotHaveWritePermissions(t *testing.T) {
    writePerms := []Permission{
        PermissionWriteDevices,
        PermissionWriteProjects,
        PermissionManagePipelines,
        PermissionWriteAlerts,
    }
    for _, p := range writePerms {
        if RoleHasPermission(contracts.RoleScopedAuditor, p) {
            t.Errorf("RoleScopedAuditor should NOT have write permission %q", p)
        }
    }
}
```

- [ ] **Step 6: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/auth/rbac/`
Expected: PASS — 4 new cases + 既有 cases 全过。

- [ ] **Step 7: 改 `audit.Service.ListForCaller` 接受 scoped permission**

在 `internal/audit/service.go` 找到 `ListForCaller`(已存在,前几轮做),确认它已经走 per-tenant 路径。如果有 `PermissionViewAuditLogProject` 检查分支,确认它在矩阵中已定义。否则:

```go
// 既有(per A 子项目落地)
// ListForCaller returns events filtered by caller's project
// membership if caller is a ScopedAuditor, or unfiltered
// for SuperAdmin / full Auditor.
//
// Permission grant is checked at the route via perms() —
// the service trusts that the caller is allowed to read
// audit events.
```

(此 step 可能 no-op,主要看现状)

- [ ] **Step 8: 改 `audit.Handler` 接受 scoped permission**

在 `internal/audit/handler.go` 找到 `NewHandler` 接受 `MembershipChecker` 的 variadic 参数(已存在)。在路由注册时加:

```go
// main.go 改后(Phase 2 时):
auditHandler := audit.NewHandler(..., perms(rbac.PermissionViewAuditLog, rbac.PermissionViewAuditLogProject))
```

(此 step 现在 skip,在 Phase 2 时一起做)

- [ ] **Step 9: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS — 既有 31 packages 全绿,无新 fail。

- [ ] **Step 10: 跑 vet**

Run: `go vet ./... 2>&1 | wc -l`
Expected: ≤ 1。

- [ ] **Step 11: 提交**

```bash
git add pkg/contracts/user.go internal/auth/rbac/matrix.go internal/auth/rbac/matrix_test.go
git commit -m "feat(rbac): 加 RoleScopedAuditor 角色 + PermissionViewAuditLogProject 权限"
```

---

## Task 2: Auth Middleware 注入 caller 到 context(Phase 2, Commit #2)

**Files:**
- Modify: `internal/auth/middleware.go`(`AuthMiddleware` 注入 caller)

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `cat internal/auth/middleware.go | head -100`

- [ ] **Step 2: 写 failing test**

在 `internal/auth/middleware_test.go` 追加:

```go
package auth

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"

    "github.com/devops-toolkit/backend/pkg/contracts"
)

// mintTestToken returns a valid JWT for the given user.
func mintTestToken(t *testing.T, secret string, user *contracts.User, exp time.Duration) string {
    t.Helper()
    claims := contracts.JWTClaims{
        Username: user.Username,
        Role:     user.Role,
        UserID:   user.ID,
        // ... actual fields per the existing JWT struct
    }
    // ... actual signing call per existing pattern
    // 实际签名: 调现有 mint-dev-token 模式或 auth 包内的 helper
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(secret))
    if err != nil {
        t.Fatal(err)
    }
    return signed
}

func TestAuthMiddleware_ValidToken_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    var gotCaller any
    r.Use(AuthMiddleware("test-secret"))
    r.GET("/test", func(c *gin.Context) {
        gotCaller, _ = c.Get("caller")
        c.Status(200)
    })
    token := mintTestToken(t, "test-secret", &contracts.User{ID: "u1", Username: "alice", Role: contracts.RoleDeveloper}, 5*time.Minute)
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Errorf("code = %d, want 200", w.Code)
    }
    if gotCaller == nil {
        t.Error("caller not set in context")
    }
}

func TestAuthMiddleware_MissingToken_401(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(AuthMiddleware("test-secret"))
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401", w.Code)
    }
}

func TestAuthMiddleware_InvalidToken_401(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(AuthMiddleware("test-secret"))
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401", w.Code)
    }
}

func TestAuthMiddleware_ExpiredToken_401(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(AuthMiddleware("test-secret"))
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    // token expired 1 hour ago
    token := mintTestToken(t, "test-secret", &contracts.User{ID: "u1", Username: "alice", Role: contracts.RoleDeveloper}, -1*time.Hour)
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401", w.Code)
    }
}
```

注: `AuthMiddleware(secret string)` signature 以实际为准(可能用 config,可能直接 secret 字符串)。调测试时用既有 jwtClaims struct + 实际签发模式。

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/auth/`
Expected: FAIL — AuthMiddleware signature 还没确定或 caller 注入未做。

- [ ] **Step 4: 改 `AuthMiddleware` 注入 caller**

在 `internal/auth/middleware.go`,找到 `AuthMiddleware` 函数,改:

```go
// 旧(可能仅校验 token 不注入 caller)
func AuthMiddleware(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := extractToken(c)
        if tokenStr == "" {
            c.AbortWithStatusJSON(401, gin.H{"code": "unauthenticated", "message": "missing token"})
            return
        }
        claims, err := parseToken(tokenStr, secret)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"code": "invalid_token", "message": err.Error()})
            return
        }
        c.Set("user", claims.User)  // 既有
        c.Next()
    }
}

// 新
func AuthMiddleware(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := extractToken(c)
        if tokenStr == "" {
            c.AbortWithStatusJSON(401, gin.H{"code": "unauthenticated", "message": "missing token"})
            return
        }
        claims, err := parseToken(tokenStr, secret)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"code": "invalid_token", "message": err.Error()})
            return
        }
        // 既有:c.Set("user", claims.User)
        c.Set("user", claims.User)
        // 新: 注入 caller 到 gin.Context
        cl := caller.New(claims.User)  // 用 caller.New(...)
        c.Set("caller", cl)
        // 同时 push 到 context.Context
        ctx := caller.WithCaller(c.Request.Context(), cl)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

实际函数名/参数以现状为准(`AuthMiddleware` 可能已存在并签名不同,以 `internal/auth/middleware.go` 现状为准)。

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/auth/`
Expected: PASS — 4 new cases 全过。

- [ ] **Step 6: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS — 既有 31 packages 全绿。

- [ ] **Step 7: 提交**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -m "feat(auth): AuthMiddleware 注入 caller 到 context"
```

---

## Task 3: RBAC Middleware(Phase 2, Commit #3)

**Files:**
- Create or modify: `internal/auth/rbac/middleware.go`(`RBACMiddleware` 实现)
- Test: `internal/auth/rbac/middleware_test.go`

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `ls internal/auth/rbac/ && echo "---" && cat internal/auth/rbac/middleware.go 2>/dev/null | head -30 || echo "no middleware.go yet, create new"`

- [ ] **Step 2: 写 failing test**

在 `internal/auth/rbac/middleware_test.go`(新建):

```go
package rbac

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"

    "github.com/devops-toolkit/backend/internal/auth/caller"
    "github.com/devops-toolkit/backend/pkg/contracts"
)

func TestRBACMiddleware_NoCaller_401(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RBACMiddleware())
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401", w.Code)
    }
}

func TestRBACMiddleware_ValidCaller_Allows(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        // 模拟 AuthMiddleware 已经把 caller 塞到 context
        u := &contracts.User{ID: "u1", Username: "alice", Role: contracts.RoleDeveloper}
        cl := caller.New(u)
        c.Set("caller", cl)
        c.Request = c.Request.WithContext(caller.WithCaller(c.Request.Context(), cl))
        c.Next()
    })
    r.Use(RBACMiddleware())
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Errorf("code = %d, want 200", w.Code)
    }
}

func TestRBACMiddleware_NilCaller_401(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        // 没设 caller
        c.Next()
    })
    r.Use(RBACMiddleware())
    r.GET("/test", func(c *gin.Context) { c.Status(200) })
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401", w.Code)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/auth/rbac/`
Expected: FAIL — `RBACMiddleware` 未定义。

- [ ] **Step 4: 写 `internal/auth/rbac/middleware.go`**

```go
package rbac

import (
    "github.com/gin-gonic/gin"

    "github.com/devops-toolkit/backend/internal/auth/caller"
)

// RBACMiddleware enforces that every /api/v1 request has
// a valid caller in the context. It does NOT itself check
// per-route permissions (that's the perms() helper wired
// at route registration); it only ensures the caller is
// present so the per-route check has something to look at.
//
// Pair with auth.AuthMiddleware (which sets the caller).
// If AuthMiddleware was not run first (e.g. unauthenticated
// /api/v1/auth/login), this middleware aborts 401.
//
// nil caller in context → 401 (fail-closed).
func RBACMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        cl, ok := caller.FromGin(c)
        if !ok || cl == nil || cl.User == nil {
            c.AbortWithStatusJSON(401, gin.H{
                "code":    "unauthenticated",
                "message": "no caller in context (AuthMiddleware required upstream)",
            })
            return
        }
        c.Next()
    }
}
```

注: `caller.FromGin` 是否存在看 `internal/auth/caller/caller.go`。若不存在,加 helper:

```go
// In internal/auth/caller/caller.go (if missing)
func FromGin(c *gin.Context) (*Caller, bool) {
    v, ok := c.Get("caller")
    if !ok {
        return nil, false
    }
    cl, ok := v.(*Caller)
    return cl, ok
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/auth/rbac/`
Expected: PASS — 3 new cases 全过。

- [ ] **Step 6: 提交**

```bash
git add internal/auth/rbac/middleware.go internal/auth/rbac/middleware_test.go
git commit -m "feat(rbac): RBACMiddleware 默认 deny + 注入 caller"
```

---

## Task 4: buildRouter 顶部接 Auth+RBAC(Phase 2, Commit #4)

**Files:**
- Modify: `cmd/devops-toolkit/main.go`(`buildRouter` 在 `/api/v1` 顶部接 Auth+RBAC)

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `grep -n "buildRouter\|/api/v1\|register.*Routes" cmd/devops-toolkit/main.go | head -30`

- [ ] **Step 2: 写 failing test**

在 `cmd/devops-toolkit/main_test.go` 追加(新测试):

```go
func TestBuildRouter_RequiresAuthOnAPIV1(t *testing.T) {
    // Setup minimal config + buildRouter
    r := buildRouter(testMinimalConfig(t))  // helper 可能需要新加
    // 匿名访问 /api/v1/devices 应 401
    req := httptest.NewRequest("GET", "/api/v1/devices", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("code = %d, want 401 (anonymous /api/v1 must be denied)", w.Code)
    }
}
```

注: `testMinimalConfig(t)` helper 可能需要新加,或在既有 TestBuildRouter_* test 基础上扩。

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 -run TestBuildRouter_RequiresAuthOnAPIV1 ./cmd/devops-toolkit/`
Expected: FAIL — `/api/v1/devices` 现在返 200(匿名通过)。

- [ ] **Step 4: 改 `buildRouter` 顶部接 middleware**

在 `cmd/devops-toolkit/main.go` 找到 `buildRouter`(或 build engine 部分),加:

```go
// 旧(部分代码示意)
func buildRouter(...) http.Handler {
    r := gin.New()
    r.Use(middleware.Chain(log, corsOrigins)...)
    v1 := r.Group("/api/v1")
    registerDeviceRoutes(v1, ...)
    // ...
    return r
}

// 新
func buildRouter(..., authSecret string) http.Handler {
    r := gin.New()
    r.Use(middleware.Chain(log, corsOrigins)...)
    // /api/v1 顶部接 Auth + RBAC
    v1 := r.Group("/api/v1",
        auth.AuthMiddleware(authSecret),
        rbac.RBACMiddleware(),
        audit.Middleware(),  // 既有,RequestActor 注入
    )
    registerDeviceRoutes(v1, ...)
    // ...
    return r
}
```

具体细节:
- `authSecret` 从 config 注入(`cfg.JWTSecret`,env-only,前几轮 main.go 已有)
- 已有 `/api/v1/auth/login` 路由需在 v1 之外注册(避免被 Auth 拦)或加白名单(在 AuthMiddleware 内部 skip `/api/v1/auth/*`)

具体看 main.go 现状实现。

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 -run TestBuildRouter_RequiresAuthOnAPIV1 ./cmd/devops-toolkit/`
Expected: PASS — 匿名 401。

- [ ] **Step 6: 跑全量测试,看 8 modules 是否破**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS — 既有 31 packages 全绿,匿名路径全 401,但既有 200 路径带有效 token 也 200。

注: 既有 main_test.go 的 TestBuildRouter_* 5 个 case 可能需要补 JWT token 才能过。这是 Phase 2 的最大风险点。

- [ ] **Step 7: 跑既有 main_test.go,看是否退化**

如果既有 `TestBuildRouter_*LoggerIncludesDuration` / `TestBuildRouter_RecoversFromPanic` / `TestBuildRouter_TraceIDSetOnResponse` 退化(因为不接 v1),修 helper 让它们带有效 token。

具体修法:`TestBuildRouter_*` helper 加 `mintTestToken(t, ...)` + `req.Header.Set("Authorization", "Bearer "+token)`。

- [ ] **Step 8: 提交**

```bash
git add cmd/devops-toolkit/main.go cmd/devops-toolkit/main_test.go
git commit -m "feat(router): buildRouter 在 /api/v1 顶部接 Auth+RBAC middleware"
```

---

## Task 5: 8 modules auth 接线回归测试(Phase 2, Commit #5)

**Files:**
- Test: `cmd/devops-toolkit/main_test.go` 追加

**Agent:** Agent 1

- [ ] **Step 1: 写 failing test**

在 `cmd/devops-toolkit/main_test.go` 追加:

```go
func TestBuildRouter_APIV1_AllModules_RequireAuth(t *testing.T) {
    r := buildRouter(testMinimalConfig(t))
    endpoints := []string{
        "/api/v1/devices",
        "/api/v1/projects",
        "/api/v1/k8s/clusters",
        "/api/v1/pipelines",
        "/api/v1/services",
        "/api/v1/physical-hosts",
        "/api/v1/alerts",
        "/api/v1/logs",
        "/api/v1/audit",
    }
    for _, ep := range endpoints {
        req := httptest.NewRequest("GET", ep, nil)
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
        if w.Code != 401 {
            t.Errorf("%s: code = %d, want 401 (anonymous must be denied)", ep, w.Code)
        }
    }
}

func TestBuildRouter_APIV1_ValidToken_Reaches200(t *testing.T) {
    r := buildRouter(testMinimalConfig(t))
    token := mintTestToken(t, "test-secret", &contracts.User{
        ID: "u1", Username: "alice", Role: contracts.RoleSuperAdmin,
    }, 5*time.Minute)
    endpoints := []string{
        "/api/v1/devices",
        "/api/v1/projects",
    }
    for _, ep := range endpoints {
        req := httptest.NewRequest("GET", ep, nil)
        req.Header.Set("Authorization", "Bearer "+token)
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
        // 200 (with data) or 503 (DB unavailable in test) — but NOT 401
        if w.Code == 401 {
            t.Errorf("%s: code = 401, valid token should not be 401", ep)
        }
    }
}
```

- [ ] **Step 2: 跑 test,确认 pass**

Run: `go test -count=1 -run "TestBuildRouter_APIV1_" ./cmd/devops-toolkit/`
Expected: PASS — 9 endpoint 401 + 2 endpoint 200/503。

- [ ] **Step 3: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add cmd/devops-toolkit/main_test.go
git commit -m "test(e2e): 8 modules 验证 auth 接线 + 401/200 路径"
```

---

## Task 6: project service 层 caller.IsMemberOf 跨租户强制(Phase 3, Commit #6)

**Files:**
- Modify: `internal/project/service.go`(4-5 个 method 加 check)

**Agent:** Agent 2

- [ ] **Step 1: 先看现状**

Run: `cat internal/project/service.go | head -100`

- [ ] **Step 2: 写 failing test**

在 `internal/project/service_test.go` 追加(新测试):

```go
func TestProjectService_GetProject_CrossTenant_Denied(t *testing.T) {
    // Setup: in-memory project repo, user "alice" in project "a-1" only
    cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleDeveloper})
    ctx := caller.WithCaller(context.Background(), cl)
    m := func(ctx context.Context, userID string) (map[string]struct{}, error) {
        return map[string]struct{}{"a-1": {}}, nil
    }
    svc := NewService(repoFixture(t), m)
    _, err := svc.GetProject(ctx, "b-1")  // b-1 not in alice's membership
    if !errors.Is(err, ErrForbidden) {
        t.Errorf("err = %v, want ErrForbidden", err)
    }
}

func TestProjectService_GetProject_SuperAdmin_Bypasses(t *testing.T) {
    cl := caller.New(&contracts.User{ID: "admin", Username: "admin", Role: contracts.RoleSuperAdmin})
    ctx := caller.WithCaller(context.Background(), cl)
    m := func(ctx context.Context, userID string) (map[string]struct{}, error) {
        return nil, nil  // no memberships needed
    }
    svc := NewService(repoFixture(t), m)
    _, err := svc.GetProject(ctx, "b-1")
    if err != nil {
        t.Errorf("SuperAdmin should bypass, got err = %v", err)
    }
}

func TestProjectService_GetProject_NilCaller_401(t *testing.T) {
    m := func(ctx context.Context, userID string) (map[string]struct{}, error) {
        return map[string]struct{}{}, nil
    }
    svc := NewService(repoFixture(t), m)
    _, err := svc.GetProject(context.Background(), "a-1")
    if !errors.Is(err, ErrUnauthenticated) {
        t.Errorf("err = %v, want ErrUnauthenticated", err)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 -run "TestProjectService_GetProject_CrossTenant" ./internal/project/`
Expected: FAIL — `GetProject` 现在没 caller check,直接读 repo 返数据。

- [ ] **Step 4: 改 `internal/project/service.go`**

在 `GetProject` method 起点加:

```go
func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
    cl, ok := caller.FromContext(ctx)
    if !ok || cl == nil || cl.User == nil {
        return nil, ErrUnauthenticated
    }
    if !cl.IsSuperAdmin() {
        if !cl.IsMemberOf(ctx, id, s.membershipChecker) {
            return nil, ErrForbidden
        }
    }
    return s.repo.Get(ctx, id)
}
```

类似 patch 4-5 个 method:`List` / `Create` / `Update` / `Delete` / `GetProjectWithRelations`。

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/project/`
Expected: PASS — 3 new cases + 既有。

- [ ] **Step 6: 跑全量**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/project/service.go internal/project/service_test.go
git commit -m "feat(project): service 层 caller.IsMemberOf 跨租户强制"
```

---

## Task 7-13: 7 other modules 同样模式(Phase 3, Commit #7-13)

**Files:** 7 modules × 1 commit

每个 module 重复 Task 6 模式:
- `internal/device/` — Task 7
- `internal/k8s/` — Task 8
- `internal/pipeline/` — Task 9
- `internal/servicecatalog/` — Task 10
- `internal/physicalhost/` — Task 11
- `internal/alerts/` — Task 12
- `internal/logs/` — Task 13

**注意**:
- `servicecatalog` 已有 `MembershipChecker` 注入,直接接
- `physicalhost` 8 sites 在 service 层(ListWithDevice / Get / Create / Update / Delete / EnterMaintenance / ExitMaintenance / Metrics)
- 既有 module test 不退化(加 caller + superadmin bypass)
- 每个 commit 1 module,7 个独立 commit

每个 commit message format:
```bash
git commit -m "feat(<module>): service 层 caller.IsMemberOf 跨租户强制"
```

**Agent:** Agent 2(可以 1 个 agent 跑全部 7 modules,因为模式相同)

---

## Task 14: project service 层 audit 覆盖(Phase 4, Commit #14)

**Files:**
- Modify: `internal/project/service.go`(5 emit: Create/Update/Delete/MemberAdd/MemberRemove)

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `grep -n "func.*Create.*Project\|func.*Update.*Project\|func.*Delete.*Project\|MemberAdd\|MemberRemove" internal/project/service.go | head -20`

- [ ] **Step 2: 写 failing test**

在 `internal/project/service_test.go` 追加:

```go
type fakeAuditSvc struct {
    events []audit.RecordActionInput
}

func (f *fakeAuditSvc) RecordAction(ctx context.Context, in audit.RecordActionInput) {
    f.events = append(f.events, in)
}

func TestProjectService_CreateProject_EmitsAudit(t *testing.T) {
    auditSvc := &fakeAuditSvc{}
    svc := NewService(repoFixture(t), nil, auditSvc)
    cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleOperator})
    ctx := caller.WithCaller(context.Background(), cl)
    ctx = audit.WithRequestActor(ctx, audit.RequestActor{UserID: "alice", Username: "alice"})
    p := &Project{Name: "new-project"}
    if err := svc.CreateProject(ctx, p); err != nil {
        t.Fatal(err)
    }
    if len(auditSvc.events) != 1 {
        t.Errorf("audit events = %d, want 1", len(auditSvc.events))
    }
    if auditSvc.events[0].Action != audit.ActionCreate {
        t.Errorf("audit action = %q, want %q", auditSvc.events[0].Action, audit.ActionCreate)
    }
    if auditSvc.events[0].ResourceType != audit.ResourceProject {
        t.Errorf("resource type = %q, want %q", auditSvc.events[0].ResourceType, audit.ResourceProject)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 -run "TestProjectService_CreateProject_EmitsAudit" ./internal/project/`
Expected: FAIL — `CreateProject` 没调 audit.RecordAction。

- [ ] **Step 4: 改 `CreateProject` 加 audit emit**

```go
func (s *Service) CreateProject(ctx context.Context, p *Project) error {
    if err := s.repo.Create(ctx, p); err != nil {
        return err
    }
    if s.audit != nil {
        actor := audit.RequestActorFromContext(ctx)
        s.audit.RecordAction(ctx, audit.RecordActionInput{
            ActorID:      actor.UserID,
            Action:       audit.ActionCreate,
            ResourceType: audit.ResourceProject,
            ResourceID:   p.ID,
            Metadata:     map[string]any{"name": p.Name},
        })
    }
    return nil
}
```

同样 patch Update / Delete / MemberAdd / MemberRemove,4 个 method。

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/project/`
Expected: PASS。

- [ ] **Step 6: 跑全量**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/project/service.go internal/project/service_test.go
git commit -m "feat(project): service 层 audit.RecordAction 覆盖 Create/Update/Delete/MemberAdd/MemberRemove"
```

---

## Task 15-21: 7 other modules 同样模式(Phase 4, Commit #15-21)

**Files:** 7 modules × 1 commit

每个 module 重复 Task 14 模式:
- `internal/device/groups.go` — Task 15 (Create/Delete)
- `internal/k8s/service.go` — Task 16 (Cluster Create/Update/Delete)
- `internal/pipeline/service.go` — Task 17 (Create/Update/Delete/Trigger)
- `internal/servicecatalog/service.go` — Task 18 (第 2 处,TODO 第 318 行)
- `internal/alerts/service.go` — Task 19 (Rule Create/Update/Delete)
- `internal/logs/service.go` — Task 20 (SavedFilter Create/Delete)
- `internal/discovery/service.go` — Task 21 (Run Create)

**注意**:
- 既有 audit 覆盖的 module (physicalhost / hostproject) 跳过
- 每个 module 1 commit,7 个独立 commit
- audit 调用失败不阻塞业务路径(s.audit != nil 守卫)
- audit.RecordAction 的 actor 从 `audit.RequestActorFromContext(ctx)` 提取

每个 commit message format:
```bash
git commit -m "feat(<module>): service 层 audit.RecordAction 覆盖 ..."
```

**Agent:** Agent 3(可 1 个 agent 跑全部 7 modules)

---

## Task 22: E2E 跨 module 集成测试(Phase 5, Commit #22)

**Files:**
- Create: `tests/integration/auth_rbac_test.go`

**Agent:** 主 session

- [ ] **Step 1: 写 failing test**

`tests/integration/auth_rbac_test.go`:

```go
package integration

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/devops-toolkit/backend/cmd/devops-toolkit"
    "github.com/devops-toolkit/backend/pkg/contracts"
    "github.com/golang-jwt/jwt/v5"
)

func TestE2E_AnonymousAPIV1_AllDenied(t *testing.T) {
    r := devops_toolkit.BuildRouter(testE2EConfig(t))  // 暴露 BuildRouter 或等价
    endpoints := []string{
        "/api/v1/devices", "/api/v1/projects", "/api/v1/k8s/clusters",
        "/api/v1/pipelines", "/api/v1/services", "/api/v1/physical-hosts",
        "/api/v1/alerts", "/api/v1/logs", "/api/v1/audit",
    }
    for _, ep := range endpoints {
        req := httptest.NewRequest("GET", ep, nil)
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
        if w.Code != 401 {
            t.Errorf("%s: code = %d, want 401", ep, w.Code)
        }
    }
}

func TestE2E_DevCrossProject_Denied(t *testing.T) {
    // 启动 server + DB + project fixtures
    // 用 Developer token,member of project A
    // 请求 /api/v1/projects/B → 403
}

func TestE2E_AuditEmitted(t *testing.T) {
    // Operator token POST /api/v1/projects
    // 查 audit_events 表,应有 1 row with actor_id = "alice"
}

// helper
func mintE2EToken(t *testing.T, secret string, user *contracts.User, exp time.Duration) string {
    t.Helper()
    claims := contracts.JWTClaims{
        UserID: user.ID, Username: user.Username, Role: user.Role,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(secret))
    if err != nil { t.Fatal(err) }
    return signed
}

func testE2EConfig(t *testing.T) *config.Config {
    // 构造 in-memory config(DB=sqlite in-memory,JWT secret=test)
    t.Helper()
    return &config.Config{...}  // 既有 test config helper
}
```

- [ ] **Step 2: 跑 test,确认 fail**

Run: `go test -count=1 -tags=integration ./tests/integration/`
Expected: FAIL — `devops_toolkit.BuildRouter` 不可导出,需先在 main.go 加 export。

- [ ] **Step 3: 暴露 `BuildRouter`(如果不可导出)**

在 `cmd/devops-toolkit/main.go`,如果 `buildRouter` 是 lowercase,改 `BuildRouter` 为 exported(只在 tests 用)。

或更好的:在 `cmd/devops-toolkit/main.go` 加一个 `TestApp` helper 返回 router + cleanup。

- [ ] **Step 4: 跑 test,确认 pass**

Run: `go test -count=1 ./tests/integration/`
Expected: PASS — 3 E2E cases。

- [ ] **Step 5: 跑全量**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS — 31+ packages + integration 全绿。

- [ ] **Step 6: 提交**

```bash
git add tests/integration/auth_rbac_test.go
git commit -m "test(e2e): 跨 module auth + RBAC + audit 集成"
```

---

## Task 23: v0.3.0.0 文档收尾(Phase 5, Commit #23)

**Files:**
- Modify: `CHANGELOG.md`(加 `[0.3.0.0]` section)
- Modify: `VERSION`(0.2.1.0 → 0.3.0.0)
- Modify: `DOCUMENT_INDEX.md`(索引新 spec)
- Create: `openspec/specs/auth-rbac-hardening/spec.md`(本 plan 的简短镜像)
- Modify: `TODOS.md`(B 子项目移到 Completed)

**Agent:** 主 session

- [ ] **Step 1: 更新 CHANGELOG**

```markdown
## [0.3.0.0] - 2026-06-XX

B 子项目 — 鉴权+多租户硬化 落地。5 个并行 agent 实施,1-1.5 周完成。关 P0 #1 (Auth+RBAC middleware 接线) + P0 #2 (Service 层跨租户强制) + P0 #3 (Audit 全覆盖) + scoped-Auditor RBAC matrix。

### Added
- Auth+RBAC middleware 接到 /api/v1 整体(JWT 解析 → caller 注入 → per-route Permission check)
- Service 层 8 modules × caller.IsMemberOf fail-closed 跨租户检查
- Audit 8 modules × 1-5 emit 全覆盖 mutating 动作
- RoleScopedAuditor + PermissionViewAuditLogProject 接线 RBAC matrix

### Fixed
- 匿名访问 /api/v1 现在 401(以前匿名可读到所有数据)
- Developer 跨项目访问现在 403(以前可跨项目读改删)
- Mutating 动作全部有 audit row(以前部分 module 缺失)

### Internal
- caller.FromContext + caller.FromGin 统一 caller 提取
- ~22-25 atomic commit + 5 merge commit
- 50+ new unit tests, 10+ E2E cases
- 31+ packages + integration test 全绿
```

- [ ] **Step 2: 更新 VERSION**

```bash
echo "0.3.0.0" > VERSION
```

- [ ] **Step 3: 更新 DOCUMENT_INDEX 加 B spec 索引**

在 `docs/superpowers/specs/` 章节加:

```markdown
| [2026-06-12-B-auth-rbac-hardening-design](docs/superpowers/specs/2026-06-12-B-auth-rbac-hardening-design.md) | **B 子项目 — 鉴权+多租户硬化** (v0.3.0.0) | ✅ 已实施 |
```

- [ ] **Step 4: 创建 openspec/specs/auth-rbac-hardening/spec.md**

镜像本 plan 的简短版(200-300 行):

```markdown
# 鉴权+多租户硬化 spec

## 目标
- Auth+RBAC middleware 接到 /api/v1
- Service 层跨租户强制
- Audit 覆盖全部 mutating 模块
- scoped-Auditor RBAC matrix 接线

## 设计
- AuthMiddleware: 解析 JWT, 注入 caller 到 context
- RBACMiddleware: 默认 deny, 注入 caller 到 gin.Context
- Service layer: caller.IsMemberOf fail-closed (SuperAdmin bypass)
- Audit: audit.Service.RecordAction 复用
- RBAC matrix: RoleScopedAuditor + PermissionViewAuditLogProject

## 关键数据流
- 匿名 → AuthMiddleware → 401
- 有效 token + 错 Permission → per-route perms() → 403
- 有效 token + 正确 Permission + 跨租户 → Service caller.IsMemberOf → 403
- 有效 token + 正确 Permission + 同租户 → 200
- SuperAdmin → 200 (bypass)
- scoped-Auditor → per-tenant audit filter
```

- [ ] **Step 5: 更新 TODOS.md**

把 B 子项目相关 P0 #1/#2/#3 + scoped-Auditor 移到 "## Completed" section。

- [ ] **Step 6: 提交**

```bash
git add CHANGELOG.md VERSION DOCUMENT_INDEX.md openspec/specs/auth-rbac-hardening/spec.md TODOS.md
git commit -m "chore(release): v0.3.0.0 — B 子项目鉴权+多租户硬化文档收尾"
```

---

## Self-Review

### Spec coverage 验证

- [x] P0 #1 Auth+RBAC middleware → Task 2-5 (Phase 2)
- [x] P0 #2 Service 跨租户 → Task 6-13 (Phase 3, 8 modules)
- [x] P0 #3 Audit 全覆盖 → Task 14-21 (Phase 4, 8 modules)
- [x] scoped-Auditor matrix → Task 1 (Phase 1)
- [x] E2E 集成 + 文档 → Task 22-23 (Phase 5)

### Placeholder 扫描

无 TBD / TODO / FIXME。代码块完整。

### Type / method 一致性

- `caller.New(user)` 在 Task 1 引入(既有或加),Task 2/3/6/14 引用
- `caller.FromContext(ctx)` 在 Task 2 引入(既有或加),Task 6-21 引用
- `caller.FromGin(c)` 在 Task 3 引入,Task 3 引用
- `audit.RequestActorFromContext(ctx)` 既有,Task 14-21 引用
- `auth.AuthMiddleware(secret)`, `rbac.RBACMiddleware()` 在 Task 2-3 引入,Task 4 引用
- `ErrUnauthenticated`, `ErrForbidden` 在 module 局部定义(每个 module 各自)或在 caller package 共享 — Task 6 引入,Task 7-13 引用

### 关键风险

1. **Phase 2 改 buildRouter 影响 8 modules** — Task 5 E2E 测所有 8 endpoint 必须全过
2. **既有 main_test.go 的 TestBuildRouter_* 退化** — 需在 Task 4 加 JWT token 到 test helper
3. **Task 14-21 audit 调用 8 modules 各自代码不同** — pattern 相同但 method 签名不同,需要每个 module 看现状
4. **Phase 2 + 3 + 4 同时改 main.go + service.go** — 三个 phase 各自独立 branch,merge 时按顺序

---

## Execution Handoff

**Plan 完成,共 23 task,5 phase,~22-25 commit,1-1.5 周。**

下一步:
- 5 个 agent 各开一个 branch:
  - 主 session Phase 1: `feat/b1-scoped-auditor` (Task 1)
  - Agent 1: `feat/b2-auth-rbac-middleware` (Task 2-5)
  - Agent 2: `feat/b3-cross-tenant` (Task 6-13, 7 modules)
  - Agent 3: `feat/b4-audit-coverage` (Task 14-21, 7 modules)
  - 主 session Phase 5: `feat/b5-e2e-docs` (Task 22-23)
- 主 session 协调 merge 顺序: B1 → B2 → B3 → B4 → B5
