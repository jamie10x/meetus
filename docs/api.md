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
             "cityId": null, "district": null, "language": "uz", "createdAt": "..." },
  "tokens": { "accessToken": "...", "refreshToken": "...",
               "accessExpiresIn": 900, "refreshExpiresIn": 2592000 }
}
```

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

## Organizers

### POST /organizers (auth)
Body: `{ "displayName" (≤100), "bio"? (≤1000) }` → 201 `data`: organizer.
409 if the user is already an organizer.

### GET /organizers/me (auth)
→ `data`: `{ "id", "displayName", "bio", "avatarUrl", "createdAt" }`. 404 if none.

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

### DELETE /events/:id
Drafts only (409 otherwise) → `data`: `{ "deleted": true }`.

## Explore (public)

### GET /explore/events
Query params (all optional): `city` (slug), `category` (slug), `from`/`to`
(RFC3339), `online` (bool), `q` (full-text search), `cursor`, `limit` (≤50,
default 20). Returns published public upcoming events, soonest first.

→ `data`: `{ "items": [event...], "nextCursor": string|null }`
Pass `nextCursor` back as `cursor` for the next page.

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

## Uploads

### POST /uploads (auth)
Multipart field `file`: JPEG/PNG/WebP ≤ 5 MB → 201 `data`: `{ "url" }`.
Files are served publicly at `/uploads/<name>`.
