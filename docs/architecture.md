# Architecture

## System overview

```text
                    ┌────────────────────────────── VPS ──────────────────────────────┐
 Browser ── HTTPS ──►  Caddy (:443, auto-TLS)                                         │
 Telegram app ──┐   │    ├── /api/*, /uploads/*, /healthz ──► api (Go/Gin :8080)      │
  (Mini App,    │   │    └── everything else ──────────────► frontend (Next.js :3000) │
   web_app btn) │   │                                                                 │
    │ long poll │   │                                                                 │
    └───────────┼───┼──► worker (Go: Telegram bot + reminder loop)                    │
                │   │         │                │                                      │
   (same site,  │   │      PostgreSQL 16    Redis 7                                   │
    HTTPS)──────┘   │      (data + FTS)     (locks, rate limits)                      │
                    └─────────────────────────────────────────────────────────────────┘
```

The frontend serves both regular browsers and Telegram Mini Apps from the
same Next.js deployment — a Mini App is just the same site loaded in
Telegram's in-app WebView with an extra SDK script; there is no separate
build or route tree for it. See "Telegram Mini App" below.

Three deployable processes, one database:

| Process | Binary | Responsibility |
|---|---|---|
| API | `backend/cmd/api` | All HTTP endpoints, static `/uploads` serving |
| Worker | `backend/cmd/worker` | Telegram bot (long polling) + reminder scan every minute |
| Frontend | `frontend/` (Next standalone) | SSR pages, client UI |

`backend/cmd/migrate` is a fourth, short-lived binary: it embeds `db/migrations/`
and runs before the API starts in production (compose `migrate` service).

## Backend module map

Modules live in `backend/internal/<name>`, each with the same internal shape:
**repository** (SQL via pgx) → **service** (business rules) → **handler**
(Gin binding + response envelope). Small modules collapse layers when a
service would be pure pass-through (`meta`, `organizer`, `upload`).

| Module | Owns | Notable exports |
|---|---|---|
| `auth` | Telegram login verification (widget **and** Mini App), refresh-token store, login/refresh/logout | `VerifyTelegramLogin`, `VerifyMiniAppInitData`, `Service` |
| `platform/authn` | JWT issue/parse, `RequireAuth` middleware, `UserID(c)` | leaf package — `auth` and `user` both import it (avoids an import cycle) |
| `platform/tglang` | Telegram `language_code` → uz/ru/en mapping | leaf package — shared by `auth` (Mini App login) and `tgbot` so both first-contact paths agree |
| `user` | users table, profile GET/PATCH | `Repository.UpsertTelegramUser` (used by auth **and** bot) |
| `organizer` | organizer profiles, `RequireOrganizer` middleware, `OrganizerID(c)` | |
| `event` | event CRUD + lifecycle (owner side), public explore queries | `Repository.ListPublic` (keyset pagination), `Service` lifecycle rules |
| `rsvp` | RSVPs, tickets, QR signing, check-in, attendees | `TicketSigner`, transactional `Repository.Join` |
| `notification` | due-reminder + due-feedback queries, sent log | `Repository.Due`/`MarkSent`, `DueFeedback`/`MarkFeedbackSent` |
| `tgbot` | Telegram bot handlers, i18n catalog, reminder/feedback message sending | `Bot.Start`, `Bot.SendReminder`, `Bot.SendFeedbackRequest`, `i18n.go` |
| `feedback` | post-event 1-5 star ratings | `Repository.Submit` (upsert, requires an RSVP), `SummaryFor` |
| `meta` | cities/categories reference data | |
| `upload` | cover image upload + static serving | content-type sniffing, 5 MB cap |
| `admin` | platform moderation (stats, event override, ban/unban) | `RequireAdmin` middleware, gated by `users.is_admin` |
| `housekeeping` | hourly janitorial pass | `Runner.Run` — finishes past events, purges expired refresh tokens |
| `platform/*` | apperr (typed errors), httpx (envelope), db, redisx, ratelimit | |

**All dependency wiring happens in `internal/server/router.go`** — constructors
take their dependencies explicitly; there is no DI framework and no globals.

## Key flows

