# Audit 3 — Production-readiness / ops maturity (v0.2.0.0)

Captured 2026-06-10. Read-only. The user's framing: "what
would block a real production rollout?" Full report at the
agent JSONL log; this is the decision-relevant summary.

## Top 7 to fix before production

Severity: 🔴 = production blocker · 🟡 = should fix soon · 🟢 = nice to have

### 1. 🔴 Wire `auth.NewAuthMiddleware` and `rbac.RequirePermission` on `/api/v1`

Currently every business endpoint is **anonymous** — the
middleware constructors are never called in `main.go`. The
`internal/middleware/chain.go:12` comment even says "Auth
and RBAC are injected by their respective owners (Phase 2)".
Estimate: 1-2 days. Touches every registrar.

### 2. 🔴 Cross-tenant access is not isolated

A Developer-role user in project A can read, modify, or
delete any project's resources by passing `project_id` in
URL params or body. No handler filters by the caller's
`project_id`. `internal/project/service.go:130-141`
`GetProjectWithRelations` returns everything regardless of
membership. Even with #1 wired, the role matrix is global,
not per-project — needs a `rbac.HasPermissionInProject`
lookup in each service layer. Estimate: 3-5 days.

### 3. 🔴 Auth log coverage is essentially zero

Only `physicalhost.EnterMaintenance` / `ExitMaintenance` call
`audit.Service.RecordAction`. Project CRUD, member
grant/revoke, device CRUD, k8s cluster CRUD, pipeline
runs, alert rules, log saved-filters, discovery runs — none
audited. `internal/audit/models.go:72` defines
`ResourceProjectMember` but it is never emitted. Estimate:
2-3 days.

### 4. 🔴 `APP_JWT_SECRET` falls back to a hard-coded dev value with only `log.Warn`

`cmd/devops-toolkit/main.go:352-356` and again at line 926
(in the WS hub signer). `config.Validate`
(`internal/config/config.go:213`) does not refuse to start
when `APP_JWT_SECRET` is unset. Same for `K8S_CRYPTO_KEY`
at main.go:643-648. `config-prod.yaml` comment claims the
loader "refuses to boot" — code does not match. Also: no
`Validate` rule that refuses `ldap.dev_bypass: true` when
`app.env == "production"`. Estimate: 2 hr.

### 5. 🔴 `/health` is "process is alive" — not deep

`main.go:230-232` returns 200 regardless of DB, LDAP, K8s,
Redis, InfluxDB state. A separate `/api/v1/auth/ldap/health`
exists, but the orchestrator endpoint is a lie. Use it as a
K8s liveness probe and you route traffic to a pod that
cannot reach its DB. Should fan out: DB ping, LDAP ping,
K8s clientset ping; return 503 on any failure. Add
`/live` (process up) and `/ready` (deps up) split. Add
`healthcheck:` block to `deploy/docker-compose.yml:24-48`.
Estimate: 1 day.

### 6. 🔴 No resource limits on any container, no healthcheck on app, hard-coded creds

`deploy/docker-compose.yml` has no `mem_limit`/`cpus` on any
service. The app container has no `healthcheck:` block. DB
`devops:devops`, Grafana `admin:admin`, LDAP `admin:admin`,
JWT `ci-secret-rotate-me` are inline. The compose is
documented dev/CI but a copy-paste to prod is plausible.
`config.Validate` does not flag this. mTLS is opt-in via
env vars. No backup/restore story. Estimate: 1 day compose
+ 1 day backup script.

### 7. 🟡 No chaos / failure-mode tests, no no-auth-shipped regression test

Nothing tests the system with: DB down, K8s unreachable,
LDAP timeout, audit emitter buffer full, InfluxDB 5xx.
`BufferedEmitter`'s drop-on-overflow is documented but
unverified under load. A test that asserts
`r.Group("/api/v1/projects").GET("", ...)` returns 401 in
production wiring would have caught the missing-middleware
bug. There is no such test because the production wiring
is missing the middleware. Estimate: 1-2 days.

## Smaller findings

- **OTLP exporter is not wired**: only stdouttrace. With
  `OTEL_EXPORTER_OTLP_ENDPOINT` set, traces vanish. 30 min.
- **WS hub `CheckOrigin = true`**: Acceptable only because
  the upgrade verifies the JWT — but the WS path is "auth via
  token in query string" with no origin lock and no
  middleware on the route group. 15 min.
- **Trace context not propagated through background workers**:
  `async_writer.go`, `emitter.go` launch goroutines with
  fresh contexts, no parent-child span link. 1 day.
- **Per-route error rate metric missing**: only total counter
  + raw histogram. No `status="5xx"` slice. 1 hr.
- **No per-service / per-cluster / per-pod metrics on
  /metrics**: the k8s module exports none. 1-2 days.
- **5xx handlers leak `err.Error()`**: `internal/k8s/handler.go:472,490,496`,
  `internal/audit/handler.go:153`, `internal/discovery/handler.go:183`,
  `internal/pipeline/handler.go:405`, `internal/k8s/logstream/handler.go:198-211,261,343`.
  Surfaces K8s client-go internals / file paths / DNS errors
  to the caller. 1 hr to fix.
- **All `panic` calls are in constructors / boot paths** —
  intentional fail-fast, not reachable in request handlers.
  Logged for completeness; not a blocker.
