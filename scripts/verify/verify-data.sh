#!/bin/bash
# verify-data.sh — sanity-check seeded business data.
# Usage: ./scripts/verify/verify-data.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/log.sh"

PG="psql -h localhost -U devops -d devops -t -A"

check_count() {
  local name="$1"
  local table="$2"
  local min="$3"
  local actual
  actual="$($PG -c "SELECT COUNT(*) FROM $table")"
  if [[ "$actual" -ge "$min" ]]; then
    log_ok "$name: $actual (>= $min)"
  else
    log_fail "$name: $actual rows, want >= $min"
  fi
}

main() {
  log_step "Verifying seeded data"

  check_count "physical_hosts"   "physical_hosts"   0
  check_count "devices"          "devices"          0
  check_count "projects"         "projects"         0
  check_count "project_types"    "project_types"    0
  check_count "audit_logs"       "audit_logs"       0

  log_ok "Data verification complete"
}

main "$@"
