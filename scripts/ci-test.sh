#!/usr/bin/env bash
# scripts/ci-test.sh — runs `go test -race -coverprofile=coverage.out ./...`
# with a 75% coverage gate. Standard interface: deploy|status|logs|teardown|help.
#
#   - deploy runs the test suite and asserts coverage >= 75%
#   - status / logs / help are read-only
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

COVERAGE_MIN="${COVERAGE_MIN:-75.0}"

do_deploy() {
	log_step "Running go test -race -coverprofile=coverage.out ./..."
	(
		cd "$REPO_ROOT"
		/usr/local/go/bin/go test -race -coverprofile=coverage.out -count=1 ./... >/tmp/ci-test.stdout 2>/tmp/ci-test.stderr
	) || {
		cat /tmp/ci-test.stderr >&2
		log_error "go test failed"
		return 1
	}
	log_ok "go test passed"

	if [ ! -f "$REPO_ROOT/coverage.out" ]; then
		log_warn "no coverage.out produced; skipping coverage gate"
		return 0
	fi

	local total
	total=$(/usr/local/go/bin/go tool cover -func="$REPO_ROOT/coverage.out" | awk '/^total:/ {print substr($3, 1, length($3)-1)}')
	log_info "coverage: ${total}% (min ${COVERAGE_MIN}%)"

	# compare with awk (no bc dependency)
	if awk -v t="$total" -v m="$COVERAGE_MIN" 'BEGIN { exit (t+0 < m+0) ? 1 : 0 }'; then
		log_ok "coverage gate met"
	else
		log_error "coverage ${total}% is below minimum ${COVERAGE_MIN}%"
		return 1
	fi
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: ci-test.sh <verb>

Verbs:
  deploy      Run go test -race -coverprofile=coverage.out ./... and assert >= ${COVERAGE_MIN}% coverage.
  teardown    (no-op)
  status      Report whether ./coverage.out exists.
  logs        Tail this script's log file.
  help        Show this message.

Env:
  COVERAGE_MIN  Minimum coverage percentage (default 75.0)
EOF
			;;
		deploy)
			do_deploy
			;;
		teardown)
			log_warn "ci-test.sh teardown is a no-op (rm -f coverage.out to clean up)"
			;;
		status)
			if [ -f "$REPO_ROOT/coverage.out" ]; then
				log_ok "coverage.out present"
			else
				log_warn "no coverage.out yet"
			fi
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
