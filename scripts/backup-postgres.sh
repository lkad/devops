#!/usr/bin/env bash
# scripts/backup-postgres.sh — PostgreSQL backup + restore for the
# devops-toolkit stack. Two modes:
#
#   1) backup (default) — pg_dump --format=custom to
#      backups/devops-toolkit-YYYY-MM-DD-HHMM.sqlc. Optionally
#      uploads to S3 when S3_BUCKET is set. Retains the last
#      ${BACKUP_RETENTION_DAYS:-7} days of local backups and
#      deletes older ones.
#
#   2) restore --file <path> [--yes-i-know]
#      Drops + recreates the devops database and loads a given
#      pg_dump --format=custom archive via pg_restore.
#      DESTRUCTIVE. Requires --yes-i-know to acknowledge.
#
# Configuration is read from deploy/.env.prod (relative to the
# repo root) or from the environment directly. Recognised vars:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE
#   S3_BUCKET, S3_PREFIX, AWS_REGION, BACKUP_RETENTION_DAYS
#
# Add to root's crontab: 0 2 * * * /opt/devops-toolkit/scripts/backup-postgres.sh
#
# Exit codes:
#   0  success
#   1  configuration / argument error
#   2  pg_dump / pg_restore failure
#   3  S3 upload failure (after a successful local dump; the local
#      file is preserved so retry is safe)
#   4  retention cleanup failure (non-fatal; logged)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_ROOT/deploy/.env.prod"
BACKUP_DIR="$REPO_ROOT/backups"
TS="$(date -u +%Y-%m-%d-%H%M)"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"

log() { printf '[%s] %s\n' "$(date -u +%FT%TZ)" "$*"; }
err() { printf '[%s] ERROR: %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }

# Load env file if it exists; existing process env wins.
load_env_file() {
  if [[ -f "$ENV_FILE" ]]; then
    log "loading env from $ENV_FILE"
    # shellcheck disable=SC1090
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
  fi
}

# ---- backup ----
do_backup() {
  load_env_file

  : "${PGHOST:=postgres}"
  : "${PGPORT:=5432}"
  : "${PGUSER:=devops}"
  : "${PGDATABASE:=devops}"
  : "${S3_BUCKET:=}"
  : "${S3_PREFIX:=postgres-backups/devops-toolkit}"
  : "${AWS_REGION:=us-east-1}"

  if [[ -z "${PGPASSWORD:-}" ]]; then
    err "PGPASSWORD is not set (define it in $ENV_FILE or the environment)"
    exit 1
  fi

  mkdir -p "$BACKUP_DIR"
  local outfile="$BACKUP_DIR/devops-toolkit-${TS}.sqlc"

  log "starting pg_dump host=$PGHOST db=$PGDATABASE user=$PGUSER"
  if ! pg_dump \
      --format=custom \
      --no-owner \
      --no-privileges \
      --host="$PGHOST" \
      --port="$PGPORT" \
      --username="$PGUSER" \
      --dbname="$PGDATABASE" \
      --file="$outfile"; then
    err "pg_dump failed"
    exit 2
  fi
  local size
  size="$(du -h "$outfile" | cut -f1)"
  log "backup written: $outfile ($size)"

  # S3 upload (optional). The local file is kept regardless.
  if [[ -n "$S3_BUCKET" ]]; then
    if ! command -v aws >/dev/null 2>&1; then
      err "S3_BUCKET is set but aws CLI is not installed; skipping upload"
    else
      local s3_uri="s3://${S3_BUCKET}/${S3_PREFIX}/$(basename "$outfile")"
      log "uploading to $s3_uri"
      if AWS_REGION="$AWS_REGION" aws s3 cp "$outfile" "$s3_uri" --no-progress; then
        log "s3 upload ok: $s3_uri"
      else
        err "s3 upload failed; local copy preserved at $outfile"
        exit 3
      fi
    fi
  else
    log "S3_BUCKET not set; skipping upload"
  fi

  # Retention. Use -mtime +N (find) for "older than N days".
  log "pruning local backups older than $RETENTION_DAYS days"
  if ! find "$BACKUP_DIR" -maxdepth 1 -type f \
        -name 'devops-toolkit-*.sqlc' \
        -mtime "+$RETENTION_DAYS" \
        -print -delete; then
    err "retention cleanup encountered errors (non-fatal)"
    # exit 4 would be too strict; the new backup is on disk.
  fi

  log "backup complete"
}

# ---- restore ----
do_restore() {
  local file=""
  local confirmed=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --file)
        file="${2:-}"
        shift 2
        ;;
      --yes-i-know)
        confirmed="yes"
        shift
        ;;
      -h|--help)
        cat <<EOF