### Login (web browser)
1. Telegram Login Widget on the Next.js site calls `onTelegramAuth` with a signed field map.
2. `POST /api/auth/telegram` → `auth.VerifyTelegramLogin`: rebuild data-check-string
   (sorted `key=value` lines minus `hash`), secret = SHA256(bot token),
   compare HMAC-SHA256, reject `auth_date` older than 24 h.
3. `user.Repository.UpsertTelegramUser` creates/refreshes the row by `telegram_id`.
4. `auth.Service.issuePair`: JWT access (15 min, HS256, sub = user ID) + random
   32-byte refresh token, stored **SHA-256-hashed** in `refresh_tokens`.
5. Refresh rotates: old row revoked, new pair issued. Reuse of a revoked token → 401.

The bot shares identity for free: bot users have the same `telegram_id`,
so `/start` and inline RSVP call the same `UpsertTelegramUser`.

### Login (Telegram Mini App) — a different signing scheme, not a variant
`window.Telegram.WebApp.initData` uses a **different HMAC derivation** from
the Login Widget — don't reuse `VerifyTelegramLogin` for it:
- Widget: `secret = SHA256(botToken)`.
- Mini App: `secret = HMAC-SHA256(key="WebAppData", message=botToken)`.
- Both then do `hash = HMAC-SHA256(key=secret, message=dataCheckString)` —
  same final step, different secret, so a payload valid for one never
  verifies under the other (guarded by a test:
  `auth.TestVerifyMiniAppInitData_NotInterchangeableWithLoginWidget`).

`auth.VerifyMiniAppInitData` also excludes `signature` (not just `hash`)
from the data-check-string — that field belongs to a separate, newer
ed25519 verification scheme this project doesn't implement — and rejects
`auth_date` older than **1 hour** (tighter than the widget's 24h, since
initData is minted fresh every time the Mini App launches, not something
that sits on a static page). `POST /api/auth/telegram-miniapp` wraps it;
`auth.Service.loginTelegramUser` is the shared tail (upsert, ban check,
issue tokens) both login paths call.

Frontend (`src/lib/auth-context.tsx`): on mount, if a session is already
stored, restore it as before. Otherwise check
`window.Telegram.WebApp.initData` (populated by the SDK script — see
"Telegram Mini App" below) — if present, silently POST it to
`/auth/telegram-miniapp` and sign the user in with no tap required; if
empty (a plain browser), fall through to the normal Login Widget page.

### RSVP + ticket (the concurrency-sensitive path)
`rsvp.Repository.Join` runs one transaction:
1. `SELECT ... FOR UPDATE` on the event row — serializes concurrent joins.
2. Reject if not published or already started.
3. If an RSVP exists: `going` → 409; `canceled` → reactivate (same ticket).
4. If capacity set: count `going` rows, reject when full.
5. Insert RSVP; insert ticket (random 16-byte hex code) if none exists.

Ticket QR value = `code + "." + HMAC-SHA256(code, TICKET_SECRET)`.
Check-in (`POST /api/checkin`) verifies the signature **before** any DB read,
then: ticket exists → caller owns the event → RSVP active → not already
checked in → set `checked_in_at` (guarded by `WHERE checked_in_at IS NULL`).

### Reminders
Worker ticks every minute. Steps:
1. Redis `SETNX meetus:worker:reminder-scan` (50 s TTL) — one scan across instances.
2. `notification.Repository.Due(kind)` for `reminder_24h` (starts within 24 h,
   but > 2 h away) and `reminder_1h` (starts within 1 h). The query anti-joins
   `notification_log` so each (event, user, kind) fires once.
3. `tgbot.Bot.SendReminder` → Telegram; `MarkSent` **always** logs the attempt,
   even on send failure (a user who never opened the bot chat 403s forever).
4. Same tick, same lock: `notification.Repository.DueFeedback` finds attendees
   of events the hourly `housekeeping.Runner` has already flipped to
   `finished` and sends a 1-5 star rating prompt (`tgbot.Bot.SendFeedbackRequest`).
   No time window here — dedup via `notification_log` is the only guard,
   since "ask once, whenever the scan next runs after the event ends" is
   sufficient and needs no extra state.

