#!/usr/bin/env bash
# scripts/status.sh — show health of every environment component without
# starting anything. Pure local inspection.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_status() {
	log_step "Reporting health of all components"
	for comp in db-setup ldap-seed seed-data clab k3d-setup; do
		local logfile="$LOG_DIR/$comp.sh.log"
		if [ -f "$logfile" ]; then
			local last
			last="$(tail -n 1 "$logfile" || true)"
			log_info "$comp: ${last:-<no entries>}"
		else
			log_info "$comp: <no log file yet>"
		fi
	done
	log_ok "Status report complete (no services started)"
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: status.sh <verb>

Verbs:
  deploy      (no-op)
  teardown    (no-op)
  status      Show health of every component (read-only).
  logs        Tail the status log file.
  help        Show this message.
EOF
			;;
		deploy)
			log_warn "status.sh deploy is a no-op (use setup.sh deploy to start services)."
			;;
		teardown)
			log_warn "status.sh teardown is a no-op (use teardown.sh to remove services)."
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
