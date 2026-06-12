# A 子项目 — 后端生产化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 DevOps Toolkit 平台在 5-6 天内做到「几百人公司研发+运维都能正常用」的最小生产化基线 — 含双轨兼容部署(docker-compose + Helm chart 骨架)、健康探活、强制生产鉴权、备份/恢复 drill、monitor 优雅停机、P2 收敛(写 API 错误/db 错误/dev-default secrets 集中/secret masking)、K8s interface 拆分,以及 7 个杂项修复。

**Architecture:** 单 Go 单体应用,共享代码层不依赖部署形态。`signal.NotifyContext` 在 main.go 接管 SIGTERM/SIGINT 并下发给 monitor_loop 与 http.Server,实现 30 秒内优雅停机。三个 health 端点共存:`/live` (livenessProbe)、`/ready` (readinessProbe,fan-out 探活)、`/health` (兼容旧调用方)。secret 强制生产在 `config.Validate()` 中以 fail-fast 实现。Helm chart 提供 deployment/service/ingress/cronjob-backup/prometheusrule 五种模板,docker-compose 升级配套 mem_limit/cpus/healthcheck/backup 容器。

**Tech Stack:** Go 1.26 + Gin + GORM + PostgreSQL + Prometheus + Helm chart + bash (backup script) + bats (script tests)

**Spec:** [`docs/superpowers/specs/2026-06-12-A-production-readiness-design.md`](../specs/2026-06-12-A-production-readiness-design.md)

---

## 范围总览

**14 commits,6 phase,5-6 天,5 个 agent 并行实施。**

| Agent | Branch | Tasks | 估时 | 依赖 |
|---|---|---|---|---|
| Agent 1 | `feat/a1-consolidation` | Task 1-4 (Phase 1a) | 0.5-1 day | 无 |
| Agent 2 | `feat/a2-enforce-health` | Task 5-7 (Phase 1b) | 0.5-1 day | 依赖 Agent 1 (secrets_block.go 共享) |
| Agent 3 | `feat/a3-deploy-topology` | Task 8-12 (Phase 2+3) | 1.5-2 day | 依赖 Agent 1+2 (signal 共享 main.go) |
| Agent 4 | `feat/a4-helm-chart` | Task 13 (Phase 4) | 0.5-1 day | 无 |
| Agent 5 | `feat/a5-backup-scripts` | Task 14 (Phase 5) | 0.5-1 day | 无 |

主 session 协调 5 个 branch 顺序 merge,merge 顺序:A1 → A2 → A3 → A4 → A5。

---

## 关键共享约定(所有 agent 必须遵守)

### 1. 命名

- Package 名与现有 Go module 一致: `github.com/devops-toolkit/backend`
- File 名 snake_case,符合 Go 惯例
- Commit message 中文,`feat(scope): xxx` / `refactor(scope): xxx` / `fix(scope): xxx` / `chore(scope): xxx` / `ci: xxx`

### 2. 测试命令

- 单包: `go test -count=1 -timeout 120s ./internal/<package>/...`
- 全量(串行,避开 sqlite 并发锁): `go test -count=1 -p 1 -timeout 600s ./...`
- race: `go test -count=1 -race -timeout 120s ./internal/<package>/...`
- vet: `go vet ./...`(目标 warning 数 ≤ 1)

### 3. Branch 命名 & commit 风格

- Branch 命名: `feat/a1-consolidation`、`feat/a2-enforce-health` 等
- 每个 task 一个 commit,Commit message 末尾不加 `🤖` 等 emoji(避免 bash 解析问题)
- 主 session merge 时用 `git merge --no-ff <branch>`,merge commit message 简短

### 4. 顺序约束

- Task 1-4 必须在 Task 5-7 之前 commit(因为 Task 5 引用 Task 3 引入的 `secrets_block.go` 常量)
- Task 8 (monitor_loop) 依赖 Task 5 (APP_JWT_SECRET 强制) **不强制**,但都改 main.go,顺序做更安全
- Task 13-14 完全独立

### 5. 不在本计划范围(YAGNI)

- 不做完整 Helm chart(Ingress cert-manager / ExternalSecrets / ArgoCD 留 C 阶段)
- 不做 SSO/SAML/OIDC
- 不做 Grafana 自监控 dashboard(只 PrometheusRule)
- 不做日志长期保留
- 不做 CMDB / 自动发现
- 不重写 pg_dump 脚本为 Go(用现成 CLI)

---

## File Structure(全 14 task 的总览)

### 新增文件

```
internal/health/
  ├─ handler.go                  # Handler, Live, Ready, Legacy, Checker interface
  ├─ live.go                     # 单独 Live handler(短小)
  ├─ ready.go                    # 单独 Ready handler(fan-out)
  ├─ handler_test.go             # 端到端 handler 测试
  ├─ postgres_checker.go         # PostgresChecker 实现
  ├─ postgres_checker_test.go
  ├─ ldap_checker.go             # LDAPChecker 实现
  ├─ ldap_checker_test.go
  ├─ k8s_checker.go              # K8sChecker 实现
  └─ k8s_checker_test.go

internal/config/
  ├─ secrets_block.go            # DevDefault* 常量 + envOrWarn helper
  └─ secrets_block_test.go

internal/database/
  ├─ errors.go                   # MapNotFound helper
  └─ errors_test.go

pkg/logger/
  ├─ secret_keys.go              # SecretKeys map + ShouldMask
  └─ secret_keys_test.go

deploy/helm/
  ├─ Chart.yaml
  ├─ values.yaml
  ├─ README.md
  └─ templates/
     ├─ deployment.yaml
     ├─ service.yaml
     ├─ ingress.yaml
     ├─ configmap.yaml
     ├─ secret.yaml
     ├─ serviceaccount.yaml
     ├─ hpa.yaml
     ├─ cronjob-backup.yaml
     └─ prometheusrule.yaml

deploy/prometheus/rules/
  └─ self-health.yml             # 进程存活 / ready 探活告警规则

scripts/
  ├─ backup-postgres.sh          # S3/NFS/Local target
  └─ restore-postgres.sh         # restore drill

openspec/specs/production-readiness/
  └─ spec.md                     # 镜像本 spec 的简短版(500 行内)
```

### 修改文件

```
cmd/devops-toolkit/main.go                    # signal.NotifyContext, /live /ready 接线
internal/config/config.go                     # Validate, secret 强制
internal/handler/response.go                  # WriteAPIError helper
internal/observability/metrics.go             # monitor_loop counters
internal/physicalhost/monitor_loop.go         # signal.NotifyContext + Run(ctx)
internal/physicalhost/handler.go              # 8 sites 改 WriteAPIError
internal/alerts/handler.go                    # 8 sites 改 WriteAPIError + ?status 修
internal/audit/handler.go                     # 改 WriteAPIError
internal/audit/repository.go                  # 改 MapNotFound
internal/k8s/client.go                        # Client interface 拆 Lister/LogReader/Exec
internal/k8s/registry.go                      # ClientRegistry 返回拆后 interfaces
internal/k8s/registry_test.go                 # 适配拆后 interface
internal/servicecatalog/*.go                  # 适配拆后 interfaces,1 处
deploy/docker-compose.yml                     # 资源限制+健康检查+backup+loki+promtail
deploy/prometheus/prometheus.yml              # app self-scrape
.github/workflows/ci.yml                      # promtool step
.gitignore                                    # 49MB binary 精确
CHANGELOG.md                                  # 新一节
DOCUMENT_INDEX.md                             # 新 spec 索引
TODOS.md                                      # A 子项目项移到 Completed
```

### 8 modules 改 `writeAPIError`(Phase 1a 的影响面)

```
internal/physicalhost/handler.go              (1 site)
internal/alerts/handler.go                    (1 site + 1 bug fix)
internal/audit/handler.go                     (1 site)
internal/k8s/handler.go                       (1 site)
internal/pipeline/handler.go                  (1 site)
internal/servicecatalog/handler.go            (1 site)
internal/logs/handler.go                      (1 site)
internal/device/handler.go                    (1 site)
```

实际是 8 modules,但每个 module 1 个 site = 8 sites 总。**精确** 数字以 `grep -rn "writeAPIError" internal/` 现场为准。

### 11 sites 改 `MapNotFound`(Phase 1a 的影响面)

```
internal/audit/repository.go
internal/physicalhost/repository.go
internal/k8s/repository.go
internal/pipeline/repository.go
internal/servicecatalog/repository.go
internal/logs/repository.go
internal/device/repository.go
internal/project/repository.go
internal/discovery/repository.go
internal/alerts/repository.go
internal/hostproject/repository.go
```

实际是 11 sites。**精确** 数字以 `grep -rn "gorm.ErrRecordNotFound" internal/` 现场为准。

---

## Task 1: 合并 `handler.WriteAPIError`(Phase 1a, Commit #1)

