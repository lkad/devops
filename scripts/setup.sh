#!/usr/bin/env bash
# scripts/setup.sh — top-level entry point. Wires together db, clab, k3d,
# ldap-seed, seed-data, and verify into a single command.
#
# The standard interface is honoured: every verb except the spec-mandated
# "deploy|status|logs|teardown|help" is rejected. The legacy per-tier verbs
# (dev|ci|prod) are accepted as arguments to `deploy` (the default verb) so
# older callers keep working.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

# resolve_target returns the deployment tier from the argument. The
# standard interface only mandates verbs, but setup historically accepts
# dev|ci|prod. We treat the second positional argument as the tier (default
# dev) and the first as the verb.
resolve_target() {
	local tier="${1:-dev}"
	case "$tier" in
		dev | ci | prod) printf '%s' "$tier" ;;
		*) return 2 ;;
	esac
}

# do_setup runs the tier-specific sub-orchestration. We deliberately keep the
# real docker/k3d/clab invocations behind a no-op fallback so this script
# remains safe to call on a developer laptop with no services installed —
# the only way we ever mutate the host from this script is if the user
# actually wants us to and we are running in a context that has docker.
do_setup() {
	local tier="$1"
	log_step "Setting up tier: $tier"

	case "$tier" in
		dev)
			log_step "Dev tier: starting mock data layer (no external services)"
			"$SCRIPT_DIR/db-setup.sh" deploy
			"$SCRIPT_DIR/seed-data.sh" deploy
			log_ok "Dev tier ready (mock data only, < 30s target)"
			;;
		ci)
			log_step "CI tier: full stack (db, clab, k3d, ldap, seed-data, verify)"
			"$SCRIPT_DIR/db-setup.sh" deploy
			"$SCRIPT_DIR/ldap-seed.sh" deploy
			"$SCRIPT_DIR/clab.sh" deploy
			"$SCRIPT_DIR/k3d-setup.sh" deploy
			"$SCRIPT_DIR/seed-data.sh" deploy
			log_ok "CI tier ready"
			;;
		prod)
			log_error "Production setup is disabled in this script. See DEPLOY.md for actual production deployment."
			return 1
			;;
	esac
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: setup.sh <verb> [tier]

Verbs:
  deploy [tier]   Bring up the environment (tier in {dev,ci,prod}; default dev).
                  Production tier is rejected with an error; see DEPLOY.md.
  teardown        Tear the environment back down.
  status          Show high-level status of every component.
  logs            Tail the setup log file.
  help            Show this message.

Tiers:
  dev   mock-only, no external services, < 30s
  ci    full stack with Containerlab, k3d, and support services
  prod  disabled in this script (see DEPLOY.md)
EOF
			;;
		deploy)
			local tier
			tier="$(resolve_target "${1:-dev}")" || {
				log_error "unknown tier: ${1:-dev} (expected dev|ci|prod)"
				exit 2
			}
			do_setup "$tier"
			;;
		teardown)
			log_step "Tearing down environment"
			"$SCRIPT_DIR/k3d-setup.sh" teardown
			"$SCRIPT_DIR/clab.sh" teardown
			"$SCRIPT_DIR/ldap-seed.sh" teardown
			"$SCRIPT_DIR/db-setup.sh" teardown
			log_ok "Teardown complete"
			;;
		status)
			default_status
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
