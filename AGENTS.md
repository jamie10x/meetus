# Meetus.uz — Agent & Developer Onboarding

Events platform for Uzbekistan: **discover events → RSVP → QR ticket → check-in**.
Identity is a Telegram account (no passwords, no Google). Free events only in v1.

Read this file first, then the doc that matches your task:

| Doc | Read when |
|---|---|
| [docs/architecture.md](docs/architecture.md) | Touching backend structure, flows, or adding a module |
| [docs/data-model.md](docs/data-model.md) | Touching SQL, migrations, or repositories |
| [docs/api.md](docs/api.md) | Adding/changing endpoints (**contract-first: update it before frontend work**) |
| [docs/development.md](docs/development.md) | Setting up, running, testing, common task recipes |
| [deploy/README.md](deploy/README.md) | Anything about the VPS, Docker, systemd, Caddy |
| [docs/roadmap.md](docs/roadmap.md) | Deciding whether a feature is in scope (v2 backlog lives here) |

## Stack (fixed decisions — do not re-litigate)

- **Backend**: Go 1.25 + Gin, pgx (no ORM), PostgreSQL 16 (full-text search), Redis 7
- **Auth**: Telegram Login Widget → HMAC verify → JWT access (15 min) + rotating refresh (30 d)
- **Frontend**: Next.js 16 App Router + Tailwind 4, TypeScript, web-only (no mobile app in v1)
- **Bot**: go-telegram/bot, runs inside `cmd/worker` with the reminder loop
- **Deploy**: Docker images + one systemd unit on a single VPS, Caddy for TLS
- **Out of scope for v1**: payments, admin panel, analytics dashboard, Telegram Mini App

## Repo layout

```text
backend/            Go module "meetus.uz/backend"
  cmd/api           HTTP server        cmd/worker  bot + reminders     cmd/migrate  DB migrations
  internal/<module> auth, user, organizer, event, rsvp, notification, tgbot, meta, upload
  internal/platform apperr, authn (JWT+middleware), db, httpx, ratelimit, redisx
  internal/server   router.go — all wiring/DI happens here
  db/migrations     NNNN_name.up.sql / .down.sql (embedded into the migrate binary)
frontend/           Next.js app (src/app pages, src/lib api client, src/components)
deploy/             Dockerfiles, prod compose, Caddyfile, systemd unit, deploy/backup scripts
docs/               api.md (contract), architecture, data-model, development, roadmap
```

## Commands (run from repo root)

```bash
make infra          # start dev Postgres + Redis (docker compose)
make migrate-up     # apply migrations
make api            # run API on :8080
make frontend       # run Next.js on :3000
make test           # backend tests (integration tests skip if Postgres is down)
make vet            # go vet
cd frontend && npm run build   # frontend typecheck + build (do this before committing frontend changes)
```

## Hard conventions

1. **Response envelope**: success `{"data": ...}`, failure `{"error": {"code", "message"}}`. Handlers use `httpx.OK` / `httpx.Error`; services return `apperr.*` errors (`Validation`→400, `Unauthorized`→401, `Forbidden`→403, `NotFound`→404, `Conflict`→409).
2. **Module shape**: `repository.go` (SQL only) → `service.go` (rules) → `handler.go` (binding + envelope). Wiring only in `internal/server/router.go`. Keep handlers thin.
3. **Contract-first**: any endpoint change updates `docs/api.md` in the same commit.
4. **Migrations are append-only**: never edit an applied migration; add a new numbered pair.
5. **camelCase JSON** field names; pointers for nullable DB columns.
6. **No secrets in code or logs** — no tokens, no passwords, no service keys. Config comes from env (`internal/config`).
7. Frontend API calls go through `src/lib/api.ts` (`api<T>()` / `uploadImage()`) — it handles the envelope and 401→refresh→retry. Never call `fetch` directly to the API from components.
8. One commit per coherent change; backend must pass `go build ./... && go vet ./... && go test ./...`, frontend must pass `npm run build`.

## Gotchas that will bite you

- **Next.js 16 breaking changes**: `params` in pages/layouts/generateMetadata is a **Promise** (`await params` server-side, `use(params)` client-side). Before writing nontrivial frontend code, check the bundled docs: `frontend/node_modules/next/dist/docs/`. There is no `middleware.ts` — the convention is `proxy`.
- **Dev login without a real bot**: with `TELEGRAM_BOT_TOKEN` unset, the backend verifies Telegram payloads against the empty-string token, so you can mint valid logins with the `tg_login` shell helper in [docs/development.md](docs/development.md#dev-login-helper). The full login+RSVP smoke-test script is there too.
- `internal/` packages cannot be imported from outside the module — write throwaway checks as Go tests inside the module, not scratch programs.
- `goingCount` is a live subquery in `event.Repository` (`eventSelect`), not a column.
- The `search` tsvector column on `events` is **generated** — never INSERT/UPDATE it.
- RSVP capacity is enforced inside a transaction with `SELECT ... FOR UPDATE` on the event row (`rsvp.Repository.Join`). Don't add RSVP writes that bypass it.
- Ticket QR = `code + "." + HMAC-SHA256(code, TICKET_SECRET)`. Verify with `rsvp.TicketSigner`, never by string comparison.
- Reminder dedup = UNIQUE `(event_id, user_id, kind)` in `notification_log` + Redis lock `meetus:worker:reminder-scan`. Send attempts are logged even on Telegram failure (403 = user never opened the bot) to avoid retry storms.
- Bot messages use Telegram HTML parse mode — user content must go through `tgbot.escape()`.
- Working directory: Go commands need `cd backend`; the Makefile handles this.
