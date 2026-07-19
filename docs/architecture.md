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
| `event` | event CRUD + lifecycle (owner side), public explore queries, trending ranking | `Repository.ListPublic`/`ListTrending` (keyset pagination), `Service` lifecycle rules, `Handler.SetOnPublished` (auto-announce hook, fired async after a successful publish) |
| `rsvp` | RSVPs, tickets, QR signing, check-in, attendees | `TicketSigner`, transactional `Repository.Join` |
| `notification` | due-reminder + due-feedback queries, sent log | `Repository.Due`/`MarkSent`, `DueFeedback`/`MarkFeedbackSent` |
| `tgbot` | Telegram bot handlers, i18n catalog, reminder/feedback/announcement message sending | `Bot.Start`, `Bot.SendReminder`, `Bot.SendFeedbackRequest`, `Announcer` (non-polling sender), `i18n.go`, Redis-backed pending-comment marker (`awaitFeedbackComment`/`popPendingFeedbackComment`) |
| `feedback` | post-event 1-5 star ratings + optional free-text comment | `Repository.Submit` (upsert, requires an RSVP), `SetComment`, `ListComments`, `SummaryFor` |
| `channel` | verified organizer↔Telegram-channel links, per-channel language override, announce-to-channel endpoint | `Repository.ConnectByTelegramID` (called from tgbot's `my_chat_member` handler), `Repository.SetLanguage`, `Announcer` interface (defined here, satisfied by `tgbot.Announcer`, to avoid channel↔tgbot import cycle) |
| `meta` | cities/categories reference data + admin CRUD | `Handler.RegisterAdmin` (generic create/update/delete keyed by a hardcoded table name, never user input) |
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
pattern as attendees/CSV). There is no web submission UI for the *rating*
by design — the bot is the only place a rating is collected; the website
only displays the result (attendees page).

**Free-text comment (bot conversational follow-up)**: right after a star
tap, the bot sends a second message — "want to add a comment?" with an
inline Skip button — and sets a Redis key
(`meetus:feedback-comment-await:<telegramID>` → eventID, 10 min TTL) marking
that user as "awaiting a comment". This is necessary because Telegram bot
updates are stateless per-request; Redis is the only place to park
"what was this user just asked" between messages. The attendee's very next
plain-text message (checked in `handleDefault`, before the generic fallback
hint) is popped via `GETDEL` — atomic, so a message can only ever be
consumed once, even under concurrent webhook delivery — and, if a marker was
found, attached via `feedback.Repository.SetComment` and cleared; otherwise
it falls through to the ordinary "try /events" hint. Tapping Skip (a
separate `fbskip:<eventID>` callback) just deletes the marker. Comments
never block the underlying rating: `Submit` and `SetComment` are separate
calls, so a skipped or abandoned (TTL-expired) comment prompt still leaves
the star rating recorded. Organizers read comments via
`GET /events/:id/feedback/comments` (owner-only), rendered on the
attendees page below the attendee list.

### Explore
`event.Repository.ListPublic` builds a dynamic WHERE (status published + public,
upcoming by default, optional city/category slug, date range, online flag,
`websearch_to_tsquery` against the generated `search` tsvector). Ordering is
`(starts_at, id)` with an opaque base64 keyset cursor — no OFFSET.

SSR: `frontend/src/app/[locale]/events/[id]/page.tsx` fetches the event
server-side (deduped via React `cache`) and emits Open Graph tags — this is
what makes event links unfurl nicely when shared in Telegram chats. That
page is the product's viral loop; don't break its SSR.

### Trending
`event.Repository.ListTrending` ranks published public upcoming events by
RSVP velocity — `going` RSVPs created in the **last 7 days** — not lifetime
total or date. It's a separate query (`trendingSelect`) rather than a
parameter on `eventSelect`/`ListPublic`, since those are shared by several
well-tested read paths that don't need the extra column. Ties break by
soonest start. `frontend/src/components/TrendingSection.tsx` renders it as
its own headed rail on both the home page (nationwide) and the Explore
page (respecting the current city filter only — category/date/search
filters don't apply, by design, so the rail stays a stable "what's hot"
signal rather than re-filtering with the grid below it); it renders nothing
when there's no signal yet, so a fresh install never shows an awkward empty
section.

### Channel connections and announcements
Organizers can push a published event to a Telegram channel they control.
Connection is **never** a typed-in chat ID — it's proof the bot can
actually post there:

1. Organizer adds the bot as **admin** to their channel. Telegram sends a
   `my_chat_member` update (delivered by default — no `allowed_updates`
   config needed; excluded update types are `chat_member`,
   `message_reaction`, `message_reaction_count`, and this isn't one of
   them).
2. The `go-telegram/bot` library has no dedicated handler type for
   `my_chat_member` (only message-text/callback-data/photo-caption pattern
   handlers exist) — it falls through to the **default handler**
   (`tgbot.Bot.handleDefault`), which checks `update.MyChatMember != nil`
   first, before assuming `update.Message != nil`.
