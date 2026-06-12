# A 子项目 — 后端生产化(Production-Readiness)

> **范围**:把现有 DevOps Toolkit 平台做到「几百人公司研发+运维都能正常用」的第一步
> **日期**: 2026-06-12
> **作者**: Claude
> **状态**: Design (待用户 review)
> **对应子项目**: A(5 个子项目分解中的第一个)
> **总估时**: 5-6 天

---

## 1. 背景与动机

### 1.1 当前状态(v0.2.0.0)

- 后端 31 packages 绿,前端 17 vitest 绿,scoped-Auditor 关闭,auditor/servicecatalog/observability 全部端到端
- 部署: docker-compose(Koyeb/Oracle Cloud/Cloud Run 推荐),单 Go 单体 + PostgreSQL
- 监控: Prometheus + Grafana(已有 4 个 dashboard)
- 44 commits 已 ahead of origin/main(未 push)

### 1.2 已知缺口(从 `TODOS.md` 提取)

12 项需要做。**所有 P0 #1-#6 都还没完全关上**(P0 #1/#2 留给 B 阶段)。

### 1.3 目标

把以下 12 项在 5-6 天内做完,让单 docker-compose 部署 + Helm chart 骨架都达到「几百人公司可用」水平:

| ID | 事项 | 来源 | 工作量 |
|---|---|---|---|
| A1 | `APP_JWT_SECRET`/`K8S_CRYPTO_KEY`/`ldap.dev_bypass` 生产强制 | TODOS P0 #4 | 2 hr |
| A2 | `/health` 拆 `/live`+`/ready`+`/health` 兼容,fan-out 探活 | TODOS P0 #5 | 1 day |
| A3 | docker-compose 资源限制+健康检查+备份脚本,S3/NFS/Local target | TODOS P0 #6 | 1+1 day |
| A4 | `monitor_loop.go` 优雅停机 + Prometheus metrics | TODOS P1 monitor | 1 day |
| A5 | `writeAPIError` 8 sites 合并到 `handler.WriteAPIError` | TODOS P2 | 1 hr |
| A6 | `database.MapNotFound` 11 sites 合并 | TODOS P2 | 30 min |
| A7 | dev-default secrets 集中 const block + `envOrWarn` helper | TODOS P2 | 30 min |
| A8 | `.gitignore` 修 `devops-toolkit` 49MB binary + `git rm` | TODOS P3 | 1 min |
| A9 | dashboard `?status=open` bug 修(`alerts/handler.go` 改 `status`) | TODOS P3 | 5 min |
| A10 | `promtool check rules` 接到 CI | TODOS P3 | 15 min |
| A11 | secret masking 名单补全,放 `pkg/logger/secret_keys.go` | TODOS P3 | 1 hr |
| A12 | K8s `Client` interface 拆 `Lister`/`LogReader`/`Exec` | TODOS P2 K8s | 2-3 hr |

**总估时:5-6 天**(实际:含 review + 测试 + 文档约 1 周内可完成)

---

## 2. 架构总览(双轨兼容)

```
┌──────────────────────────────────────────────────────────────────┐
│                  A 子项目 — 双轨兼容架构                          │
├──────────────────────────────────────────────────────────────────┤
│  ┌──────────────────── 部署形态 ────────────────────┐            │
│  │ docker-compose 单机   │  Helm chart (k8s)         │            │
│  │   app  :1            │   Deployment: replicas=2  │            │
│  │   postgres:15        │   Service + Ingress       │            │
│  │   ldap:osixia        │   Postgres (StatefulSet)  │            │
│  │   prometheus         │   LDAP (Deployment)       │            │
│  │   grafana            │   Prometheus+Grafana stack│            │
│  │   loki               │   Loki stack              │            │
│  │   alertmanager       │   Alertmanager            │            │
│  │   backup (cron)      │   CronJob backup          │            │
│  └──────────────────────┴───────────────────────────┘            │
│                                                                  │
│  共享代码层: cmd/devops-toolkit/main.go                          │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐    │
│  │ 健康探活     │ │ 监控/告警    │ │ 共享配置+生命周期     │    │
│  │ /live /ready │ │ prom metrics │ │ signal.NotifyContext │    │
│  │ /health*     │ │ self-scrape  │ │ monitor_loop         │    │
│  └──────────────┘ └──────────────┘ └──────────────────────┘    │
│                                                                  │
│  共享后端代码层:                                                  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐    │
│  │ config       │ │ handler/     │ │ database/            │    │
│  │ - Secret强制 │ │ response.go  │ │ MapNotFound          │    │
│  │ - mask set   │ │ WriteAPIError│ │ dev-default 集中     │    │
│  └──────────────┘ └──────────────┘ └──────────────────────┘    │
│                                                                  │
│  备份/恢复: scripts/backup-postgres.sh                          │
│    目标: S3 (默认) / NFS (备选) / Local (开发)                   │
└──────────────────────────────────────────────────────────────────┘
```

