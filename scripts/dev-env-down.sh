#!/usr/bin/env bash
# scripts/dev-env-down.sh — counterpart to dev-env-up.sh.
# Brings down containerlab + docker compose, leaves prober key
# (regenerate with dev-env-up.sh on next start).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"
log_init "$(basename "${BASH_SOURCE[0]}")"

DEPLOY_DIR="$REPO_ROOT/deploy"

if command -v containerlab >/dev/null 2>&1; then
	log_step "Destroying containerlab topology"
	(cd "$DEPLOY_DIR" && containerlab destroy -t containerlab/topology.yml --cleanup) \
		|| log_warn "containerlab destroy failed (continuing)"
else
	log_warn "containerlab not installed; skipping clab teardown"
fi

if command -v docker >/dev/null 2>&1; then
	log_step "Bringing down docker compose (with -v to drop volumes)"
	(cd "$DEPLOY_DIR" && docker compose down -v) \
		|| log_warn "docker compose down failed (continuing)"
else
	log_warn "docker not installed; skipping compose teardown"
fi

log_ok "Dev env down. Prober key kept at deploy/secrets/prober/ — delete manually if you want a fresh one on next start."
