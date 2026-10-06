# Project review and improvement plan — 2026-10-05

Meetus has a workable modular foundation, but privacy, authentication, and event lifecycle defects should be addressed before adding features. Keep the chosen Go/Gin/pgx, Next.js, Telegram identity, single-VPS architecture, dark branding, and free-event scope. A rewrite is unnecessary.

This is a source audit and local verification of the working tree, not a production penetration test. Findings below distinguish executed reproductions, source-confirmed behavior, and deployment risks requiring a live check. No production actions or application fixes were made.

## Scope and verification

Reviewed auth and authorization, event discovery and editing, RSVP/waitlist/check-in, profiles, moderation, reference data, channels/groups, bot callbacks, reminders/digests, uploads, database constraints, frontend auth/API/PWA flows, maps, calendars, localization, CI, containers, deployment, and backups. Reviewed the onboarding instructions and architecture/API/data-model/development/roadmap/deployment documentation.

| Check | Result |
|---|---|
| Go build and vet | Passed with `-buildvcs=false` |
| Existing Go tests | 46 passing test/subtest result records; 11 skipped; no failures |
| Frontend production build | Passed, including TypeScript and static generation |
| Frontend ESLint | Passed |
| Translation catalog leaf keys | All three locales have the same 305 keys |
| Production npm dependency audit | 6 affected packages: 2 critical, 3 high, 1 moderate; exploitability needs per-advisory analysis |
| Service worker account-isolation reproduction | Failed: a modeled request from account B received account A's cached `/api/me`; only one network call |
| Frontend refresh recovery reproduction | Failed: a request without a refresh token permanently disabled later refresh attempts until reload |
| Mini App HMAC regression probe | Failed: independently HMAC-signed initData containing `signature` was rejected |

The service worker reproduction executed the actual worker in a Node VM with a URL-keyed Cache API model; it was not a real-browser end-to-end test. The refresh reproduction executed transpiled `api.ts` with synthetic storage and HTTP responses. The temporary Go signature probe was removed after execution.

Local constraints: Apple Git fails because the Xcode license has not been accepted, so Git status/history could not be inspected. Docker is installed but its daemon is unavailable; PostgreSQL and Redis integration tests skipped. No camera/device, Telegram account, VPS, restore exercise, race/load test, or Go vulnerability scan was performed. Passing build/lint does not validate these flows. Frontend build used the existing local environment; secret values were not read or reported.

## High-priority findings

### R1 — P1: Service worker caches private and mutable APIs across accounts

Evidence: [sw.js](../frontend/public/sw.js:90), [logout](../frontend/src/lib/auth-context.tsx:101).

Every same-origin GET except navigation and tickets falls through to cache-first. In the production same-origin topology this includes `/api/me`, organizer event lists, admin data, attendee lists, metadata, discovery, and RSVP state. Cached responses do not vary by Authorization, and logout never clears caches. A later account can receive the earlier account's profile/private data; edits and new RSVPs can also appear never to take effect. The ticket cache is network-first but likewise lacks account isolation for offline fallback. This is a local browser/account-switch leak, not evidence of cross-device access or server authorization bypass.

Fix: cache-first only explicitly allowlisted immutable assets. Exclude all other APIs and Next navigation data from that fallback. Implement account-scoped ticket storage with an explicit offline identity model; erase private data and pending writes on logout/account switch. Bump worker cache versions and delete old unsafe entries, including entries held by already-installed clients. Disable registration in development. Add browser tests for A → logout → B, expired sessions, canceled RSVPs, edits, and offline relaunch.

### R2 — P1: Mini App HMAC rejects signature-bearing initData

Evidence: [miniapp.go](../backend/internal/auth/miniapp.go:62). The test signing helper repeats the same exclusion.

The bot-token HMAC verifier excludes both `hash` and `signature`. Telegram's HMAC path covers received fields other than the hash; excluding both belongs to the separate Ed25519 third-party path. Signature-bearing valid payloads therefore fail auto-login. A synthetic payload signed independently with `signature` included reproduced the rejection. This is not a captured production Telegram payload. [Telegram validation documentation](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app).

Fix: correct only the HMAC field set, retaining the distinct Widget and Mini App secret derivations. Add independent fixtures with and without `signature`, tampering tests, duplicate-key rejection, and future-date bounds. Validate with a real Telegram launch before rollout. Correct the misleading implementation comment/test helper/documentation together.

### R3 — P1: Refresh rotation is not atomic

Evidence: [Refresh](../backend/internal/auth/service.go:100), [revoke](../backend/internal/auth/repository.go:56).

