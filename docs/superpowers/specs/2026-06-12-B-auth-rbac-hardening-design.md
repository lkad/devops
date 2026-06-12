# B 子项目 — 鉴权+多租户硬化 设计

> **范围**:把 v0.2.0.0 P0 #1 (Auth+RBAC middleware 接线) + P0 #2 (跨租户强制) + P0 #3 (Audit 覆盖全部 mutating 模块) + scoped-Auditor RBAC matrix 全部落地。
>
> **日期**: 2026-06-12
> **作者**: Claude
> **状态**: Design (待用户 review)
> **对应子项目**: B(5 子项目分解的第二个,接 A 之后)
> **总估时**: 1-1.5 周
> **前序**: A 子项目(`cb5ea906` spec + `6a09794b` plan + `70c4c916` release)已完成 31 packages 全绿

---

## 1. 背景与动机

### 1.1 现状(v0.2.1.0)

| 现状 | 来源 |
|---|---|
| **Auth middleware 完整** | `internal/auth/middleware.go` 有 `NewAuthMiddleware` + JWT 解析 |
| **Caller package 完整** | `internal/auth/caller/caller.go` 有 `IsMemberOf`/`IsSuperAdmin`/`CheckProjectAccess`/`RequireProjectAccess`/`RequireAnyProjectAccess` |
| **RBAC matrix 完整** | `internal/auth/rbac/matrix.go` 4 role × 30+ Permission,`RolePermissions` map 完整 |
| **Audit system 完整** | `internal/audit/{service,handler,middleware}.go` + `RecordAction` + `RequestActor` |
| **Audit 覆盖 8 modules** | physicalhost / hostproject / device / servicecatalog(部分) / 等 |

### 1.2 已知缺口(v0.2.1.0 仍然未做)

| P0 项 | 缺口 | 估时 |
|---|---|---|
| **P0 #1** | `internal/middleware/chain.go:12` 注释明说"Auth and RBAC are injected by their respective owners (Phase 2)" — `buildRouter` 顶部**没接** `auth.AuthMiddleware` / `rbac.RequirePermission` | 1-2 天 |
| **P0 #2** | `internal/project/service.go:130-141` `GetProjectWithRelations` 不管 membership,Developer 可跨项目读 | 3-5 天 |
| **P0 #3** | Audit 只 8 modules 覆盖。缺口:servicecatalog 第 2 处 + project CRUD + member grant + device CRUD + k8s cluster CRUD + pipeline runs + alert rules + log saved-filters + discovery runs | 2-3 天 |
| **scoped-Auditor matrix** | `internal/audit/handler.go` 走 per-tenant 路径,但 `internal/auth/rbac/matrix.go` 没 `RoleScopedAuditor` 角色 | 1 hr |

### 1.3 目标

1. **Auth+RBAC middleware 接到 `/api/v1` 整体**,8 modules 全部获益
2. **Service 层 caller.IsMemberOf 检查**,8 modules × 1-2 处 fail-closed
3. **Audit 覆盖全部 mutating 动作**,不留盲点
4. **scoped-Auditor role + permission 接线**,matrix 完整

---

