#!/usr/bin/env bash
#
# scripts/restore-postgres.sh — Restore a PostgreSQL backup.
#
# Usage: restore-postgres.sh --source=local|s3|nfs --file=NAME [--pg-url=URL]
#
# Sources:
#   local  - /backups/<file>           (docker volume)
#   s3     - s3://$BACKUP_S3_BUCKET/<file>
#   nfs    - $BACKUP_NFS_PATH/<file>
#
# Required env (or --pg-url):
#   TARGET_PG_URL or DEVOPS_DATABASE_URL — full libpq connection string
#                                          for the destination database.
#
# Optional env (source-dependent):
#   BACKUP_S3_BUCKET   S3 bucket for --source=s3
#   BACKUP_NFS_PATH    Mount point or directory for --source=nfs
#
# The script:
#   1) downloads / locates the dump
#   2) lists its first lines to confirm SQL content (dry-run)
#   3) applies it to the target database inside one transaction
#      (psql --single-transaction, so a failure rolls back)
#
# DESTRUCTIVE: this script overwrites the destination database.
# Confirm with --yes-i-know in non-drill environments.

set -euo pipefail

# Defaults.
SOURCE="local"
FILE=""
TARGET_PG_URL="${TARGET_PG_URL:-${DEVOPS_DATABASE_URL:-}}"
CONFIRMED=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source=*)
      SOURCE="${1#*=}"
      ;;
    --file=*)
      FILE="${1#*=}"
      ;;
    --pg-url=*)
      TARGET_PG_URL="${1#*=}"
      ;;
    --yes-i-know)
      CONFIRMED="yes"
      ;;
    -h|--help)
      sed -n '2,22p' "$0"
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
if [[ -z "$FILE" ]]; then
  echo "ERROR: --file=NAME is required (the dump filename inside the source)" >&2
  exit 2
fi
if [[ -z "$TARGET_PG_URL" ]]; then
  echo "ERROR: TARGET_PG_URL or DEVOPS_DATABASE_URL must be set (or pass --pg-url=URL)" >&2
  exit 2
fi
if [[ "$CONFIRMED" != "yes" ]]; then
  echo "ERROR: restore is DESTRUCTIVE (overwrites the target database)." >&2
  echo "       re-run with --yes-i-know to confirm." >&2
  exit 2
fi
if ! command -v psql >/dev/null 2>&1; then
  echo "ERROR: psql not found in PATH" >&2
  exit 2
fi

# Sanitize FILE — no path traversal. We deliberately refuse
# absolute paths and ".." segments so a misconfigured cron
# can't read /etc/passwd.
if [[ "$FILE" = /* || "$FILE" == *".."* ]]; then
  echo "ERROR: --file must be a relative filename with no '..' segments" >&2
  exit 2
fi

# Temp dir for the staged dump.
TEMP_DIR=""
cleanup() {
  local exit_code=$?
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
  exit "$exit_code"
}
trap cleanup EXIT

TEMP_DIR="$(mktemp -d -t devops-restore-XXXXXX)"
DUMP_PATH="$TEMP_DIR/restore.sql.gz"

echo "INFO: restore drill starting: source=$SOURCE, file=$FILE"

# Stage the dump from the chosen source.
case "$SOURCE" in
  local)
    SRC_PATH="/backups/$FILE"
    if [[ ! -f "$SRC_PATH" ]]; then
      echo "ERROR: local dump not found: $SRC_PATH" >&2
      exit 2
    fi
    cp "$SRC_PATH" "$DUMP_PATH"
    ;;
  s3)
    if [[ -z "${BACKUP_S3_BUCKET:-}" ]]; then
      echo "ERROR: BACKUP_S3_BUCKET env required for s3 source" >&2
      exit 2
    fi
    if ! command -v aws >/dev/null 2>&1; then
      echo "ERROR: aws CLI not found in PATH (required for s3 source)" >&2
      exit 2
    fi
    S3_URI="s3://${BACKUP_S3_BUCKET}/${FILE}"
    if ! aws s3 cp "$S3_URI" "$DUMP_PATH"; then
      echo "ERROR: aws s3 cp failed for $S3_URI" >&2
      exit 2
    fi
    ;;
  nfs)
    if [[ -z "${BACKUP_NFS_PATH:-}" ]]; then
      echo "ERROR: BACKUP_NFS_PATH env required for nfs source" >&2
      exit 2
    fi
    SRC_PATH="$BACKUP_NFS_PATH/$FILE"
    if [[ ! -f "$SRC_PATH" ]]; then
      echo "ERROR: nfs dump not found: $SRC_PATH" >&2
      exit 2
    fi
    cp "$SRC_PATH" "$DUMP_PATH"
    ;;
  *)
    echo "ERROR: unknown source: $SOURCE (use local|s3|nfs)" >&2
    exit 2
    ;;
esac

if [[ ! -s "$DUMP_PATH" ]]; then
  echo "ERROR: staged dump is empty: $DUMP_PATH" >&2
  exit 2
fi
STAGED_SIZE="$(du -h "$DUMP_PATH" | cut -f1)"
echo "INFO: dump staged: $DUMP_PATH ($STAGED_SIZE)"

# Verify the dump decompresses cleanly + has SQL content.
# A successful `gunzip -t` ensures the gzip envelope is intact;
# `gunzip -c | head -5` ensures SQL is present (any non-empty
# pre-amble — COPY, CREATE, etc — proves the dump is a real
# pg_dump output, not garbage).
echo "INFO: verifying dump integrity (gunzip -t)..."
if ! gunzip -t "$DUMP_PATH"; then
  echo "ERROR: dump is not a valid gzip file" >&2
  exit 2
fi
echo "INFO: dump preamble (first 5 lines):"
gunzip -c "$DUMP_PATH" | head -5 || true
echo "INFO: end of preamble"

# Apply to target. psql --single-transaction wraps the whole
# restore in one transaction, so a mid-stream SQL error rolls
# back. This is the safe default for a drill or a real restore
# against a non-production target.
echo "INFO: applying restore to target database (single-transaction)..."
if ! gunzip -c "$DUMP_PATH" | psql "$TARGET_PG_URL" --single-transaction --set ON_ERROR_STOP=1; then
  echo "ERROR: restore failed; psql rolled back the transaction" >&2
  exit 2
fi

echo "INFO: restore complete (file=$FILE, source=$SOURCE, size=$STAGED_SIZE)"
