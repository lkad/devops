#!/usr/bin/env bash
# scripts/k3d-setup.sh — k3d cluster bring-up/destroy using the spec at
# deploy/k3d/cluster.yaml. No-op on hosts without k3d.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

CLUSTER_CONFIG="$REPO_ROOT/deploy/k3d/cluster.yaml"
CLUSTER_NAME="dev-cluster-1"

do_deploy() {
	if [ ! -f "$CLUSTER_CONFIG" ]; then
		log_error "missing $CLUSTER_CONFIG"
		return 1
	fi
	if ! command -v k3d >/dev/null 2>&1; then
		log_warn "k3d not installed; skipping k3d cluster create"
		return 0
	fi
	log_step "Creating k3d cluster $CLUSTER_NAME"
	k3d cluster create --config "$CLUSTER_CONFIG"
	log_ok "k3d cluster $CLUSTER_NAME up"
}

do_teardown() {
	if ! command -v k3d >/dev/null 2>&1; then
		return 0
	fi
	k3d cluster delete "$CLUSTER_NAME" || log_warn "k3d delete failed"
}

do_status() {
	if ! command -v k3d >/dev/null 2>&1; then
		log_info "k3d not installed"
		return 0
	fi
	if k3d cluster list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
		log_ok "k3d: $CLUSTER_NAME is up"
	else
		log_warn "k3d: $CLUSTER_NAME is not running"
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: k3d-setup.sh <verb>

Verbs:
  deploy      k3d cluster create --config deploy/k3d/cluster.yaml
  teardown    k3d cluster delete dev-cluster-1
  status      Report whether dev-cluster-1 is running.
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
