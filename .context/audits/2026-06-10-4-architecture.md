# Audit 4 — Architectural smells / improvements (v0.2.0.0)

Captured 2026-06-10. Read-only. Focus: structural
improvements, not features. Full report at the agent JSONL
log; this is the decision-relevant summary.

## Top 7 architectural improvements

Severity: 🔴 = blocks scaling · 🟡 = slows future work · 🟢 = polish

### 1. 🔴 `monitor_loop.go:56-72` runs with `context.Background()`

The physical-host monitor loop is started with
`context.Background()` in `main.go:542`. On SIGTERM the loop
is not cancelled before `srv.Shutdown(ctx)` returns. On a
200-host fleet the per-host 5s sequential check can hold
for 1000s+ after shutdown. No rate limiting beyond per-host
5s. No Prometheus metric for `loop_iterations_total`,
`loop_errors_total`, `loop_last_tick_timestamp_seconds`.

Fix: `signal.NotifyContext` shared with the HTTP server;
pass ctx to `MonitorLoop.Run` and the per-host `Check`;
add a single Counter + Gauge to `internal/observability/metrics.go`.
Estimate: 1 day.

### 2. 🔴 Layering cracks in 3 packages

The repo's load-bearing invariant is "handler → service →
repository → model" (documented in `project/handler.go:18-19`,
`hostproject/handler.go:17-18`). Cracks:

- `internal/servicecatalog/handler.go:183,190` — handler
  reaches `h.cat.repo.CurrentOnCall(...)` and
  `h.cat.repo.ListRunbook(...)`. 30 min to fix.
- `internal/physicalhost/handler.go:188,205,229,253,273,287,305,368`
  — 8 direct `h.repo.*` calls. 1 day to introduce a
  `physicalhost.Service`.
- `internal/project/handler.go:198` — `children` reaches
  `h.repo.List` directly. 15 min.

Decision: either (a) move all CRUD through services, 3-5
days, medium risk; or (b) accept the exception and document
it, 1 hour. Recommend (a).

### 3. 🟡 Split the k8s `Client` interface (6 methods → 3 sub-interfaces)

`internal/k8s/client.go:139-178` — 6 methods, all
consumed by the same `Service` (which uses 5). The
`servicecatalog` walker (`cmd/devops-toolkit/main.go:805-817`)
already narrows to a single method via an inline adapter;
that workaround should be promoted into the k8s package.

Split into `Lister` (List* + Ping), `LogReader`
(GetLogsBySelector), `Exec` (ExecInPod). FakeClient
naturally splits too. 2-3 hours + test updates.

### 4. 🟡 `AsyncInfluxWriter` constructor has side effects

`internal/physicalhost/async_writer.go:48-79` spins
workers in the constructor. `Start` is a no-op kept for
"symmetry". This means: forget to wire `Enqueue` before
any producer calls it, the workers are already running
against a never-closed channel. Workers use
`context.Background()` always, so a stopped InfluxDB is
hammered for the lifetime of the process. Reorder:
constructor = allocate channel + counter, `Start(ctx)` =
spin workers; pass ctx into `flushOne`. Add a drop counter
to `internal/observability/metrics.go`. 1 day.

### 5. 🟡 Eleven near-identical `Repository` structs + eight `writeAPIError` adapters

GORM-only `Repository` structs in 11 packages: each does
the same `Create / Get / List / Update / Delete` skeleton
translating `gorm.ErrRecordNotFound` into a package-local
`ErrNotFound`. Highest-value, lowest-risk wins:

- (a) `writeAPIError` → `handler.WriteAPIError(w, err)` in
  `internal/handler/response.go`, delete 8 copies. 1 hr.
- (b) `database.MapNotFound(err, sentinelErr)` helper. 30
  min.
- (c) `List + total` count+find pattern, 1 helper, 1 day.

Bigger "generic repo" refactor with generics: 1 week,
uncertain payoff. Skip for now.

### 6. 🟡 Dev-default secrets scattered across `main.go`

`APP_JWT_SECRET` default at main.go:354, 926 (TWO places —
if constant ever changes, one will be missed). `K8S_CRYPTO_KEY`
default at main.go:643. `INFLUX_*` at 506-518. `LOG_STORAGE_DIR`
at 870. `PROBER_*` at 288-319. Each has a "warn and fall
back" pattern but the pattern is implemented inline 8
times. Extract a `secrets.Loader` (or just a `const ( ... )`
block at the top of main.go) and a small `envOrWarn(key,
fallback)` helper. 1 file, ~30 lines. Zero regression
risk.

### 7. 🟢 49 MB Go binary at `/mnt/devops/devops-toolkit` is in the repo

`/devops-toolkit` binary in the repo root (not
`cmd/devops-toolkit/devops-toolkit` — the gitignore already
catches that). Add to `.gitignore` and `git rm`. 1 min.

## Smaller findings (mention, don't act yet)

- **N+1 walk in `servicecatalog/multicluster_k8s.go`**:
  iterates clusters sequentially. Fine for 5 clusters; add
  a comment before someone wires 100. 5 min.
- **`Hub.Publish` TOCTOU** (`hub.go:202-216`): the second
  channel send is unconditional. Switch to select with
  `default: drop`. 30 min.
- **`BufferedEmitter` drain race** (`emitter.go:177-201`):
  inner-drain `for + select{default:return}` can miss
  events enqueued concurrently. Switch to standard
  `for { select{...} }` pattern. 2 hr.
- **`/devops-toolkit/registry.go` and `registry_test.go`
  untracked**: they should be committed; main.go
  references `k8s.NewClientRegistry`. 1 min.
- **`Config.String()` masking list is incomplete**: masking
  only `password`; misses `kubeconfig`, `bind_password`,
  `token`, `secret`. Move the set of names to `pkg/logger`. 1
  hr.
- **No concurrent-access test for `defaultRegistry`**: 30
  min to add `TestClientRegistry_ConcurrentFirstCallDecryptsOnce`.

## Bottom line

The codebase is **well-layered overall**: services avoid
importing `gin`, no import cycles, the `RWMutex` choices in
`ws/hub` and `physicalhost/cache` are correct, the
`audit.BufferedEmitter` is idiomatic. The two real bugs are
`Hub.Publish` and the `monitor_loop` `context.Background()`
trap. The cleanest, highest-leverage wins: 8 `writeAPIError`
duplicates → 1 helper (1 hr, zero risk), and the monitor
loop context fix (1 day, blocks scaling).