### Bot i18n and language selection
Bot messages render in the caller's `users.language` (uz/ru/en; `tgbot/i18n.go`
holds whole-sentence templates per language — not word-by-word — so grammar
stays natural). Language is resolved once per user, at row creation, and
never silently changed again:
- **New user via the bot**: `mapTelegramLangCode(from.LanguageCode)` guesses
  from Telegram's own IETF language tag.
- **New user via web login**: defaults to `"uz"` (the Login Widget payload
  carries no language hint).
- **Existing user, any subsequent login/upsert**: language is left untouched
  — the SQL `ON CONFLICT DO UPDATE SET` list in
  `user.Repository.UpsertTelegramUser` deliberately omits `language`.
- **Explicit change**: the bot's `/language` command (inline uz/ru/en picker)
  or `PATCH /me` on the website — both go through `user.Repository.UpdateProfile`.

Feedback and RSVP-error messages reuse the same catalog; `friendlyError`
maps the small closed set of known service error strings (event full,
already joined, ...) to a translated equivalent, falling back to a generic
translated message for anything unrecognized — service-layer error text
itself stays English (it also serves the JSON API), only the bot's
*rendering* of a handful of known cases is translated.

### Post-event feedback
Attendees of a `finished` event get one Telegram prompt (5 inline star
buttons, `fb:<eventID>:<rating>` callback data) via the flow above. Tapping
calls `feedback.Repository.Submit`, which requires an existing `rsvps` row
(any status) and upserts into `event_feedback` — repeat taps just update
the rating, no special dedup needed beyond that. Organizers see the
aggregate via `GET /events/:id/feedback` (owner-only, same ownership-check
pattern as attendees/CSV). There is no web submission UI by design — the
bot is the only place a rating is collected; the website only displays
the resulting average (attendees page).

### Explore
`event.Repository.ListPublic` builds a dynamic WHERE (status published + public,
upcoming by default, optional city/category slug, date range, online flag,
`websearch_to_tsquery` against the generated `search` tsvector). Ordering is
`(starts_at, id)` with an opaque base64 keyset cursor — no OFFSET.

SSR: `frontend/src/app/[locale]/events/[id]/page.tsx` fetches the event
server-side (deduped via React `cache`) and emits Open Graph tags — this is
what makes event links unfurl nicely when shared in Telegram chats. That
page is the product's viral loop; don't break its SSR.

### Website i18n and locale routing
The website (not just the bot) is fully translated uz/ru/en via
[next-intl](https://next-intl.dev), with **locale-prefixed URLs**
(`/uz/events`, `/ru/events/5`, `/en/tickets`, ...) rather than a
cookie-only scheme — every language gets its own clean, indexable,
shareable URL, which matters here specifically because event links are
the product's viral loop (see above).

- `frontend/src/app/[locale]/...` — every page lives under the `[locale]`
  dynamic segment; there is no separate root layout above it (`[locale]/layout.tsx`
  *is* the root layout, returning `<html>`/`<body>`).
- `frontend/src/i18n/routing.ts` — `defineRouting({locales: ["uz","ru","en"], defaultLocale: "uz", localePrefix: "always"})`.
- `frontend/src/proxy.ts` — Next.js 16 renamed `middleware.ts` → `proxy.ts`
  (same `NextRequest`-based API); this just re-exports next-intl's
  `createMiddleware(routing)` under the new name. Visiting `/` redirects
  to a locale via cookie → `Accept-Language` → `uz` default.
- `frontend/messages/{uz,ru,en}.json` — one flat-ish namespaced catalog per
  language, kept in exact key-parity across all three (an out-of-sync key
  set is a bug — check with a script before adding new keys by hand).
- `frontend/src/i18n/navigation.ts` — locale-aware `Link`/`redirect`/
  `useRouter`/`usePathname` via `createNavigation(routing)`. **Always**
  import these instead of `next/link`/`next/navigation` inside
  `src/app/[locale]/**` and its components — the plain Next.js versions
  don't carry the locale prefix and will send users to the wrong language.
  (`notFound()` from `next/navigation` is the one exception — it's
  locale-agnostic and fine to use directly.)
- Server Components use `getTranslations()`/`getMessages()` from
  `next-intl/server`; Client Components use `useTranslations()`/`useLocale()`
  from `next-intl` — same message keys, different hook source per next-intl's
  dual react-server/react-client design.

### Telegram Mini App
The same Next.js deployment doubles as a Telegram Mini App: `[locale]/layout.tsx`
loads `https://telegram.org/js/telegram-web-app.js` with
`strategy="beforeInteractive"`, guaranteeing `window.Telegram.WebApp` exists
by the time `AuthProvider`'s mount effect runs (see the Mini App login flow
above) — no polling or timing guesswork needed. Outside Telegram, the SDK
object still loads (it's just a script tag) but `initData` stays empty, so
the code correctly falls through to the normal browser Login Widget.

One documented quirk: the SDK sets `--tg-viewport-height`/`--tg-viewport-stable-height`
CSS custom properties on `<html>` as soon as it runs, in **every** browser,
not just inside real Telegram. Since this happens before React hydrates,
it's a genuine (if benign) server/client attribute mismatch on `<html>` —
`suppressHydrationWarning` is set there deliberately for exactly this
reason, not as a blanket "hide problems" workaround. If you see a real
hydration bug, it will still surface as a *content* mismatch, which
`suppressHydrationWarning` does not silence.

The bot's event-detail message uses a `web_app` inline button (not a plain
`URL` button) for "Open on Meetus.uz" (`tgbot.go`, `handleEventDetail`) —
Telegram opens `web_app` buttons as a Mini App in place instead of
switching to an external browser tab. This button type is only valid in
private-chat messages, which is the bot's only context, and requires an
HTTPS URL. Bot-generated links use `Bot.webURL(lang, path)` to build
locale-correct URLs (`webBaseURL + "/" + lang + path`) so a message already
being read in Russian, say, opens the Russian page rather than bouncing
through the site's own browser-side locale detection.

