# Meetus.uz

Events and meetup platform for Uzbekistan: discover events, RSVP, get a QR ticket, check in.

## Stack

- **Backend:** Go + Gin, PostgreSQL 16 (full-text search), Redis 7
- **Frontend:** Next.js (App Router) + Tailwind CSS
- **Auth:** Telegram Login → JWT (access + refresh)
- **Bot:** Telegram bot for browsing, RSVP, and reminders
- **Deploy:** Docker + systemd on a VPS, Caddy reverse proxy

## Layout

```text
backend/    Go API, worker, migrations
frontend/   Next.js web app
deploy/     Dockerfiles, compose, systemd units, scripts
docs/       API contract
```

## Development

Requirements: Go 1.25+, Node 22+, Docker.

```bash
cp .env.example .env        # fill in TELEGRAM_BOT_TOKEN etc.
make infra                  # start Postgres + Redis
make migrate-up             # apply DB migrations
make api                    # run API on :8080
make frontend               # run Next.js on :3000
```

Tests: `make test` · Vet: `make vet`
