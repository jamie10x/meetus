#!/usr/bin/env bash
# Deploy the latest code on the VPS.
# Usage: ./deploy/scripts/deploy.sh   (run from /opt/meetus)
set -euo pipefail

cd "$(dirname "$0")/../.."

ENV_FILE=/etc/meetus/meetus.env
COMPOSE=(docker compose -f deploy/docker-compose.yml --env-file "$ENV_FILE")

if [[ ! -f "$ENV_FILE" ]]; then
    echo "ERROR: $ENV_FILE not found. Create it from .env.example first." >&2
    exit 1
fi

echo "==> Pulling latest code"
git pull --ff-only

echo "==> Building images"
"${COMPOSE[@]}" build

echo "==> Applying migrations and restarting services"
"${COMPOSE[@]}" up -d --remove-orphans

echo "==> Waiting for API health"
for i in $(seq 1 30); do
    if "${COMPOSE[@]}" exec -T caddy wget -qO- http://api:8080/healthz >/dev/null 2>&1; then
        echo "==> Healthy."
        break
    fi
    [[ $i -eq 30 ]] && { echo "ERROR: API did not become healthy" >&2; exit 1; }
    sleep 2
done

echo "==> Pruning old images"
docker image prune -f >/dev/null

echo "==> Done. Status:"
"${COMPOSE[@]}" ps