Two requests can both read an unrevoked token, both revoke it successfully, and both issue successors. Revoke ignores affected-row count; issuance is a separate transaction. If successor insertion fails, the valid session is already revoked. Source-confirmed interleaving; not exercised against PostgreSQL locally.

Fix: consume the old token and insert its successor atomically, with a row lock or conditional UPDATE RETURNING. Require exactly one consumer and rollback on issuance failure. Coordinate refresh across browser tabs and prevent late refresh completion from restoring a logged-out session. Test simultaneous refresh, refresh/logout, expiration, and insert failure. Consider token-family replay detection after basic rotation correctness is fixed.

### R4 — P1: Editing an unlisted event silently makes it public

Evidence: [default visibility](../backend/internal/event/service.go:75), [form submission](../frontend/src/components/EventForm.tsx:146).

The form does not send visibility, while shared validation defaults missing visibility to public on update as well as create. Edit the title of an unlisted event created through the API, save, and it becomes discoverable.

Fix: preserve existing visibility when omitted from an update, and expose/preserve it in frontend input types and form state. Document update semantics and test create-unlisted → ordinary edit → still-unlisted.

### R5 — P1: Bot actions bypass user bans

Evidence: [bot join](../backend/internal/tgbot/tgbot.go:335), [auth middleware](../backend/internal/platform/authn/middleware.go:17).

Login and refresh reject banned users, but bot upsert/join does not inspect `IsBanned`; a banned account can keep joining via bot callbacks indefinitely. Existing HTTP access tokens also remain usable until expiration because the middleware only checks JWT validity. The latter matches the narrow documented login/refresh ban semantics, but is weaker than immediate moderation.

Fix: enforce an active-user policy shared by HTTP and bot mutation entrypoints. Explicitly decide immediate versus bounded access-token revocation, document it, and test ban enforcement across join, feedback, uploads, event management, and bot callbacks.

### R6 — P1: Check-in is not scoped to the selected event and ignores event status

Evidence: [CheckIn](../backend/internal/rsvp/service.go:169), [scanner](../frontend/src/app/[locale]/organizer/events/[id]/scan/page.tsx:45), [MarkCheckedIn](../backend/internal/rsvp/repository.go:379).

The scanner page has an event ID but sends only the QR. The server verifies organizer ownership, so scanning a ticket for event B at event A succeeds if the organizer owns both. `EventStatus` is loaded but ignored, allowing canceled/unpublished event tickets to be marked used. RSVP eligibility is read separately from the ticket update, so cancellation can race with check-in.

Fix: make event ID part of the check-in contract; atomically validate event, organizer, RSVP status, and single-use state with the write. Define the allowed attendance time window. Test wrong event/same organizer, canceled event, concurrent scans, and cancellation during a scan.

### R7 — P1: Event state transitions race and duplicate announcements

Evidence: [Publish](../backend/internal/event/service.go:152), [SetStatus](../backend/internal/event/repository.go:242).

Lifecycle preconditions are checked in a SELECT, followed by an unconditional status UPDATE. Concurrent publishes can both succeed and each fire announcements; a publish racing with cancel can overwrite cancellation. Update/Delete have similar check-then-write gaps.

Fix: apply expected-state/owner predicates atomically and check affected rows. Keep event mutation lock ordering consistent with RSVP. Generate a durable announcement job once per successful transition. Add concurrency tests for publish/publish, publish/cancel, update/cancel, and delete/publish.

### R8 — P1: Dependency security updates are overdue

Evidence: `frontend/package.json`, lockfile, and `npm audit --omit=dev --json` on the review date. Affected packages reported: Next.js and MapLibre GL (critical), nanoid/PostCSS/sharp (high), baseline-browser-mapping (moderate).

