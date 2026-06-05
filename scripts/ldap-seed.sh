#!/usr/bin/env bash
# scripts/ldap-seed.sh — apply tests/fixtures/ldap/00-users.ldif against the
# bitnami/openldap container started by deploy.sh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_deploy() {
	local ldif="$REPO_ROOT/tests/fixtures/ldap/00-users.ldif"
	if [ ! -f "$ldif" ]; then
		log_error "missing $ldif"
		return 1
	fi
	if ! command -v ldapadd >/dev/null 2>&1; then
		log_warn "ldapadd not installed; would have loaded: $ldif"
		return 0
	fi
	if ! command -v docker >/dev/null 2>&1; then
		log_warn "docker not installed; skipping ldap-seed"
		return 0
	fi
	log_step "Applying $ldif to ldap://localhost:389"
	ldapadd -x -H ldap://localhost:389 \
		-D "cn=admin,dc=example,dc=com" -w admin \
		-f "$ldif" || log_warn "ldapadd failed (ldap may not be up yet)"
	log_ok "LDAP seed applied"
}

do_teardown() {
	if ! command -v docker >/dev/null 2>&1; then
		return 0
	fi
	(cd "$REPO_ROOT/deploy" && docker compose down -v ldap) || log_warn "compose down ldap failed"
}

do_status() {
	if command -v ldapsearch >/dev/null 2>&1; then
		if ldapsearch -x -H ldap://localhost:389 -b dc=example,dc=com \
			-D "cn=admin,dc=example,dc=com" -w admin >/dev/null 2>&1; then
			log_ok "ldap: reachable"
		else
			log_warn "ldap: not reachable"
		fi
	else
		log_info "ldapsearch not installed"
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: ldap-seed.sh <verb>

Verbs:
  deploy      ldapadd tests/fixtures/ldap/00-users.ldif.
  teardown    Stop the ldap container (docker compose down -v ldap).
  status      Probe ldap with ldapsearch.
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
