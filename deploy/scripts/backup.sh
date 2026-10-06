#!/usr/bin/env bash
# Atomic backups; external replication and restore checks are still required.
set -euo pipefail
umask 077
cd "$(dirname "$0")/../.."
ENV_FILE=/etc/meetus/meetus.env
BACKUP_DIR=${BACKUP_DIR:-/var/backups/meetus}
KEEP_DAYS=14
COMPOSE=(docker compose -f deploy/docker-compose.yml --env-file "$ENV_FILE")
mkdir -p "$BACKUP_DIR"
STAMP=$(date +%Y%m%d-%H%M%S)
OUT="$BACKUP_DIR/meetus-$STAMP.sql.gz"
UPLOADS="$BACKUP_DIR/uploads-$STAMP.tar.gz"
trap 'rm -f "$OUT.tmp" "$UPLOADS.tmp"' EXIT
"${COMPOSE[@]}" exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > "$OUT.tmp"
gzip -t "$OUT.tmp"
# A short-lived utility container mounts the same upload volume read-only.
"${COMPOSE[@]}" run --rm --no-deps backup-uploads > "$UPLOADS.tmp"
gzip -t "$UPLOADS.tmp"
mv "$OUT.tmp" "$OUT"
mv "$UPLOADS.tmp" "$UPLOADS"
find "$BACKUP_DIR" -type f \( -name 'meetus-*.sql.gz' -o -name 'uploads-*.tar.gz' \) -mtime "+$KEEP_DAYS" -delete
printf 'Backups written: %s and %s\n' "$OUT" "$UPLOADS"
