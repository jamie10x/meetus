#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
mkdir "$fixture/bin" "$fixture/backups"
touch "$fixture/environment" "$fixture/backups/meetus-test.sql.gz" "$fixture/backups/uploads-test.tar.gz"
cat > "$fixture/bin/docker" <<'MOCK'
#!/usr/bin/env bash
case "$*" in
  *'redis-cli TTL'*) if [[ "${HEALTH_CASE:-}" == stale ]]; then echo -2; else echo 80; fi ;;
  *'exec -T postgres'*) if [[ "${HEALTH_CASE:-}" == backlog ]]; then echo '3|0|600'; else echo '0|0|0'; fi ;;
  *'inspect --format'*) echo "$HEALTH_MOUNT" ;;
  *'ps -q api'*) echo fixture-api ;;
  *'wget'*) [[ "${HEALTH_CASE:-}" != unavailable ]] ;;
  *) exit 2 ;;
esac
MOCK
chmod +x "$fixture/bin/docker"
export PATH="$fixture/bin:$PATH" DEPLOY_ROOT="$ROOT" MEETUS_ENV_FILE="$fixture/environment" BACKUP_DIR="$fixture/backups" HEALTH_MOUNT="$fixture" MIN_FREE_KB=1
bash "$ROOT/deploy/scripts/check-health.sh" >/dev/null
for HEALTH_CASE in stale backlog unavailable; do
 export HEALTH_CASE
 if bash "$ROOT/deploy/scripts/check-health.sh" >/dev/null; then echo "Missed unhealthy $HEALTH_CASE"; exit 1; fi
done
unset HEALTH_CASE
rm "$fixture/backups/uploads-test.tar.gz"
if bash "$ROOT/deploy/scripts/check-health.sh" >/dev/null; then echo 'Missed absent upload backup'; exit 1; fi
echo 'Health-check scenarios passed'
