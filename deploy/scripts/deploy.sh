#!/usr/bin/env bash
# Deploy exactly the revision validated by CI. No implicit pull of branch head.
set -euo pipefail
cd "${DEPLOY_ROOT:-$(dirname "$0")/../..}"
REVISION=${1:?usage: deploy.sh <full tested commit SHA>}
[[ "$REVISION" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected full commit SHA' >&2; exit 1; }
exec 9>.deploy.lock
flock -n 9 || { echo 'Another deployment is running' >&2; exit 1; }
ENV_FILE=/etc/meetus/meetus.env
[[ -f "$ENV_FILE" ]] || { echo 'Deployment env file missing' >&2; exit 1; }
[[ -z "$(git status --porcelain --untracked-files=no)" ]] || { echo 'Tracked deployment checkout is dirty' >&2; exit 1; }
git fetch origin main
git merge-base --is-ancestor "$REVISION" origin/main
PREVIOUS=$(git rev-parse HEAD)
if [[ "$PREVIOUS" != "$REVISION" && "${ALLOW_ROLLBACK:-0}" != 1 ]] && git merge-base --is-ancestor "$REVISION" "$PREVIOUS"; then
  echo "Skipping an older successful CI run; newer revision is already checked out"
  exit 0
fi
git checkout --detach "$REVISION"
# Bake the deployed revision into each backend image.
export APP_REVISION="$REVISION"
COMPOSE=(docker compose -f deploy/docker-compose.yml --env-file "$ENV_FILE")
"${COMPOSE[@]}" build
"${COMPOSE[@]}" run --rm migrate
"${COMPOSE[@]}" up -d --remove-orphans
for i in $(seq 1 30); do
  if "${COMPOSE[@]}" exec -T caddy wget -qO- http://api:8080/readyz | grep -q "$REVISION" &&
     "${COMPOSE[@]}" exec -T caddy wget -qO- http://frontend:3000/uz >/dev/null; then
    printf 'Deployed %s (previous %s)\n' "$REVISION" "$PREVIOUS"
    "${COMPOSE[@]}" ps
    exit 0
  fi
  sleep 2
done
printf 'Readiness failed for %s; previous revision: %s. Inspect logs before rollback; do not reverse migrations automatically.\n' "$REVISION" "$PREVIOUS" >&2
exit 1
