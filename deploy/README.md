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
# TELEGRAM_OFFICIAL_CHANNEL_ID=<see "Meetus.uz's own official channel" below — optional, add once you have it>
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

**Meetus.uz's own official channel** (separate from organizers' own
channels — every published event, from every organizer, posts here) is
configured via `TELEGRAM_OFFICIAL_CHANNEL_ID`, not a DB row, so there's a
one-time manual step to find the chat ID:

1. Create (or pick) the channel you want to be the official one.
2. Add the bot as **admin** there, same as an organizer would for their own
   channel.
3. Read the chat ID the worker just logged:
   `docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env logs worker | grep "channel connected"`
   — it's a large negative number, e.g. `-1001234567890`.
4. Add `TELEGRAM_OFFICIAL_CHANNEL_ID=<that number>` (and optionally
   `TELEGRAM_OFFICIAL_CHANNEL_LANGUAGE=uz|ru|en`, defaults to `uz`) to
   `/etc/meetus/meetus.env`, then `systemctl restart meetus`.

Whoever's account added the bot still needs an organizer profile for the
connection step above to succeed (same rule as any organizer channel) —
that's fine even if you never actually use that organizer profile for
anything else, and there's no need to disconnect it afterward: the
auto-announce hook skips a channel from its own organizer's send list when
that channel's ID matches `TELEGRAM_OFFICIAL_CHANNEL_ID`, so it can't ever
get the same event posted twice.

**Getting the chat ID**: use the one the worker logs when the connection
succeeds (step 3 above), not a number from a third-party "get channel ID"
bot — those commonly report the short public ID without Telegram's `-100`
supergroup/channel prefix that the Bot API actually requires (e.g. a
checker bot reporting `3238861564` when the real `chat_id` the Bot API
needs is `-1003238861564`). Using the wrong one means every send silently
fails with a "chat not found" error.

## Deploying updates

**Automatic (CI/CD):** every push to `main` runs the backend + frontend
test jobs in `.github/workflows/ci.yml`; if they pass, the `deploy` job
SSHs into the VPS (using the `VPS_HOST`/`VPS_USER`/`VPS_SSH_KEY`/
`VPS_HOST_KEY` repo secrets — a dedicated deploy keypair, not any personal
credential) and runs the same script below. No manual step needed for a
normal change. Trigger it manually from the Actions tab
(`workflow_dispatch`) if you need to redeploy without a new commit.

**Manual:**

```bash
cd /opt/meetus && ./deploy/scripts/deploy.sh <full-tested-commit-sha>
```

The script locks the deployment checkout, fetches and checks out the exact
tested SHA, builds images with `APP_REVISION`, applies migrations, restarts,
and verifies API dependency readiness/revision plus the frontend. CI also
serializes deployments and streams the tested deployment script, so an old
VPS checkout cannot run outdated deployment logic. Older successful runs
are skipped when a newer revision is already checked out; intentional
rollback requires `ALLOW_ROLLBACK=1`. Images are retained for rollback; database down
migrations are never applied automatically.

## Operations

```bash
systemctl status meetus                       # stack status
docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env ps
docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env logs -f api worker
./deploy/scripts/backup.sh                    # manual backup
```

Volumes: `pgdata` (database), `uploads` (covers/avatars), `caddy_data`
(TLS certs). The backup script creates restricted, gzip-verified, atomically
renamed database and upload archives, retained locally for 14 days. Copy
both off the VPS using an operator-configured destination. No offsite
credentials or destination are assumed by this repository.

`/healthz` is liveness; `/readyz` checks PostgreSQL and Redis and returns the
baked image revision. Monitor failed `delivery_jobs` and pending jobs older
than a few minutes, along with disk space and backup age.

### Restore drill
Restore into an empty disposable database, never over live data:

```bash
gzip -dc meetus-<stamp>.sql.gz | psql -v ON_ERROR_STOP=1 "$RESTORE_DATABASE_URL"
gzip -t uploads-<stamp>.tar.gz
tar xzf uploads-<stamp>.tar.gz -C "$EMPTY_UPLOAD_DIRECTORY"
```

Verify migration version/dirty flag, user/event/RSVP counts, upload files,
and a login/RSVP smoke test before switching traffic. Preserve upload
ownership `65532:65532` when restoring the production volume. The runtime
image seeds this ownership for fresh Docker volumes.

Local review drill (2026-10-05): PostgreSQL 16 dump restored into a separate
empty database with migration version 16 and `dirty=false`. A fresh Docker
upload volume accepted an authenticated PNG and served it back. This does
not verify VPS offsite replication or production restore credentials.

### Read-only operational checks

Run `sudo bash deploy/scripts/check-health.sh` on the Docker host. It reads the
production Compose services and `/etc/meetus/meetus.env`, checks API/frontend,
worker progress, pending delivery age, recent failures, completed backup age,
and upload filesystem free space. Exit 0 is healthy, 1 needs attention, and 2
means invalid thresholds. It sends no messages and performs no repairs.
Configure your monitoring agent separately; no alert destination is installed.
Overrides: `MEETUS_ENV_FILE`, `BACKUP_DIR`, `BACKUP_MAX_AGE_MINUTES` (2160),
`MIN_FREE_KB` (1048576), `MAX_PENDING_AGE_SECONDS` (300), and
`MAX_RECENT_FAILURES` (20 in the last 24 hours). Worker progress is expected
when the production Telegram worker is configured.

The admin page polls `/api/admin/operations` every 30 seconds. Worker progress
expires after 90 seconds; an idle but functioning delivery loop still pulses.
Redis errors display unavailable status rather than a healthy cached result.

Run an upload inventory with the deployed backend image:

```bash
docker compose -f deploy/docker-compose.yml --env-file /etc/meetus/meetus.env \
  exec -T api /usr/local/bin/audit-uploads -older-than 720h -details 20
```

The command only reads the database and upload directory. It counts managed
images, their bytes, references, and old unreferenced candidates; details are
capped at 1000 filenames. It ignores symlinks and unmanaged names. A reference
snapshot is not permission to delete: uploads may be awaiting a profile/event
save, and backups or external links may still need them. Establish retention
and storage budgets before scheduling any reclamation.
