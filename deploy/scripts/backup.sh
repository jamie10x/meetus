#!/usr/bin/env bash
# Nightly PostgreSQL backup with 14-day rotation.
# Cron example (root): 0 3 * * * /opt/meetus/deploy/scripts/backup.sh
set -euo pipefail

cd "$(dirname "$0")/../.."

ENV_FILE=/etc/meetus/meetus.env
BACKUP_DIR=/var/backups/meetus
KEEP_DAYS=14

# shellcheck disable=SC1090
source "$ENV_FILE"

mkdir -p "$BACKUP_DIR"
STAMP=$(date +%Y%m%d-%H%M%S)
OUT="$BACKUP_DIR/meetus-$STAMP.sql.gz"

docker compose -f deploy/docker-compose.yml --env-file "$ENV_FILE" \
    exec -T postgres pg_dump -U "${POSTGRES_USER:-meetus}" "${POSTGRES_DB:-meetus}" \
    | gzip > "$OUT"

find "$BACKUP_DIR" -name 'meetus-*.sql.gz' -mtime "+$KEEP_DAYS" -delete

echo "Backup written: $OUT ($(du -h "$OUT" | cut -f1))"
