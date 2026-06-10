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

## What "baseline" means here

This file is a **floor**, not a target. If a future change
pops list p95 above 25ms (still 20x under SLO), that's worth
a 10-minute look — the new code is doing something the old
wasn't, and we want to know what. If a future change pushes
it above 100ms, something is wrong even though the SLO still
holds.

When you re-run, append a new row rather than overwriting —
the trend line is the value, not the single number.