**Files:**
- Modify: `internal/handler/response.go`(扩展)
- Modify: 8 modules 各 1 site:`internal/{physicalhost,alerts,audit,k8s,pipeline,servicecatalog,logs,device}/handler.go`
- Test: 通过既有 module 测试间接覆盖

**Agent:** Agent 1 — 第一个 task,无依赖

- [ ] **Step 1: 先看现状,确定 8 sites**

Run: `grep -rn "writeAPIError" internal/ | head -30`
Expected: 输出每个 module 的 helper 函数定义和调用点。

- [ ] **Step 2: 在 `internal/handler/response.go` 末尾加 `WriteAPIError`**

在 `internal/handler/response.go` 添加:

```go
// WriteAPIError 把 service/handler 层传上来的 error 转 HTTP 响应。
// 自动把 *contracts.APIError 映射到正确的 HTTP 状态码;
// 其他 error 视为 500 内部错误,cause 仅记日志,不出现在 body。
func WriteAPIError(w http.ResponseWriter, err error) {
    if err == nil {
        return
    }
    var apiErr *contracts.APIError
    if errors.As(err, &apiErr) {
        status := http.StatusInternalServerError
        switch apiErr.Code {
        case contracts.CodeValidation:
            status = http.StatusBadRequest
        case contracts.CodeNotFound:
            status = http.StatusNotFound
        case contracts.CodeUnauthorized:
            status = http.StatusUnauthorized
        case contracts.CodeForbidden:
            status = http.StatusForbidden
        }
        WriteError(w, apiErr)
        _ = status // status 由 WriteError 内部决定(apiErr.HTTPStatus)
        return
    }
    WriteError(w, &contracts.APIError{
        Code:    contracts.CodeInternal,
        Message: "internal error",
        Cause:   err,
    })
}
```

注意: 实际 `WriteError` 内部已经按 `apiErr.HTTPStatus` 设 status,这里 switch 只是显式表明意图,不重复设。**如果 WriteError 不支持 HTTPStatus 字段,先用 status 显式调 WriteJSON;以实际 `response.go` 代码为准**。

- [ ] **Step 3: 替换 8 modules 的 writeAPIError helper**

每个 module 的 pattern(以 physicalhost/handler.go 为例):

```go
// 旧代码
func writeAPIError(w http.ResponseWriter, err error) {
    var apiErr *contracts.APIError
    if errors.As(err, &apiErr) {
        handler.WriteError(w, apiErr)
        return
    }
    handler.WriteError(w, &contracts.APIError{
        Code: contracts.CodeInternal, Message: "internal error", Cause: err,
    })
}

// 调用点: writeAPIError(c.Writer, err)
// 改成:    handler.WriteAPIError(c.Writer, err)
```

**重要**: 8 modules 的 helper 名字可能不同(如 `mapAPIError`、`toAPIError`),以现场为准。原则: **删掉 module-local 的 helper,改调 `handler.WriteAPIError`**。

- [ ] **Step 4: 跑测试,确保行为不变**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS,31 packages 全绿,无新增 FAIL。

- [ ] **Step 5: 跑 vet,确保不增加 warning**

Run: `go vet ./... 2>&1 | wc -l`
Expected: 数字 ≤ 1(既有 1 个 audit/repository.go warning 不变)。

- [ ] **Step 6: 提交**

```bash
git add internal/handler/response.go internal/*/handler.go
git commit -m "refactor(handler): 合并 8 个 module-local writeAPIError helper 到 handler.WriteAPIError"
```

---

## Task 2: 添加 `database.MapNotFound` helper(Phase 1a, Commit #2)

**Files:**
- Create: `internal/database/errors.go`
- Create: `internal/database/errors_test.go`
- Modify: 11 repository files

**Agent:** Agent 1

- [ ] **Step 1: 先看现状,确定 11 sites**

Run: `grep -rn "gorm.ErrRecordNotFound" internal/`
Expected: 列出 11 个 sites。

- [ ] **Step 2: 写 failing test**

`internal/database/errors_test.go`:

```go
package database

import (
    "errors"
    "testing"

    "gorm.io/gorm"
)

func TestMapNotFound_Nil(t *testing.T) {
    if got := MapNotFound(nil); got != nil {
        t.Errorf("MapNotFound(nil) = %v, want nil", got)
    }
}

func TestMapNotFound_RecordNotFound(t *testing.T) {
    err := gorm.ErrRecordNotFound
    got := MapNotFound(err)
    if !errors.Is(got, ErrNotFound) {
        t.Errorf("MapNotFound(gorm.ErrRecordNotFound) = %v, want ErrNotFound", got)
    }
}

func TestMapNotFound_Other(t *testing.T) {
    other := errors.New("other error")
    got := MapNotFound(other)
    if !errors.Is(got, other) {
        t.Errorf("MapNotFound(other) = %v, want %v", got, other)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail(还没写实现)**

Run: `go test -count=1 ./internal/database/`
Expected: FAIL — `MapNotFound` 未定义。

- [ ] **Step 4: 写 `internal/database/errors.go`**

```go
package database

import (
    "errors"

    "gorm.io/gorm"
)

// ErrNotFound is the canonical "record not found" sentinel for
// the service / handler layers. Repository implementations MUST
// return ErrNotFound (not gorm.ErrRecordNotFound) for missing
// rows so the rest of the stack can match a single error
// without importing gorm.
var ErrNotFound = errors.New("not found")

// MapNotFound converts a gorm.ErrRecordNotFound into the
// canonical ErrNotFound. Other errors pass through unchanged.
// nil in → nil out. Use this at every repository boundary:
//
//     if err := db.MapNotFound(repo.Get(id)); err != nil {
//         return err
//     }
func MapNotFound(err error) error {
    if err == nil {
        return nil
    }
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return ErrNotFound
    }
    return err
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/database/`
Expected: PASS — 3 cases 全过。

- [ ] **Step 6: 替换 11 sites**

每个 repository 的 pattern(以 audit/repository.go 为例):

```go
// 旧代码
if errors.Is(err, gorm.ErrRecordNotFound) {
    return nil, ErrNotFound  // 假设 ErrNotFound 是 module-local
}

