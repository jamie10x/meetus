#!/usr/bin/env bash
# Read-only VPS check for an operator/monitoring agent. Sends no messages and
# makes no repairs. Exit 1 means attention is required; stdout names the issue.
set -euo pipefail
cd "${DEPLOY_ROOT:-$(dirname "$0")/../..}"
ENV_FILE=${MEETUS_ENV_FILE:-/etc/meetus/meetus.env}
BACKUP_DIR=${BACKUP_DIR:-/var/backups/meetus}
BACKUP_MAX_AGE_MINUTES=${BACKUP_MAX_AGE_MINUTES:-2160}
MIN_FREE_KB=${MIN_FREE_KB:-1048576}
MAX_PENDING_AGE_SECONDS=${MAX_PENDING_AGE_SECONDS:-300}
MAX_RECENT_FAILURES=${MAX_RECENT_FAILURES:-20}
for value in "$BACKUP_MAX_AGE_MINUTES" "$MIN_FREE_KB" "$MAX_PENDING_AGE_SECONDS" "$MAX_RECENT_FAILURES"; do
  [[ "$value" =~ ^(0|[1-9][0-9]{0,11})$ ]] || { echo 'Invalid health-check threshold'; exit 2; }
done
[[ -f "$ENV_FILE" ]] || { echo 'Deployment environment file unavailable'; exit 1; }
COMPOSE=(docker compose -f deploy/docker-compose.yml --env-file "$ENV_FILE")
issues=0
problem() { echo "$1"; issues=$((issues+1)); }
if ! "${COMPOSE[@]}" exec -T caddy wget -qO- http://api:8080/readyz >/dev/null 2>&1; then problem 'API dependencies are not ready'; fi
if ! "${COMPOSE[@]}" exec -T caddy wget -qO- http://frontend:3000/uz >/dev/null 2>&1; then problem 'Frontend is unavailable'; fi
worker_ttl=$("${COMPOSE[@]}" exec -T redis redis-cli TTL meetus:worker:delivery-progress 2>/dev/null || true)
if [[ ! "$worker_ttl" =~ ^[0-9]+$ ]] || ((worker_ttl == 0)); then problem 'Worker delivery progress is missing or expired'; fi
QUERY="SELECT count(*) FILTER (WHERE done_at IS NULL AND failed_at IS NULL),count(*) FILTER (WHERE failed_at>now()-interval '24 hours'),coalesce(extract(epoch FROM now()-min(created_at) FILTER (WHERE done_at IS NULL AND failed_at IS NULL))::bigint,0) FROM delivery_jobs"
queue=$("${COMPOSE[@]}" exec -T postgres sh -c 'psql -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB" -c "$1"' sh "$QUERY" 2>/dev/null || true)
IFS='|' read -r pending failed oldest <<< "$queue"
if [[ "$pending" =~ ^[0-9]+$ && "$failed" =~ ^[0-9]+$ && "$oldest" =~ ^[0-9]+$ ]]; then
  ((oldest <= MAX_PENDING_AGE_SECONDS)) || problem 'Notification backlog exceeds the age threshold'
  ((failed <= MAX_RECENT_FAILURES)) || problem 'Recent delivery failures exceed the threshold'
else
  problem 'Delivery queue status is unavailable'
fi
for pattern in 'meetus-*.sql.gz' 'uploads-*.tar.gz'; do
  recent=$(find "$BACKUP_DIR" -maxdepth 1 -type f -name "$pattern" -mmin "-$BACKUP_MAX_AGE_MINUTES" -print -quit 2>/dev/null || true)
  [[ -n "$recent" ]] || problem "Recent completed backup missing: $pattern"
done
api_id=$("${COMPOSE[@]}" ps -q api 2>/dev/null || true)
mount=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/uploads"}}{{.Source}}{{end}}{{end}}' "$api_id" 2>/dev/null || true)
free_kb=$(df -Pk "$mount" 2>/dev/null | awk 'NR==2 {print $4}' || true)
if [[ "$free_kb" =~ ^[0-9]+$ ]]; then
  ((free_kb >= MIN_FREE_KB)) || problem 'Upload filesystem free space is below the threshold'
else
  problem 'Upload filesystem space is unavailable (run on the Docker host)'
fi
if ((issues)); then exit 1; fi
echo 'API, frontend, worker progress, delivery queue, backup freshness and upload disk space are healthy'
