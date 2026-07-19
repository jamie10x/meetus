# Meetus.uz API Contract

Base URL: `/api`. All responses use the envelope:

```json
{ "data": ... }                                          // success
{ "error": { "code": "...", "message": "..." } }         // failure
```

Error codes: `validation_error` (400), `unauthorized` (401), `forbidden` (403),
`not_found` (404), `conflict` (409), `internal_error` (500).

Authenticated endpoints require `Authorization: Bearer <accessToken>`.

## Auth

### POST /auth/telegram
Body: the raw field map from the Telegram Login Widget callback
(`id`, `first_name`, `last_name?`, `username?`, `photo_url?`, `auth_date`, `hash`).

Response `data`:
```json
{
  "user": { "id": 1, "name": "...", "username": null, "avatarUrl": null,
             "cityId": null, "district": null, "language": "uz", "isAdmin": false, "createdAt": "..." },
  "tokens": { "accessToken": "...", "refreshToken": "...",
               "accessExpiresIn": 900, "refreshExpiresIn": 2592000 }
}
```

### POST /auth/telegram-miniapp
Body: `{ "initData": "<raw window.Telegram.WebApp.initData string>" }` — pass
it through unparsed; the backend does its own URL-query decoding.

Different signing scheme from the Login Widget (see architecture.md) — do
not confuse `VerifyMiniAppInitData` with `VerifyTelegramLogin`, and initData
is rejected if older than 1 hour (vs. 24h for the widget, since initData is
minted fresh on every Mini App launch). Response shape is identical to
`/auth/telegram`. A brand-new user's language is guessed from initData's
`user.language_code` (the widget has no such field, so it defaults to `uz`
there instead).

### POST /auth/refresh
Body: `{ "refreshToken": "..." }` → `data`: token pair (rotation: old token is revoked).

### POST /auth/logout
Body: `{ "refreshToken": "..." }` → `data`: `{ "loggedOut": true }`. Idempotent.

## Users

### GET /me (auth)
→ `data`: user object (shape above).

### PATCH /me (auth)
Body (all optional): `{ "name", "cityId", "district", "language" }`.
`language` ∈ `uz | ru | en`. → `data`: updated user.

## Meta

### GET /meta/cities · GET /meta/categories
→ `data`: `[{ "id", "slug", "nameUz", "nameRu", "nameEn" }]`

### POST /admin/cities · POST /admin/categories (auth + admin)
Body: `{ "slug"* (≤50), "nameUz"* (≤100), "nameRu"* (≤100), "nameEn"* (≤100) }`
→ 201 `data`: the created row. 400 if the slug is already in use.

### PATCH /admin/cities/:id · PATCH /admin/categories/:id (auth + admin)
Body: any subset of the fields above → `data`: the updated row.

### DELETE /admin/cities/:id · DELETE /admin/categories/:id (auth + admin)
409 if still referenced by an existing event (cities: or a user) — delete or
reassign those first. → `data`: `{ "deleted": true }`.

## Organizers

### POST /organizers (auth)
Body: `{ "displayName" (≤100), "bio"? (≤1000) }` → 201 `data`: organizer.
409 if the user is already an organizer.

### GET /organizers/me (auth)
→ `data`: `{ "id", "displayName", "bio", "avatarUrl", "createdAt" }`. 404 if none.

### GET /organizers/me/stats (auth)
→ `data`: `{ "totalEvents", "upcomingPublished", "totalRsvps", "totalCheckins" }`

## Events (organizer-only management)

All routes require auth **and** an organizer profile (403 otherwise).

Event object:
```json
{ "id", "organizerId", "organizerName", "title", "description",
  "categoryId", "categorySlug", "cityId", "citySlug", "district",
  "locationName", "address", "lat", "lng", "isOnline",
  "startsAt", "endsAt", "capacity", "coverUrl",
  "status", "visibility", "goingCount", "createdAt" }
```
`status` ∈ `draft | published | canceled | finished`.

### POST /events
Body: `{ "title"*, "description", "categoryId"*, "cityId", "district",
"locationName", "address", "lat", "lng", "isOnline", "startsAt"* (RFC3339),
"endsAt", "capacity", "coverUrl", "visibility" }`.
Offline events require `cityId`. → 201, status `draft`.

### GET /events/mine
→ `data`: array of the organizer's events, newest start first.

### PATCH /events/:id
Same body as create. Rejected for canceled/finished events (409).

### POST /events/:id/publish · /unpublish · /cancel
Lifecycle transitions: draft→published (start must be in the future),
published→draft, any active→canceled. Invalid transitions → 409.