## Error handling

Services return `*apperr.Error` (`Validation`, `Unauthorized`, `Forbidden`,
`NotFound`, `Conflict`, `Internal`). `httpx.Error` maps code → HTTP status and
renders the envelope; anything that isn't an `apperr` is logged and returned
as a generic 500 — raw errors never reach clients. Postgres FK/check
violations are translated to `Validation` in repositories (`mapWriteErr`).

## Security notes

- Refresh tokens and ticket codes: only hashes/HMACs matter server-side; DB
  leak ≠ usable credentials.
- Rate limits (Redis fixed-window, per IP): auth group 20/min, RSVP group
  60/min. Fails open if Redis is down (availability over strictness).
- Uploads: content sniffed via `http.DetectContentType`, JPEG/PNG/WebP only,
  5 MB cap, random hex filenames — client filename is never trusted.
- CORS: `localhost:3000` in dev, `meetus.uz` origins in production
  (`corsMiddleware` in router.go).
- JWT parser pins the HMAC signing method (rejects `alg` confusion).

## Decision log (why it is the way it is)

| Decision | Why |
|---|---|
| Telegram-only auth | UZ market is Telegram-first; makes bot linking free (same `telegram_id`); one auth path to secure |
| Gin over Chi | User preference; original description said Chi |
| pgx + hand-written SQL, no ORM | Boring, explicit, easy to review; sqlc can be adopted later without rewrites |
| Keyset pagination | Stable under inserts, no OFFSET cost |
| Signed QR + DB lookup | Signature rejects junk cheaply; DB is authority on state |
| Reminder log even on failure | Prevents infinite retries to unreachable users |
| Local-disk uploads | Single VPS; swap for S3-compatible storage only when a second server appears |
| Bot inside worker binary | One process to babysit; split later if polling load demands |
| Locale-prefixed URLs (`localePrefix: "always"`), not cookie-only | Every language gets its own shareable, indexable, OG-taggable URL — matches why SSR/OG mattered for events in the first place |
| Mini App reuses the same Next.js deployment | A Telegram Mini App is just this site + one SDK script; a separate build would duplicate the entire frontend for no benefit |
| `beforeInteractive` SDK script + `suppressHydrationWarning`, not `afterInteractive` + polling | Tried the polling approach first — `afterInteractive` doesn't strictly guarantee execution after hydration finishes, so it still raced; deterministic availability + suppressing the one known benign attribute mismatch is simpler and correct |
