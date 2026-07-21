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
and Explore pages), **bot channel announcements** (organizers add the
bot as channel admin to connect it — verified via `my_chat_member`, never
a typed-in chat ID — then push a published event to the channel from the
organizer dashboard), **admin category/city CRUD** (create/edit/delete
directly in `/admin`, not just via migration), **per-channel language
override** (a channel can pin its own announcement language, independent
of whichever organizer triggers the send), **auto-announce on publish**
(publishing a draft now pushes to every connected channel automatically, in
the background — the manual announce button still exists for re-sends),
**feedback comments** (a bot conversational follow-up after the star
rating — reply with a message or tap Skip — shown to organizers on the
attendees page), **Mini App native chrome** (Telegram's `BackButton`
wired to in-app navigation, `MainButton` replacing the on-page join button,
header/background color synced to Telegram's own theme), **an official
platform-wide channel** (every published event, from every organizer,
posted automatically — configured via `TELEGRAM_OFFICIAL_CHANNEL_ID`, not
tied to any one organizer's own channel connections) and **group feed
support** (a Telegram group opts into that same platform-wide feed the
same way a channel connects — bot added as admin, verified via
`my_chat_member`), **QR tickets delivered as an in-bot photo** (join
returns the ticket QR as an actual image message, not just a link out to
the website; `/tickets` resends every upcoming one), an **RSVP waitlist**
(a full event waitlists instead of rejecting the join; canceling a
confirmed spot auto-promotes the longest-waiting waitlisted attendee and
messages them their ticket), a **weekly "what's on" digest** (opt-in via
`/digest`, sent Monday mornings, scoped to the subscriber's city if set),
**`/mute`** (dials reminder and feedback-prompt frequency to zero without
leaving events), and **location-based event suggestions** (`/nearby`
shares live location, bot replies with the closest published events by
great-circle distance).

Also done, website-side: a **map view on Explore** (Leaflet + CARTO's free
dark basemap, toggled against the plain list — no paid API key), **add to
calendar** (.ics download + Google Calendar link on tickets and event
pages, works inside the Telegram Mini App webview too), **recurring weekly
events** (an organizer creates a whole series at once; each occurrence is
independently publishable/editable/cancelable, sharing a `seriesId`),
**related and "other dates in this series" sections** on the event detail
page, **SEO** (a generated `sitemap.xml` covering every published event ×
locale, schema.org `Event` structured data on event pages for search rich
results), a **PWA** (installable, offline-viewable tickets — the QR
already renders fully client-side, so a cached ticket list works with no
signal at the door), and an **organizer verification badge** (admin-set
trust signal shown next to a verified organizer's name everywhere their
events appear). See docs/architecture.md for how all of these work.

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
2. **Monetization tiers** (Free/Pro/Business) — plan table + limits
   middleware; depends on payments.
3. **Object storage for uploads** — S3-compatible; needed only when a second
   app server appears.
4. **Google auth fallback** — only if data shows Telegram-only login loses
   users.

## Operational debt (small, do opportunistically)

- golangci-lint config (CI runs build/vet/test today).
- `frontend/src/app/[locale]/organizer/events/[id]/edit` loads via
  `/events/mine` list; switch to a dedicated owner-scoped GET when convenient.
- RSVP-trend analytics for organizers (current stats are lifetime totals).
- Bot date formatting uses numeric DD.MM + a translated weekday abbreviation
  (no localized month names); revisit only if it reads as insufficient.
- Message catalogs (`frontend/messages/*.json`) are good-faith translations,
  not professionally reviewed — same caveat as the bot's i18n from day one.
