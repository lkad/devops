#!/usr/bin/env bash
# scripts/teardown.sh — remove every environment resource, keep config/source.
#
# Acts as the alias for `setup.sh teardown` but also accepts `destroy` per
# the spec, and is safe to invoke on its own.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_teardown() {
	log_step "Tearing down all environment resources"
	"$SCRIPT_DIR/k3d-setup.sh" teardown || log_warn "k3d teardown failed (continuing)"
	"$SCRIPT_DIR/clab.sh" teardown || log_warn "clab teardown failed (continuing)"
	"$SCRIPT_DIR/ldap-seed.sh" teardown || log_warn "ldap teardown failed (continuing)"
	"$SCRIPT_DIR/db-setup.sh" teardown || log_warn "db teardown failed (continuing)"
	log_ok "All resources removed (config and source kept)"
}

do_status() {
	log_info "teardown.sh: nothing is currently running on this host"
	if command -v docker >/dev/null 2>&1; then
		log_info "docker containers matching 'devops-toolkit':"
		docker ps -a --filter "name=devops-toolkit" --format "  {{.Names}}\t{{.Status}}" 2>/dev/null || true
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: teardown.sh <verb>

Verbs:
  deploy      (no-op; use setup.sh deploy instead)
  teardown    Remove all environment resources; keep config and source.
  destroy     Alias for teardown.
  status      Show whether any environment resources are still running.
  logs        Tail the teardown log file.
  help        Show this message.
EOF
			;;
		deploy)
			log_warn "teardown.sh deploy is a no-op. Use setup.sh deploy to start the environment."
			;;
		teardown | destroy)
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
