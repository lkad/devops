#!/usr/bin/env bash
# scripts/deploy.sh — argument-driven dispatcher that fronts the
# `docker compose up -d` stack under deploy/.
#
# Standard interface: deploy|status|logs|teardown|help.
# `deploy` starts the support-service stack (postgres, loki, promtail,
# prometheus, grafana, alertmanager, ldap, redis, devops-toolkit).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_deploy() {
	if ! command -v docker >/dev/null 2>&1; then
		log_warn "docker not installed; nothing to do (test env only)"
		return 0
	fi
	log_step "Bringing up docker-compose stack under deploy/"
	(cd "$REPO_ROOT/deploy" && docker compose up -d)
	log_ok "Stack up"
}

do_teardown() {
	if ! command -v docker >/dev/null 2>&1; then
		log_warn "docker not installed; nothing to do"
		return 0
	fi
	log_step "Bringing down docker-compose stack (with -v)"
	(cd "$REPO_ROOT/deploy" && docker compose down -v)
	log_ok "Stack down"
}

do_status() {
	if ! command -v docker >/dev/null 2>&1; then
		log_info "docker not installed"
		return 0
	fi
	if [ -f "$REPO_ROOT/deploy/docker-compose.yml" ]; then
		(cd "$REPO_ROOT/deploy" && docker compose ps) || true
	else
		log_info "no deploy/docker-compose.yml"
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: deploy.sh <verb>

Verbs:
  deploy      Start the support-service stack via docker compose up -d.
  teardown    Stop the stack and remove volumes (docker compose down -v).
  status      Print docker compose ps.
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
