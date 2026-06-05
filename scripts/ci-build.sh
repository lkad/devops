#!/usr/bin/env bash
# scripts/ci-build.sh — runs `make build` and asserts the produced binary is
# under 50 MB.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"

log_init "$(basename "${BASH_SOURCE[0]}")"

MAX_MB="${MAX_MB:-50}"

do_deploy() {
	log_step "make build"
	(
		cd "$REPO_ROOT"
		make build >/tmp/ci-build.stdout 2>/tmp/ci-build.stderr
	) || {
		cat /tmp/ci-build.stderr >&2
		log_error "make build failed"
		return 1
	}
	log_ok "make build succeeded"

	local bin="$REPO_ROOT/bin/devops-toolkit"
	if [ ! -f "$bin" ]; then
		log_error "expected $bin not found after make build"
		return 1
	fi

	local size_mb
	size_mb=$(du -m "$bin" | awk '{print $1}')
	log_info "binary size: ${size_mB} MB (max ${MAX_MB} MB)"

	if [ "${size_mb}" -gt "${MAX_MB}" ]; then
		log_error "binary ${size_mb} MB exceeds max ${MAX_MB} MB"
		return 1
	fi
	log_ok "binary size within budget"
}

main() {
	require_verb "$@"
	local verb="$1"
	shift || true

	case "$verb" in
		help)
			cat <<EOF
Usage: ci-build.sh <verb>

Verbs:
  deploy      make build ; assert bin/devops-toolkit < ${MAX_MB} MB.
  teardown    (no-op; `make clean` removes the binary)
  status      (no-op)
  logs        Tail this script's log file.
  help        Show this message.

Env:
  MAX_MB  Maximum allowed binary size in MB (default 50)
EOF
			;;
		deploy)
			do_deploy
			;;
		teardown)
			log_warn "ci-build.sh teardown is a no-op (run `make clean` to remove the binary)"
			;;
		status)
			log_info "ci-build.sh has no persistent state; run `deploy` to build"
			;;
		logs)
			default_logs
			;;
	esac
}

main "$@"
