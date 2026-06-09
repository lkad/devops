#!/bin/bash
# verify-conn.sh — check that every support service is up.
# Usage: ./scripts/verify/verify-conn.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${B_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/log.sh"

check() {
  local name="$1"
  local cmd="$2"
  if eval "$cmd" >/dev/null 2>&1; then
    log_ok "$name"
  else
    log_fail "$name"
    return 1
  fi
}

main() {
  log_step "Verifying connectivity to all support services"

  check "PostgreSQL" "pg_isready -h localhost -p 5432 -U devops"
  check "Loki"        "curl -sf http://localhost:3100/ready"
  check "ES"          "curl -sf http://localhost:9200/_cluster/health"
  check "Prometheus"  "curl -sf http://localhost:9090/-/ready"
  check "InfluxDB"    "curl -sf http://localhost:8086/health"
  check "LDAP"        "ldapsearch -x -H ldap://localhost:389 -b dc=example,dc=com -s base 2>/dev/null"
  check "App health"  "curl -sf http://localhost:3000/health"

  if command -v containerlab >/dev/null 2>&1; then
    check "Containerlab" "docker ps | grep -q clab-devops-toolkit-test"
  fi
  if command -v k3d >/dev/null 2>&1; then
    check "k3d dev-cluster-1" "k3d cluster list | grep -q dev-cluster-1"
  fi

  log_ok "All connectivity checks passed"
}

main "$@"