3. `channel.Repository.ConnectByTelegramID` resolves the adder's Telegram
   ID → `users.telegram_id` → `organizers.user_id` in one query. No
   organizer profile for that user → `ok=false`, no row written, and the
   bot DMs a "you need an organizer profile first" message instead of
   silently failing. A channel already connected to a different organizer
   is reassigned (`ON CONFLICT (chat_id) DO UPDATE`) — the last person to
   (re-)add the bot as admin owns the channel.
4. Any other membership transition (demoted, kicked, left) calls
   `Disconnect` — the bot can no longer post there, so the link is removed
   rather than left dangling.

Sending an announcement is **not** routed through the worker's
send-everything-async pattern (reminders, feedback prompts) — it's an
organizer clicking a button and wanting to know now whether it worked, not
a scheduled batch job. So `internal/server/router.go` constructs a second,
lightweight Telegram client — `tgbot.Announcer` — directly in the API
process, built with `bot.WithSkipGetMe()` to skip the network round trip
`bot.New` would otherwise make on every API server boot. It shares
`tgbot`'s i18n catalog and formatting helpers (`formatEventTime`,
`eventPlaceLabel`, `buildWebURL` — all package-level functions, not `*Bot`
methods, precisely so `Announcer` can reuse them without needing a `*Bot`).
`POST /events/:id/announce` (`channel.Handler.announce`) checks: caller
owns the event, event is `published`, caller owns the target channel — then
calls `Announcer.SendAnnouncement`, rendered in the channel's own
`language` override if one is set, else the **caller's own**
`users.language`.

`channel.Announcer` is a Go **interface**, defined in the `channel`
package itself and satisfied structurally by `*tgbot.Announcer` — `channel`
never imports `tgbot`. It can't: `tgbot` already imports `channel` for the
`my_chat_member` handler, and Go doesn't allow the reverse. If a dev
environment has no `TELEGRAM_BOT_TOKEN` configured, `router.go` passes a
nil `channel.Announcer` rather than failing the whole server to boot —
`announce` checks for that and returns a clear "not configured" error
instead of crashing.

**Per-channel language override**: `channel_connections.language` is
nullable — `NULL` means "use whatever language the organizer who
publishes/announces happens to have", a non-null value pins that one
channel to a language regardless of who triggers the send. Set via
`PATCH /organizers/me/channels/:id`. This matters because one organizer
account often runs channels for different audiences (e.g. a uz-language
channel and a separate ru-language one) — the override lives on the
channel, not the event or the organizer, since it's a property of *that
audience*.

**Auto-announce on publish**: `event.Handler` accepts an optional
`onPublished func(ctx context.Context, e *Event)` hook (`SetOnPublished`),
fired only by the `/:id/publish` route (not unpublish/cancel) and only in
a background goroutine with `context.Background()` — the request's own
context is canceled the moment the HTTP response is written, so reusing it
would abort the send before it even starts. `router.go` wires the actual
closure: list the organizer's connected channels, resolve the organizer's
own language (`organizer.Repository.GetLanguage`, a join to `users`), then
call `Announcer.SendAnnouncement` per channel — channel override wins,
organizer's language is the fallback, same precedence as manual announce.
Failures are logged (`slog.Error("auto-announce failed", ...)`), never
surfaced to the publish response, since publish already succeeded by the
time announcing runs — a channel that lost bot admin rights shouldn't make
publishing *look* like it failed. The manual `POST /events/:id/announce`
endpoint still exists for re-sends (e.g. after editing channel language, or
if auto-announce failed and the organizer wants to retry one channel).

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

**Native chrome (BackButton, MainButton, header color)**: the Mini App
WebView has no browser chrome of its own — no back gesture bar, no
address-bar-colored status area — so the SDK exposes a few controls to fake
it convincingly:

- `useTelegramBackButton()` (`frontend/src/lib/useTelegramBackButton.ts`)
  wires Telegram's native `BackButton` to `router.back()`, shown/hidden by
  `usePathname()` — hidden on the home page (nowhere to go back to), shown
  everywhere else. Mounted once via `TelegramChrome`
  (`frontend/src/components/TelegramChrome.tsx`) in the root layout, so it
  applies across every route without each page wiring it individually.
- `RsvpSection` swaps its on-page "Join event" button for Telegram's native
  `MainButton` when running inside the Mini App (`isTelegramMiniApp()`) and
  there's actually something to join (logged in, no ticket yet, event not
  full/past) — `MainButton.showProgress()`/`disable()` during the request,
  `hide()` once a ticket exists. Outside Telegram, or once joined/full/past,
  the ordinary in-page button (or ticket/cancel UI) is used instead — the
  two never show at once.
- `TelegramChrome` also calls `setHeaderColor`/`setBackgroundColor` once on
  mount, unconditionally set to the app's single committed background color
  (`--color-ink`, `#160f16`) — the frontend is a dark-first brand identity
  (see "Frontend visual identity" below), not an OS/`prefers-color-scheme`
  toggle, so there's no light variant to pick between and `tg.colorScheme`
  is intentionally ignored here.

