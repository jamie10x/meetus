# Meetus.uz

Events and meetup platform for Uzbekistan: discover events, RSVP, get a QR ticket, check in.
Fully translated uz/ru/en, and runs as both a regular website and a Telegram Mini App.

## Stack

- **Backend:** Go + Gin, PostgreSQL 16 (full-text search), Redis 7
- **Frontend:** Next.js (App Router) + Tailwind CSS, i18n via next-intl (uz/ru/en, locale-prefixed URLs)
- **Auth:** Telegram Login Widget (browser) + Telegram Mini App `initData` (in-Telegram) → JWT (access + refresh)
- **Bot:** Telegram bot for browsing, RSVP, reminders, and post-event feedback — i18n'd, opens events as an in-app Mini App
- **Deploy:** Docker + systemd on a VPS, Caddy reverse proxy

## Layout

```text
backend/    Go API, worker, migrations
frontend/   Next.js web app
deploy/     Dockerfiles, compose, systemd units, scripts
docs/       architecture, data model, API contract, dev guide, roadmap
```

## Documentation

New here (human or AI agent)? Read **[AGENTS.md](AGENTS.md)** first — stack,
conventions, and gotchas in one page. Then:

- [docs/architecture.md](docs/architecture.md) — system design and key flows
- [docs/data-model.md](docs/data-model.md) — schema and query conventions
- [docs/api.md](docs/api.md) — HTTP contract (contract-first)
- [docs/development.md](docs/development.md) — setup, dev-login helper, smoke tests, recipes
- [docs/roadmap.md](docs/roadmap.md) — v2 backlog

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

## Deployment

See [deploy/README.md](deploy/README.md) — Docker images, one systemd unit,
Caddy with automatic TLS, `deploy.sh` and nightly `backup.sh`.