## 2. 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│              B 子项目 — 鉴权+多租户硬化 架构                      │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─ /api/v1 整体 (buildRouter) ────────────────────────────┐    │
│  │                                                          │    │
│  │  AuthMiddleware  ─ JWT 解析 + caller 注入 context       │    │
│  │       ↓                                                 │    │
│  │  RBACMiddleware  ─ 默认 deny + caller 注入 Gin          │    │
│  │       ↓                                                 │    │
│  │  AuditMiddleware ─ RequestActor 注入 context (既有)     │    │
│  │       ↓                                                 │    │
│  │  PerRoute perms(rbacpkg.Permission) ─ 细化 8 modules    │    │
│  │       ↓                                                 │    │
│  │  RequireProjectAccess / RequireAnyProjectAccess (既有)  │    │
│  │       ↓                                                 │    │
│  │  Service layer ─ caller.IsMemberOf fail-closed          │    │
│  │                                                          │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Service 层(8 modules × 1-2 处 caller.IsMemberOf):              │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  cl := caller.FromContext(ctx)                            │    │
│  │  if cl == nil || cl.User == nil { return ErrUnauth }    │    │
│  │  if !cl.IsSuperAdmin() {                                 │    │
│  │      if !cl.IsMemberOf(ctx, projectID, m) {              │    │
│  │          return ErrForbidden                             │    │
│  │      }                                                   │    │
│  │  }                                                       │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Audit 覆盖(8 modules × 1-5 emit):                              │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  audit.RecordAction(ctx, RecordActionInput{              │    │
│  │      ActorID: caller.User.ID,                            │    │
│  │      Action: "create",                                   │    │
│  │      ResourceType: "project",                            │    │
│  │      ResourceID: id,                                     │    │
│  │      Metadata: {...},                                    │    │
│  │  })                                                     │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  RBAC matrix:                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  RoleScopedAuditor = 新 role                             │    │
│  │  PermissionViewAuditLogProject = 新 permission           │    │
│  │  RolePermissions 加 RoleScopedAuditor 映射                │    │
│  │  audit.Service.ListForCaller 走 per-tenant 路径          │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### 关键设计原则

1. **Auth+RBAC 在 buildRouter 顶部一次接** — 所有 8 modules 自动获益
2. **Service 层 caller.IsMemberOf 检查**只针对 Developer 路径(SuperAdmin bypass)
3. **Audit 复用现有 `audit.Service.RecordAction`** — 无需新 API
4. **scoped-Auditor** 加新 role + new permission,不影响 `RoleAuditor` 现有行为
5. **Fail-closed** 任何 nil caller / nil membership → 401 / 403

---

## 3. 关键 Component 详解

### 3.1 Auth+RBAC Middleware 接线(Agent 1)

**目标**:`/api/v1` 整体顶部接 2 个 middleware,JWT 验证 + 默认 deny RBAC。

**接口**(在 `cmd/devops-toolkit/main.go` 的 `buildRouter` 内):

```go
v1 := r.Group("/api/v1",
    auth.AuthMiddleware(),       // 解析 JWT,注入 caller 到 context
    rbac.RBACMiddleware(),       // 默认 deny,注入 caller 到 gin.Context
    audit.Middleware(),          // 既有,注入 RequestActor
)
```

**Auth Middleware 职责**(`internal/auth/middleware.go` 已有 `NewAuthMiddleware`? 若有则用,否则补):
- 解析 `Authorization: Bearer <jwt>` header
- JWT 验签 + 解析 → `contracts.User` (含 Role)
- 注入到 `c.Set("caller", *caller.Caller)`(新 caller 包装)
- 同时 push 到 `c.Request.Context()`(service 层可读)
- 无 header / 无效 token → 401

**RBAC Middleware 职责**(新建 `internal/auth/rbac/middleware.go` 或追加):
- 不强制 Permission(由 per-route `perms()` 处理)
- 负责注入 caller 到 context.Context
- 401 / 403 fallback 留给 per-route

**路由分组改造**:
- 现状:每个 `register*Routes(v1, ...)` 函数自接 `perms()` + 各种 project-scope middleware
- 改造:`v1` 已被 Auth+RBAC 中间件包裹,register 函数去掉 Auth 调用,只接 `perms()` 和 project-scope

**单元测试**(`internal/auth/middleware_test.go` + `internal/auth/rbac/middleware_test.go`):
- `TestAuthMiddleware_ValidToken_OK`:带有效 JWT,`c.Get("caller")` 返非 nil Caller
- `TestAuthMiddleware_MissingToken_401`:无 header,401
- `TestAuthMiddleware_InvalidToken_401`:token 解析失败,401
- `TestAuthMiddleware_ExpiredToken_401`:exp claim 过期,401
- `TestAuthMiddleware_NilUser_401`:JWT 解析成功但 user 不存在
- `TestRBACMiddleware_ValidCaller_Allows`:JWT 解析成功,继续
- `TestRBACMiddleware_NoCaller_401`:无 caller,401

### 3.2 Service 层跨租户强制(Agent 2)

**目标**:8 modules 的 service 层在 read/write/delete 起点加 `caller.IsMemberOf` 检查。

