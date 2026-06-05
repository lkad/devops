#!/usr/bin/env bash
# scripts/reset.sh — clear data (PostgreSQL, Loki, ES, InfluxDB), keep services
# running. Spec requires an interactive confirm prompt unless NONINTERACTIVE=1.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_reset() {
	if [ -z "${NONINTERACTIVE:-}" ]; then
		printf "This will DELETE all data in postgres, loki, es, influxdb.\n"
		printf "Continue? (yes/no): "
		read -r confirm
		if [ "${confirm:-}" != "yes" ]; then
			log_info "Reset aborted by user (entered: '${confirm:-}')"
			exit 1
		fi
	else
		log_warn "NONINTERACTIVE=1: skipping confirm prompt"
	fi

	log_step "Resetting data layer (services stay running)"
	"$SCRIPT_DIR/db-setup.sh" deploy
	"$SCRIPT_DIR/seed-data.sh" deploy
	log_ok "Data layer reset complete"
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: reset.sh <verb>

Verbs:
  deploy      Reset data with confirm prompt (or NONINTERACTIVE=1).
  teardown    Alias for deploy.
  status      Show what reset would touch.
  logs        Tail the reset log file.
  help        Show this message.
EOF
			;;
		deploy | teardown)
			do_reset
			;;
		status)
			log_info "reset.sh would clear: postgres, loki, es, influxdb"
			log_info "reset.sh would keep: docker containers, k3d cluster, clab topology"
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
