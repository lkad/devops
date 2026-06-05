#!/usr/bin/env bash
# scripts/clab.sh — Containerlab deploy/destroy. Two simulated physical
# hosts in deploy/containerlab/topology.yml. Safe on hosts without
# containerlab: logs a warning and returns 0.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

TOPOLOGY="$REPO_ROOT/deploy/containerlab/topology.yml"

do_deploy() {
	if [ ! -f "$TOPOLOGY" ]; then
		log_error "missing $TOPOLOGY"
		return 1
	fi
	if ! command -v containerlab >/dev/null 2>&1; then
		log_warn "containerlab not installed; skipping clab deploy"
		return 0
	fi
	log_step "Deploying containerlab topology"
	(cd "$REPO_ROOT/deploy" && containerlab deploy -t containerlab/topology.yml)
	log_ok "Containerlab topology up"
}

do_teardown() {
	if ! command -v containerlab >/dev/null 2>&1; then
		return 0
	fi
	if [ -f "$TOPOLOGY" ]; then
		(cd "$REPO_ROOT/deploy" && containerlab destroy -t containerlab/topology.yml) \
			|| log_warn "containerlab destroy failed"
	fi
}

do_status() {
	if ! command -v docker >/dev/null 2>&1; then
		log_info "docker not installed"
		return 0
	fi
	docker ps --filter "label=clab-topology" --format "  {{.Names}}\t{{.Status}}" 2>/dev/null \
		| while read -r line; do
			log_info "clab: $line"
		done || true
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: clab.sh <verb>

Verbs:
  deploy      containerlab deploy -t deploy/containerlab/topology.yml
  teardown    containerlab destroy -t deploy/containerlab/topology.yml
  status      List clab-* containers.
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