The audit suggests Next.js 16.3.8 as a fix target; resolve a supported patched release when implementing, keeping eslint-config-next aligned. The critical Next ImageResponse advisory requires attacker-controlled SVG values; reviewed icon routes use fixed artwork/constrained sizes, so this review does not establish that exploit path here. Map popups use DOM text rather than HTML strings, which is a useful mitigation but not a substitute for upgrading. [Next advisory](https://github.com/vercel/next.js/security/advisories/GHSA-vcvr-r3jv-pc5j), [MapLibre advisory](https://github.com/maplibre/maplibre-gl-js/security/advisories/GHSA-jrc7-96c5-q579).

Fix: make a focused dependency-update change, inspect each advisory's reachability, rebuild/lint, and smoke-test maps, generated icons, locale routes, and PWA updates. Add automated dependency auditing and Go vulnerability scanning. Do not treat npm severity totals as six proven exploitable application vulnerabilities.

## Medium-priority bugs and operational risks

| ID | Finding and evidence | Fix and acceptance test |
|---|---|---|
| R9 | **Refresh stays disabled after a missing-token attempt.** [api.ts:46](../frontend/src/lib/api.ts:46) returns before the `finally`, leaving `refreshInFlight` set to a resolved false promise. Synthetic reproduction: no-token 401 → login → later 401 resulted in zero refresh calls. | Check token before assigning the shared promise, or clear the promise outside the inner function for every completion. Test login without reloading after an unauthenticated protected request. |
| R10 | **Transient session-load errors erase credentials.** [auth-context.tsx:52](../frontend/src/lib/auth-context.tsx:52) clears tokens on any `/me` failure and returns without Mini App fallback. Cross-origin offline dev has no cached `/me`; fixing R1 also exposes this flaw in production unless offline session restoration is redesigned. | Distinguish transport/5xx failures from terminal session rejection; keep credentials on transient failures. Reattempt Mini App auth on definitively invalid sessions. Test cold offline ticket access and API outages. |
| R11 | **Profile save disables Telegram avatar sync without uploading.** [profile save](../frontend/src/app/[locale]/profile/page.tsx:66) sends the existing avatar on every save, and [UpdateProfile](../backend/internal/user/repository.go:95) marks any non-null avatar custom. | Send avatar changes only when explicitly edited; validate the intended custom-upload path. Test changing language with a Telegram avatar and then logging in with a new Telegram photo. |
| R12 | **Profile city/district cannot be cleared.** [UpdateProfile](../backend/internal/user/repository.go:91) uses COALESCE, treating explicit null as omitted. The UI submits null for “not set”/empty district, so old values survive. | Use presence-aware nullable PATCH fields; omission preserves and explicit null clears. Update contract and test both cases. Map invalid city FK errors to validation instead of 500. |
| R13 | **Waitlist promotion ignores lifecycle and capacity edits.** [Cancel](../backend/internal/rsvp/repository.go:283) promotes whenever a going RSVP cancels, even after cancellation/start. Capacity updates do not reconcile existing waitlists: increasing capacity lets fresh joins take seats ahead of waiting users; lowering below attendance can leave over-capacity state. | Define capacity-edit rules and reconcile under the event lock. Suppress promotion for closed events. Test increase/decrease/unlimited, late cancellation, and races with join. |
| R14 | **Leaving and rejoining preserves old waitlist priority.** [promotion ordering](../backend/internal/rsvp/repository.go:127) uses original `created_at`, while reactivation updates only `updated_at`. | Record a dedicated waitlisted-at timestamp and stable ID tie-breaker. Test A leaves, B waits, A rejoins, then B is promoted first. |
| R15 | **Feedback is accepted before an event happens.** [Submit](../backend/internal/feedback/feedback.go:35) checks only existence of an RSVP. Join a future event and immediately POST a rating. | Enforce completion/time eligibility in shared service logic used by HTTP and bot. Preserve the documented decision about canceled RSVPs unless deliberately changing it; separately decide eligibility for never-promoted waitlisted users. |
| R16 | **Mini App MainButton remains visible after unmount.** [RsvpSection cleanup](../frontend/src/components/RsvpSection.tsx:87) removes the handler but does not hide/reset the button. Its effect also omits event ID and translation changes. | Hide/reset in cleanup, use a stable callback with correct dependencies, reset RSVP loading on account/event changes, and ignore stale responses. Test event A → B → tickets and locale changes in Telegram. |
| R17 | **CSV export bypasses refresh and exports formula-like names verbatim.** [attendee page](../frontend/src/app/[locale]/organizer/events/[id]/attendees/page.tsx:90) uses raw fetch; [CSV handler](../backend/internal/rsvp/handler.go:175) sends user-controlled cells through CSV quoting only. | Add a shared authenticated blob download helper with refresh/status handling. Neutralize spreadsheet formula prefixes in textual cells. Test expired access with valid refresh and names beginning with `=`, `+`, `-`, `@`, tabs/newlines. |
| R18 | **Explore pagination races.** [loadMore](../frontend/src/app/[locale]/events/page.tsx:140) has no in-flight guard or query-generation check. Double clicks append duplicates; a slow old-filter page can append into a new filter result. | Disable while loading, key requests by filter generation, dedupe IDs, and display retryable errors. Test rapid filtering while a page request is delayed. |
| R19 | **Calendar UID collisions.** [ics.ts:59](../frontend/src/lib/ics.ts:59) derives UID from start timestamp and title length. Distinct simultaneous events with equal-length titles share an identity; edits change it. | Use immutable event ID as UID, pass it through calendar props, and preserve UID on edits. Test equal-time/equal-length events and rescheduling. |
| R20 | **Geocoding autocomplete conflicts with the provider's rules.** [LocationPicker](../frontend/src/components/LocationPicker.tsx:248) sends debounced typing directly to public Nominatim. Its policy disallows client autocomplete and caps aggregate app traffic at one request/second. | Replace with explicit submitted searches through a throttled/cached configurable service, or select a service that permits autocomplete. Debouncing per browser is insufficient. [Provider policy](https://operations.osmfoundation.org/policies/nominatim/). |
| R21 | **Fresh upload volume has no ownership initialization.** [nonroot image](../deploy/Dockerfile.backend:12), [volume mount](../deploy/docker-compose.yml:61). No owned `/uploads` directory is seeded in the image. A fresh root-owned volume is not writable by the runtime user; an existing VPS may have manual permissions masking this. | Seed the directory with the runtime UID or use a controlled initialization step, without world-writable permissions. Verify authenticated upload on a fresh disposable Compose volume. Docker verification remains outstanding. |
| R22 | **Deployment can run a commit CI did not test.** [deploy.sh:17](../deploy/scripts/deploy.sh:17) pulls current branch head instead of the successful workflow SHA, and workflows/deploys are not serialized. A newer push can be pulled by an older passing run. | Deploy immutable images or the exact tested SHA; serialize production deployment. Check version/readiness through Caddy and frontend, retain rollback images, and exercise migration compatibility. |

## Improvements and decisions after the urgent fixes

These are hardening/product work or behavior decisions, not all demonstrated bugs.

- **Unlisted distribution policy:** all published events are intentionally auto-announced platform-wide. That conflicts with a common expectation for “unlisted,” although the onboarding explicitly requires every published event to reach the official channel. Decide whether unlisted means only hidden from Explore or also excluded from feeds/series/SEO. Do not silently change the official-channel guarantee. Series listing currently exposes published unlisted siblings too.
- **Moderation policy:** admin unpublish returns an event to an ordinary draft which the owner can publish again; there is no held-for-review state or reason. Decide whether this is intentional reversible moderation or needs a publication hold and audit trail.
- **Event deletion:** unpublish an event with RSVPs and then delete the draft: non-cascading references prevent deletion and surface a generic 500. Return a documented conflict and preserve attendance history, or define deliberate archival semantics. Avoid blanket cascading deletion as a shortcut.
- **Durable notifications:** publish and promotion sends run in detached goroutines and are lost on process restart. Reminder scanning has a 50-second lease without renewal and logs after sending, so overlapping workers/lease expiry can duplicate deliveries. Weekly digest takes an eight-day lock before work, so failure can suppress the whole remaining week's batch. Introduce a small PostgreSQL outbox and per-recipient delivery state, bounded retries/timeouts and explicit permanent-failure handling. Keep detached request context behavior, but give jobs their own deadlines. Do not promise exactly-once Telegram delivery across ambiguous network failures.
- **Upload resource control:** enforce per-user quotas/rate limits, validate dimensions and decoded content, remove partial writes, and account for orphan files. Five MB per request does not cap total disk consumption. Maintain local storage for the single-VPS scope; S3 is not required now.
- **Build isolation:** there is no Docker ignore file. `COPY frontend/ ./` can copy local node_modules, `.next`, and local env files into the builder after `npm ci`. Exclude generated artifacts, uploads, credentials and `.git`; use explicit build inputs. This is a packaging risk, not evidence that a secret was leaked.
- **Health and recovery:** `/healthz` always returns OK. Add separate dependency-aware readiness, worker heartbeat, error/latency metrics and disk/backup-age alerts. Back up uploads and DB off the VPS; write backups to a temporary file and rename only after success; restrict permissions and perform restore drills. Keep liveness separate from dependency failures.
- **API robustness:** enforce JSON body limits and request/query timeouts; validate URL schemes, string lengths, coordinate ranges/pairing and business invariants server-side. Configure trusted proxy behavior for the supported direct/proxied topology; use per-account limits for authenticated heavy operations to avoid throttling an entire event venue by its shared IP. Check `rows.Err()` in admin loops. Add pagination to admin/owner lists and an owner-scoped event GET.
- **Frontend recovery and access:** distinguish empty results, not-found, permission denial and network errors; several catches currently erase that distinction. Preserve the requested event through login, gate protected pages on auth initialization, cancel stale requests, guard scanner start/stop races, and offer manual QR entry. Show meeting links for confirmed users after an online event starts; currently RsvpSection returns the “started” notice first and users must discover the tickets page.
- **Accessibility and i18n:** add accessible names to search/filter inputs, keyboard/combobox semantics to location search, announced loading/error states, and visible focus checks. Translate QR alt text, backend error codes, calendar labels and raw category/city slugs. Key parity passes, but does not prove every visible string or translation is correct. Make the event timezone explicit to organizers and align filters/calendar/bot displays.
- **Performance:** fetch independent event-detail related/series data concurrently; profile discovery and trending queries with representative data before adding indexes. Consider a keyset-aligned partial index and waitlist ordering index based on EXPLAIN results. Avoid prematurely replacing PostgreSQL search or the monolith.
- **Documentation:** replace remaining Leaflet descriptions with MapLibre, correct stale canceled-event comments, document update/null semantics and real auth/ban behavior, and keep endpoint changes contract-first. Do not expand into payments, tiers, native mobile, or alternate identity providers during this repair work.

## Ordered implementation plan

Each row is a work package; split it into coherent commits. Update `docs/api.md` before frontend changes whenever contracts change. Use new numbered migration pairs only.

| Order | Work package | Dependencies | Definition of done |
|---|---|---|---|
| 1 | **Privacy and compatibility:** R1, R2, R4; dependency update R8 in its own change | None | Old caches purged; no account crossover; fresh mutable API data; real Mini App login works; edits preserve unlisted state; patched dependency audit and map/icon smoke tests |
| 2 | **Sessions and moderation:** R3, R5, R9, R10 | Coordinate with account-scoped storage in 1 | Single refresh winner; no lost session on DB failure; network outage preserves credentials; logout cannot be undone by a late refresh; banned bot actions rejected |
| 3 | **Attendance and lifecycle correctness:** R6, R7, R13, R14, R15; deletion policy | Contract update and shared transaction policy | Event-scoped atomic check-in; no double transition; waitlist remains fair under capacity edits; closed events do not promote; early feedback rejected |
| 4 | **User-facing repairs:** R11, R12, R16–R20 | 1–3 where API changes apply | Profile clearing/avatar sync correct; native button cleanup; reliable CSV/download; stable pagination/calendar identity; permitted geocoding flow; translated errors |
| 5 | **Deployment/recovery:** R21, R22; Docker ignore, readiness, backup/restore | Can be prepared independently | Fresh-volume install and upload pass; exact tested version deployed; serialized deploy; readiness catches DB/frontend failure; documented successful restore |
| 6 | **Notification durability and abuse controls** | Lifecycle transition work in 3 | Jobs survive restart; bounded/reconciled retries; digest resumes per recipient; quota/timeout failures are observable |
| 7 | **Maintenance and usability** | Core repairs complete | Owner event GET, pagination, accessibility/device checks, query benchmarks, updated docs; no unapproved product scope expansion |

The recommended first deliverable is package 1, followed immediately by session and attendance correctness. Schedule estimates should be made after the unavailable integration environment and live Mini App checks are restored, rather than treating source review as proof of production behavior.

## Test plan and release gates

1. Provision isolated PostgreSQL 16 and Redis 7 for integration testing. CI currently provisions PostgreSQL only; add Redis. Make required integration tests fail rather than silently skip in CI, while retaining optional local skips.
2. Add concurrency integration tests for refresh, lifecycle transitions, capacity changes, waitlist promotions and check-in. Run the Go race detector for code concurrency as well; it does not detect SQL transaction races by itself.
3. Add frontend unit tests for refresh reset, logout/refresh coordination, calendar UID, nullable PATCH payloads and asynchronous request ordering.
4. Add Playwright tests against a production build with the actual worker enabled: account switching, old-cache migration, offline tickets, expired auth, create/publish/join/cancel/check-in, and locale navigation. Test both same-origin production and separate-origin development behavior where supported.
5. Add independent Telegram fixtures, then manually exercise real iOS/Android Telegram login, MainButton cleanup, BackButton deep links, QR camera lifecycle and calendar downloads.
6. Run fresh-database migrations, build/vet/test, frontend lint/build, translation parity, npm audit, Go vulnerability scan, and a container smoke test. Restore a backup to an empty environment before declaring recovery ready.
7. On rollout, verify deployed revision, Caddy/frontend/API readiness, upload permissions, worker progress, error rates and cache migration. Preserve rollback artifacts and verify schema compatibility; never automatically reverse a data migration merely because an application rollback is needed.

Production readiness remains conditional on these fixes and the live checks above; the successful local builds alone are not a release sign-off.
