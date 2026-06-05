#!/usr/bin/env bash
# scripts/seed-data.sh — drive the devops-toolkit REST API to seed
# 1 BL, 1 System, 1 Project, 2 Devices, 1 K8s cluster. Uses dev_bypass auth.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

API_BASE="${API_BASE:-http://localhost:3000/api/v1}"

post_json() {
	local path="$1"
	local body="$2"
	if ! command -v curl >/dev/null 2>&1; then
		log_warn "curl not installed; cannot POST $path"
		return 0
	fi
	local code
	code=$(curl -s -o /dev/null -w "%{http_code}" \
		-X POST -H "Content-Type: application/json" \
		-d "$body" "$API_BASE$path" || echo 000)
	if [ "$code" -ge 200 ] && [ "$code" -lt 300 ]; then
		log_ok "POST $path -> $code"
	else
		log_warn "POST $path -> $code (server may not be up yet)"
	fi
}

do_deploy() {
	log_step "Seeding via REST API at $API_BASE"
	post_json "/business-lines" '{"id":"bl-ecommerce","name":"E-Commerce","weight":0.6}'
	post_json "/systems" '{"id":"sys-order","name":"Order System","business_line_id":"bl-ecommerce"}'
	post_json "/projects" '{"id":"proj-order-backend","name":"order-backend","system_id":"sys-order","type_id":"pt-backend"}'
	post_json "/devices" '{"id":"dev-dc1-web-21","name":"dc1-web-21","type":"physical_host","ip":"172.30.30.21","dc":"dc1"}'
	post_json "/devices" '{"id":"dev-dc2-web-41","name":"dc2-web-41","type":"physical_host","ip":"172.30.30.41","dc":"dc2"}'
	post_json "/kubernetes/clusters" '{"id":"cl-dev","name":"dev-cluster","api_server":"https://kubernetes.default.svc:443"}'
	log_ok "Seed-data complete"
}

do_teardown() {
	log_warn "seed-data.sh teardown is a no-op (use reset.sh to wipe data)"
}

do_status() {
	log_info "seed-data.sh: would seed 1 BL + 1 System + 1 Project + 2 Devices + 1 K8s cluster"
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: seed-data.sh <verb>

Verbs:
  deploy      POST seed entities to the devops-toolkit REST API.
  teardown    (no-op)
  status      Describe what would be seeded.
  logs        Tail this script's log file.
  help        Show this message.
EOF
			;;
		deploy)
			do_deploy
			;;
		teardown)
			do_teardown
			;;
		status)
			do_status
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
