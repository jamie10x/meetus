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
- **Auth**: Telegram Login Widget (browser) **and** Telegram Mini App `initData` (in-Telegram) → HMAC verify (two different schemes — see architecture.md) → JWT access (15 min) + rotating refresh (30 d)
- **Frontend**: Next.js 16 App Router + Tailwind 4, TypeScript. Fully i18n'd uz/ru/en via next-intl with locale-prefixed URLs (`/uz`, `/ru`, `/en`). Doubles as a Telegram Mini App — same deployment, no separate build; no native mobile app
- **Bot**: go-telegram/bot, runs inside `cmd/worker` with the reminder loop; i18n'd (uz/ru/en, see `tgbot/i18n.go`); its event-detail link opens the Mini App in place (`web_app` button), not an external tab
- **Deploy**: Docker images + one systemd unit on a single VPS, Caddy for TLS
- **Payments**: decided on **Payme**, not built yet — free 2-month trial, see [docs/roadmap.md](docs/roadmap.md)
- **Out of scope for v1**: monetization tiers

## Repo layout

```text
backend/            Go module "meetus.uz/backend"
  cmd/api           HTTP server        cmd/worker  bot + reminders     cmd/migrate  DB migrations
  internal/<module> auth, user, organizer, event, rsvp, notification, tgbot, feedback, channel, admin, housekeeping, meta, upload
  internal/platform apperr, authn (JWT+middleware), tglang (lang-code mapping), db, httpx, ratelimit, redisx
  internal/server   router.go — all wiring/DI happens here
  db/migrations     NNNN_name.up.sql / .down.sql (embedded into the migrate binary)
frontend/           Next.js app
  src/app/[locale]  every page lives under this dynamic segment — [locale]/layout.tsx IS the root layout
  src/i18n          routing.ts, navigation.ts, request.ts (next-intl setup)
  src/proxy.ts      Next 16's renamed middleware.ts — locale negotiation
  messages/         uz.json, ru.json, en.json — full translation catalogs, kept in exact key-parity
  src/lib           api client, auth-context (incl. Mini App auto-login), telegram-webapp.ts, useTelegramBackButton.ts
  src/components    shared UI (Header, EventCard, EventForm, LanguageSwitcher, MetaManager, TelegramChrome, ...)
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

- **Next.js 16 breaking changes**: `params` in pages/layouts/generateMetadata is a **Promise** (`await params` server-side, `use(params)` client-side). Before writing nontrivial frontend code, check the bundled docs: `frontend/node_modules/next/dist/docs/`. There is no `middleware.ts` — the convention is `proxy.ts` (`src/proxy.ts` here, already wired to next-intl).
- **Always import `Link`/`redirect`/`useRouter`/`usePathname` from `@/i18n/navigation`**, never `next/link`/`next/navigation`, inside `src/app/[locale]/**` — the plain versions drop the locale prefix and send users to the wrong language. `notFound()` from `next/navigation` is the one exception (locale-agnostic, fine as-is).
- **New frontend page/string?** Add the key to all three `frontend/messages/*.json` files (same key, same nesting) — a key present in only one language silently falls back to English at render time with no build error. Use `useTranslations`/`getTranslations`, never hardcoded UI strings.
- **Two different Telegram HMAC schemes, don't cross them**: `auth.VerifyTelegramLogin` (Login Widget, secret = `SHA256(botToken)`) and `auth.VerifyMiniAppInitData` (Mini App, secret = `HMAC-SHA256(key="WebAppData", message=botToken)`). A payload valid for one does not verify under the other — see architecture.md.
- The Telegram Mini App SDK script (`[locale]/layout.tsx`) is `beforeInteractive` on purpose (deterministic `window.Telegram.WebApp` availability for auto-login) and `<html>` has `suppressHydrationWarning` on purpose (the SDK mutates `<html>`'s style attribute on every browser, not just inside Telegram). Don't "fix" either without re-reading the Mini App section of architecture.md first — the `afterInteractive` alternative was tried and still raced.
- Mini App native chrome (`TelegramChrome`, `useTelegramBackButton`, the `MainButton` wiring in `RsvpSection`) all guard on `getTelegramWebApp()` returning non-null and no-op otherwise — every call site must keep that guard, since these components render for plain-browser visitors too.
- The frontend is a **committed dark-first brand identity** (the "Bold Dark" design system in `globals.css`'s `@theme` — `bg-ink`/`text-bone`/`text-registan`/etc.), not an OS-`prefers-color-scheme` toggle — there is no light theme to preserve. `TelegramChrome` syncs Telegram's own chrome color to the same fixed `--color-ink` value unconditionally, ignoring `tg.colorScheme`. Don't reintroduce `dark:` variants or reference `zinc-*`/`sky-*` classes — use the registered brand tokens everywhere.
- **Dev login without a real bot**: with `TELEGRAM_BOT_TOKEN` unset, the backend verifies Telegram payloads against the empty-string token, so you can mint valid logins with the `tg_login` shell helper in [docs/development.md](docs/development.md#dev-login-helper). The full login+RSVP smoke-test script is there too.
- `internal/` packages cannot be imported from outside the module — write throwaway checks as Go tests inside the module, not scratch programs.
- `goingCount` is a live subquery in `event.Repository` (`eventSelect`), not a column.
- The `search` tsvector column on `events` is **generated** — never INSERT/UPDATE it.
- RSVP capacity is enforced inside a transaction with `SELECT ... FOR UPDATE` on the event row (`rsvp.Repository.Join`). Don't add RSVP writes that bypass it.
- Ticket QR = `code + "." + HMAC-SHA256(code, TICKET_SECRET)`. Verify with `rsvp.TicketSigner`, never by string comparison.
- Reminder dedup = UNIQUE `(event_id, user_id, kind)` in `notification_log` + Redis lock `meetus:worker:reminder-scan`. Send attempts are logged even on Telegram failure (403 = user never opened the bot) to avoid retry storms.
- Bot messages use Telegram HTML parse mode — user content must go through `tgbot.escape()`.
- **`users.language` is set once, on insert, and never overwritten** by later logins (see `user.Repository.UpsertTelegramUser` — `language` is deliberately absent from the `ON CONFLICT DO UPDATE SET` list). Only `/language` (bot) or `PATCH /me` (web) change it after creation. Guarded by a test: `user.TestUpsertTelegramUser_LanguageSetOnInsertOnly`.
- Bot strings live in `tgbot/i18n.go` as whole-sentence templates per language (not word-by-word) — add a new `msgKey` to both the const block and `i18n_test.go`'s `allKeys`, or the completeness test catches it, not a runtime fallback.
- Feedback comments are collected via a **stateless bot conversation**: after a star tap, a Redis key (`meetus:feedback-comment-await:<telegramID>`, 10 min TTL) marks the user as awaiting a comment; the next plain-text message is popped with `GETDEL` (atomic — never add a plain `GET` + `DEL` pair here) and attached via `feedback.Repository.SetComment`. Don't add new pending-conversation state without a TTL — an unbounded key would eventually swallow an unrelated later message as a "comment".
- Auto-announce (`event.Handler.onPublished`) fires **only** from the publish route, in a **background goroutine with `context.Background()`**, never the request's own context — the request context is canceled the instant the HTTP response is written. Failures are logged, not surfaced on the publish response; don't change that without re-reading the "Auto-announce on publish" section of architecture.md.
- Channel announcement language precedence: **channel's own `language` override, if set, wins; otherwise the triggering organizer's `users.language`.** Both the manual `POST /events/:id/announce` and the auto-announce hook must resolve it the same way — don't hardcode the organizer's language in one path only.
- Meetus.uz's own **official channel** (`TELEGRAM_OFFICIAL_CHANNEL_ID` in config, not a `channel_connections` row) posts *every* published event platform-wide, independent of the publishing organizer's own channels — in `router.go`'s `onPublished` closure, that send must stay **before** and **ungated by** the per-organizer `channelRepo.ListForOrganizer` lookup, or an organizer with zero channels of their own would silently skip the official-channel post too. See "Official channel" in architecture.md.
- Admin meta CRUD (`meta.Handler.RegisterAdmin`) uses one generic handler parameterized by a **hardcoded** table name (`"cities"` / `"categories"`) — never make that parameter accept user input; it's a Go string constant at each call site, not something read off the request.
- **A channel connects only via `my_chat_member`** (bot added as channel admin) — never accept a typed-in chat ID from a user; that would let anyone claim a channel they don't control. See `channel.Repository.ConnectByTelegramID` and architecture.md.
- `channel.Announcer` is an **interface defined in `channel`**, satisfied by `*tgbot.Announcer` — `channel` must never import `tgbot` (the reverse already does, for `my_chat_member`; Go disallows the cycle). If you need the bot to call into `channel` for something new, extend the existing interface pattern rather than adding a direct import.
- `tgbot.Announcer` and `tgbot.Bot` share formatting helpers as **package-level functions** (`formatEventTime`, `eventPlaceLabel`, `buildWebURL`, `tashkentLocation`) rather than `*Bot` methods, specifically so `Announcer` (used in the API process, never polls) can reuse them without needing a `*Bot`. Keep new shared bot logic at this level too, not as a `*Bot`-only method, if `Announcer` might ever need it.
- `event.Repository.ListTrending` ranks by **RSVP velocity in the last 7 days**, not lifetime `goingCount` — a separate query (`trendingSelect`) from `ListPublic`, deliberately not parameterized onto the shared `eventSelect`.
- Working directory: Go commands need `cd backend`; the Makefile handles this.