A successful publish also fires an **auto-announcement**: the organizer's
connected channels each get the event posted automatically, in the
background, right after the response is sent — no extra request needed. See
[Channels & announcements](#channels--announcements) below for the language
each channel gets it in. Auto-announce failures (e.g. the bot lost admin
rights in a channel) are logged server-side, not surfaced in the publish
response.

### DELETE /events/:id
Drafts only (409 otherwise) → `data`: `{ "deleted": true }`.

## Explore (public)

### GET /explore/events
Query params (all optional): `city` (slug), `category` (slug), `from`/`to`
(RFC3339), `online` (bool), `q` (full-text search), `cursor`, `limit` (≤50,
default 20). Returns published public upcoming events, soonest first.

→ `data`: `{ "items": [event...], "nextCursor": string|null }`
Pass `nextCursor` back as `cursor` for the next page.

### GET /explore/trending
Query params (all optional): `city` (slug), `limit` (≤20, default 6). Ranks
published public upcoming events by RSVP velocity — joins in the **last 7
days**, not lifetime total or date — ties broken by soonest start.

→ `data`: array of event objects, each with one extra field:
`"recentGoing"` (int — the count that drove the ranking).

### GET /explore/events/:id
→ `data`: event. Resolves published, finished, and canceled events
(unlisted events resolve by direct link); drafts → 404.

## RSVP & Tickets

Ticket object: `{ "code", "qr", "checkedInAt" }`. The `qr` value
(`code.signature`, HMAC-SHA256) is what gets rendered as the QR code.

### POST /events/:id/rsvp (auth)
Joins the event; capacity-checked in a transaction. Re-joining after a
cancel re-activates the same ticket. → 201 ticket.
409: already joined / event full / not published / already started.

### DELETE /events/:id/rsvp (auth)
Cancels the caller's RSVP → `{ "canceled": true }`. 404 if not joined.

### GET /events/:id/rsvp (auth)
→ the caller's active ticket for the event, 404 if none.

### GET /me/tickets (auth)
→ `data`: array of tickets with event info
(`eventId`, `eventTitle`, `eventStatus`, `startsAt`, `isOnline`,
`locationName`, `citySlug`, `coverUrl` + ticket fields), soonest first.

## Check-in (organizer)

### POST /checkin (auth + organizer)
Body: `{ "qr": "<scanned value>" }`. Verifies the HMAC signature, that the
ticket belongs to one of the caller's events, the RSVP is active, and the
ticket is unused. → `data`: `{ "attendeeName", "eventTitle", "checkedInAt" }`.
409 on duplicate scan, 403 for another organizer's ticket.

### GET /events/:id/attendees (auth + organizer, owner only)
→ `data`: `[{ "userId", "name", "username", "avatarUrl", "rsvpAt", "checkedInAt" }]`

### GET /events/:id/attendees.csv (auth + organizer, owner only)
CSV download (`name`, `username`, `rsvp_at`, `checked_in_at`), not the JSON envelope.

## Feedback

### POST /events/:id/feedback (auth)
Body: `{ "rating": 1-5 }`. Caller must have an RSVP row for the event (any
status — canceling afterward doesn't retract the right to rate). Upserts:
resubmitting changes the rating. → `data`: `{ "submitted": true }`.
403 if the caller never RSVP'd.

### GET /events/:id/feedback (auth + organizer, owner only)
→ `data`: `{ "count", "average" }` (average is `0` when count is `0`).

### GET /events/:id/feedback/comments (auth + organizer, owner only)
→ `data`: `[{ "userName", "rating", "comment", "createdAt" }]` — newest
first, only rows that actually have a comment (most ratings won't).

Delivery: the worker prompts each attendee once via the Telegram bot
(inline 1-5 star buttons) shortly after their event is auto-marked
`finished` — see [architecture.md](architecture.md). After tapping a star,
the bot asks a follow-up "want to add a comment?" with a Skip button; the
attendee's next free-text reply (within 10 minutes) is attached as the
comment, via a short-lived Redis marker.

## Admin

All routes require auth **and** `users.is_admin` (403 otherwise). Admin is
granted by SQL only: `UPDATE users SET is_admin = true WHERE telegram_id = ...`.
The `/me` response includes `isAdmin` so the frontend can show admin nav.

### GET /admin/stats
→ `data`: `{ "users", "organizers", "eventsByStatus": {status: n},
"upcomingEvents", "rsvps7d", "rsvps30d", "checkins30d" }`

### GET /admin/events?status=
All events, any status, newest first (limit 50). → `data`: event array.

### POST /admin/events/:id/unpublish · /cancel
Moderation overrides — force-set status regardless of lifecycle rules. → event.

### GET /admin/users?q=
Search by name/username (ILIKE, limit 50)
→ `data`: `[{ "id", "name", "username", "isBanned", "isAdmin", "createdAt" }]`

### POST /admin/users/:id/ban · /unban
Ban blocks login **and** token refresh. Admins cannot ban admins or
themselves. → `data`: `{ "id", "isBanned" }`

## Channels & announcements

A channel is connected by adding the bot as an **admin** to a Telegram
channel — never by submitting a chat ID. See architecture.md for the
verified-linking flow (`my_chat_member`).

### GET /organizers/me/channels (auth + organizer)
→ `data`: `[{ "id", "chatTitle", "language", "connectedAt" }]`. `language` is
`null` until the organizer sets a per-channel override (see below).

### PATCH /organizers/me/channels/:id (auth + organizer, owner only)
Body: `{ "language": "uz" | "ru" | "en" | null }`. Sets or clears (`null`)
the channel's own announcement language, overriding the organizer's own
language for that one channel. → `data`: the updated channel.

### DELETE /organizers/me/channels/:id (auth + organizer, owner only)
→ `data`: `{ "disconnected": true }`. 404 if not found, 403 if owned by
another organizer.

### POST /events/:id/announce (auth + organizer, owner only)
Body: `{ "channelId": number }`. Posts the event to the given channel,
rendered in the channel's own `language` override if one is set, else the
**caller's own** `users.language`. Requires the event to be **published**
and the channel to belong to the caller. (Publishing an event also
auto-announces to every connected channel this same way — see
`POST /events/:id/publish` above — this endpoint exists for manual re-sends.)

→ `data`: `{ "sent": true }`.
409 if the event isn't published, 403 if the channel or event belongs to
someone else, 400 `"channel announcements are not configured on this
server"` if the backend has no `TELEGRAM_BOT_TOKEN` (dev default), 500 if
Telegram rejects the send (e.g. the bot lost admin rights since connecting).

## Uploads

### POST /uploads (auth)
Multipart field `file`: JPEG/PNG/WebP ≤ 5 MB → 201 `data`: `{ "url" }`.
Files are served publicly at `/uploads/<name>`.