### 关键设计原则

1. **共享代码层不依赖部署形态** — `cmd/devops-toolkit/main.go` 启动时不感知"我在 docker-compose 还是 k8s"。通过 `signal.NotifyContext` 接 `SIGTERM`/`SIGINT`,在任何形态下都正确停机。
2. **`/health` 三个端点**:
   - `/live` — 进程活着返 200。K8s livenessProbe 用。
   - `/ready` — 依赖全健康返 200。K8s readinessProbe 用。
   - `/health` — 兼容旧调用方,聚合返回。
3. **备份脚本**: `scripts/backup-postgres.sh` 单一入口,两种部署都用同一脚本。
4. **monitor_loop**: 用 `signal.NotifyContext(parent, SIGINT, SIGTERM)`,App-wide shared context。
5. **secret 强制生产**: `config.Validate()` 在 `env == "production"` 时强制三个值,`log.Fatal` 退出。

---

## 3. 关键 Component 详解

### 3.1 Health 探活层(`internal/health/`)

**接口**:

```go
type Checker interface {
    Name() string                       // "postgres" | "ldap" | "k8s"
    Check(ctx context.Context) error    // nil = healthy
}

type Handler struct {
    checkers []Checker
    timeout  time.Duration              // 默认 2s,per-checker
}

func (h *Handler) Live(c *gin.Context)    // 200, body: {"status":"ok","uptime_seconds":N}
func (h *Handler) Ready(c *gin.Context)   // fan-out to checkers, 200/503
func (h *Handler) Legacy(c *gin.Context)  // /health, 聚合 body
```

**Checker 实现**:
- `PostgresChecker{ db *gorm.DB }` — `db.Raw("SELECT 1").Row()` 探活
- `LDAPChecker{ ldapCfg config.LDAP }` — 复用 `auth/ldap` 客户端的 `Bind` 方法
- `K8sChecker{ registry *k8s.ClientRegistry }` — 取第一个 cluster,`/healthz` 探活;registry 空则 skip
- `ServiceCatalogChecker`(可选)— `servicecatalog.Service.Get("__health__")` 不 panic

**Endpoint 契约**:

```
GET /live
  → 200  {"status":"ok","uptime_seconds":N}
  → 503  (不预期,K8s 会把 pod restart)

GET /ready
  → 200  {"status":"ok","checks":{"postgres":"ok","ldap":"ok","k8s":"ok"}}
  → 503  {"status":"degraded","checks":{"postgres":"ok","ldap":"timeout: deadline 2s","k8s":"ok"}}

GET /health   (保留,兼容旧调用方)
  → 200/503,行为同 /ready
```

**路由**:不放在 `/api/v1` 下,在 root `/` 路径。

**单元测试**:
- `TestPostgresChecker_OK` / `TestPostgresChecker_Fail`
- `TestK8sChecker_NoClusters_Ok`
- `TestHandler_Ready_AllOK` / `TestHandler_Ready_Degraded`
- `TestHandler_Live` / `TestHandler_Legacy_SameAsReady`

### 3.2 Secret 强制生产(`internal/config/secrets_block.go`)

```go
const (
    DevDefaultJWTSecret       = "dev-jwt-secret-change-me"
    DevDefaultK8SCryptoKey     = "dev-k8s-crypto-key-32bytes!!"
    DevDefaultInfluxPassword   = "dev-influx-pwd"
    DevDefaultLDAPBindPassword = "dev-ldap-bind"
    DevDefaultProberSSHKey     = "dev-prober-key"
)

func envOrWarn(envName, devDefault string) string {
    if v := os.Getenv(envName); v != "" {
        return v
    }
    log.Warn("using dev default for %s — DO NOT use in production", envName)
    return devDefault
}

func (c *Config) Validate() error {
    var errs []error
    if c.Env == "production" {
        if c.JWTSecret == DevDefaultJWTSecret || c.JWTSecret == "" {
            errs = append(errs, errors.New("APP_JWT_SECRET must be set in production"))
        }
        if c.K8SCryptoKey == DevDefaultK8SCryptoKey || c.K8SCryptoKey == "" {
            errs = append(errs, errors.New("K8S_CRYPTO_KEY must be set in production"))
        }
        if c.LDAP.DevBypass {
            errs = append(errs, errors.New("ldap.dev_bypass must be false in production"))
        }
    }
    return errors.Join(errs...)
}
```