All three read from the same extended `TelegramWebApp` type
(`frontend/src/lib/telegram-webapp.ts`: `BackButton`, `MainButton`,
`themeParams`, `colorScheme`, `setHeaderColor`, `setBackgroundColor`) —
outside Telegram, `getTelegramWebApp()` returns `null` and every one of
these call sites no-ops via its own `if (!tg) return` guard, so none of
this affects plain-browser visitors.

### Frontend visual identity ("Bold Dark")

The frontend commits to one dark visual identity — near-black surfaces with
a warm plum-dusk undertone, not an OS-`prefers-color-scheme` toggle. There
is no light theme; `dark:` Tailwind variants and the old `zinc-*`/`sky-*`
palette are gone from the codebase. Tokens are registered as real Tailwind
v4 utilities via `@theme` in `frontend/src/app/[locale]/globals.css`:

- Surfaces: `bg-ink` (`#160F16`, base), `bg-ink-raised` (`#211722`, cards),
  `bg-ink-overlay` (`#2C1F2C`), `border-line` (`#3B2B3A`, all borders).
- Text: `text-bone` (`#F6EFE4`, primary), `text-dust` (`#BBA8B6`,
  secondary), `text-dust-dim` (`#8C7A88`, tertiary/fine print).
- Accents: `registan` (`#18ADA0`, primary teal — the saturated cobalt of
  Shah-i-Zinda/Registan majolica tilework; `registan-strong` `#3FD8C9` for
  links/highlights on dark backgrounds) and `atlas` (`#F2A73B`, secondary
  gold — the warm marigold of hand-dyed atlas silk). `pomegranate`
  (`#E1523A`) is reserved strictly for semantic error/danger states, never
  decoration.
- Type: `font-display` (Fraunces, a serif with real personality — applied
  automatically to every `h1`/`h2`/`h3` via a global CSS rule), `font-sans`
  (Hanken Grotesk, body default), `font-mono` (IBM Plex Mono — dates,
  ticket codes, counts, eyebrow labels), all loaded via `next/font/google`
  in `[locale]/layout.tsx`.
- Shape/elevation: `rounded-card` (16px, the standard card radius) and the
  `shadow-card`/`shadow-pop` utilities, all registered the same way.

Category covers (event card thumbnails, the event-detail hero, the home
hero's ticket preview) are CSS-only patterns per category slug —
`frontend/src/lib/categoryStyle.ts` — deliberately restricted to the two
brand accents (registan, atlas) so a mixed grid of categories still reads
as one palette instead of an arbitrary per-category rainbow; there are no
cover images to source or maintain.

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
| Channel linking via `my_chat_member`, never a typed-in chat ID | The only way to get *proof* the bot can actually post there; a typed ID could belong to a channel the organizer doesn't control |
| Announcer as a second, non-polling bot client (`bot.WithSkipGetMe`) in the API process, not routed through the worker's async queue pattern | Organizer wants to know *now* whether the send worked — a scheduled/queued job is the wrong shape for a one-off, user-initiated action |
| `channel.Announcer` interface defined in `channel`, not imported from `tgbot` | `tgbot` already imports `channel` (for `my_chat_member`); Go doesn't allow the reverse, so the interface lives at the consumer instead |
| Trending is a separate query, not a parameter on `eventSelect` | `eventSelect` backs several already-tested read paths that don't need the extra RSVP-velocity column; duplicating one query is simpler than parameterizing a shared one |
| Per-channel language lives on `channel_connections`, not `events` or `organizers` | It's a property of the channel's *audience*, not the event or the organizer — one organizer commonly runs channels for different language audiences |
| Auto-announce hook fires in a goroutine with `context.Background()`, not the request context | The request context is canceled the instant the HTTP response is written; reusing it would abort the Telegram send before it starts |
| Auto-announce failures are logged, never surfaced on the publish response | Publishing already succeeded by the time announcing runs; a channel that lost bot admin rights shouldn't make publish *look* like it failed |
| Feedback comment state lives in Redis (`GETDEL`), not a new DB table or in-memory map | Bot updates are stateless per-request — Redis is the only place to park "what was this user just asked" between two separate Telegram messages; `GETDEL` makes the pop atomic against concurrent webhook delivery |
| Comment prompt and rating submission are separate calls (`Submit` then `SetComment`), not one combined call | A skipped or TTL-expired comment prompt must not affect the already-recorded star rating |
| Admin meta CRUD uses one generic handler keyed by a hardcoded table name string, not two near-identical handlers | `cities` and `categories` share the exact same shape and validation; the table name is never user-supplied, so there's no injection risk, just de-duplication |
| Frontend is a committed dark-first brand identity, not an OS-`prefers-color-scheme` toggle | A single considered dark palette (see "Frontend visual identity") reads as a deliberate premium product identity; a light/dark split would have doubled the design surface for every component with no clear product need |
| Category cover art is CSS-only (gradients/patterns), never uploaded images | No image sourcing/licensing/storage needed, renders instantly, and staying disciplined to two brand accents keeps a mixed-category grid coherent instead of an arbitrary rainbow |
