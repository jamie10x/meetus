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
GitHub Actions CI, **bot i18n** (uz/ru/en, language guessed from Telegram's
`language_code` on first contact, switchable via `/language`), **post-event
feedback** (bot-delivered 1-5 star prompts + organizer-facing average).

## Payments

Decided: **Payme** (not Click, not Telegram Bot Payments/Stars — the latter
was considered since it needs no separate merchant account, but only works
inside a Telegram chat, not from the plain website).

**Launch strategy: free for a 2-month trial**, Payme configured after. No
code changes were needed for this — the schema has no price/payment field
at all, so the platform is free by default already. Do not build
trial-timer/date-gating logic ahead of need.

**Open decision before building the payment module:** what Payme will
charge for — per-event ticket price (`price` column on `events`) vs.
organizer subscription tiers (`plan` on `organizers`, Free/Pro/Business).
Confirm with the user first; these attach to different parts of the schema.

## v2 backlog (rough priority order)

1. **Payments & paid events** (Payme — see above). The single biggest scope
   item; touches events, rsvp, and a new `payment` module. Keep provider
   callbacks in the backend only.
2. **Telegram Mini App** — reuse the API; auth via Mini App `initData`
   validation (same HMAC family as the login widget, different data-check
   rules — do not reuse `VerifyTelegramLogin` blindly).
3. **Trending/popular sections** — RSVP-velocity ranking on explore.
4. **Channel announcements from bot** — organizers push events to their
   Telegram channels.
5. **Category/city management in admin** — reference data is
   migration-seeded today.
6. **Monetization tiers** (Free/Pro/Business) — plan table + limits
   middleware; depends on payments.
7. **Object storage for uploads** — S3-compatible; needed only when a second
   app server appears.
8. **Google auth fallback** — only if data shows Telegram-only login loses
   users.
9. **Feedback comments** — `event_feedback` currently has no `comment`
   column (deliberately, see data-model.md); add one only if free-text
   feedback becomes a real requirement, alongside a bot conversational-reply
   flow to collect it.

## Operational debt (small, do opportunistically)

- golangci-lint config (CI runs build/vet/test today).
- `frontend/src/app/organizer/events/[id]/edit` loads via `/events/mine` list;
  switch to a dedicated owner-scoped GET when convenient.
- RSVP-trend analytics for organizers (current stats are lifetime totals).
- Bot date formatting uses numeric DD.MM + a translated weekday abbreviation
  (no localized month names); revisit only if it reads as insufficient.
