#!/usr/bin/env bash
# scripts/lib/log.sh — shared helpers for orchestration scripts.
#
# Source from a script via:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/lib/log.sh"
#
# This file must NOT use `set -euo pipefail` (sourcing it would propagate
# failure), but every public function below is written so that callers
# running under `set -e` can use it safely.

# Color detection: only emit ANSI codes when stdout is a TTY. CI runners
# typically run with no TTY; in that case we drop colours so the log files
# under ./logs stay clean.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	C_RESET=$'\033[0m'
	C_RED=$'\033[31m'
	C_GREEN=$'\033[32m'
	C_YELLOW=$'\033[33m'
	C_BLUE=$'\033[34m'
else
	C_RESET=""
	C_RED=""
	C_GREEN=""
	C_YELLOW=""
	C_BLUE=""
fi

LOG_DIR="${LOG_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/logs}"

# log_init writes the script name into a global so log_* can route to
# ./logs/<script-name>.log. Call this at the top of every script:
#   log_init "${BASH_SOURCE[0]##*/}"
log_init() {
	SCRIPT_NAME="${1:-unknown}"
	mkdir -p "$LOG_DIR"
	# Truncate the per-script log on each run.
	: > "$LOG_DIR/$SCRIPT_NAME.log"
}

# log_to_file emits a line to the per-script log file (always, regardless of
# TTY). Each log line is prefixed with a UTC timestamp so multiple runs can be
# diffed in the same log file.
log_to_file() {
	local level="$1"
	shift
	local ts
	ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	printf '%s [%s] %s\n' "$ts" "$level" "$*" >> "$LOG_DIR/$SCRIPT_NAME.log"
}

log_step() {
	printf '%s[STEP]%s %s\n' "$C_BLUE" "$C_RESET" "$*"
	log_to_file "STEP" "$*"
}

log_ok() {
	printf '%s[ok]%s %s\n' "$C_GREEN" "$C_RESET" "$*"
	log_to_file "OK" "$*"
}

log_fail() {
	printf '%s[fail]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2
	log_to_file "FAIL" "$*"
}

log_warn() {
	printf '%s[warn]%s %s\n' "$C_YELLOW" "$C_RESET" "$*"
	log_to_file "WARN" "$*"
}

log_info() {
	printf '%s[info]%s %s\n' "$C_BLUE" "$C_RESET" "$*"
	log_to_file "INFO" "$*"
}

# log_error prints to stderr and to the log file, then returns non-zero so
# callers can `log_error ... && return 1` patterns under `set -e`.
log_error() {
	printf '%s[error]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2
	log_to_file "ERROR" "$*"
	return 1
}

# standard_usage prints the help message shared by every script. The four
# standard verbs come first; "destroy" is accepted as a legacy alias for
# "teardown" so existing caller scripts keep working.
standard_usage() {
	local script="${1:-$(basename "${BASH_SOURCE[1]:-?}")}"
	cat <<EOF
Usage: $script <verb>

Standard subcommands:
  deploy      Deploy / start this component (alias for any install step)
  teardown    Destroy / remove this component (alias: destroy)
  status      Report local health / state of this component (no side effects)
  logs        Print the tail of ./logs/$script.log
  help        Show this message

Any other argument is rejected with exit code 2 and this message on stderr.
EOF
}

# require_verb checks that a script was called with exactly one argument and
# that it is one of the five standard verbs. Returns 0 on success, 2 on usage
# error, prints standard_usage() on stderr in the latter case.
require_verb() {
	if [ "$#" -ne 1 ]; then
		log_error "expected exactly one argument (the verb)" >&2
		standard_usage "${SCRIPT_NAME:-script}" >&2
		return 2
	fi
	local verb="$1"
	case "$verb" in
		deploy | status | logs | teardown | destroy | help)
			return 0
			;;
		*)
			log_error "unknown verb: $verb" >&2
			standard_usage "${SCRIPT_NAME:-script}" >&2
			return 2
			;;
	esac
}

# dispatch_status is the default "show me local state" path. Each script
# overrides this to print component-specific info. The default
# implementation just records the last log line.
default_status() {
	log_info "$SCRIPT_NAME: status requested"
	if [ -f "$LOG_DIR/$SCRIPT_NAME.log" ]; then
		local last
		last="$(tail -n 1 "$LOG_DIR/$SCRIPT_NAME.log" 2>/dev/null || true)"
		log_info "last log line: ${last:-<no log entries>}"
	else
		log_info "no log file yet"
	fi
	return 0
}

# dispatch_logs prints the last 50 lines of the per-script log file. This is
# the spec's "logs" verb behaviour: read-only, no side effects.
default_logs() {
	if [ -f "$LOG_DIR/$SCRIPT_NAME.log" ]; then
		tail -n 50 "$LOG_DIR/$SCRIPT_NAME.log"
	else
		log_info "no log file at $LOG_DIR/$SCRIPT_NAME.log yet"
	fi
	return 0
}