**测试**:
- `TestConfig_Validate_Production_OK` — env=prod + 三个真值 → nil
- `TestConfig_Validate_Production_RejectsDefaults` — env=prod + JWTSecret=DevDefault → 包含 "APP_JWT_SECRET" 错误
- `TestConfig_Validate_Dev_OK_AllowsDefaults` — env=dev + dev defaults → nil
- `TestEnvOrWarn_FromEnv` / `TestEnvOrWarn_DefaultWithWarn`

### 3.3 合并 helper

**`handler.WriteAPIError(w, err)`**(8 sites 合并):

```go
func WriteAPIError(w http.ResponseWriter, err error) {
    var apiErr *contracts.APIError
    if errors.As(err, &apiErr) {
        status := http.StatusInternalServerError
        switch apiErr.Code {
        case contracts.CodeValidation: status = http.StatusBadRequest
        case contracts.CodeNotFound:   status = http.StatusNotFound
        case contracts.CodeUnauthorized: status = http.StatusUnauthorized
        case contracts.CodeForbidden:    status = http.StatusForbidden
        }
        WriteError(w, apiErr)
        return
    }
    WriteError(w, &contracts.APIError{
        Code: contracts.CodeInternal, Message: "internal error", Cause: err,
    })
}
```

**`database.MapNotFound(err)`**(11 sites 合并):

```go
func MapNotFound(err error) error {
    if err == nil { return nil }
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return ErrNotFound
    }
    return err
}
```

**变更范围**:
- 8 modules `writeAPIError` 删,改 `handler.WriteAPIError`(2 行 diff × 8 sites)
- 11 repositories `if errors.Is(err, gorm.ErrRecordNotFound)` 改 `db.MapNotFound(err)`(1 行 × 11 sites)
- 行为不变,既有测试 100% 应通过

### 3.4 monitor_loop 优雅停机

**当前**:`context.Background()`,`for { ... time.Sleep(5s) }`。

**改造**:

```go
type Loop struct {
    repo  *Repository
    probe Prober
    cache *MetricsCache
    log   *slog.Logger

    iterCounter   prometheus.Counter
    errCounter    prometheus.Counter
    successGauge  prometheus.Gauge
}

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

**main.go 接线**:

```go
rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

srv := &http.Server{Handler: buildRouter(...), Addr: cfg.Server.Addr}
go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatal(err)
    }
}()
go monitorLoop.Run(rootCtx)

<-rootCtx.Done()
log.Info("shutdown signal received")
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)
```

**测试**:
- `TestLoop_Run_StopsOnContextCancel`
- `TestLoop_Run_IncrementsCounters`
- `TestLoop_Run_IncrementsErrorCounter`
- `TestLoop_Run_StopsWithinTimeout`

### 3.5 Helm chart 骨架

只交付**可用的最小骨架**,完整 Helm 化是 C 子项目。

- `deployment.yaml`:2 replicas,probes 指向 `/live` `/ready`,resources requests/limits
- `service.yaml`:ClusterIP
- `ingress.yaml`:TLS enabled,annotations 留 cert-manager 接入点(注释)
- `configmap.yaml`:非 secret 配置
- `secret.yaml`:留空,README 写明"用 external-secrets/sealed-secrets 注入"
- `serviceaccount.yaml`:RBAC 最小权限
- `hpa.yaml`:CPU 70% 扩缩,min 2 max 10
- `cronjob-backup.yaml`:每日 02:00 调 `scripts/backup-postgres.sh --target=s3`
- `prometheusrule.yaml`:`PhysicalHostLoopNoSuccess` (5m 无成功 → page),`ReadyCheckFailing` (3m ready 不 ok → page)

**chart 验收**: `helm template deploy/helm` 完整 render,`helm install --dry-run` 无 error。

### 3.6 docker-compose 升级

**新增**:
- 每个 service 加 `mem_limit` / `cpus` / `healthcheck`(app 用 `/live`)
- `app` 加 `restart: always`
- 加 `backup` 容器(基于 postgres 镜像,跑 cron 调 `scripts/backup-postgres.sh --target=local`)
- 加 `prometheus` 自监控 scrape `app:8080/metrics`
- 加 `loki` + `promtail` 收集 app 日志
- 外部敏感变量全部走 `.env` file,`deploy/docker-compose.yml` 中不出现明文 secret

### 3.7 备份脚本(`scripts/backup-postgres.sh`)

```bash
#!/usr/bin/env bash
set -euo pipefail

