# Roadmap

## v1 (built)

Telegram login · profiles · organizers · event CRUD with draft/publish
lifecycle · cover uploads · explore with filters + full-text search · SSR
event pages with OG previews · RSVP with capacity · QR tickets · web check-in
scanner · attendee lists · reminder worker (24h/1h) · Telegram bot (/start,
/events, inline RSVP, reminders) · VPS deploy stack.

Post-MVP, also done: admin moderation (`/admin` UI: stats, event
unpublish/cancel, user ban/unban), organizer stats + attendee CSV export,
`finished`-status automation + refresh-token purge (worker housekeeping),
GitHub Actions CI.

## v2 backlog (rough priority order)

1. **Payments & paid events** — Payme/Click integration; price fields on
   events; paid-ticket issuance. The single biggest scope item; touches
   events, rsvp, and a new `payment` module. Keep provider callbacks in the
   backend only.
2. **Telegram Mini App** — reuse the API; auth via Mini App `initData`
   validation (same HMAC family as the login widget, different data-check
   rules — do not reuse `VerifyTelegramLogin` blindly).
3. **Bot i18n** — uz/ru/en strings keyed off `users.language` (bot is
   English-only today).
4. **Post-event feedback** — new `notification` kind + rating table.
5. **Trending/popular sections** — RSVP-velocity ranking on explore.
6. **Channel announcements from bot** — organizers push events to their
   Telegram channels.
7. **Category/city management in admin** — reference data is
   migration-seeded today.
8. **Monetization tiers** (Free/Pro/Business) — plan table + limits
   middleware; depends on payments.
9. **Object storage for uploads** — S3-compatible; needed only when a second
   app server appears.
10. **Google auth fallback** — only if data shows Telegram-only login loses
    users.

## Operational debt (small, do opportunistically)

- golangci-lint config (CI runs build/vet/test today).
- `frontend/src/app/organizer/events/[id]/edit` loads via `/events/mine` list;
  switch to a dedicated owner-scoped GET when convenient.
- RSVP-trend analytics for organizers (current stats are lifetime totals).