**Pattern**(每个 module 加 1-2 行):

```go
// internal/project/service.go (示例)
func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
    cl := caller.FromContext(ctx)
    if cl == nil || cl.User == nil {
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

**8 modules 影响面**:
- `internal/project/` — 4-5 个 method (Get/List/Create/Update/Delete)
- `internal/device/` — 3-4 个 method
- `internal/k8s/` — 3 个 method (Cluster)
- `internal/pipeline/` — 3-4 个 method
- `internal/servicecatalog/` — 2-3 个 method
- `internal/physicalhost/` — 2 个 method (Get/List)
- `internal/alerts/` — 1-2 个 method
- `internal/logs/` — 1-2 个 method

**测试**(每个 module 加 1 case):
- `TestService_*_CrossTenant_Denied`:Developer in project A 调用 project B 资源,403
- 既有测试 + SuperAdmin bypass 路径
- nil caller → 401(fail-closed)

**注意**:
- `servicecatalog` 已有 `MembershipChecker` 接入,参考其 pattern
- 既有 call site 不能改外部 API,只改 service 内部
- List method 走 `ListForCaller` helper(参照 audit.Service.ListForCaller 既有)或加 per-row check

### 3.3 Audit 覆盖 8 modules(Agent 3)

**目标**:未发 audit 的 mutating handler 全接 `audit.Service.RecordAction`。

**8 modules 影响面**:
- `internal/project/service.go` — Create / Update / Delete / MemberAdd / MemberRemove (5 emit)
- `internal/device/groups.go` — 既有部分,补 Create / Delete (2 emit)
- `internal/k8s/` — Cluster Create / Update / Delete (3 emit)
- `internal/pipeline/` — Pipeline Create / Update / Delete / Trigger (4 emit)
- `internal/servicecatalog/` — 第 2 处(`service.go:318` 有 `TODO(audit)` 标记)
- `internal/hostproject/` — 已发,无新增
- `internal/alerts/` — Rule Create / Update / Delete (3 emit)
- `internal/logs/` — SavedFilter Create / Delete (2 emit)
- `internal/discovery/` — Run Create (1 emit)

**Pattern**(每个 emit 站点 5-10 行):

```go
// internal/project/service.go Create (示例)
func (s *Service) CreateProject(ctx context.Context, p *Project) error {
    if err := s.repo.Create(ctx, p); err != nil {
        return err
    }
    if s.audit != nil {
        actor := audit.RequestActorFromGin(caller.FromContext(ctx))
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

**注意**:
- `audit.Service` 用 variadic 注入(`auditSvc ...*audit.Service`),多数 module 已接
- 若没接,加 1 个 `auditSvc` 字段 + `NewService` 改
- actor 从 `caller.FromContext` 提取(因为 `audit.RequestActor` 已在 middleware 阶段塞到 context)
- `audit.RecordAction` 失败**不阻塞**业务路径(只记日志)

**测试**:
- 每个 emit 加 `TestService_*_EmitsAudit`:成功路径 1 row,失败路径 0 row
- 既有 test 路径继续过

### 3.4 scoped-Auditor RBAC matrix(Agent 4)

**目标**:`internal/auth/rbac/matrix.go` 加 `RoleScopedAuditor` + `PermissionViewAuditLogProject`。

**改动**:
- `pkg/contracts/user.go` 加 `RoleScopedAuditor Role = "ScopedAuditor"`
- `internal/auth/rbac/matrix.go`:
  - 加 `PermissionViewAuditLogProject Permission = "audit.log.view.project"`
  - 加 `RoleScopedAuditor → [PermissionViewAuditLog, PermissionViewAuditLogProject, ...所有 view.*]`(同 RoleAuditor 权限 + ViewAuditLogProject)
- `internal/audit/service.go`: `ListForCaller` 接受 `PermissionViewAuditLogProject` 时走 per-tenant 路径
- `internal/audit/handler.go`: per-route check 加 `PermissionViewAuditLogProject` 选项

**测试**:
- `TestRBAC_RoleScopedAuditor_HasPermission`:matrix 查表返回 true
- `TestRBAC_RoleAuditor_DoesNotHaveScopedPermission`:matrix 查表返回 false
- `TestRBAC_RoleScopedAuditor_DoesNotHaveWrite`:matrix 不应给 write 权限
- `TestAudit_ListForCaller_ScopedAuditor_AppliesFilter`:已存在(6 case,前几轮做的)

### 3.5 E2E 集成 + 文档(Agent 5)

**目标**:跨 module 集成测试 + 文档 + 验证。

**E2E test**(`tests/integration/auth_rbac_test.go`):
- 启动 in-process server,用 `httptest`
- 测各种场景:未认证 / 错 role / cross-tenant
- 覆盖 8 modules 至少 1 个 endpoint

**文档**:
- `openspec/specs/auth-rbac-hardening/spec.md`(本 spec 的简短镜像)
- `CHANGELOG.md` 加 `[0.3.0.0] - 2026-06-XX` section
- `DOCUMENT_INDEX.md` 索引

---

## 4. 数据流与错误处理

### 4.1 关键数据流

**Flow 1: 匿名请求 401**

```
curl /api/v1/devices  (no Authorization header)
   ↓
buildRouter
   ↓
v1 group: AuthMiddleware() → 401 (no token)
```

**Flow 2: 有效 token + 错 Permission 403**

```
curl /api/v1/devices  (Bearer valid JWT, role=Auditor)
   ↓
v1: AuthMiddleware() → set caller in context
   ↓
registerDeviceRoutes: r.GET("/devices", perms(PermissionWriteDevices), ...)
   ↓
perms(PermissionWriteDevices) → check RolePermissions[Auditor] ⊃ [PermissionWriteDevices]? NO → 403
```

**Flow 3: 有效 token + 正确 Permission,但跨租户**

```
curl /api/v1/projects/B  (Bearer JWT, role=Developer, member of A only)
   ↓
v1: AuthMiddleware() → caller.User{role=Developer}
   ↓
perms(PermissionViewProjects) → OK (Developer has it)
   ↓
Service.GetProject(ctx, "B"):
  cl := caller.FromContext(ctx)
  if !cl.IsSuperAdmin() {  // false
      if !cl.IsMemberOf(ctx, "B", m) {  // false
          return ErrForbidden  // 403
      }
  }
```

**Flow 4: SuperAdmin bypass**

```
curl /api/v1/projects/B  (Bearer JWT, role=SuperAdmin)
   ↓
Auth → caller.User{role=SuperAdmin}
   ↓
perms(PermissionViewProjects) → OK
   ↓
Service.GetProject:
  if !cl.IsSuperAdmin() {  // false (bypass path)
  return s.repo.Get(ctx, "B")  // OK
```

**Flow 5: Audit 发射链**

```
POST /api/v1/projects (Bearer JWT, role=Operator)
   ↓
Auth → caller.User{id: "alice", role: "Operator"}
   ↓
AuditMiddleware → RequestActor{UserID: "alice", Username: "alice", IP, UA}
   ↓
perms(PermissionWriteProjects) → OK
   ↓
Service.CreateProject → s.audit.RecordAction(ctx, RecordActionInput{
    ActorID: "alice", Action: "create", ResourceType: "project", ResourceID: newID
})
   ↓
audit.Service.RecordAction → BufferedEmitter → DBEmitter → audit_events row
```

**Flow 6: scoped-Auditor per-tenant 过滤**

```
curl /api/v1/audit?project_id=X  (Bearer JWT, role=ScopedAuditor)
   ↓
Auth → caller.User{role: ScopedAuditor}
   ↓
perms(PermissionViewAuditLogProject) → OK
   ↓
audit.Service.ListForCaller:
  cl.IsSuperAdmin()? false
  cl.User.Role == ScopedAuditor? true
  → load membership → List(AuditFilter{ProjectIDsIn: [...]})
  → sql: WHERE json_extract(metadata, '$.project_id') IN (...)
```

### 4.2 错误处理约定

| 场景 | HTTP status | Body | log |
|---|---|---|---|
| 无 Authorization header | 401 | `{code: "unauthenticated", ...}` | warn |
| Token 解析失败 / 过期 | 401 | `{code: "invalid_token", ...}` | warn |
| Token 有效,但 user 不存在 | 401 | `{code: "user_not_found", ...}` | warn |
| RBAC 缺 Permission | 403 | `{code: "forbidden", ...}` | info |
| 跨租户访问 | 403 | `{code: "forbidden", message: "not a member of project X"}` | info |
| 资源不存在 | 404 | `{code: "not_found", ...}` | debug |
| Service 内 err | 500 | `{code: "internal", ...}` | error |

**关键错误传递约定**:
- **Fail-closed** 任何 nil caller / nil membership → 401 / 403(不静默放行)
- **Audit 失败不阻塞业务路径** — `s.audit.RecordAction` 失败仅记日志
- **RBAC 检查不阻塞登录** — `/api/v1/auth/login` 不需 JWT(单独路由,无 Auth 中间件)

### 4.3 测试矩阵

| Component | Unit | Integration | E2E |
|---|---|---|---|
| Auth Middleware | ✅ 5 case (valid/missing/invalid/expired/bad-user) | ✅ | ✅ |
| RBAC Middleware | ✅ 3 case (caller/missing/superadmin) | ✅ | ✅ |
| Service caller.IsMemberOf pattern | ✅ 每个 module 1 case (cross-tenant denied) | ✅ | ✅ |
| Audit Service.RecordAction | ✅ 既有 (前几轮测试) | ✅ 既有 | ✅ |
| Audit 覆盖 8 modules | ✅ 每个 module 1 case (success → row) | ✅ | ✅ |
| RBAC matrix (RoleScopedAuditor) | ✅ 4 case (matrix 查表 4 role) | ✅ | ✅ |
| Audit.ListForCaller scoped | ✅ 既有 6 case (前几轮) | ✅ | ✅ |
| Cross-tenant 8 modules E2E | ✅ | ✅ 集成 | ✅ 集成 |

**总测试新增**:~50 unit case(8 modules × 2 + audit × 9 + rbac × 4 + auth × 8)
**E2E 集成**:~10 case(覆盖 8 modules 至少各 1 endpoint)
**总耗时**:`go test -count=1 -p 1 -timeout 600s ./...` 估计 6-7 分钟

### 4.4 风险 & 缓解

| 风险 | 缓解 |
|---|---|
| 现有 8 modules 可能有匿名 handler 行为依赖,接 auth 后 401 | E2E test 跑全部模块,行为不一致就补 caller 注入 |
| `caller.IsMemberOf` 调用加 DB round-trip,影响 latency | `caller.Caller` 已有 per-request membership cache(看 caller.go),复用 |
| `audit.RecordAction` 在 hot path,加 buffer 满后 drop warn | 既有 `BufferedEmitter` 已处理 drop-on-overflow |
| RBAC matrix 加新 role 影响既有 LDAP group mapping | LDAP 不动,只加新 role 不删;group→role 映射后续做 |
| Service 层加 caller 检查可能绕过 middleware | middleware + service 双重保护 (defense-in-depth),但 service 是 trusted path(因为 middleware 已 401) |
| 既有 8 modules 用了 `r := r :=` 重复声明(前几轮 bug 修过) | 改 caller pattern 时小心,先 `go build` 验证 |

---

## 5. 实施计划(5 个 phase,5 agent 并行)

### Phase 1: scoped-Auditor matrix 接线 (Agent 4, 0.5 day, 1 PR)
- 加 `RoleScopedAuditor` + `PermissionViewAuditLogProject`
- 加 matrix 单元测试

### Phase 2: Auth+RBAC middleware 接线 (Agent 1, 1-1.5 day, 1 PR)
- `AuthMiddleware()` + `RBACMiddleware()` 实现
- `buildRouter` 在 `/api/v1` 顶部接
- 8 modules 回归测试(确认 200 路径不破)

### Phase 3: Service 层跨租户强制 (Agent 2, 2-3 day, 1 PR)
- 8 modules × 1-2 `caller.IsMemberOf` 检查
- 8 cross-tenant tests

### Phase 4: Audit 覆盖 8 modules (Agent 3, 2-3 day, 1 PR)
- 8 modules × 1-5 audit emit
- 8 emit tests

### Phase 5: E2E 集成 + 文档 (Agent 5, 1 day, 1 PR)
- `tests/integration/auth_rbac_test.go`
- `openspec/specs/auth-rbac-hardening/spec.md`
- `CHANGELOG.md` `[0.3.0.0]` section
- `DOCUMENT_INDEX.md` 索引

### Agent 分工方案

| 时间窗 | Agent 任务 | 主 session 任务 |
|---|---|---|
| Day 0-1 | (sync) Phase 1 by 主 session | 写 matrix + 跑测试 + commit |
| Day 1-2 | /loop Agent 1: Auth+RBAC middleware 接线(8 modules) | 监控,merge |
| Day 1-3 | /loop Agent 2: Service 层跨租户强制(8 modules) | 监控,merge |
| Day 1-3 | /loop Agent 3: Audit 覆盖 8 modules | 监控,merge |
| Day 4-5 | (sync) Phase 5 主 session E2E + 文档 | 写 spec, CHANGELOG |

**关键**:
- Phase 1 主 session 串行做(只 1 个 commit,小)
- Phase 2/3/4 可并行(改不同代码路径,无冲突)
- Phase 5 主 session 收尾

### 关键 commit 命名(预计 ~22-25 commits)

**Phase 1 (1 commit)**
1. `feat(rbac): 加 RoleScopedAuditor 角色 + PermissionViewAuditLogProject 权限`

**Phase 2 (3-5 commits)**
2. `feat(auth): AuthMiddleware 注入 caller 到 context`
3. `feat(rbac): RBACMiddleware 默认 deny + 注入 caller`
4. `feat(router): buildRouter 在 /api/v1 顶部接 Auth+RBAC middleware`
5. `test(e2e): 8 modules 验证 auth 接线 + 401/403 路径`

**Phase 3 (8-10 commits,1 commit / module)**
6. `feat(project): service 层 caller.IsMemberOf 跨租户强制`
7. `feat(device): service 层 caller.IsMemberOf 跨租户强制`
8. `feat(k8s): service 层 caller.IsMemberOf 跨租户强制`
9. `feat(pipeline): service 层 caller.IsMemberOf 跨租户强制`
10. `feat(servicecatalog): service 层 caller.IsMemberOf 跨租户强制`
11. `feat(physicalhost): service 层 caller.IsMemberOf 跨租户强制`
12. `feat(alerts): service 层 caller.IsMemberOf 跨租户强制`
13. `feat(logs): service 层 caller.IsMemberOf 跨租户强制`

**Phase 4 (8-10 commits,1 commit / module)**
14. `feat(project): service 层 audit.RecordAction 覆盖 Create/Update/Delete/MemberAdd/MemberRemove`
15. `feat(device): service 层 audit.RecordAction 补 Create/Delete`
16. `feat(k8s): service 层 audit.RecordAction 覆盖 Cluster CRUD`
17. `feat(pipeline): service 层 audit.RecordAction 覆盖 Create/Update/Delete/Trigger`
18. `feat(servicecatalog): service 层 audit.RecordAction 补 TODO 处`
19. `feat(alerts): service 层 audit.RecordAction 覆盖 Rule CRUD`
20. `feat(logs): service 层 audit.RecordAction 覆盖 SavedFilter Create/Delete`
21. `feat(discovery): service 层 audit.RecordAction 覆盖 Run Create`

**Phase 5 (2 commits)**
22. `test(e2e): 跨 module auth + RBAC + audit 集成`
23. `chore(release): v0.3.0.0 文档收尾`

### 每个 commit 的完成定义 (DoD)

- 该 commit 的 code 改动落地
- 单元测试新增/调整齐全,`go test ./...` 在该目录全绿
- 既有 31 packages 测试不退化
- `go vet ./...` warning 数 ≤ 1
- CHANGELOG.md 增一行
- commit message 用中文,符合 git log 风格

---

## 6. 风险与回退

### 风险

| 风险 | 缓解 |
|---|---|
| A 子项目改动面广(8+ directories),/loop agent 跑可能撞并发 | **分批 commit**: 每个 component 1-2 commit,/loop 起 3-4 agent 各管 1 组,主 session 协调 merge |
| Phase 2 改 buildRouter 一次性影响 8 modules | E2E test 必须 Phase 2 跑过,行为不一致就补 caller 注入 |
| `caller.IsMemberOf` 在 hot path | 已有 per-request membership cache,无新 round-trip |
| LDAP group 映射新 role 漏做 | LDAP 不动,Role→group 映射后续 spec |

### 回退(每个 phase 独立可回退)

| Phase | 回退方法 | 风险等级 |
|---|---|---|
| Phase 1 scoped-Auditor matrix | `git revert <merge commit>` (1 PR) | 🟢 零 |
| Phase 2 Auth+RBAC middleware | `git revert <merge commit>` (1 PR) | 🟡 中 — 8 modules 立刻 401 |
| Phase 3 Service 跨租户 | `git revert <merge commit>` (1 PR) | 🟡 中 — Developer 跨项目读能恢复 |
| Phase 4 Audit 覆盖 | `git revert <merge commit>` (1 PR) | 🟢 低 — 业务不阻塞 |
| Phase 5 E2E + 文档 | `git revert <merge commit>` | 🟢 零 |

---

## 7. 不做 (YAGNI)

- 不做 LDAP group → Role 映射自动化(`internal/auth/ldap` 已存在,不动)
- 不做 RBAC matrix 动态配置(YAML/DB 存,不在本子项目)
- 不做 multi-tenancy 跨 region / 跨 cluster(单 region 单 cluster 假设)
- 不做 SAML/OIDC SSO(认证方式只 JWT + LDAP,留后续)
- 不做 audit retention policy(留 ops 工具)
- 不做 RBAC UI(管理 role/permission 仍手改 matrix.go)
- 不做 dynamic policy engine(OPA 之类,留后续)

---

## 8. 验收标准

### 完成定义 (Definition of Done)

B 子项目**完成**意味着:

1. **代码**: ~22-25 个 commit 全部在 main 上,既有 31 packages 测试不退化
2. **测试**: 50+ 新单元测试全绿,`go test -count=1 -p 1 -timeout 600s ./...` 6-7 分钟跑完
3. **质量**: `go vet ./...` warning 数 ≤ 1(不增加)
4. **文档**: `CHANGELOG.md` `[0.3.0.0]` section,`openspec/specs/auth-rbac-hardening/` 新 spec,`DOCUMENT_INDEX.md` 索引
5. **安全**: 8 modules 的所有 mutating endpoint 都要求 JWT + 正确 Permission;Developer 跨项目访问返 403;SuperAdmin bypass OK
6. **审计**: 全部 mutating 动作有 audit row(`actor_id` 来自 JWT,不是 request body)
7. **可回退**: 每个 phase 一个 PR,`git revert` 单 PR 即可回退

### 关键验收用例

| 验收项 | 命令 / 验证 |
|---|---|
| 全部 unit test 绿 | `go test -count=1 -p 1 -timeout 600s ./...` |
| 既有 vet warning 不增加 | `go vet ./... 2>&1 \| wc -l` (应 ≤ 1) |
| 匿名访问 /api/v1 | `curl -i localhost:8080/api/v1/devices` → 401 |
| 有效 token + 错 Permission | `curl -i -H "Authorization: Bearer <Auditor token>" localhost:8080/api/v1/devices` → 403 |
| Developer 跨项目 | Developer in A 请求 /api/v1/projects/B → 403 |
| SuperAdmin bypass | SuperAdmin 请求 /api/v1/projects/B → 200 |
| Audit row 入库 | `SELECT * FROM audit_events ORDER BY occurred_at DESC LIMIT 1;` 应有 row |
| scoped-Auditor 过滤 | ScopedAuditor 请求 /api/v1/audit → 只返 member 项目的 audit row |
| 8 modules E2E | `go test ./tests/integration/auth_rbac_test.go` 8 modules 至少各 1 endpoint 200 |

---

## 9. 未来子项目 Preview

B 完成后(预计 1-1.5 周):

**C 子项目** (1 周): Helm chart 完整化 + 部署拓扑深化
- cert-manager / ExternalSecrets / ArgoCD 完整集成
- minikube / k3d 实装验证
- Velero 备份跨 K8s

**D 子项目** (2 周): UI 空白补全
- K8s pod log streaming UI (WS panel)
- K8s pod exec (Shell) UI (xterm.js)
- Service catalog on-call/runbook 写 UI
- Audit log UI (接入 scoped-Auditor)
- Middleware 链顺序修正 (CORS 完整)

**E 子项目** (1 周): 性能 + 容量基线
- k6 100/300/1000 VU 场景
- p95+p99+error rate 报告
- Go pprof 内存/协程 profile

B→C→D→E 总估时 5-6 周,完成 v0.2.0.0 → v0.3.0.0(几百人公司完整可用)。

---

## 附录 A:文件清单(详细变更范围)

### 修改文件

```
cmd/devops-toolkit/main.go                  (buildRouter 顶部接 Auth+RBAC)
internal/auth/middleware.go                 (AuthMiddleware 注入 caller,既有)
internal/auth/rbac/middleware.go            (RBACMiddleware 新建/追加)
internal/auth/rbac/matrix.go                (RoleScopedAuditor + PermissionViewAuditLogProject)
pkg/contracts/user.go                       (RoleScopedAuditor 常量)
internal/audit/handler.go                   (per-route 接受 PermissionViewAuditLogProject)
internal/audit/service.go                   (ListForCaller 走 scoped 路径)
internal/project/service.go                 (caller.IsMemberOf + audit emit × 5)
internal/device/groups.go                   (audit emit × 2)
internal/k8s/service.go                     (caller.IsMemberOf + audit emit × 3)
internal/pipeline/service.go                (caller.IsMemberOf + audit emit × 4)
internal/servicecatalog/service.go          (audit emit 第 2 处)
internal/physicalhost/service.go            (caller.IsMemberOf × 2)
internal/alerts/service.go                  (caller.IsMemberOf + audit emit × 3)
internal/logs/service.go                    (caller.IsMemberOf + audit emit × 2)
internal/discovery/service.go               (audit emit × 1)
internal/hostproject/service.go             (既已发,无新增)
```

### 新增文件

```
tests/integration/auth_rbac_test.go         (E2E 8 modules 验证)
openspec/specs/auth-rbac-hardening/spec.md  (本 spec 的简短镜像)
docs/superpowers/plans/2026-06-12-B-...md  (实施 plan)
```

### 8 modules 改 `caller.IsMemberOf`(Phase 3 的影响面)

```
internal/project/service.go              (5 sites)
internal/device/service.go               (3-4 sites)
internal/k8s/service.go                  (3 sites)
internal/pipeline/service.go             (3-4 sites)
internal/servicecatalog/service.go       (2-3 sites)
internal/physicalhost/service.go         (2 sites)
internal/alerts/service.go               (1-2 sites)
internal/logs/service.go                 (1-2 sites)
```

### 8 modules 改 `audit.RecordAction`(Phase 4 的影响面)

```
internal/project/service.go              (5 emit: Create/Update/Delete/MemberAdd/MemberRemove)
internal/device/groups.go                (2 emit: Create/Delete)
internal/k8s/service.go                  (3 emit: Cluster Create/Update/Delete)
internal/pipeline/service.go             (4 emit: Create/Update/Delete/Trigger)
internal/servicecatalog/service.go       (1 emit: 第 2 处)
internal/alerts/service.go               (3 emit: Rule Create/Update/Delete)
internal/logs/service.go                 (2 emit: SavedFilter Create/Delete)
internal/discovery/service.go            (1 emit: Run Create)
```

### 跨 phase 依赖

```
B.Phase1 ──→ B.Phase5
            └─→ C.Phase?(scoped-Auditor 启 audit log UI)

B.Phase2 ──→ D.Phase?(Auth+RBAC 是 K8s exec / service catalog 写 UI 的前提)

B.Phase3 ──→ E.Phase?(跨租户强制 + audit 全覆盖,性能基线才有意义)

B.Phase4 ──→ D.Phase?(audit log UI 显示数据来源)
```

B.Phase1 必须先做。Phase 2/3/4 可并行。Phase 5 收尾。