TARGET="local"
RETENTION_DAYS=7
while [[ $# -gt 0 ]]; do
  case "$1" in
    --target=*) TARGET="${1#*=}" ;;
    --retention-days=*) RETENTION_DAYS="${1#*=}" ;;
    --pg-url=*) PG_URL="${1#*=}" ;;
    *) echo "unknown arg: $1"; exit 2 ;;
  esac
  shift
done

: "${PG_URL:=${DEVOPS_DATABASE_URL:-}}"
[[ -n "$PG_URL" ]] || { echo "PG_URL not set"; exit 2; }

TS=$(date -u +%Y%m%dT%H%M%SZ)
DUMP_FILE="/tmp/devops-${TS}.sql.gz"

pg_dump "$PG_URL" | gzip > "$DUMP_FILE"

case "$TARGET" in
  local) cp "$DUMP_FILE" /backups/ ;;
  s3)    aws s3 cp "$DUMP_FILE" "s3://${BACKUP_S3_BUCKET:-}/$(date -u +%Y/%m/%d)/" ;;
  nfs)   cp "$DUMP_FILE" "${BACKUP_NFS_PATH:-/mnt/nfs/backups}/" ;;
esac

find /backups -type f -name "*.sql.gz" -mtime +"$RETENTION_DAYS" -delete 2>/dev/null || true

echo "backup complete: $(du -h "$DUMP_FILE")"
gunzip -c "$DUMP_FILE" | head -5

rm -f "$DUMP_FILE"
```

**restore drill**:`scripts/restore-postgres.sh` 调同 script 倒序(下载 → 解压 → 干跑 `pg_restore --list` 验证)。

### 3.8 secret masking 完整名单(`pkg/logger/secret_keys.go`)

```go
var SecretKeys = map[string]bool{
    "password":          true,
    "kubeconfig":        true,
    "bind_password":     true,
    "token":             true,
    "secret":            true,
    "jwt_secret":        true,
    "k8s_crypto_key":    true,
    "private_key":       true,
    "ssh_key":            true,
    "client_secret":     true,
    "api_key":            true,
    "ldap_bind_pw":      true,
    "tls_cert":           true,
}

