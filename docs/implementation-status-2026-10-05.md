# Review implementation status — 2026-10-05

The original review records baseline findings; its line references describe
the code before these changes. This document records the implementation and
remaining release checks. Changes have not been deployed; the review branch contains the implementation.

## Implemented repairs

| Review | Implementation |
|---|---|
| R1 | API/auth/RSC responses bypass the service worker; old caches are purged; offline tickets are scoped to a login session and cleared on account change. |
| R2 | Mini App HMAC includes signature; duplicate fields and future timestamps are rejected. |
| R3 | Refresh consumption and successor insertion share a locked transaction; failed insertion rolls back consumption. |
| R4 | Event edits preserve omitted visibility; the edit form carries visibility. |
| R5 | Authenticated HTTP requests and bot user entrypoints reject banned users. |
| R6 | Check-in requires the selected event and locks event/RSVP/ticket; ownership, status, time window and single use are enforced. |
| R7 | Lifecycle changes compare expected status/version; publication jobs commit with the successful transition. Draft deletion with attendance history returns conflict. |
| R8 | Next/MapLibre and Go dependencies patched; Go remains on the 1.25 line at 1.25.13. CI audits production npm dependencies and reachable Go vulnerabilities. |
| R9–R10 | Refresh promises reset, transient errors preserve credentials, tabs coordinate rotation, and late responses cannot undo logout or replay a write under another account. |
| R11–R12 | Unchanged avatars are omitted from profile PATCH; explicit nullable city/district fields clear correctly. |
| R13–R14 | Capacity cannot drop below attendance or change with a waitlist; only future published events promote; rejoining resets waitlist priority. |
| R15 | Feedback requires a finished event. |
| R16 | Native MainButton listeners/progress are cleaned up and stale RSVP loads ignored. |
| R17 | CSV downloads use the shared refresh path; spreadsheet formula prefixes are neutralized. |
| R18 | Explore requests track generations, lock pagination, and deduplicate returned IDs. |
| R19 | Calendar UID uses the event ID and remains stable across edits. |
| R20 | Submitted geocoding goes through a cached server proxy with a shared one-request-per-second upstream limit. |
| R21 | Docker seeds upload ownership for UID 65532; fresh-volume authenticated upload/read verified. |
| R22 | CI streams the tested deploy script and SHA; deployment is serialized, avoids stale-run rollback, checks readiness/revision, and retains old images. |

Supporting changes: durable per-recipient jobs for announcements, promotions,
reminders, feedback and digests; bounded retries/leases and terminal failures;
per-user RSVP/upload rate limits; full image decoding, dimensions and partial-file cleanup;
request limits/timeouts; dependency readiness; atomic DB/upload backups;
owner event GET and management pagination; scanner startup cleanup/manual
entry; event return path through login; accessible controls and synchronized
translation catalogs; parallel related/series reads; current architecture,
API, deployment and onboarding documentation.

## Verified locally

- PostgreSQL 16 and Redis 7 in isolated containers; migrations through 0017.
- Go build, vet and race-enabled integration suite, with CI mode preventing
  missing-database skips.
- Concurrency/rollback refresh tests, lifecycle single-winner/outbox dedupe,
  event-scoped check-in, feedback timing, nullable profile, bans, queue retry,
  waitlist rejoin/capacity/closed-event behavior, and management pagination.
- Frontend lint and production build; eight Node regressions covering
  refresh, logout, offline isolation, calendar identity and catalog parity.
- Chromium service-worker test covering legacy-cache purge, account-separated
  API responses, and offline ticket shell. This uses a fixture server and the
  actual worker, not a full Telegram or production-app journey.
- `npm audit --omit=dev`: zero findings. `govulncheck`: zero reachable
  findings; it also reports non-reachable advisories in transitive modules.
- Production-build Chromium smoke: uz/ru/en login pages render without page
  errors; icon/apple-icon/PWA icon/manifest return 200 without locale redirects.
- Backend Docker build; fresh non-root upload volume accepts and serves PNG;
  upload archive extracted successfully into an empty directory.
- PostgreSQL dump restored into a separate empty database (version 16,
  dirty=false). Shell deployment/backup scripts pass syntax checks.

## Remaining release and product work

- Real Telegram iOS/Android login, native chrome, camera and calendar checks;
  and external map tile-provider availability.
- Production deployment, offsite backup destination/replication, production
  restore drill, and connecting the read-only host health check to operator alerts.
