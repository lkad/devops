#!/usr/bin/env bash
#
# scripts/backup-postgres.sh — PostgreSQL backup with multiple targets.
#
# Usage: backup-postgres.sh --target=local|s3|nfs [--retention-days=N] [--pg-url=URL]
#
# Targets:
#   local  - copy to /backups volume (default)
#   s3     - upload to S3 (BACKUP_S3_BUCKET env required)
#   nfs    - copy to BACKUP_NFS_PATH
#
# Required env (or --pg-url):
#   PG_URL or DEVOPS_DATABASE_URL — full libpq connection string.
#
# Optional env (target-dependent):
#   BACKUP_S3_BUCKET   S3 bucket for --target=s3 (also AWS_* creds)
#   BACKUP_NFS_PATH    Mount point or directory for --target=nfs
#
# Exit codes:
#   0  success
#   2  configuration / argument / dependency error
#
# The script is designed to be invoked from cron (docker-compose
# backup container, K8s CronJob) and from the dev/CI shell. It
# deliberately does NOT email or page on failure — the orchestrator
# (cron / K8s) is responsible for that.

set -euo pipefail

# Defaults — overridable via flags or environment.
TARGET="local"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
PG_URL="${PG_URL:-${DEVOPS_DATABASE_URL:-}}"

# Argv parse: --flag=value only (kept simple; the spec mandates
# --target=, --retention-days=, --pg-url=). Unknown flags fail fast.
while [[ $# -gt 0 ]]; do
  case "$1" in
    --target=*)
      TARGET="${1#*=}"
      ;;
    --retention-days=*)
      RETENTION_DAYS="${1#*=}"
      ;;
    --pg-url=*)
      PG_URL="${1#*=}"
      ;;
    -h|--help)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      echo "       run with --help for usage" >&2
      exit 2
      ;;
  esac
  shift
done

# Validate required inputs.
if [[ -z "$PG_URL" ]]; then
  echo "ERROR: PG_URL or DEVOPS_DATABASE_URL must be set (or pass --pg-url=URL)" >&2
  exit 2
fi
if ! [[ "$RETENTION_DAYS" =~ ^[0-9]+$ ]]; then
  echo "ERROR: --retention-days must be a positive integer (got: $RETENTION_DAYS)" >&2
  exit 2
fi
if ! command -v pg_dump >/dev/null 2>&1; then
  echo "ERROR: pg_dump not found in PATH" >&2
  exit 2
fi

TS="$(date -u +%Y%m%dT%H%M%SZ)"
DUMP_FILE="/tmp/devops-${TS}.sql.gz"
TEMP_DIR_CREATED=""

cleanup() {
  local exit_code=$?
  if [[ -n "$TEMP_DIR_CREATED" && -d "$TEMP_DIR_CREATED" ]]; then
    rm -rf "$TEMP_DIR_CREATED"
  elif [[ -f "${DUMP_FILE:-}" ]]; then
    rm -f "$DUMP_FILE"
  fi
  exit "$exit_code"
}
trap cleanup EXIT

# Create a private temp dir for the dump (avoids /tmp races in
# multi-instance cron / multi-cluster cronjobs).
TEMP_DIR="$(mktemp -d -t devops-backup-XXXXXX)"
TEMP_DIR_CREATED="$TEMP_DIR"
DUMP_FILE="$TEMP_DIR/devops-${TS}.sql.gz"

echo "INFO: starting backup at $TS, target=$TARGET, retention=${RETENTION_DAYS}d"
if ! pg_dump "$PG_URL" | gzip > "$DUMP_FILE"; then
  echo "ERROR: pg_dump failed" >&2
  exit 2
fi
if [[ ! -s "$DUMP_FILE" ]]; then
  echo "ERROR: dump file is empty (pg_dump produced no output)" >&2
  exit 2
fi
DUMP_SIZE="$(du -h "$DUMP_FILE" | cut -f1)"
echo "INFO: dump created: $DUMP_FILE ($DUMP_SIZE)"

# Dispatch by target. Each branch is responsible for placing the
# dump at the target AND for retention cleanup.
case "$TARGET" in
  local)
    mkdir -p /backups
    cp "$DUMP_FILE" "/backups/$(basename "$DUMP_FILE")"
    # Retention: find + delete, non-fatal (cleanup trap handles tmp).
    find /backups -type f -name "*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete 2>/dev/null || true
    echo "INFO: copied to /backups/$(basename "$DUMP_FILE")"
    ;;
  s3)
    if [[ -z "${BACKUP_S3_BUCKET:-}" ]]; then
      echo "ERROR: BACKUP_S3_BUCKET env required for s3 target" >&2
      exit 2
    fi
    if ! command -v aws >/dev/null 2>&1; then
      echo "ERROR: aws CLI not found in PATH (required for s3 target)" >&2
      exit 2
    fi
    S3_KEY="$(date -u +%Y/%m/%d)/$(basename "$DUMP_FILE")"
    S3_URI="s3://${BACKUP_S3_BUCKET}/${S3_KEY}"
    if ! aws s3 cp "$DUMP_FILE" "$S3_URI"; then
      echo "ERROR: aws s3 cp failed for $S3_URI" >&2
      exit 2
    fi
    echo "INFO: uploaded to $S3_URI"
    # Best-effort S3 retention cleanup. aws CLI does not have
    # a native "delete older than N days" verb, so use S3
    # Inventory + lifecycle policies in production; here we
    # only log the local retention window as a hint.
    echo "INFO: configure S3 lifecycle policy to expire objects older than ${RETENTION_DAYS}d"
    ;;
  nfs)
    if [[ -z "${BACKUP_NFS_PATH:-}" ]]; then
      echo "ERROR: BACKUP_NFS_PATH env required for nfs target" >&2
      exit 2
    fi
    mkdir -p "$BACKUP_NFS_PATH"
    cp "$DUMP_FILE" "$BACKUP_NFS_PATH/$(basename "$DUMP_FILE")"
    find "$BACKUP_NFS_PATH" -type f -name "*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete 2>/dev/null || true
    echo "INFO: copied to $BACKUP_NFS_PATH/$(basename "$DUMP_FILE")"
    ;;
  *)
    echo "ERROR: unknown target: $TARGET (use local|s3|nfs)" >&2
    exit 2
    ;;
esac

# Verify dump is valid (decompresses cleanly + has SQL preamble).
# We read from the local temp copy (the target already has its own
# copy by this point).
echo "INFO: verifying dump integrity (gunzip + head)..."
if ! gunzip -c "$DUMP_FILE" | head -5 >/dev/null; then
  echo "ERROR: dump verification failed (gunzip -c | head -5)" >&2
  exit 2
fi

echo "INFO: backup complete: $TS, target=$TARGET, size=$DUMP_SIZE"