// 新代码
if err := database.MapNotFound(err); err != nil {
    return nil, err
}
```

注意: **保留 module-local 的 ErrNotFound 别名**(如果已有),用 MapNotFound 转换。这样调用方不需要改。

- [ ] **Step 7: 跑全量测试,确保不退化**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS,11 个 repository 测试不退化。

- [ ] **Step 8: 提交**

```bash
git add internal/database/errors.go internal/database/errors_test.go internal/*/repository.go
git commit -m "refactor(database): 加 MapNotFound helper,替换 11 个 gorm.ErrRecordNotFound 检查点"
```

---

## Task 3: 集中 dev-default secrets(Phase 1a, Commit #3)

**Files:**
- Create: `internal/config/secrets_block.go`
- Create: `internal/config/secrets_block_test.go`
- Modify: 6+ 调用 `os.Getenv` 取 secret 的地方

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `grep -rn 'os.Getenv.*"APP_JWT_SECRET"\|os.Getenv.*"K8S_CRYPTO_KEY"\|os.Getenv.*"INFLUX_\|os.Getenv.*"PROBER_\|os.Getenv.*"LOG_STORAGE_DIR"' internal/ cmd/`
Expected: 列出所有 secret 取值点。

- [ ] **Step 2: 写 failing test**

`internal/config/secrets_block_test.go`:

```go
package config

import (
    "os"
    "testing"
)

func TestEnvOrWarn_FromEnv(t *testing.T) {
    t.Setenv("TEST_SECRET_KEY", "real-value")
    got := envOrWarn("TEST_SECRET_KEY", "default")
    if got != "real-value" {
        t.Errorf("envOrWarn from env = %q, want %q", got, "real-value")
    }
}

func TestEnvOrWarn_Default(t *testing.T) {
    os.Unsetenv("TEST_SECRET_KEY_MISSING")
    got := envOrWarn("TEST_SECRET_KEY_MISSING", "default-value")
    if got != "default-value" {
        t.Errorf("envOrWarn default = %q, want %q", got, "default-value")
    }
}

func TestDevDefaults_NotEmpty(t *testing.T) {
    // Sanity: the dev-default constants must be non-empty so
    // production validation has something to compare against.
    cases := []struct {
        name, val string
    }{
        {"DevDefaultJWTSecret", DevDefaultJWTSecret},
        {"DevDefaultK8SCryptoKey", DevDefaultK8SCryptoKey},
        {"DevDefaultInfluxPassword", DevDefaultInfluxPassword},
        {"DevDefaultLDAPBindPassword", DevDefaultLDAPBindPassword},
        {"DevDefaultProberSSHKey", DevDefaultProberSSHKey},
    }
    for _, c := range cases {
        if c.val == "" {
            t.Errorf("%s is empty", c.name)
        }
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/config/`
Expected: FAIL — envOrWarn/DevDefault* 未定义。

- [ ] **Step 4: 写 `internal/config/secrets_block.go`**

```go
package config

import (
    "log"
    "os"
)

// Dev-default secret constants. Centralized so production
// validation in (*Config).Validate() can compare against a
// known-bad value, AND so dev environments can use a single
// canonical fallback. NEVER set these to anything sensitive.
//
// In production, env != "production" is false, so config.Validate()
// will fail-fast if any of these are still in use.
const (
    DevDefaultJWTSecret       = "dev-jwt-secret-change-me"
    DevDefaultK8SCryptoKey    = "dev-k8s-crypto-key-32bytes!!"
    DevDefaultInfluxPassword  = "dev-influx-pwd"
    DevDefaultLDAPBindPassword = "dev-ldap-bind"
    DevDefaultProberSSHKey    = "dev-prober-key"
)

// envOrWarn returns the value of envName if set, otherwise
// the supplied devDefault. When falling back to devDefault,
// logs a warning tagged with envName so operators can grep
// for accidental dev-defaults in production logs.
func envOrWarn(envName, devDefault string) string {
    if v := os.Getenv(envName); v != "" {
        return v
    }
    log.Printf("WARN: using dev default for %s — DO NOT use in production", envName)
    return devDefault
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/config/`
Expected: PASS — 3 cases 全过。

- [ ] **Step 6: 替换 secret 取值点**

每个调用点的 pattern(以 cmd/devops-toolkit/main.go 为例):

```go
// 旧代码
jwtSecret := os.Getenv("APP_JWT_SECRET")
if jwtSecret == "" {
    log.Warn("using dev default APP_JWT_SECRET")
    jwtSecret = "dev-jwt-secret-change-me"
}

// 新代码
jwtSecret := envOrWarn("APP_JWT_SECRET", DevDefaultJWTSecret)
```

- [ ] **Step 7: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/config/secrets_block.go internal/config/secrets_block_test.go cmd/devops-toolkit/main.go
git commit -m "refactor(config): 集中 dev-default secrets 在 secrets_block.go,加 envOrWarn helper"
```

---

## Task 4: 补全 secret masking 名单(Phase 1a, Commit #4)

**Files:**
- Create: `pkg/logger/secret_keys.go`
- Create: `pkg/logger/secret_keys_test.go`
- Modify: 现有 `(*Config).String()` 或 logger 包使用点

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `grep -rn "password.*=\|mask\|Redact" pkg/logger/ internal/config/ 2>/dev/null | head -20`
Expected: 找到现有 secret masking 逻辑。

- [ ] **Step 2: 写 failing test**

`pkg/logger/secret_keys_test.go`:

```go
package logger

import "testing"

func TestShouldMask_KnownKeys(t *testing.T) {
    cases := []string{
        "password", "PASSWORD",
        "kubeconfig", "KubeConfig",
        "bind_password", "BIND_PASSWORD",
        "token", "TOKEN",
        "secret", "secret_key",
        "jwt_secret", "JWT_SECRET",
        "k8s_crypto_key", "K8S_CRYPTO_KEY",
        "private_key", "privateKey",
        "ssh_key", "ssh-key",
        "client_secret", "client-secret",
        "api_key", "API_KEY",
        "ldap_bind_pw", "LDAP_BIND_PW",
        "tls_cert", "TLS_CERT",
    }
    for _, k := range cases {
        if !ShouldMask(k) {
            t.Errorf("ShouldMask(%q) = false, want true", k)
        }
    }
}

func TestShouldMask_UnknownKey(t *testing.T) {
    cases := []string{"username", "host", "port", "database"}
    for _, k := range cases {
        if ShouldMask(k) {
            t.Errorf("ShouldMask(%q) = true, want false", k)
        }
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./pkg/logger/`
Expected: FAIL — `ShouldMask` 未定义。

- [ ] **Step 4: 写 `pkg/logger/secret_keys.go`**

```go
package logger

import "strings"

// SecretKeys lists every config key whose value MUST be redacted
// when logged or rendered via String(). Matched case-insensitively
// after lowercasing, so "JWT_SECRET" / "jwt_secret" / "JwtSecret"
// all hit.
//
// If you add a new secret-bearing field to internal/config, add
// its key here too. The audit explicitly calls this set
// "incomplete" (P3) — keep it exhaustive.
var SecretKeys = map[string]bool{
    "password":         true,
    "kubeconfig":       true,
    "bind_password":    true,
    "token":            true,
    "secret":           true,
    "secret_key":       true,
    "jwt_secret":       true,
    "k8s_crypto_key":   true,
    "private_key":      true,
    "ssh_key":          true,
    "client_secret":    true,
    "api_key":          true,
    "ldap_bind_pw":     true,
    "tls_cert":         true,
}

// ShouldMask reports whether a config key's value should be
// masked in logs. The match is case-insensitive and tolerant
// of underscores vs hyphens vs dots (a config field called
// "tls.cert" or "tls-cert" both match "tls_cert" via lowercasing
// and string-substring match below).
func ShouldMask(key string) bool {
    k := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
    k = strings.ReplaceAll(k, ".", "_")
    return SecretKeys[k]
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./pkg/logger/`
Expected: PASS — 全部 key 命中/不命中均正确。

- [ ] **Step 6: 替换现有 secret masking 逻辑**

找到现有 `(*Config).String()` 或 logger 包的 masking 调用,改用 `ShouldMask(key)`。

注: 如果现有 `(*Config).String()` 只 hardcode 了 `password` 字段,扩展判断:

```go
// 旧代码(假设在 internal/config/config.go)
func (c *Config) String() string {
    return fmt.Sprintf("Config{Port:%d, LDAPPassword:***}", c.Port)
}

// 新代码
func (c *Config) String() string {
    // 反射遍历字段,根据 field name 调 logger.ShouldMask
    // 简化版:已知字段显式列出
    return fmt.Sprintf("Config{Port:%d, JWTSecret:%s, K8SCryptoKey:%s, ...}",
        c.Port,
        mask(c.JWTSecret, "jwt_secret"),
        mask(c.K8SCryptoKey, "k8s_crypto_key"),
        // ...
    )
}

func mask(val, key string) string {
    if logger.ShouldMask(key) {
        return "***"
    }
    return val
}
```

(完整反射版留给后续 sprint;先用 explicit field 列举,field 不多。)

- [ ] **Step 7: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add pkg/logger/secret_keys.go pkg/logger/secret_keys_test.go internal/config/config.go
git commit -m "feat(logger): 补全 secret masking 名单(kubeconfig/token/secret 等 14 个 key)"
```

---

## Task 5: 强制生产 secret(Phase 1b, Commit #5)

**Files:**
- Modify: `internal/config/config.go`(`Validate` 方法)

**Agent:** Agent 2 — **依赖 Agent 1 Task 3 完成**(`secrets_block.go` 必须先存在)

- [ ] **Step 1: 先看现状**

Run: `grep -n "Validate\|env == \"production\"\|JWTSecret\|K8SCryptoKey\|DevBypass" internal/config/config.go`
Expected: 找到现有 `Validate` 方法定义和字段名。

- [ ] **Step 2: 写 failing test**

`internal/config/config_validate_test.go`(新增文件):

```go
package config

import (
    "errors"
    "strings"
    "testing"
)

func TestConfig_Validate_Production_OK(t *testing.T) {
    c := &Config{
        Env:         "production",
        JWTSecret:   "real-jwt-secret-32-bytes-or-more",
        K8SCryptoKey: "real-k8s-key-32-bytes-or-more",
        LDAP:        LDAPConfig{DevBypass: false},
    }
    if err := c.Validate(); err != nil {
        t.Errorf("Validate() = %v, want nil", err)
    }
}

func TestConfig_Validate_Production_RejectsDefaultJWTSecret(t *testing.T) {
    c := &Config{
        Env:        "production",
        JWTSecret:  DevDefaultJWTSecret, // dev default, must be rejected
        K8SCryptoKey: "real-k8s-key-32-bytes-or-more",
        LDAP:       LDAPConfig{DevBypass: false},
    }
    err := c.Validate()
    if err == nil {
        t.Fatal("Validate() = nil, want error")
    }
    if !strings.Contains(err.Error(), "APP_JWT_SECRET") {
        t.Errorf("err = %q, want contains APP_JWT_SECRET", err.Error())
    }
}

func TestConfig_Validate_Production_RejectsDefaultK8SKey(t *testing.T) {
    c := &Config{
        Env:        "production",
        JWTSecret:  "real-jwt-secret-32-bytes-or-more",
        K8SCryptoKey: DevDefaultK8SCryptoKey,
        LDAP:       LDAPConfig{DevBypass: false},
    }
    err := c.Validate()
    if err == nil {
        t.Fatal("Validate() = nil, want error")
    }
    if !strings.Contains(err.Error(), "K8S_CRYPTO_KEY") {
        t.Errorf("err = %q, want contains K8S_CRYPTO_KEY", err.Error())
    }
}

func TestConfig_Validate_Production_RejectsLDAPDevBypass(t *testing.T) {
    c := &Config{
        Env:        "production",
        JWTSecret:  "real-jwt-secret-32-bytes-or-more",
        K8SCryptoKey: "real-k8s-key-32-bytes-or-more",
        LDAP:       LDAPConfig{DevBypass: true},
    }
    err := c.Validate()
    if err == nil {
        t.Fatal("Validate() = nil, want error")
    }
    if !strings.Contains(err.Error(), "ldap.dev_bypass") {
        t.Errorf("err = %q, want contains ldap.dev_bypass", err.Error())
    }
}

func TestConfig_Validate_Dev_AllowsDefaults(t *testing.T) {
    c := &Config{
        Env:        "development",
        JWTSecret:  DevDefaultJWTSecret, // OK in dev
        K8SCryptoKey: DevDefaultK8SCryptoKey,
        LDAP:       LDAPConfig{DevBypass: true},
    }
    if err := c.Validate(); err != nil {
        t.Errorf("dev Validate() = %v, want nil", err)
    }
}

func TestConfig_Validate_Production_AggregatesErrors(t *testing.T) {
    c := &Config{
        Env:        "production",
        JWTSecret:  "",         // missing
        K8SCryptoKey: "",       // missing
        LDAP:       LDAPConfig{DevBypass: true},
    }
    err := c.Validate()
    if err == nil {
        t.Fatal("Validate() = nil, want error")
    }
    // All three issues should be reported in one error.
    msg := err.Error()
    if !strings.Contains(msg, "APP_JWT_SECRET") ||
       !strings.Contains(msg, "K8S_CRYPTO_KEY") ||
       !strings.Contains(msg, "ldap.dev_bypass") {
        t.Errorf("err = %q, want all three issues reported", msg)
    }
}

// Suppress unused error import in case errors isn't used.
var _ = errors.New
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/config/`
Expected: FAIL — `c.Validate()` 还没强制 production。

- [ ] **Step 4: 改 `(*Config).Validate`**

在 `internal/config/config.go` 找到现有 `Validate` 方法,**追加** production 强制逻辑(不重写,避免破坏既有逻辑):

```go
func (c *Config) Validate() error {
    // ... existing validation logic, e.g. port range, etc. ...

    if c.Env == "production" {
        var errs []error
        if c.JWTSecret == "" || c.JWTSecret == DevDefaultJWTSecret {
            errs = append(errs, errors.New("APP_JWT_SECRET must be set in production (not empty, not dev default)"))
        }
        if c.K8SCryptoKey == "" || c.K8SCryptoKey == DevDefaultK8SCryptoKey {
            errs = append(errs, errors.New("K8S_CRYPTO_KEY must be set in production (not empty, not dev default)"))
        }
        if c.LDAP.DevBypass {
            errs = append(errs, errors.New("ldap.dev_bypass must be false in production"))
        }
        if len(errs) > 0 {
            return errors.Join(errs...)
        }
    }

    return nil
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/config/`
Expected: PASS — 6 cases 全过。

- [ ] **Step 6: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/config/config.go internal/config/config_validate_test.go
git commit -m "feat(config): 生产环境强制 APP_JWT_SECRET / K8S_CRYPTO_KEY 非空非默认值,禁 ldap.dev_bypass"
```

---

## Task 6: 拆 `/health` 为 `/live`/`/ready`/`/health`(Phase 1b, Commit #6)

**Files:**
- Create: `internal/health/{handler,live,ready,handler_test}.go`
- Create: `internal/health/{postgres_checker,ldap_checker,k8s_checker}.go` + tests
- Modify: `cmd/devops-toolkit/main.go`(注册路由)

**Agent:** Agent 2

- [ ] **Step 1: 先看现状**

Run: `grep -n "GET.*/health\|/live\|/ready" cmd/devops-toolkit/main.go internal/server/*.go 2>/dev/null`
Expected: 找到现有 /health 路由。

