#!/usr/bin/env bash
# scripts/db-setup.sh — PostgreSQL bring-up + AutoMigrate + seed SQL apply.
# Safe on hosts without docker: it logs a warning and exits 0.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_deploy() {
	if ! command -v docker >/dev/null 2>&1; then
		log_warn "docker not installed; skipping postgres bring-up"
		log_info "local SQLite (tests/fixtures/db/dev.db) is used by config-dev.yaml"
		return 0
	fi
	log_step "Starting postgres via deploy/docker-compose.yml"
	(cd "$REPO_ROOT/deploy" && docker compose up -d postgres)
	log_step "Applying tests/fixtures/db/seed.sql"
	if command -v psql >/dev/null 2>&1; then
		PGPASSWORD=devops psql -h localhost -U devops -d devops \
			-f "$REPO_ROOT/tests/fixtures/db/seed.sql" || log_warn "psql apply failed (postgres may not be up yet)"
	else
		log_warn "psql not installed; please run: psql -f tests/fixtures/db/seed.sql"
	fi
	log_ok "DB deploy complete"
}

do_teardown() {
	if ! command -v docker >/dev/null 2>&1; then
		return 0
	fi
	(cd "$REPO_ROOT/deploy" && docker compose down -v postgres) || log_warn "compose down failed"
}

do_status() {
	if command -v pg_isready >/dev/null 2>&1; then
		if pg_isready -h localhost -p 5432 -U devops >/dev/null 2>&1; then
			log_ok "postgres: ready"
		else
			log_warn "postgres: not ready"
		fi
	else
		log_info "pg_isready not installed"
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: db-setup.sh <verb>

Verbs:
  deploy      Bring up postgres (or local SQLite fallback) and apply seed.sql.
  teardown    Stop postgres and drop its volume.
  status      Report postgres readiness via pg_isready.
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
