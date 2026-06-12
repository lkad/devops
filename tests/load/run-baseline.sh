#!/bin/bash
# run-baseline.sh — run the physical-hosts load test
# baseline. Tries k6 first (the canonical load tester);
# falls back to the Go-native loadgen if k6 is not
# installed. The two paths produce equivalent SLO
# assertions; whichever runs, the contract is the same
# (p95 < 500ms, err rate < 1%).
#
# Required env:
#   BASE_URL — server root, e.g. http://localhost:3000
#   TOKEN    — bearer token (dev bypass works in dev)
#   VUS, DURATION — passed to k6 (defaults 50 / 30s)
#
# Usage:
#   BASE_URL=http://localhost:3000 TOKEN=dev-bypass \
#     ./tests/load/run-baseline.sh
#
# The script intentionally does NOT start or stop the
# server. Operators seed hosts + start the binary
# themselves (see tests/load/baseline.md "How to
# reproduce" for the canonical recipe).

set -euo pipefail

cd "$(dirname "$0")/../.."

if command -v k6 >/dev/null 2>&1; then
  echo "[run-baseline] k6 found at $(command -v k6); running k6-physicalhost.js"
  BASE_URL="${BASE_URL:-http://localhost:3000}" \
  TOKEN="${TOKEN:-dev-bypass}" \
  k6 run \
    --vus "${VUS:-50}" \
    --duration "${DURATION:-30s}" \
    tests/load/k6-physicalhost.js
else
  echo "[run-baseline] k6 not installed; falling back to Go-native loadgen"
  if [[ -z "${BASE_URL:-}" || -z "${TOKEN:-}" ]]; then
    echo "[run-baseline] WARN: BASE_URL and TOKEN must be set for the Go-native path" >&2
    echo "[run-baseline] the Go test reads them from LOAD_BASE_URL / LOAD_TOKEN" >&2
  fi
  LOAD_BASE_URL="${BASE_URL:-${LOAD_BASE_URL:-http://localhost:3000}}" \
  LOAD_TOKEN="${TOKEN:-${LOAD_TOKEN:-dev-bypass}}" \
  LOAD_VUS="${VUS:-${LOAD_VUS:-50}}" \
  LOAD_DURATION="${DURATION:-${LOAD_DURATION:-20s}}" \
    go test -tags=load -count=1 -run TestLoadPhysicalHosts \
      -v ./tests/load/...
fi