func ShouldMask(key string) bool {
    return SecretKeys[strings.ToLower(key)]
}
```

**测试**:所有 key 命中 mask;大小写不敏感;不存在的 key 不 mask;`(*Config).String()` 改用 `ShouldMask`。

### 3.9 几个小修

- `alerts/handler.go:344-372` `c.Query("state")` → `c.Query("status")`
- `.gitignore` 改 `devops-toolkit` → `cmd/devops-toolkit/devops-toolkit` + `git rm --cached devops-toolkit`
- `.github/workflows/ci.yml` 加 step: `promtool check rules deploy/prometheus/rules/*.yml`
- `k8s.Client` interface 拆 `Lister`/`LogReader`/`Exec`,`servicecatalog` 适配 1 处

---

## 4. 数据流与错误处理

### 4.1 关键数据流

**K8s readinessProbe 触发**:

```
k8s probe  →  pod:8080/ready  →  Handler.Ready
                                    ├─ PostureChecker.Check(ctx)  2s timeout
                                    ├─ LDAPChecker.Check(ctx)     并发
                                    └─ K8sChecker.Check(ctx)      并发
```

所有 checker 并发执行,2s timeout 是 per-checker。任一失败 → 503 + 完整 checks body。

**SIGTERM 优雅停机**:

```
k8s/docker  →  SIGTERM  →  signal.NotifyContext 收到,rootCtx.Done() close

并行触发:
  1. monitor_loop.Run()  — select ctx.Done() 立刻返回
  2. http.Server.Shutdown — 等待 in-flight,30s timeout
  3. 其他 long-running: K8s 缓存预热、InfluxDB writer、WS hub — 同样 select ctx.Done() 返回
```

**生产环境 secret 缺失**:

```
main() 启动  →  config.Load()  →  config.Validate()
                                      ├─ env == "production" 
                                      └─ JWTSecret == "" or == DevDefault
                                          →  log.Fatal,exit code 1
```

### 4.2 错误处理约定

| 错误类型 | HTTP status | 行为 |
|---|---|---|
| `*contracts.APIError{CodeValidation}` | 400 | handler.WriteError,带 details |
| `*contracts.APIError{CodeNotFound}` | 404 | handler.WriteError |
| `*contracts.APIError{CodeUnauthorized}` | 401 | handler.WriteError |
| `*contracts.APIError{CodeForbidden}` | 403 | handler.WriteError |
| `*contracts.APIError{CodeInternal}` | 500 | handler.WriteError(不暴露内部 err) |
| 其他 error | 500 | handler.WriteAPIError 兜底 |
| health `/ready` 任一 checker fail | 503 | 所有 checks 都返回 |
| health `/live` | 200/503 | 进程 up → 200 |

**关键错误传递约定**:
- service 层返回 `*contracts.APIError`(类型化)
- handler 层用 `handler.WriteAPIError` 统一映射 HTTP
- 仓库层不返回 `*contracts.APIError`,只返回 `ErrNotFound` 等 sentinel + raw err
- **monitor_loop 的 iteration error 不上抛,只计数 + 写日志**

### 4.3 测试矩阵

| Component | Unit | Integration | E2E |
|---|---|---|---|
| `health/PostgresChecker` | ✅ 真 sqlite + close db | ✅ main_test.go 加 /ready 全 ok | 文档化 |
| `health/LDAPChecker` | ✅ mock LDAP server | ✅ /ready with mock LDAP | 文档化 |
| `health/K8sChecker` | ✅ fake clientset | ✅ /ready with registered cluster | 文档化 |
| `health/Handler` | ✅ Live/Ready/Legacy 三 path | ✅ 200/503 路径 | 文档化 |
| `config.Validate` | ✅ 4 case | ✅ main_test.go TestConfigValidation | 文档化 |
| `envOrWarn` | ✅ from env / default+warn | ❌ | ❌ |
| `handler.WriteAPIError` | ✅ 5 case | ✅ 既有 handler 测试不退化 | 文档化 |
| `database.MapNotFound` | ✅ 3 case | ✅ 既有 repo 测试不退化 | 文档化 |
| `monitor_loop.Run` | ✅ 4 case | ✅ main_test.go 跑后进程退出 | 文档化 |
| `secret masking` | ✅ 大小写、命中、不命中、嵌套 | ✅ Config.String() 输出无明文 | ❌ |
| `backup-postgres.sh` | ⚠️ bats 跑 mock pg_dump | ❌ | ⚠️ restore drill 跑真 PG |
| Helm chart | ⚠️ `helm template` + `helm lint` | ❌ | ⚠️ minikube 跑通(可选) |
| docker-compose | ⚠️ `docker compose config` 验证 | ❌ | ⚠️ dev 环境启动验证 |
| `alerts/handler` `?status=open` | ✅ status 取值 | ✅ /api/v1/alerts?status=open 200 | ❌ |
| `.gitignore` `devops-toolkit` | ❌ | ❌ | ✅ `git status` 不再显示 |
| `promtool` CI | ⚠️ 本地跑 promtool | ✅ CI step | ✅ rules 通过 |

**总测试统计预估**:
- 单元测试新增: ~30 cases
- 既有测试: 31 packages 全不退化
- `go test -count=1 -p 1 -timeout 600s ./...` 估计 5-6 分钟

---

## 5. 实施计划(5 个 phase,1 个 PR 一个 phase)

### Phase 1a: 合并 helper + secret 集中 (1 day)
- A5 writeAPIError 合并(8 sites)
- A6 database.MapNotFound(11 sites)
- A7 dev-default secrets 集中 + envOrWarn
- A11 secret masking 完整名单
- 注: 这些都是小改动,合并到 1 个 PR 便于 review,无功能影响

### Phase 1b: 强制生产 + health 拆 (1 day)
- A1 secret 强制生产(基于 Phase 1a 的 dev-default 集中)
- A2 health 拆 /live /ready + /health + checker fan-out
- A9 dashboard ?status=open bug(顺手修)

### Phase 2: monitor_loop 升级 (1 day)
- A4 signal.NotifyContext 共享 + Prometheus metrics

### Phase 3: 部署拓扑 (2 days)
- A3 docker-compose 资源限制+健康检查+备份脚本
- A8 .gitignore 修 49MB binary
- A10 promtool check rules CI
- A12 K8s Client interface 拆 Lister/LogReader/Exec

### Phase 4: Helm chart 骨架 (1 day)
- A3b Helm chart 骨架

### Phase 5: backup 恢复 drill + 文档 (1 day)
- A3c scripts/backup-postgres.sh 跨 target 验证
- A3d scripts/restore-postgres.sh + drill
- CHANGELOG.md / DOCUMENT_INDEX.md / openspec/specs/

### Agent 分工方案

| 时间窗 | Agent 任务 | 主 session 任务 |
|---|---|---|
| Day 1 上午 - 下午 | (sync) Phase 1a + 1b by 主 session(7 个 commit: #1-#7) | 写代码 + 跑测试 + commit |
| Day 1 下午 - Day 2 | /loop Agent 1: 备份脚本 bats 测试 + restore script | 监控,merge |
| Day 2 上午 | /loop Agent 2: Helm chart skeleton(Phase 4) | 监控,merge |
| Day 2 下午 | /loop Agent 3: promtool CI step(Phase 3 内 #11) | 监控,merge |
| Day 3 上午 | (sync) Phase 5 主 session 做 backup drill + 写文档 | 写 CHANGELOG, spec |

### 关键 commit 命名(14 个)

**Phase 1a: 合并 helper + secret 集中** (commit #1-#4)
1. `refactor(handler): consolidate 8 writeAPIError sites into handler.WriteAPIError`
2. `refactor(database): add MapNotFound helper, replace 11 sites`
3. `refactor(config): centralize dev-default secrets in secrets_block.go`
4. `feat(logger): complete secret masking list with kubeconfig/token/secret`

**Phase 1b: 强制生产 + health 拆** (commit #5-#7)
5. `feat(config): enforce APP_JWT_SECRET and K8S_CRYPTO_KEY in production`
6. `feat(health): split /health into /live and /ready with checker fan-out`
7. `fix(alerts): read ?status query param (was: ?state) on dashboard`

**Phase 2: monitor_loop 升级** (commit #8)
8. `feat(monitor): graceful shutdown via signal.NotifyContext + prometheus metrics`

**Phase 3: 部署拓扑** (commit #9-#12)
9. `feat(deploy): add mem_limit/cpus/healthcheck/backup to docker-compose`
10. `chore(gitignore): exclude 49MB devops-toolkit binary from repo`
11. `ci: add promtool check rules step`
12. `refactor(k8s): split Client interface into Lister/LogReader/Exec`

**Phase 4: Helm chart 骨架** (commit #13)
13. `feat(deploy): add Helm chart skeleton (deployment/service/ingress/cronjob)`

**Phase 5: backup 脚本 + 文档** (commit #14)
14. `feat(scripts): backup-postgres.sh with local/s3/nfs target + restore drill`

(实际 14 个 commit,Day 1-3 完成)

### 每个 commit 的完成定义 (DoD)

- 该 commit 的 code 改动落地
- 单元测试新增/调整齐全,`go test ./...` 在该目录全绿
- 既有 31 packages 测试不退化
- `go vet ./...` warning 不增加
- CHANGELOG.md 增一行
- commit message 用中文,符合 git log 风格

---

## 6. 风险与回退

### 风险

| 风险 | 缓解 |
|---|---|
| A 子项目改动面广(8+ directories),/loop agent 跑可能撞并发 | **分批 commit**: 每个 component 1-2 commit,/loop 起 3-4 agent 各管 1 组,主 session 协调 merge |
| Helm chart 写得不正确 | `helm template` + `helm lint` 在 plan step 验证 |
| monitor_loop 改 signal.NotifyContext 影响其他 goroutine | 先在 dev 跑一周,看 `kill -TERM` 后进程是否 30s 内退出 |
| secret 强制导致 dev 环境也拒 | `env != "production"` 才允许默认值 |
| 49MB devops-toolkit 在 git 历史 | `git filter-branch` 或 BFG 改历史,单独一个 PR |
| backup 脚本在 postgres 15 vs 14 兼容性 | pg_dump 跨大版本通常向下兼容,restore drill 用同版本 |
| WriteAPIError 行为微变 | 既有 8 module 测试不退化 = 行为不变证据 |
| 既有 monitor_loop 测试可能依赖 context.Background() | 改为参数化:Loop.Run(ctx) 接受 caller 传入的 ctx |

### 回退(每个 phase 独立可回退)

| Phase | 回退方法 | 风险等级 |
|---|---|---|
| Phase 1a 合并 helper | `git revert <merge commit>`(单 PR,无功能影响) | 🟢 低 |
| Phase 1b 强制 + health | 同上,既有 API 行为不变 | 🟢 低 |
| Phase 2 monitor_loop | 同上,signal.NotifyContext 改动仅 main.go + monitor_loop.go | 🟢 低 |
| Phase 3 部署拓扑 | docker-compose.yml `git revert`,Helm chart 仅新增 | 🟢 低 |
| Phase 4 Helm 骨架 | `rm -rf deploy/helm/`(纯新增) | 🟢 零 |
| Phase 5 backup + 文档 | `git revert` 文档 + `rm scripts/backup-*.sh` | 🟢 零 |

---

## 7. 不做 (YAGNI)

- 不做完整的 Helm chart(只骨架,cert-manager / sealed-secrets 接入是 C 阶段)
- 不做 Grafana dashboard 自监控(只部署 PrometheusRule,可视化后续)
- 不做 SSO/SAML/OIDC(认证改造留给 B 后续或独立阶段)
- 不做日志长期保留(默认 30d,后续单独 spec)
- 不做 CMDB / 自动发现
- 不重写 `pg_dump` 脚本为 Go(用现成 CLI,跨平台)
- 不做 P3 拼写修正(留给 lint pass)
- 不做 P0 #1 (auth middleware 接线) / P0 #2 (per-module project-scope 强制) — 留给 B 阶段

---

## 8. 验收标准

### 完成定义 (Definition of Done)

A 子项目**完成**意味着:

1. **代码**: 14 个 commit 全部在 main 上,既有 31 packages 测试不退化
2. **测试**: 30+ 新单元测试全绿,`go test -count=1 -p 1 -timeout 600s ./...` 5-6 分钟跑完
3. **质量**: `go vet ./...` warning 数 ≤ 1(不增加)
4. **文档**: `CHANGELOG.md` 增一节,`openspec/specs/production-readiness/` 新 spec 文档,`DOCUMENT_INDEX.md` 更新
5. **运维**: `deploy/docker-compose.yml` 一键起,docker compose config 验证通过;Helm chart `helm template` + `helm lint` 验证通过
6. **备份**: scripts/backup-postgres.sh 三种 target 都验证过(至少 local + s3 跑过)
7. **可观测性**: `/live` `/ready` `/health` 三个端点全通,Prometheus self-scrape 配好
8. **可回退**: 每个 phase 一个 PR,`git revert` 单 PR 即可回退

### 关键验收用例

| 验收项 | 命令 / 验证 |
|---|---|
| 全部 unit test 绿 | `go test -count=1 -p 1 -timeout 600s ./...` |
| 既有 vet warning 不增加 | `go vet ./... 2>&1 \| wc -l` (应 ≤ 1) |
| /live /ready 端点 | `curl -i localhost:8080/live` `curl -i localhost:8080/ready` |
| Secret 强制 | `env=production JWT_SECRET="" ./devops-toolkit` → exit code 1 |
| 备份脚本 | `./scripts/backup-postgres.sh --target=local` 跑通,生成 dump 文件 |
| Helm chart | `helm template deploy/helm` 完整 render,`helm lint deploy/helm` 0 warning |
| Docker compose | `docker compose -f deploy/docker-compose.yml config` 验证 |
| CI | push branch → `promtool check rules` step 跑过 |
| Bug 修复 | `curl localhost:8080/api/v1/alerts?status=open` 返正确数据 |
| 49MB binary | `git status` 不再显示 `devops-toolkit` |

---

## 9. 未来子项目 Preview

A 完成后,接下来 4-5 周的工作:

**B 子项目 (1.5 周)**: 鉴权+多租户硬化
- Auth+RBAC middleware 接到 /api/v1(8 modules)
- per-module project-scope 强制(8 service 层)
- scoped-Auditor RBAC matrix 接线(RoleScopedAuditor + PermissionViewAuditLogProject)

**C 子项目 (1 周)**: 部署拓扑深化
- Helm chart 完整化(Ingress cert-manager / ExternalSecrets / ArgoCD)
- minikube / k3d 实装验证
- 备份跨 K8s(VolumeSnapshot + Velero)

**D 子项目 (2 周)**: UI 空白补全
- K8s pod log 面板(WS)+ exec(xterm.js)
- Service catalog oncall/runbook 写 UI
- Audit log UI(接入 scoped-Auditor)
- Middleware 链顺序修正

**E 子项目 (1 周)**: 性能 + 容量基线
- k6 100/300/1000 VU 场景
- p95+p99+error rate 报告
- Go pprof 内存/协程 profile

A→B→C→D→E 总估时: 5-6 周(与用户初始估计的"4-6 周"基本一致)。

---

## 附录 A:文件清单(详细变更范围)

```
新增:
  internal/health/handler.go
  internal/health/live.go
  internal/health/ready.go
  internal/health/handler_test.go
  internal/health/postgres_checker.go
  internal/health/ldap_checker.go
  internal/health/k8s_checker.go
  internal/health/postgres_checker_test.go
  internal/health/ldap_checker_test.go
  internal/health/k8s_checker_test.go
  internal/config/secrets_block.go
  internal/config/secrets_block_test.go
  internal/database/errors.go
  internal/database/errors_test.go
  pkg/logger/secret_keys.go
  pkg/logger/secret_keys_test.go
  deploy/helm/Chart.yaml
  deploy/helm/values.yaml
  deploy/helm/templates/deployment.yaml
  deploy/helm/templates/service.yaml
  deploy/helm/templates/ingress.yaml
  deploy/helm/templates/configmap.yaml
  deploy/helm/templates/secret.yaml
  deploy/helm/templates/serviceaccount.yaml
  deploy/helm/templates/hpa.yaml
  deploy/helm/templates/cronjob-backup.yaml
  deploy/helm/templates/prometheusrule.yaml
  deploy/helm/README.md
  deploy/prometheus/rules/self-health.yml
  scripts/backup-postgres.sh
  scripts/restore-postgres.sh
  scripts/test-backup-script.sh
  openspec/specs/production-readiness/spec.md

修改:
  cmd/devops-toolkit/main.go                        (signal.NotifyContext, /live /ready 接线)
  internal/config/config.go                         (Validate, secret 强制)
  internal/handler/response.go                      (WriteAPIError helper)
  internal/database/                                (11 sites 改 MapNotFound)
  internal/observability/metrics.go                 (monitor_loop counters)
  internal/physicalhost/monitor_loop.go             (signal.NotifyContext)
  internal/physicalhost/handler.go                  (8 sites 改 WriteAPIError)
  internal/alerts/handler.go                        (8 sites 改 WriteAPIError + ?status 修)
  internal/audit/handler.go                         (改 WriteAPIError)
  internal/audit/repository.go                       (改 MapNotFound)
  internal/k8s/client.go                            (Client interface 拆)
  internal/k8s/registry_test.go                     (调整 fake clientset 适配拆后 interface)
  internal/k8s/registry.go                          (k8s.ClientRegistry 返回拆后 interfaces)
  internal/servicecatalog/                          (适配拆后 interfaces,1 处)
  internal/config/config.go                         (Validate + 改用 envOrWarn)
  pkg/logger/                                       (改用 ShouldMask)
  deploy/docker-compose.yml                         (资源限制+健康检查+backup+loki+promtail)
  deploy/prometheus/prometheus.yml                  (app self-scrape)
  .github/workflows/ci.yml                          (promtool step)
  .gitignore                                        (49MB binary 精确)
  CHANGELOG.md                                      (新一节)
  DOCUMENT_INDEX.md                                 (新 spec 索引)
  TODOS.md                                          (A 子项目项移到 Completed)
```

## 附录 B:跨 phase 依赖

```
A.Phase1 ──┐
            ├── A.Phase5 (bash scripts, docs)
A.Phase2 ──┤
            ├── B.Phase1 (auth+RBAC middleware)
A.Phase3 ──┤
A.Phase4 ──┘
            └── D.Phase? (K8s log UI 需要 health endpoint 稳定)
```

A.Phase1 → B.Phase1 (必须): A 不做 B 难以开始
A.Phase3 + A.Phase4 → C (可并行): Helm 骨架 + docker-compose 升级是 C 的基础
A 全部 → D: UI 工作可与 A 后半段并行
A 全部 → E: 性能基线必须等硬化后再做
