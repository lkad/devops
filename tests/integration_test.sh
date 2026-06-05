#!/usr/bin/env bash
# tests/integration_test.sh — wraps verify-flow.sh for the CI job. Standard
# interface: deploy|status|logs|teardown|help.
#
#   - deploy runs scripts/verify-flow.sh deploy and asserts exit 0
#   - teardown cleans up
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERIFY_FLOW="$REPO_ROOT/scripts/verify-flow.sh"
# shellcheck source=/dev/null
source "$REPO_ROOT/scripts/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

main() {
	if [ "$#" -ne 1 ]; then
		log_error "expected exactly one argument (the verb)" >&2
		cat >&2 <<EOF
Usage: integration_test.sh <verb>

Verbs:
  deploy      Run scripts/verify-flow.sh deploy and assert exit 0.
  teardown    Run scripts/verify-flow.sh teardown.
  status      (no-op)
  logs        Tail this script's log file.
  help        Show this message.
EOF
		exit 2
	fi
	local verb="$1"
	case "$verb" in
		deploy | status | logs | teardown | help) ;;
		*)
			log_error "unknown verb: $verb" >&2
			exit 2
			;;
	esac

	case "$verb" in
		help)
			cat <<EOF
Usage: integration_test.sh <verb>

Verbs:
  deploy      Run scripts/verify-flow.sh deploy and assert exit 0.
  teardown    Run scripts/verify-flow.sh teardown.
  status      (no-op)
  logs        Tail this script's log file.
  help        Show this message.
EOF
			;;
		deploy)
			log_step "Running scripts/verify-flow.sh deploy"
			"$VERIFY_FLOW" deploy
			log_ok "integration_test.sh deploy complete"
			;;
		teardown)
			"$VERIFY_FLOW" teardown
			;;
		status)
			log_info "integration_test.sh is a one-shot wrapper; run `deploy`"
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