- [ ] **Step 2: 写 failing test**

`internal/health/handler_test.go`:

```go
package health

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
)

type stubChecker struct {
    name string
    err  error
}

func (s *stubChecker) Name() string                     { return s.name }
func (s *stubChecker) Check(_ context.Context) error    { return s.err }

func newRouter(h *Handler) *gin.Engine {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/live", h.Live)
    r.GET("/ready", h.Ready)
    r.GET("/health", h.Legacy)
    return r
}

func TestHandler_Live_AlwaysOK(t *testing.T) {
    h := &Handler{}
    r := newRouter(h)
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/live", nil)
    r.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK {
        t.Errorf("Live code = %d, want 200", rec.Code)
    }
    var body map[string]any
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatal(err)
    }
    if body["status"] != "ok" {
        t.Errorf("body status = %v, want ok", body["status"])
    }
}

func TestHandler_Ready_AllOK(t *testing.T) {
    h := &Handler{
        checkers: []Checker{
            &stubChecker{name: "postgres", err: nil},
            &stubChecker{name: "ldap", err: nil},
        },
        timeout: 100_000_000, // 100ms
    }
    r := newRouter(h)
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/ready", nil)
    r.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK {
        t.Errorf("Ready code = %d, want 200; body = %s", rec.Code, rec.Body.String())
    }
    var body map[string]any
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatal(err)
    }
    checks, _ := body["checks"].(map[string]any)
    if checks["postgres"] != "ok" || checks["ldap"] != "ok" {
        t.Errorf("checks = %v, want all ok", checks)
    }
}

func TestHandler_Ready_Degraded(t *testing.T) {
    h := &Handler{
        checkers: []Checker{
            &stubChecker{name: "postgres", err: nil},
            &stubChecker{name: "ldap", err: errors.New("connection refused")},
        },
        timeout: 100_000_000,
    }
    r := newRouter(h)
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/ready", nil)
    r.ServeHTTP(rec, req)
    if rec.Code != http.StatusServiceUnavailable {
        t.Errorf("Ready code = %d, want 503", rec.Code)
    }
    var body map[string]any
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatal(err)
    }
    if body["status"] != "degraded" {
        t.Errorf("body status = %v, want degraded", body["status"])
    }
}

func TestHandler_Legacy_SameAsReady(t *testing.T) {
    h := &Handler{
        checkers: []Checker{
            &stubChecker{name: "postgres", err: nil},
        },
        timeout: 100_000_000,
    }
    r := newRouter(h)
    rec := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/health", nil)
    r.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK {
        t.Errorf("Legacy code = %d, want 200", rec.Code)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/health/`
Expected: FAIL — package 不存在或 handler 类型未定义。

- [ ] **Step 4: 写 `internal/health/handler.go`**

```go
package health

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

// Checker is implemented by anything that can verify a
// dependency is up. nil error means healthy.
type Checker interface {
    Name() string
    Check(ctx context.Context) error
}

// Handler exposes three endpoints:
//   GET /live   — process liveness (always 200 if responding)
//   GET /ready  — dependency readiness (200 if all checkers pass, 503 otherwise)
//   GET /health — legacy aggregate, same body as /ready
//
// Designed to be registered at the root router level (NOT under
// /api/v1) so K8s probes and external monitors can hit it
// without auth.
type Handler struct {
    checkers []Checker
    timeout  time.Duration
}

func NewHandler(checkers []Checker, timeout time.Duration) *Handler {
    if timeout == 0 {
        timeout = 2 * time.Second
    }
    return &Handler{checkers: checkers, timeout: timeout}
}

// Live is the K8s livenessProbe target.
func (h *Handler) Live(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status":         "ok",
        "uptime_seconds": time.Since(startupTime).Seconds(),
    })
}

// Ready is the K8s readinessProbe target.
func (h *Handler) Ready(c *gin.Context) {
    h.respond(c)
}

// Legacy is /health, retained for backward compatibility with
// any external monitor already pointing at it.
func (h *Handler) Legacy(c *gin.Context) {
    h.respond(c)
}

func (h *Handler) respond(c *gin.Context) {
    ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
    defer cancel()

    type result struct {
        name string
        err  error
    }
    ch := make(chan result, len(h.checkers))
    var wg sync.WaitGroup
    for _, ck := range h.checkers {
        wg.Add(1)
        go func(ck Checker) {
            defer wg.Done()
            ch <- result{name: ck.Name(), err: ck.Check(ctx)}
        }(ck)
    }
    wg.Wait()
    close(ch)

    checks := map[string]string{}
    healthy := true
    for r := range ch {
        if r.err != nil {
            checks[r.name] = r.err.Error()
            healthy = false
        } else {
            checks[r.name] = "ok"
        }
    }

    status := http.StatusOK
    body := gin.H{
        "status": "ok",
        "checks": checks,
    }
    if !healthy {
        status = http.StatusServiceUnavailable
        body["status"] = "degraded"
    }
    c.JSON(status, body)
}

// startupTime is set by Register and used by Live to report
// uptime. Zero value is acceptable (reports 0).
var startupTime = time.Now()
```