restore usage: $0 restore --file <path-to-.sqlc> --yes-i-know
EOF
        exit 0
        ;;
      *)
        err "unknown argument: $1"
        exit 1
        ;;
    esac
  done

  if [[ -z "$file" ]]; then
    err "--file <path> is required"
    exit 1
  fi
  if [[ "$confirmed" != "yes" ]]; then
    err "restore is DESTRUCTIVE (drops and recreates the database)."
    err "re-run with --yes-i-know to confirm you mean it."
    exit 1
  fi
  if [[ ! -f "$file" ]]; then
    err "backup file not found: $file"
    exit 1
  fi

  load_env_file

  : "${PGHOST:=postgres}"
  : "${PGPORT:=5432}"
  : "${PGUSER:=devops}"
  : "${PGDATABASE:=devops}"
  : "${PG_SUPERUSER:=postgres}"  # role with CREATEDB privilege.

  if [[ -z "${PGPASSWORD:-}" ]]; then
    err "PGPASSWORD is not set (define it in $ENV_FILE or the environment)"
    exit 1
  fi

  cat <<WARN
============================================================
 WARNING: this will:
   - terminate other connections to $PGDATABASE
   - drop the $PGDATABASE database
   - recreate it from scratch
   - restore $file into it
============================================================
WARN

  log "terminating other connections to $PGDATABASE"
  PGPASSWORD="$PGPASSWORD" psql \
    --host="$PGHOST" --port="$PGPORT" --username="$PG_SUPERUSER" \
    --dbname=postgres \
    --set ON_ERROR_STOP=1 \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$PGDATABASE' AND pid <> pg_backend_pid();" \
    >/dev/null

  log "dropping database $PGDATABASE"
  PGPASSWORD="$PGPASSWORD" dropdb \
    --host="$PGHOST" --port="$PGPORT" --username="$PG_SUPERUSER" \
    --if-exists "$PGDATABASE"

  log "creating database $PGDATABASE (owner=$PGUSER)"
  PGPASSWORD="$PGPASSWORD" createdb \
    --host="$PGHOST" --port="$PGPORT" --username="$PG_SUPERUSER" \
    --owner="$PGUSER" "$PGDATABASE"

  log "restoring $file into $PGDATABASE"
  if ! PGPASSWORD="$PGPASSWORD" pg_restore \
        --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" \
        --dbname="$PGDATABASE" \
        --no-owner --no-privileges \
        --exit-on-error \
        "$file"; then
    err "pg_restore failed"
    exit 2
  fi

  log "restore complete"
}

usage() {
  cat <<EOF
Usage: $0 [backup|restore] [args]

Subcommands:
  backup                      Take a pg_dump backup (default if no verb given).
  restore --file <path> --yes-i-know
                              Drop + recreate the database and load <path>.
                              DESTRUCTIVE.

Env (read from deploy/.env.prod or the environment):
  PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE
  S3_BUCKET, S3_PREFIX, AWS_REGION
  BACKUP_RETENTION_DAYS       Default 7.
EOF
}

main() {
  local verb="${1:-backup}"
  case "$verb" in
    -h|--help|help)
      usage
      ;;
    backup)
      shift
      do_backup "$@"
      ;;
    restore)
      shift
      do_restore "$@"
      ;;
    *)
      err "unknown verb: $verb"
      usage
      exit 1
      ;;
  esac
}

main "$@"
