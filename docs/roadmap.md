# Roadmap

## v1 (built)

Telegram login · profiles · organizers · event CRUD with draft/publish
lifecycle · cover uploads · explore with filters + full-text search · SSR
event pages with OG previews · RSVP with capacity · QR tickets · web check-in
scanner · attendee lists · reminder worker (24h/1h) · Telegram bot (/start,
/events, inline RSVP, reminders) · VPS deploy stack.

## v2 backlog (rough priority order)

1. **Payments & paid events** — Payme/Click integration; price fields on
   events; paid-ticket issuance. The single biggest scope item; touches
   events, rsvp, and a new `payment` module. Keep provider callbacks in the
   backend only.
2. **Telegram Mini App** — reuse the API; auth via Mini App `initData`
   validation (same HMAC family as the login widget, different data-check
   rules — do not reuse `VerifyTelegramLogin` blindly).
3. **Admin panel** — moderation (hide/feature events, ban users via
   `users.is_banned`, which auth already enforces), category/city management,
   basic platform metrics. Until then: SQL by hand.
4. **Organizer analytics + CSV export** — attendance rate, RSVP trends;
   `GET /events/:id/attendees.csv`.
5. **`finished` status automation** — worker job flipping past published
   events to `finished` (enum value already exists).
6. **Bot i18n** — uz/ru/en strings keyed off `users.language` (bot is
   English-only today).
7. **Post-event feedback** — new `notification` kind + rating table.
8. **Trending/popular sections** — RSVP-velocity ranking on explore.
9. **Channel announcements from bot** — organizers push events to their
   Telegram channels.
10. **Monetization tiers** (Free/Pro/Business) — plan table + limits
    middleware; depends on payments.
11. **Object storage for uploads** — S3-compatible; needed only when a second
    app server appears.
12. **Google auth fallback** — only if data shows Telegram-only login loses
    users.

## Operational debt (small, do opportunistically)

- Housekeeping job: purge expired `refresh_tokens`.
- golangci-lint config + CI workflow (tests run locally today).
- `frontend/src/app/organizer/events/[id]/edit` loads via `/events/mine` list;
  switch to a dedicated owner-scoped GET when convenient.