- Five development-only npm audit findings remain in the braces/fast-glob
  ESLint dependency chain; npm proposes an incompatible Next ESLint downgrade.
  Do not apply that downgrade as an audit-count workaround.
- Representative production-volume query benchmarks, broader accessibility
  review and upload orphan reclamation/storage budgets remain follow-up work.
- Unlisted distribution and moderation-hold policy remain product decisions.
  Existing official-channel distribution of every published event is preserved.

Deploy migrations 0015–0017 before the new API/worker. Telegram delivery
is at least once; a remote acceptance followed by a crash can duplicate a
message. Capacity edits deliberately reject while a waitlist exists instead
of inventing a new bulk-promotion policy. No production readiness claim is
made from local tests alone.

## Continuation implemented on 2026-10-06

- Real production-build browser journeys now cover signed development login,
  event creation/publication, join/waitlist/cancel/promotion, offline ticket
  reload, logout/account isolation, rejected canceled QR and successful check-in.
  A second journey renders the actual MapLibre canvas and marker popup using
  a deterministic style fixture; external tiles and native Telegram remain manual.
- Worker progress expires after 90 seconds without successful delivery-loop
  progress. Admin operations reports worker health and queue totals without
  exposing recipient payloads. Host checks cover readiness, queue age/failures,
  backup freshness and upload disk space; mock scenarios run in CI.
- Organizer routes require a restored session; edit failures terminate the
  loading state. Date/time controls have translated labels/locales, selected-day
  clicks close the picker, and common API failures use translated messages.
  Stale reverse-geocoding responses cannot overwrite a newer selection.
- Migration 0017 adds the public discovery cursor index. Synthetic 20,000-row
  measurements and reproduction instructions are in `performance-2026-10-06.md`.
- `audit-uploads` inventories managed images and old unreferenced candidates,
  without deleting files. Deployment runbook documents its conservative use.
- Final verification: backend build/vet/race integration suite; frontend lint,
  production build, eight Node regressions, one service-worker browser test,
  two attendance/map journeys; health-check scenarios; backend Docker build
  and upload-audit command against the disposable database all passed.
- Fresh audit detected a new production `source-map-js` advisory; the lockfile
  now selects 1.2.2. Production npm audit is clean again, and govulncheck reports
  zero reachable vulnerabilities. The five development-only findings remain.

## Push preparation

Malformed database configuration errors now omit the original DSN, preventing
credentials from entering startup logs. A regression test covers this case.
CI also runs on `codex/**` pushes; production deployment remains restricted
to `main`. Changes are prepared on `codex/review-hardening` for review.

## Website, Mini App and bot UX continuation

- Mobile Explore/Tickets/Organize/Profile navigation with safe-area spacing,
  focus states and reduced-motion support; native Telegram action avoids
  overlapping navigation and direct-entry Back has an Explore fallback.
- Attendance and tickets refresh on return/reconnect and while visible. Failed
  reads show retry states rather than an empty account or a false join action.
  Ticket history is separate, offline status explicit, inactive entry QRs hidden.
- Website language selection updates the shared account preference; returning
  from a bot language change synchronizes the route. All new copy is in uz/ru/en.
- Explore filters survive navigation and have clear/reset controls. Cards show
  uploaded covers and waitlist availability. Event forms include a preview and
  explicit Tashkent time, matching bot and event displays across timezones.
- Bot waitlist responses accurately describe attendance; ticket photos and
  waitlist replies include an event Mini App management button.
- Regression coverage adds a mobile Mini App SDK fixture with real signed
  auto-login/API calls, native RSVP action, outage/retry, language synchronization,
  external cancellation and no-reload promotion, plus timezone conversion tests.
  This does not replace real Telegram iOS/Android camera/native-chrome testing.

## Navigation and cancellation safeguards — 2026-10-09

Attendance cancellation now requires a separate confirmation explaining ticket
invalidation or loss of waitlist position. The safe action receives focus and
Escape dismisses the confirmation. No cancellation request is sent on the
initial Cancel click.

Unsaved event forms prompt before same-tab links, language selection, desktop
logout, Telegram Back, and browser unload. Automatic bot-language navigation
is deferred while edits are pending. New copy is translated in all locales.
Browser-controlled same-document history traversal and force-closing a mobile
WebView cannot be reliably blocked by this guard; this is not draft autosave.
