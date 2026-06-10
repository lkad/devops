# Load test baseline — physical-hosts

Numbers from a fresh build against a fresh sqlite DB
(`config-ci.yaml` shipped in `configs/templates/`). 3 hosts seeded
via `POST /api/v1/physical-hosts`; the loadgen hits List, Get
(per host), and Metrics (per host) at the ratios the test
defines.

Captured **2026-06-10** on the local dev box
(`/mnt/devops`, commit short `2d650276` on `main`).

## How to reproduce

```bash
# 1. Build the binary.
go build -o /tmp/devops-toolkit ./cmd/devops-toolkit

# 2. Start it with the CI config (sqlite, dev_bypass, /health on :8081).
CONFIG_PATH=./configs/templates/config-ci.yaml GIN_MODE=release \
  /tmp/devops-toolkit > /tmp/dt.log 2>&1 &
BIN=$!

# 3. Seed 3 hosts (any device_id works; we use loadtest-dev-1..3).
for i in 1 2 3; do
  curl -sS -X POST -H "Authorization: Bearer test_admin" -H "Content-Type: application/json" \
    http://localhost:8081/api/v1/physical-hosts \
    -d "{\"device_id\":\"loadtest-dev-$i\",\"ip_address\":\"10.0.0.$i\",\"ssh_port\":22,\"ssh_user\":\"root\",\"state\":\"online\"}" \
    > /dev/null
done

# 4. Run the load test (50 VUs / 20s matches the SLO contract).
LOAD_BASE_URL=http://localhost:8081 LOAD_TOKEN=test_admin \
  LOAD_VUS=50 LOAD_DURATION=20s \
  go test -tags=load -count=1 -run TestLoadPhysicalHosts \
  -v ./tests/load/... 2>&1 | tail -20

# 5. Stop the binary.
kill $BIN
```

## SLO contract (encoded in the test)

- `errRate < 1%`
- `p95_list < 500ms`

Both are `t.Errorf` so a regression fails the run. The test
fails CI, not just produces a worse number.

## Results

| Run | VUs | Duration | Total reqs | Errors | Err rate | List p50 | List p95 | Metrics p95 | Verdict |
|-----|-----|----------|------------|--------|----------|----------|----------|-------------|---------|
| 1 (SLO contract) | 50  | 20s | 2,000  | 0 | 0.000 | 2ms | **11ms** | 4ms | ✅ PASS |
| 2 (stress)        | 100 | 30s | 6,000  | 0 | 0.000 | 1ms | **6ms**  | 2ms | ✅ PASS |
| 3 (peak)          | 200 | 30s | 12,000 | 0 | 0.000 | 1ms | **7ms**  | 4ms | ✅ PASS |

**p95 list never exceeded 11ms across any run.** SLO is
500ms. **Headroom: ~45x at the SLO contract (50 VUs), ~80x at
peak (200 VUs).** Even at 200 VUs, the binary is well under
saturation: no errors, no climbing tail.

## Caveats

- Single box, single process. Production load will add cross-
  network latency, TLS termination, multiple replicas behind
  a load balancer, and concurrent /services traffic — all of
  which are additive to the in-process numbers above. Treat
  these as a floor, not a ceiling.
- The binary runs with `APP_JWT_SECRET` unset (dev-only
  fallback) and the K8s crypto key SHA-256 derived (also dev
  fallback). The auth + K8s paths are exercised, just not
  with full crypto overhead. Real production will add a few
  percent latency here.
- The 3-host dataset is intentionally tiny so the in-memory
  SQLite is the bottleneck, not DB I/O. Re-running with a
  real Postgres + a 1,000-host dataset is a separate
  benchmark and is the right next step before any production
  capacity planning.
- The `metrics` endpoint (per-host) is currently a stub that
  returns `[]`. Numbers reflect the wrapper overhead, not a
  real metrics query. Once the physical-host metrics path
  hits InfluxDB in production, re-run to capture the real
  read path.

## Postgres + 1,000-host run (added 2026-06-10)

Same binary + load test, but with a real Postgres 15 backend
(`docker run postgres:15`, `max_open_conns: 50`) and a 1,000-
host dataset (`configs/templates/config-postgres-loadtest.yaml`).
Seed was a single `\copy` for the dataset (no per-row GORM
overhead); the load test itself is identical to the
sqlite runs.

| Run | VUs | Total reqs | Errors | Err rate | List p50 | List p95 | Metrics p95 | Verdict |
|-----|-----|------------|--------|----------|----------|----------|-------------|---------|
| 4   | 200 | 11,808 | 2 | 0.017% | 4ms | **26ms** | 7ms | ✅ PASS |
| 5   | 300 | 17,510 | 5 | 0.029% | 5ms | **41ms** | 13ms | ✅ PASS |
| 6   | 500 | 26,751 | 56 | 0.21% | 19ms | **117ms** | 46ms | ✅ PASS (redline) |

**PG headroom: 19x at 200 VUs, 12x at 300 VUs, 4x at 500 VUs.**

The 500 VUs run is the **redline** — 0.21% error rate is
within the 1% SLO but very close, and p95 list of 117ms is
1/4 of the SLO. Suspected bottleneck: the 50-connection
pool. A future tuning pass should bump `max_open_conns` to
~150 and re-run to see whether the connection pool is the
bottleneck or something else.

### Diff: PG vs sqlite (200 VUs)

| Metric | sqlite (3 hosts) | PG (1000 hosts) | Delta |
|--------|------------------|-----------------|-------|
| List p95 | 7ms | 26ms | +19ms |
| Metrics p95 | 4ms | 7ms | +3ms |
| Errors | 0 | 2 | +2 |
| Throughput | 400 req/s | 393 req/s | -2% |

The PG hit is real but small: GORM is doing more work (real
SQL, real indices, real network to a separate container) but
the dataset is 333x larger and the throughput is unchanged.
The List p95 delta of 19ms is the cost of the extra index
page reads — still 19x under SLO, no operational concern.

## What "baseline" means here

This file is a **floor**, not a target. If a future change
pops list p95 above 25ms (still 20x under SLO), that's worth
a 10-minute look — the new code is doing something the old
wasn't, and we want to know what. If a future change pushes
it above 100ms, something is wrong even though the SLO still
holds.

When you re-run, append a new row rather than overwriting —
the trend line is the value, not the single number.
