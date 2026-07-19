# Deploying Meetus.uz

Target: a single VPS with Docker and systemd. Everything runs in
containers; systemd manages the stack as one unit.

## One-time setup

```bash
# 1. Code
sudo mkdir -p /opt/meetus && sudo chown $USER /opt/meetus
git clone <repo-url> /opt/meetus

# 2. Secrets (never committed)
sudo mkdir -p /etc/meetus
sudo tee /etc/meetus/meetus.env > /dev/null <<'EOF'
POSTGRES_PASSWORD=<openssl rand -hex 24>
JWT_SECRET=<openssl rand -hex 32>
TICKET_SECRET=<openssl rand -hex 32>
TELEGRAM_BOT_TOKEN=<from @BotFather>
TELEGRAM_BOT_USERNAME=<bot username, no @>
SITE_HOST=meetus.uz
API_BASE_URL=https://meetus.uz
WEB_BASE_URL=https://meetus.uz
EOF
sudo chmod 600 /etc/meetus/meetus.env

# 3. Systemd unit
sudo cp /opt/meetus/deploy/systemd/meetus.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now meetus

# 4. Nightly DB backup
sudo crontab -l | { cat; echo "0 3 * * * /opt/meetus/deploy/scripts/backup.sh"; } | sudo crontab -
```

DNS: point `meetus.uz` A record at the VPS. Caddy obtains and renews TLS
certificates automatically once the domain resolves.

Telegram: in @BotFather run `/setdomain` for the bot and set `meetus.uz`
so the Login Widget works on the site.

Telegram Mini App: also in @BotFather, run `/newapp` (or Bot Settings →
Mini App via `/mybots`) and point it at `https://meetus.uz` (or a specific
path) to give the bot a persistent Mini App menu button. This is separate
from the inline `web_app` buttons the bot already sends on event messages
(those work as soon as `WEB_BASE_URL` is HTTPS and correct — no BotFather
config needed for them specifically) — the menu button is the one thing
that genuinely requires this manual step.

Channel announcements need **no BotFather configuration at all** — it's
entirely self-service per organizer. An organizer adds the bot as an
**admin** to their own Telegram channel (Channel → Administrators → Add
Admin), and the bot automatically links it to their organizer profile via
Telegram's `my_chat_member` update (see architecture.md). This requires
`TELEGRAM_BOT_TOKEN` to be set, same as everything else bot-related; a
deployment without it simply returns a clear "not configured" error on the
announce endpoint instead of failing to start.

## Deploying updates

```bash
cd /opt/meetus && ./deploy/scripts/deploy.sh
```

The script pulls, rebuilds, applies migrations (the `migrate` service runs
before the API starts), restarts containers, and waits for `/healthz`.

## Operations

```bash
systemctl status meetus                       # stack status
docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env ps
docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env logs -f api worker
./deploy/scripts/backup.sh                    # manual backup
```

Volumes: `pgdata` (database), `uploads` (event covers), `caddy_data`
(TLS certs). Back up `pgdata` via the script; `uploads` with rsync if
covers matter to you.
