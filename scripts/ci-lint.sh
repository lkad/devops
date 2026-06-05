#!/usr/bin/env bash
# scripts/ci-lint.sh — runs go vet ./... and gofmt -l . ; exits non-zero
# on any unformatted Go file.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

do_deploy() {
	log_step "Running go vet ./..."
	(
		cd "$REPO_ROOT"
		/usr/local/go/bin/go vet ./... >/tmp/ci-lint.stdout 2>/tmp/ci-lint.stderr
	) || {
		cat /tmp/ci-lint.stderr >&2
		log_error "go vet failed"
		return 1
	}
	log_ok "go vet clean"

	log_step "Running gofmt -l ."
	local unformatted
	unformatted=$(/usr/local/go/bin/gofmt -l .)
	if [ -n "$unformatted" ]; then
		log_error "the following files are not gofmt-clean:"
		printf '%s\n' "$unformatted" >&2
		return 1
	fi
	log_ok "gofmt clean"
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: ci-lint.sh <verb>

Verbs:
  deploy      go vet ./... and gofmt -l . ; fail on any unformatted file.
  teardown    (no-op)
  status      (no-op; status of lint is a one-shot run)
  logs        Tail this script's log file.
  help        Show this message.
EOF
			;;
		deploy)
			do_deploy
			;;
		teardown)
			log_warn "ci-lint.sh teardown is a no-op"
			;;
		status)
			log_info "ci-lint.sh has no persistent state; run `deploy` to lint"
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