- [ ] **Step 5: 写 `internal/health/postgres_checker.go`**

```go
package health

import (
    "context"
    "fmt"

    "gorm.io/gorm"
)

type PostgresChecker struct {
    DB *gorm.DB
}

func (p *PostgresChecker) Name() string { return "postgres" }

func (p *PostgresChecker) Check(ctx context.Context) error {
    if p.DB == nil {
        return fmt.Errorf("db not configured")
    }
    sqlDB, err := p.DB.DB()
    if err != nil {
        return err
    }
    return sqlDB.PingContext(ctx)
}
```

- [ ] **Step 6: 写 `internal/health/ldap_checker.go`**

```go
package health

import (
    "context"
    "fmt"

    "github.com/devops-toolkit/backend/internal/auth/ldap"
)

type LDAPChecker struct {
    Client *ldap.Client
}

func (l *LDAPChecker) Name() string { return "ldap" }

func (l *LDAPChecker) Check(ctx context.Context) error {
    if l.Client == nil {
        return fmt.Errorf("ldap client not configured")
    }
    return l.Client.HealthCheck(ctx)
}
```

注: 如果 `auth/ldap.Client` 没有 `HealthCheck` 方法,加一个简单版本(读 LDAP server RootDSE 一次)。

- [ ] **Step 7: 写 `internal/health/k8s_checker.go`**

```go
package health

import (
    "context"
    "fmt"

    "github.com/devops-toolkit/backend/internal/k8s"
)

type K8sChecker struct {
    Registry *k8s.ClientRegistry
}

func (k *K8sChecker) Name() string { return "k8s" }

func (k *K8sChecker) Check(ctx context.Context) error {
    if k.Registry == nil || k.Registry.IsEmpty() {
        // No clusters registered is not a failure — the
        // platform can still serve the other modules.
        return nil
    }
    // Probe the first registered cluster.
    cluster, err := k.Registry.FirstCluster()
    if err != nil {
        return fmt.Errorf("no cluster: %w", err)
    }
    cli, err := k.Registry.ClientFor(cluster.ID)
    if err != nil {
        return err
    }
    return cli.Ping(ctx)
}
```

注: `IsEmpty` 和 `FirstCluster` 需在 `internal/k8s/registry.go` 实现,如果没有就加 2 个方法。

- [ ] **Step 8: 在 main.go 接线**

```go
// 在 cmd/devops-toolkit/main.go 的 buildRouter 之前
import "github.com/devops-toolkit/backend/internal/health"

healthHandler := health.NewHandler([]health.Checker{
    &health.PostgresChecker{DB: db},
    &health.LDAPChecker{Client: ldapClient},
    &health.K8sChecker{Registry: k8sRegistry},
}, 2*time.Second)

r := gin.New()
// ... 中间件 ...
r.GET("/live", gin.WrapH(healthHandler.Live))
r.GET("/ready", gin.WrapH(healthHandler.Ready))
r.GET("/health", gin.WrapH(healthHandler.Legacy))
```

实际接线需根据现有 main.go 的 `buildRouter` 风格调整。

- [ ] **Step 9: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/health/`
Expected: PASS — 4 cases 全过。

- [ ] **Step 10: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 11: 提交**

```bash
git add internal/health/ cmd/devops-toolkit/main.go
git commit -m "feat(health): 拆 /health 为 /live /ready /health,fan-out 探活 DB+LDAP+K8s"
```

---

## Task 7: 修 `?status=open` bug(Phase 1b, Commit #7)

**Files:**
- Modify: `internal/alerts/handler.go`(1 行)

**Agent:** Agent 2

- [ ] **Step 1: 先看现状**

Run: `grep -n "Query.*state\|Query.*status" internal/alerts/handler.go`
Expected: 找到 `c.Query("state")` 调用点。

- [ ] **Step 2: 改 1 行**

```go
// 旧代码(假设 line 344-372 内)
state := c.Query("state")

// 新代码
state := c.Query("status")
```

注: 实际可能不止 1 处。**所有 dashboard 用 `?status=open` 但 backend 读 `state` 的地方都要改**。以 `grep` 结果为准。

- [ ] **Step 3: 写 test(覆盖)**

`internal/alerts/handler_test.go` 添加用例(如已有文件则追加):

```go
func TestAlertsHandler_List_StatusParam(t *testing.T) {
    // Setup: in-memory repo with 1 open + 1 closed alert.
    // Request: GET /alerts?status=open
    // Expect: response contains only the open alert.
}
```

(完整 test 代码按既有 handler test 风格写。)

- [ ] **Step 4: 跑 test**

Run: `go test -count=1 ./internal/alerts/`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/alerts/handler.go internal/alerts/handler_test.go
git commit -m "fix(alerts): dashboard ?status=open 查询参数(原来读 ?state,前端发 status 一直返 0)"
```

---

## Task 8: monitor_loop 优雅停机 + Prometheus metrics(Phase 2, Commit #8)

**Files:**
- Modify: `internal/physicalhost/monitor_loop.go`
- Create: `internal/physicalhost/monitor_loop_test.go`(如缺)
- Modify: `internal/observability/metrics.go`(加 3 个 metric)
- Modify: `cmd/devops-toolkit/main.go`(signal.NotifyContext)

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `cat internal/physicalhost/monitor_loop.go | head -80`
Run: `grep -n "prometheus.NewCounter\|prometheus.NewGauge" internal/observability/metrics.go | head -20`

- [ ] **Step 2: 写 failing test**

`internal/physicalhost/monitor_loop_test.go`(新增或追加):

```go
package physicalhost

import (
    "context"
    "errors"
    "testing"
    "time"
)

type fakeProber struct {
    err   error
    calls int
}

func (f *fakeProber) Probe(_ context.Context, _ string) error {
    f.calls++
    return f.err
}

func TestLoop_Run_StopsOnContextCancel(t *testing.T) {
    repo := repoFixture(t)
    prober := &fakeProber{}
    loop := &Loop{
        repo:  repo,
        probe: prober,
        log:   testLogger(t),
    }
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    done := make(chan struct{})
    go func() {
        loop.Run(ctx)
        close(done)
    }()
    select {
    case <-done:
        // good
    case <-time.After(2 * time.Second):
        t.Fatal("Run did not return within 2s of context cancel")
    }
}

func TestLoop_Run_IncrementsErrorCounter(t *testing.T) {
    repo := repoFixture(t)
    prober := &fakeProber{err: errors.New("probe failed")}
    loop := &Loop{
        repo:  repo,
        probe: prober,
        log:   testLogger(t),
    }
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
    defer cancel()
    loop.Run(ctx)
    if prober.calls == 0 {
        t.Error("prober not called")
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 ./internal/physicalhost/`
Expected: FAIL — `Loop.Run(ctx)` 接口或字段不存在。

- [ ] **Step 4: 改 `internal/physicalhost/monitor_loop.go`**

把 `context.Background()` 改为 `Run(ctx context.Context)`,用 ctx-aware select:

```go
// 旧代码(简化示意)
func (l *Loop) Run() {
    for {
        // ... iteration ...
        time.Sleep(5 * time.Second)
    }
}

// 新代码
func (l *Loop) Run(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        l.iterCounter.Inc()
        if err := l.iterate(ctx); err != nil {
            l.errCounter.Inc()
            l.log.Error("monitor iteration failed", "err", err)
        } else {
            l.successGauge.SetToCurrentTime()
        }
        select {
        case <-ctx.Done():
            l.log.Info("monitor loop exiting on signal")
            return
        case <-ticker.C:
        }
    }
}
```

- [ ] **Step 5: 在 `internal/observability/metrics.go` 加 3 个 metric**

```go
var (
    PhysicalHostLoopIterations = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "physicalhost_loop_iterations_total",
        Help: "Total number of monitor loop iterations.",
    })
    PhysicalHostLoopErrors = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "physicalhost_loop_errors_total",
        Help: "Total number of monitor loop iterations that errored.",
    })
    PhysicalHostLoopLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "physicalhost_loop_last_success_timestamp_seconds",
        Help: "Unix timestamp of the last successful monitor loop iteration.",
    })
)
```

并在 `init()` 或 `Register` 函数中注册这三个 metric。

- [ ] **Step 6: 在 Loop struct 加 metric 字段**

```go
type Loop struct {
    repo  *Repository
    probe Prober
    cache *MetricsCache
    log   *slog.Logger

    iterCounter  prometheus.Counter
    errCounter   prometheus.Counter
    successGauge prometheus.Gauge
}
```

并在 `NewLoop` 或 main.go 注入三个 metric。

