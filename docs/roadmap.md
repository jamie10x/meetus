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
feedback** (bot-delivered 1-5 star prompts + organizer-facing average),
**full website i18n** (next-intl, locale-prefixed `/uz` `/ru` `/en` URLs,
every page translated), **Telegram Mini App support** (the same Next.js
deployment doubles as a Mini App — silent `initData` auto-login, falls
back to the normal Login Widget in a plain browser; bot's "Open on
Meetus.uz" button opens the event page as a Mini App in place instead of
an external tab), **trending sections** (RSVP-velocity ranking on the home
and Explore pages), and **bot channel announcements** (organizers add the
bot as channel admin to connect it — verified via `my_chat_member`, never
a typed-in chat ID — then push a published event to the channel from the
organizer dashboard). See docs/architecture.md for how all of these work.

## Payments

Decided: **Payme** (not Click, not Telegram Bot Payments/Stars — the latter
was considered since it needs no separate merchant account, but only works
inside a Telegram chat, not from the plain website — though now that the
site also runs as a Mini App, that constraint is looser than when this was
first decided; still not chosen).

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
   callbacks in the backend only. Since the site now runs as a Mini App
   too, worth revisiting whether Telegram Bot Payments (launched from
   inside the Mini App, not just the plain website) makes more sense than
   it did when only the plain website existed.
2. **Category/city management in admin** — reference data is
   migration-seeded today.
3. **Monetization tiers** (Free/Pro/Business) — plan table + limits
   middleware; depends on payments.
4. **Object storage for uploads** — S3-compatible; needed only when a second
   app server appears.
5. **Google auth fallback** — only if data shows Telegram-only login loses
   users.
6. **Feedback comments** — `event_feedback` currently has no `comment`
   column (deliberately, see data-model.md); add one only if free-text
   feedback becomes a real requirement, alongside a bot conversational-reply
   flow to collect it.
7. **Mini App native chrome** — Telegram's `BackButton`/`MainButton` APIs
   and theme-param mirroring aren't wired up; the Mini App today relies on
   in-page nav and the app's own light/dark Tailwind theme, which works
   fine but doesn't feel fully native. Low priority polish.
8. **Multiple channels per organizer, per-channel language** — connecting
   more than one channel already works (`channel_connections` has no
   uniqueness constraint on `organizer_id`), but there's no per-channel
   language override yet — an announcement always uses the *organizer's*
   language, not a per-channel setting. Add only if an organizer actually
   runs channels in different languages.
9. **Channel announcement scheduling / auto-post on publish** — today,
   announcing is a manual click per channel per event; auto-posting when
   an event is published is a natural next step once the manual flow has
   seen real use.

## Operational debt (small, do opportunistically)

- golangci-lint config (CI runs build/vet/test today).
- `frontend/src/app/[locale]/organizer/events/[id]/edit` loads via
  `/events/mine` list; switch to a dedicated owner-scoped GET when convenient.
- RSVP-trend analytics for organizers (current stats are lifetime totals).
- Bot date formatting uses numeric DD.MM + a translated weekday abbreviation
  (no localized month names); revisit only if it reads as insufficient.
- Message catalogs (`frontend/messages/*.json`) are good-faith translations,
  not professionally reviewed — same caveat as the bot's i18n from day one.
