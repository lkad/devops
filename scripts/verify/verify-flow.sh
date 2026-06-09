#!/bin/bash
# verify-flow.sh — end-to-end smoke test against a running stack.
# Usage: ./scripts/verify/verify-flow.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/log.sh"

BASE_URL="${BASE_URL:-http://localhost:3000}"
TOKEN="${TOKEN:-dev-bypass}"
HDR=(-H "Authorization: Bearer ${TOKEN}")

main() {
  log_step "End-to-end flow check"

  # 1. List physical hosts.
  local list
  list=$(curl -sf "${HDR[@]}" "${BASE_URL}/api/v1/physical-hosts?limit=20")
  log_ok "List: $(echo "$list" | head -c 80)..."

  # 2. If any host exists, exercise Get + Metrics + Maintenance
  #    endpoints. With no seeded data, the create-host path
  #    runs instead.
  local id
  id=$(echo "$list" | grep -oE '"id":"[^"]+"' | head -1 | cut -d'"' -f4)

  if [[ -z "$id" ]]; then
    log_step "No seeded host — creating one for the smoke test"
    id=$(curl -sf "${HDR[@]}" -H "Content-Type: application/json" \
      -d '{"device_id":"verify-flow","ip_address":"127.0.0.1","ssh_user":"root","ssh_port":22}' \
      "${BASE_URL}/api/v1/physical-hosts" | grep -oE '"id":"[^"]+"' | head -1 | cut -d'"' -f4)
    log_ok "Created host $id"
  fi

  # 3. Get + Metrics + maintenance-history.
  curl -sf "${HDR[@]}" "${BASE_URL}/api/v1/physical-hosts/${id}" >/dev/null
  log_ok "Get ${id}"

  curl -sf "${HDR[@]}" "${BASE_URL}/api/v1/physical-hosts/${id}/metrics" >/dev/null
  log_ok "Metrics ${id}"

  curl -sf "${HDR[@]}" "${BASE_URL}/api/v1/physical-hosts/${id}/maintenance-history" >/dev/null
  log_ok "Maintenance history ${id}"

  # 4. Maintenance flow (uses X-User header for actor; bypass auth).
  curl -sf "${HDR[@]}" -H "Content-Type: application/json" \
    -d '{"reason":"verify-flow smoke"}' \
    "${BASE_URL}/api/v1/physical-hosts/${id}/maintenance" >/dev/null
  log_ok "Entered maintenance"

  curl -sf "${HDR[@]}" -X POST "${BASE_URL}/api/v1/physical-hosts/${id}/maintenance/exit" >/dev/null
  log_ok "Exited maintenance"

  log_ok "End-to-end flow complete"
}

main "$@"