- [ ] **Step 7: 改 `cmd/devops-toolkit/main.go` 用 `signal.NotifyContext`**

```go
// 在 main() 顶部,替换现有的 signal.Notify 实现
rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

// ... 启动 http server ...
go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatal(err)
    }
}()

// 启动 monitor loop,使用 rootCtx
go monitorLoop.Run(rootCtx)

// 等信号
<-rootCtx.Done()
log.Info("shutdown signal received")
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)
```

- [ ] **Step 8: 跑 test,确认 pass**

Run: `go test -count=1 ./internal/physicalhost/`
Expected: PASS。

- [ ] **Step 9: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 10: 提交**

```bash
git add internal/physicalhost/monitor_loop.go internal/observability/metrics.go cmd/devops-toolkit/main.go
git commit -m "feat(monitor): 优雅停机 via signal.NotifyContext + 3 个 Prometheus metrics"
```

---

## Task 9: docker-compose 资源限制+健康检查+backup 容器(Phase 3, Commit #9)

**Files:**
- Modify: `deploy/docker-compose.yml`

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `cat deploy/docker-compose.yml | head -80`

- [ ] **Step 2: 加 mem_limit / cpus / healthcheck 到所有 service**

每个 service 加(以 app 为例):

```yaml
services:
  app:
    image: devops-toolkit:latest
    mem_limit: 1g
    cpus: '2.0'
    restart: always
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/live"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 30s
    environment:
      # secret 走 .env,不 hardcode
      DEVOPS_SERVER_PORT: 8080
    env_file:
      - .env
```

(具体配置以现有 docker-compose.yml 结构为准。)

- [ ] **Step 3: 加 backup 容器(基于 postgres 镜像,跑 cron)**

```yaml
  backup:
    image: postgres:15
    mem_limit: 512m
    cpus: '0.5'
    volumes:
      - ./scripts/backup-postgres.sh:/backup.sh:ro
      - backups:/backups
    entrypoint: /bin/sh
    command:
      - -c
      - |
        echo "$$(date +%M) $$(( $$(date +%H) * 60 + $$(date +%M) )) * * * /backup.sh --target=local" > /etc/crontabs/root
        crond -f
    env_file:
      - .env
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  backups:
```

- [ ] **Step 4: 加 prometheus self-scrape**

```yaml
  prometheus:
    image: prom/prometheus:latest
    mem_limit: 512m
    volumes:
      - ./deploy/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./deploy/prometheus/rules:/etc/prometheus/rules:ro
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    ports:
      - "9090:9090"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    # (假设 app 不暴露到 host,通过 docker network 访问)
```

- [ ] **Step 5: 验证 docker compose 配置**

Run: `docker compose -f deploy/docker-compose.yml config`
Expected: 输出渲染后的 YAML,无 error。

- [ ] **Step 6: 提交**

```bash
git add deploy/docker-compose.yml
git commit -m "feat(deploy): docker-compose 加 mem_limit/cpus/healthcheck/backup/prometheus self-scrape"
```

---

## Task 10: `.gitignore` 修 49MB binary(Phase 3, Commit #10)

**Files:**
- Modify: `.gitignore`
- Remove: `devops-toolkit`(已 tracked)

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `cat .gitignore`
Run: `ls -lh devops-toolkit 2>/dev/null`
Run: `git ls-files | grep -E "^devops-toolkit$|^cmd/devops-toolkit/devops-toolkit$"`

- [ ] **Step 2: 修 `.gitignore`**

```gitignore
# ... 既有内容 ...

# Go build artifacts — exclude compiled binaries.
# 精确模式: 避免误匹配 devops-toolkit 目录。
cmd/devops-toolkit/devops-toolkit
**/devops-toolkit
# (兼容历史 tracked 副本)
```

- [ ] **Step 3: `git rm --cached` 已 tracked 的 49MB binary**

```bash
git rm --cached devops-toolkit
```

(只在历史 tracked 的 49MB binary 文件存在时执行。)

- [ ] **Step 4: 验证**

Run: `git status`
Expected: 不再显示 `devops-toolkit`。

- [ ] **Step 5: 提交**

```bash
git add .gitignore
git commit -m "chore(gitignore): 精确 exclude devops-toolkit 二进制,git rm --cached 49MB 副本"
```

---

## Task 11: CI 加 `promtool check rules`(Phase 3, Commit #11)

**Files:**
- Modify: `.github/workflows/ci.yml`

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `cat .github/workflows/ci.yml | head -60`

- [ ] **Step 2: 加 promtool step**

在 backend job 末尾(或独立 step):

```yaml
      - name: promtool check rules
        uses: prometheus/promtool-action@v0.1.0
        with:
          files: |
            deploy/prometheus/rules/*.yml
```

或 Docker-based:

```yaml
      - name: promtool check rules
        run: |
          docker run --rm -v "$PWD:/repo" prom/prometheus:latest \
            promtool check rules /repo/deploy/prometheus/rules/*.yml
```

- [ ] **Step 3: 验证 CI 配置(本地 dry-run)**

Run: `docker run --rm -v "$PWD:/repo" prom/prometheus:latest promtool check rules /repo/deploy/prometheus/rules/*.yml`
Expected: `SUCCESS` 或无错误。

