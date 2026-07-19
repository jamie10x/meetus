# Architecture

## System overview

```text
                    ┌────────────────────────────── VPS ──────────────────────────────┐
 Browser ── HTTPS ──►  Caddy (:443, auto-TLS)                                         │
 Telegram app       │    ├── /api/*, /uploads/*, /healthz ──► api (Go/Gin :8080)      │
    │               │    └── everything else ──────────────► frontend (Next.js :3000) │
    │ long polling  │                                                                 │
    └───────────────┼──► worker (Go: Telegram bot + reminder loop)                    │
                    │         │                │                                      │
                    │      PostgreSQL 16    Redis 7                                   │
                    │      (data + FTS)     (locks, rate limits)                      │
                    └─────────────────────────────────────────────────────────────────┘
```

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
| `auth` | Telegram login verification, refresh-token store, login/refresh/logout | `VerifyTelegramLogin`, `Service` |
| `platform/authn` | JWT issue/parse, `RequireAuth` middleware, `UserID(c)` | leaf package — `auth` and `user` both import it (avoids an import cycle) |
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

### Login (web)
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

SSR: `frontend/src/app/events/[id]/page.tsx` fetches the event server-side
(deduped via React `cache`) and emits Open Graph tags — this is what makes
event links unfurl nicely when shared in Telegram chats. That page is the
product's viral loop; don't break its SSR.

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