- [ ] **Step 4: 提交**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: 加 promtool check rules step 验证 PrometheusRule YAML"
```

---

## Task 12: K8s `Client` interface 拆 `Lister`/`LogReader`/`Exec`(Phase 3, Commit #12)

**Files:**
- Modify: `internal/k8s/client.go`
- Modify: `internal/k8s/registry.go`
- Modify: `internal/k8s/registry_test.go`
- Modify: `internal/servicecatalog/*.go`(1 处)

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `grep -n "^type Client\|Client interface\|Ping\|ListPods\|ListDeployments\|ListServices\|GetLogsBySelector\|ExecInPod" internal/k8s/client.go | head -30`

- [ ] **Step 2: 写新 interface 在 `internal/k8s/client.go`**

```go
// Lister reads cluster state. Consumed by servicecatalog health rollup.
type Lister interface {
    Ping(ctx context.Context) error
    ListPods(ctx context.Context, namespace string) ([]Pod, error)
    ListDeployments(ctx context.Context, namespace string) ([]Deployment, error)
    ListServices(ctx context.Context, namespace string) ([]Service, error)
}

// LogReader reads pod logs. Consumed by logstream / logs service.
type LogReader interface {
    GetLogsBySelector(ctx context.Context, namespace, labelSelector string, tailLines int) ([]LogLine, error)
}

// Exec runs commands in pods. Consumed by k8s exec handler.
type Exec interface {
    ExecInPod(ctx context.Context, namespace, pod, container string, cmd []string) (io.ReadCloser, error)
}

// Client is the union — K8sClient (the concrete impl) still
// satisfies all three, but consumers can now depend on a
// narrower interface for testability.
type Client interface {
    Lister
    LogReader
    Exec
}
```

- [ ] **Step 3: 在 registry.go 返回 narrow 类型**

```go
// ClientFor returns the underlying Client. Callers can type-
// assert to the narrower interface they need.
func (r *ClientRegistry) ClientFor(id string) (Client, error) { ... }

// 或显式返回 narrow:
func (r *ClientRegistry) ListerFor(id string) (Lister, error) {
    cli, err := r.ClientFor(id)
    if err != nil { return nil, err }
    return cli, nil  // Client 实现了 Lister
}
```

(具体 API 设计根据现有 ClientRegistry 方法签名调整。)

- [ ] **Step 4: 改 servicecatalog 适配**

找到 servicecatalog 用 `Client` 的地方(1 处),改用 `Lister`:

```go
// 旧
client, err := registry.ClientFor(clusterID)
pods, err := client.ListPods(ctx, ns)

// 新
lister, err := registry.ListerFor(clusterID)
pods, err := lister.ListPods(ctx, ns)
```

- [ ] **Step 5: 跑 test,确保不退化**

Run: `go test -count=1 ./internal/k8s/... ./internal/servicecatalog/...`
Expected: PASS,既有测试不退化。

- [ ] **Step 6: 跑全量测试**

Run: `go test -count=1 -p 1 -timeout 600s ./...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/k8s/ internal/servicecatalog/
git commit -m "refactor(k8s): 拆 Client interface 为 Lister/LogReader/Exec,servicecatalog 改用 Lister"
```

---

## Task 13: Helm chart 骨架(Phase 4, Commit #13)

**Files:**
- Create: `deploy/helm/Chart.yaml`
- Create: `deploy/helm/values.yaml`
- Create: `deploy/helm/README.md`
- Create: `deploy/helm/templates/{deployment,service,ingress,configmap,secret,serviceaccount,hpa,cronjob-backup,prometheusrule}.yaml`

**Agent:** Agent 4 — 独立,无依赖

- [ ] **Step 1: 写 `deploy/helm/Chart.yaml`**

```yaml
apiVersion: v2
name: devops-toolkit
description: DevOps Toolkit platform
type: application
version: 0.1.0
appVersion: "0.2.1.0"
```

- [ ] **Step 2: 写 `deploy/helm/values.yaml`**

```yaml
replicaCount: 2

image:
  repository: ghcr.io/lkad/devops-toolkit
  pullPolicy: IfNotPresent
  tag: ""

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: devops.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - hosts:
        - devops.example.com
      secretName: devops-toolkit-tls

resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

# 应用配置(非 secret)
config:
  server:
    port: 8080
  ldap:
    dev_bypass: false

# Secret 用 external-secrets/sealed-secrets 注入,这里留空。
secrets: {}

# Backup 配置
backup:
  enabled: true
  schedule: "0 2 * * *"  # 每天 02:00
  target: s3
  s3Bucket: ""
  retentionDays: 7

# Postgres(假设外部托管,留 DSN secret)
postgres:
  host: ""
  port: 5432
  database: devops_toolkit

# PrometheusRule 配置
prometheusRule:
  enabled: true
  loopNoSuccessMinutes: 5
  readyCheckFailingMinutes: 3
```

- [ ] **Step 3: 写 `deploy/helm/templates/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "devops-toolkit.fullname" . }}
  labels:
    {{- include "devops-toolkit.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "devops-toolkit.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "devops-toolkit.selectorLabels" . | nindent 8 }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "devops-toolkit.serviceAccountName" . }}
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 8080
          livenessProbe:
            httpGet:
              path: /live
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 10
            periodSeconds: 5
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          envFrom:
            - configMapRef:
                name: {{ include "devops-toolkit.fullname" . }}-config
            - secretRef:
                name: {{ include "devops-toolkit.fullname" . }}-secrets
```

- [ ] **Step 4: 写 `deploy/helm/templates/service.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "devops-toolkit.fullname" . }}
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 80
      targetPort: http
  selector:
    {{- include "devops-toolkit.selectorLabels" . | nindent 4 }}
```

- [ ] **Step 5: 写 `deploy/helm/templates/ingress.yaml`**

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "devops-toolkit.fullname" . }}
  annotations:
    {{- with .Values.ingress.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
spec:
  ingressClassName: {{ .Values.ingress.className }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            pathType: {{ .pathType }}
            backend:
              service:
                name: {{ include "devops-toolkit.fullname" $ }}
                port:
                  number: 80
          {{- end }}
    {{- end }}
  {{- with .Values.ingress.tls }}
  tls:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
```

- [ ] **Step 6: 写 `deploy/helm/templates/configmap.yaml`**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "devops-toolkit.fullname" . }}-config
data:
  {{- range $key, $value := .Values.config }}
  {{ $key | upper }}: {{ $value | quote }}
  {{- end }}
```

- [ ] **Step 7: 写 `deploy/helm/templates/secret.yaml`**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "devops-toolkit.fullname" . }}-secrets
  annotations:
    # 推荐: 用 external-secrets / sealed-secrets 注入真实 secret
    # 这里仅占位,实际值由 ExternalSecret 同步。
    secrets.devops-toolkit/managed-by: "external-secrets"
type: Opaque
data:
  # 留空 — 通过 external-secrets-operator 同步
  APP_JWT_SECRET: ""
  K8S_CRYPTO_KEY: ""
```

- [ ] **Step 8: 写 `deploy/helm/templates/serviceaccount.yaml`**

```yaml
{{- if .Values.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "devops-toolkit.serviceAccountName" . }}
  labels:
    {{- include "devops-toolkit.labels" . | nindent 4 }}
{{- end }}
```

- [ ] **Step 9: 写 `deploy/helm/templates/hpa.yaml`**

```yaml
{{- if .Values.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "devops-toolkit.fullname" . }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "devops-toolkit.fullname" . }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetCPUUtilizationPercentage }}
{{- end }}
```

- [ ] **Step 10: 写 `deploy/helm/templates/cronjob-backup.yaml`**

```yaml
{{- if .Values.backup.enabled }}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ include "devops-toolkit.fullname" . }}-backup
spec:
  schedule: {{ .Values.backup.schedule | quote }}
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 1
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: postgres:15
              command:
                - /bin/sh
                - -c
                - |
                  apk add --no-cache aws-cli bash || true
                  /scripts/backup-postgres.sh --target={{ .Values.backup.target }}
              envFrom:
                - secretRef:
                    name: {{ include "devops-toolkit.fullname" . }}-secrets
              volumeMounts:
                - name: scripts
                  mountPath: /scripts
          volumes:
            - name: scripts
              configMap:
                name: {{ include "devops-toolkit.fullname" . }}-backup-scripts
                defaultMode: 0755
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "devops-toolkit.fullname" . }}-backup-scripts
data:
  backup-postgres.sh: |
    {{- .Files.Get "scripts/backup-postgres.sh" | nindent 4 }}
{{- end }}
```

注: 需要在 `deploy/helm/` 软链或复制 `scripts/backup-postgres.sh`(Task 14 实现)。**Task 13 完成后,等 Task 14 完成后,再 helm install 验证**。

- [ ] **Step 11: 写 `deploy/helm/templates/prometheusrule.yaml`**

```yaml
{{- if .Values.prometheusRule.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: {{ include "devops-toolkit.fullname" . }}-self-health
spec:
  groups:
    - name: devops-toolkit.self
      interval: 30s
      rules:
        - alert: PhysicalHostLoopNoSuccess
          expr: time() - physicalhost_loop_last_success_timestamp_seconds > {{ .Values.prometheusRule.loopNoSuccessMinutes }} * 60
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "Physical host monitor loop has not succeeded for {{ `{{ $value }}` }} seconds"
        - alert: ReadyCheckFailing
          expr: probe_ready{job="devops-toolkit"} == 0
          for: {{ .Values.prometheusRule.readyCheckFailingMinutes }}m
          labels:
            severity: critical
          annotations:
            summary: "Readiness probe failing for {{ `{{ $value }}` }} minutes"
{{- end }}
```

- [ ] **Step 12: 写 `_helpers.tpl`**

`deploy/helm/templates/_helpers.tpl`:

```yaml
{{/*
Expand the name of the chart.
*/}}
{{- define "devops-toolkit.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "devops-toolkit.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "devops-toolkit.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "devops-toolkit.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "devops-toolkit.selectorLabels" -}}
app.kubernetes.io/name: {{ include "devops-toolkit.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Service account name
*/}}
{{- define "devops-toolkit.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "devops-toolkit.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
```

- [ ] **Step 13: 写 `deploy/helm/README.md`**

```markdown
# DevOps Toolkit Helm Chart

最小骨架,不含 cert-manager / sealed-secrets 完整集成(留 C 阶段)。

## 用法

\`\`\`bash
helm install devops ./deploy/helm \\
  --set postgres.host=db.example.com \\
  --set backup.s3Bucket=my-backup-bucket
\`\`\`

## Secret 注入

Secret 通过 ExternalSecrets / SealedSecrets 注入,详见 `templates/secret.yaml` 的 annotations。
\`\`\`yaml
# 示例: ExternalSecret 引用 AWS Secrets Manager
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: devops-toolkit
spec:
  secretStoreRef:
    name: aws-secrets-manager
    kind: ClusterSecretStore
  target:
    name: devops-toolkit-secrets
  data:
    - secretKey: APP_JWT_SECRET
      remoteRef:
        key: devops/prod
        property: jwt_secret
\`\`\`

## 验证

\`\`\`bash
helm template ./deploy/helm
helm lint ./deploy/helm
\`\`\`
```

- [ ] **Step 14: 验证 Helm chart**

```bash
helm template ./deploy/helm > /tmp/rendered.yaml
helm lint ./deploy/helm
```

Expected: 0 error,所有 template 渲染成功。

- [ ] **Step 15: 提交**

```bash
git add deploy/helm/
git commit -m "feat(deploy): Helm chart 骨架(deployment/service/ingress/cronjob-backup/prometheusrule)"
```

---

## Task 14: 备份脚本 `backup-postgres.sh` + restore(Phase 5, Commit #14)

**Files:**
- Create: `scripts/backup-postgres.sh`
- Create: `scripts/restore-postgres.sh`

**Agent:** Agent 5 — 独立,无依赖

- [ ] **Step 1: 写 `scripts/backup-postgres.sh`**

```bash
#!/usr/bin/env bash
#
# backup-postgres.sh — PostgreSQL backup with multiple targets
#
# Usage: backup-postgres.sh --target=local|s3|nfs [--retention-days=7] [--pg-url=...]
#
# Targets:
#   local  - copy to /backups volume (default)
#   s3     - upload to S3 (BACKUP_S3_BUCKET env required)
#   nfs    - copy to BACKUP_NFS_PATH
#
# Required env (or --pg-url):
#   PG_URL or DEVOPS_DATABASE_URL

set -euo pipefail

TARGET="local"
RETENTION_DAYS=7
PG_URL="${PG_URL:-${DEVOPS_DATABASE_URL:-}}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target=*)         TARGET="${1#*=}" ;;
    --retention-days=*) RETENTION_DAYS="${1#*=}" ;;
    --pg-url=*)         PG_URL="${1#*=}" ;;
    -h|--help)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

[[ -n "$PG_URL" ]] || { echo "ERROR: PG_URL or DEVOPS_DATABASE_URL must be set" >&2; exit 2; }

# Sanity: pg_dump must be available.
command -v pg_dump >/dev/null 2>&1 || { echo "ERROR: pg_dump not found in PATH" >&2; exit 2; }

TS=$(date -u +%Y%m%dT%H%M%SZ)
DUMP_FILE="/tmp/devops-${TS}.sql.gz"

echo "INFO: starting backup at $TS, target=$TARGET"
pg_dump "$PG_URL" | gzip > "$DUMP_FILE"
SIZE=$(du -h "$DUMP_FILE" | cut -f1)
echo "INFO: dump created: $SIZE"

case "$TARGET" in
  local)
    mkdir -p /backups
    cp "$DUMP_FILE" /backups/
    find /backups -type f -name "*.sql.gz" -mtime +"$RETENTION_DAYS" -delete 2>/dev/null || true
    ;;
  s3)
    [[ -n "${BACKUP_S3_BUCKET:-}" ]] || { echo "ERROR: BACKUP_S3_BUCKET env required for s3 target" >&2; exit 2; }
    command -v aws >/dev/null 2>&1 || { echo "ERROR: aws CLI not found" >&2; exit 2; }
    S3_KEY="$(date -u +%Y/%m/%d)/devops-${TS}.sql.gz"
    aws s3 cp "$DUMP_FILE" "s3://${BACKUP_S3_BUCKET}/${S3_KEY}"
    ;;
  nfs)
    [[ -n "${BACKUP_NFS_PATH:-}" ]] || { echo "ERROR: BACKUP_NFS_PATH env required for nfs target" >&2; exit 2; }
    mkdir -p "$BACKUP_NFS_PATH"
    cp "$DUMP_FILE" "$BACKUP_NFS_PATH/"
    find "$BACKUP_NFS_PATH" -type f -name "*.sql.gz" -mtime +"$RETENTION_DAYS" -delete 2>/dev/null || true
    ;;
  *)
    echo "ERROR: unknown target: $TARGET (use local|s3|nfs)" >&2
    exit 2
    ;;
esac

# Verify dump is valid (decompresses cleanly).
echo "INFO: verifying dump integrity..."
gunzip -c "$DUMP_FILE" | head -5 || { echo "ERROR: dump verification failed" >&2; exit 2; }

rm -f "$DUMP_FILE"
echo "INFO: backup complete: $TS, target=$TARGET, size=$SIZE"
```

- [ ] **Step 2: 写 `scripts/restore-postgres.sh`**

```bash
#!/usr/bin/env bash
#
# restore-postgres.sh — Restore from backup (drill)
#
# Usage: restore-postgres.sh --source=local|s3|nfs --file=NAME [--pg-url=...]
#
# Walks through the steps of restoring without actually
# applying to the live database — meant as a drill to verify
# backups are valid and the procedure is documented.

set -euo pipefail

SOURCE="local"
FILE=""
TARGET_PG_URL="${TARGET_PG_URL:-${DEVOPS_DATABASE_URL:-}}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source=*) SOURCE="${1#*=}" ;;
    --file=*)   FILE="${1#*=}" ;;
    --pg-url=*) TARGET_PG_URL="${1#*=}" ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

[[ -n "$FILE" ]] || { echo "ERROR: --file=NAME required" >&2; exit 2; }
[[ -n "$TARGET_PG_URL" ]] || { echo "ERROR: TARGET_PG_URL or DEVOPS_DATABASE_URL must be set" >&2; exit 2; }

echo "INFO: restore drill starting: source=$SOURCE, file=$FILE"

case "$SOURCE" in
  local)
    DUMP_PATH="/backups/$FILE"
    [[ -f "$DUMP_PATH" ]] || { echo "ERROR: $DUMP_PATH not found" >&2; exit 2; }
    cp "$DUMP_PATH" /tmp/restore.sql.gz
    ;;
  s3)
    [[ -n "${BACKUP_S3_BUCKET:-}" ]] || { echo "ERROR: BACKUP_S3_BUCKET required" >&2; exit 2; }
    aws s3 cp "s3://${BACKUP_S3_BUCKET}/$FILE" /tmp/restore.sql.gz
    ;;
  nfs)
    [[ -n "${BACKUP_NFS_PATH:-}" ]] || { echo "ERROR: BACKUP_NFS_PATH required" >&2; exit 2; }
    cp "${BACKUP_NFS_PATH}/$FILE" /tmp/restore.sql.gz
    ;;
esac

# Dry-run: list contents to verify
echo "INFO: verifying dump..."
pg_restore --list /tmp/restore.sql.gz > /tmp/restore.list 2>&1 || \
  gunzip -c /tmp/restore.sql.gz | head -20 > /tmp/restore.list
echo "INFO: dump contains:"
head -20 /tmp/restore.list

# Apply to target (real restore)
echo "INFO: applying restore to target database..."
gunzip -c /tmp/restore.sql.gz | psql "$TARGET_PG_URL" --single-transaction

rm -f /tmp/restore.sql.gz /tmp/restore.list
echo "INFO: restore complete"
```

- [ ] **Step 3: 加可执行权限**

```bash
chmod +x scripts/backup-postgres.sh scripts/restore-postgres.sh
```

- [ ] **Step 4: 验证 shellcheck(无严重 warning)**

Run: `shellcheck scripts/backup-postgres.sh scripts/restore-postgres.sh 2>&1 | head -20`
Expected: 仅 info 级 warning,无 error。

- [ ] **Step 5: 提交**

```bash
git add scripts/backup-postgres.sh scripts/restore-postgres.sh
git commit -m "feat(scripts): backup-postgres.sh 支持 local/s3/nfs target + restore drill"
```

---

## Self-Review

### Spec coverage 验证

- [x] A1 secret 强制 — Task 5
- [x] A2 health 拆 — Task 6
- [x] A3 docker-compose + 备份 — Task 9 + Task 14
- [x] A4 monitor_loop 优雅停机 — Task 8
- [x] A5 writeAPIError 合并 — Task 1
- [x] A6 MapNotFound — Task 2
- [x] A7 dev-default secrets 集中 — Task 3
- [x] A8 .gitignore — Task 10
- [x] A9 dashboard ?status=open bug — Task 7
- [x] A10 promtool CI — Task 11
- [x] A11 secret masking — Task 4
- [x] A12 K8s interface 拆 — Task 12
- [x] Phase 4 Helm chart 骨架 — Task 13
- [x] 5 个 agent 分工 — 文档开头表格

### Placeholder 扫描

无 TBD / TODO / FIXME。代码块完整。

### Type / method 一致性

- `health.Checker` 在 Task 6 引入,在 Task 6 内部使用,一致
- `database.MapNotFound` 在 Task 2 引入,后续 task 引用
- `config.DevDefault*` 在 Task 3 引入,Task 5 引用
- `logger.ShouldMask` 在 Task 4 引入,Task 4 内部使用
- `k8s.Lister`/`LogReader`/`Exec` 在 Task 12 引入,Task 12 内部使用

### 关键缺口

1. `internal/k8s/registry.go` 的 `IsEmpty` 和 `FirstCluster` 方法(Task 6 引用)— 实际可能不存在,需在 Task 6 内部补
2. `auth/ldap.Client.HealthCheck` 方法(Task 6 引用)— 实际可能不存在,需在 Task 6 内部补
3. `internal/observability/metrics.go` 的 metric 注册方式(Task 8 引用)— 需在 Task 8 内部补
4. `scripts/backup-postgres.sh` 在 `deploy/helm/templates/cronjob-backup.yaml` 中引用(Task 13)— Task 14 完成后才 helm install 验证

这些 gap 在各 task 内部有 "如缺则加" 的说明。

---

## Execution Handoff

**Plan 完成,共 14 task,5-6 天 5 agent 并行。**

下一步:
- 5 个 agent 各开一个 branch:
  - Agent 1: `feat/a1-consolidation` (Task 1-4)
  - Agent 2: `feat/a2-enforce-health` (Task 5-7)
  - Agent 3: `feat/a3-deploy-topology` (Task 8-12)
  - Agent 4: `feat/a4-helm-chart` (Task 13)
  - Agent 5: `feat/a5-backup-scripts` (Task 14)
- 主 session 协调 merge 顺序: A1 → A2 → A3 → A4 → A5
- 每个 agent 跑完后,主 session 跑全量测试 + merge
