# Development Guide

## Prerequisites

Go 1.25+, Node 22+, Docker (with compose). No other global tools.

## First-time setup

```bash
cp .env.example .env          # defaults work for local dev; bot token optional
make infra                    # Postgres :5432 + Redis :6379 (docker compose)
make migrate-up               # schema + seed (14 cities, 10 categories)
cd frontend && npm install
```

`frontend/.env.local` (already present, gitignored):

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_TELEGRAM_BOT_USERNAME=   # empty = login page shows a config notice
NEXT_PUBLIC_SITE_URL=http://localhost:3000   # sitemap.xml + calendar links; = WEB_BASE_URL in prod
```

## Running

```bash
make api        # API on :8080 (JSON logs to stdout)
make frontend   # Next.js dev server on :3000
cd backend && go run ./cmd/worker   # only when working on bot/reminders; needs TELEGRAM_BOT_TOKEN
```

Health check: `curl localhost:8080/healthz`.

**The PWA service worker (`frontend/public/sw.js`) can serve stale JS during frontend dev.** Its app-shell caching is cache-first, and once your browser has registered it against `localhost:3000` it stays active across reloads and even dev-server restarts — so an edit that isn't showing up (especially in a dynamically-imported client component) may be the service worker serving an old cached chunk, not a build problem. Check via devtools → Application → Service Workers, or run in the console: `navigator.serviceWorker.getRegistrations().then(rs => rs.forEach(r => r.unregister())); caches.keys().then(ks => ks.forEach(k => caches.delete(k)))`, then hard-reload.

## Environment variables (backend)

| Var | Default (dev) | Notes |
|---|---|---|
| APP_ENV | development | `production` enforces the secrets below |
| HTTP_ADDR | :8080 | |
| DATABASE_URL | local compose DSN | |
| REDIS_ADDR | localhost:6379 | |
| JWT_SECRET | dev fallback | **required in prod** |
| TICKET_SECRET | dev fallback | **required in prod**; changing it invalidates all issued QR codes |
| TELEGRAM_BOT_TOKEN | empty | empty in dev enables the dev-login trick below; **required in prod** and for the worker |
| TELEGRAM_BOT_USERNAME | empty | widget + links |
| UPLOAD_DIR | ./uploads | gitignored |
| API_BASE_URL | http://localhost:8080 | prefix for upload URLs |
| WEB_BASE_URL | http://localhost:3000 | links inside bot messages |

## Dev login helper

Real Telegram login needs a bot with `/setdomain`. In dev, `TELEGRAM_BOT_TOKEN`
is empty, so you can sign a valid payload yourself against the empty-string
token. Paste this into your shell:

```bash
tg_login() {  # usage: tg_login <telegram_id> <name>   → prints access token
  local auth_date=$(date +%s)
  local dcs=$(printf 'auth_date=%s\nfirst_name=%s\nid=%s' "$auth_date" "$2" "$1")
  local secret=$(printf '' | openssl dgst -sha256 -binary | xxd -p -c 64)
  local hash=$(printf '%s' "$dcs" | openssl dgst -sha256 -mac HMAC -macopt hexkey:$secret -r | cut -d' ' -f1)
  curl -s -X POST http://localhost:8080/api/auth/telegram \
    -H 'Content-Type: application/json' \
    -d "{\"id\":\"$1\",\"first_name\":\"$2\",\"auth_date\":\"$auth_date\",\"hash\":\"$hash\"}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["tokens"]["accessToken"])'
}

ACCESS=$(tg_login 999 Alice)
curl -s localhost:8080/api/me -H "Authorization: Bearer $ACCESS"
```

Full happy-path smoke test (login → organizer → event → publish → RSVP as a
second user → check-in):

```bash
ORG=$(tg_login 999 Org); USR=$(tg_login 1001 Guest)
curl -s -X POST localhost:8080/api/organizers -H "Authorization: Bearer $ORG" \
  -H 'Content-Type: application/json' -d '{"displayName":"Test Org"}'
EVID=$(curl -s -X POST localhost:8080/api/events -H "Authorization: Bearer $ORG" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Smoke Test","categoryId":1,"cityId":1,"startsAt":"'"$(date -v+2d +%Y-%m-%dT%H:%M:%S%z | sed 's/\(..\)$/:\1/')"'"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
curl -s -X POST localhost:8080/api/events/$EVID/publish -H "Authorization: Bearer $ORG" > /dev/null
QR=$(curl -s -X POST localhost:8080/api/events/$EVID/rsvp -H "Authorization: Bearer $USR" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["ticket"]["qr"])')
curl -s -X POST localhost:8080/api/checkin -H "Authorization: Bearer $ORG" \
  -H 'Content-Type: application/json' -d "{\"qr\":\"$QR\"}"
```

RSVP responses are `{"status": "going"|"waitlisted", "ticket": {...}|null}` — a
full event waitlists instead of rejecting. To exercise that path:

```bash
EVID2=$(curl -s -X POST localhost:8080/api/events -H "Authorization: Bearer $ORG" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Waitlist Test","categoryId":1,"cityId":1,"capacity":1,"startsAt":"'"$(date -v+2d +%Y-%m-%dT%H:%M:%S%z | sed 's/\(..\)$/:\1/')"'"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
curl -s -X POST localhost:8080/api/events/$EVID2/publish -H "Authorization: Bearer $ORG" > /dev/null
USR2=$(tg_login 1002 Guest2)
curl -s -X POST localhost:8080/api/events/$EVID2/rsvp -H "Authorization: Bearer $USR"   # status: going
curl -s -X POST localhost:8080/api/events/$EVID2/rsvp -H "Authorization: Bearer $USR2"  # status: waitlisted
curl -s -X DELETE localhost:8080/api/events/$EVID2/rsvp -H "Authorization: Bearer $USR" > /dev/null
curl -s localhost:8080/api/events/$EVID2/rsvp -H "Authorization: Bearer $USR2"          # now: going, with a ticket
```

For a real end-to-end bot test: create a throwaway bot via @BotFather, set
`TELEGRAM_BOT_TOKEN` in `.env`, run the worker, message the bot `/start`.

## Testing

```bash
make test               # everything; notification integration test skips if Postgres is down
cd backend && go test ./internal/rsvp/ -run TestTicketSigner -v   # focused run
```

Test conventions:
- Table-driven unit tests for pure logic (see `event/service_test.go`,
  `auth/telegram_test.go` — it re-implements Telegram's signing to forge valid
  payloads).
- Validation-only paths can be tested with `NewService(nil)` because
  validation runs before any repository call.
- Integration tests hit the real dev Postgres, create their own fixtures with
  unique telegram_ids (≥ 900000001), clean up via `t.Cleanup`, and `t.Skip`
  when the DB is unreachable (see `notification_integration_test.go`).

## Recipes

### Add an endpoint
1. Repository method (SQL) in the module's `repository.go`.
2. Service method with the business rules; return `apperr.*` for user-facing failures.
3. Handler: bind → call service → `httpx.OK` / `httpx.Error`.
4. Route in the module's `Register(...)`; new modules get wired in `internal/server/router.go`.
5. Document in `docs/api.md` (same commit).
6. Frontend: type in `src/lib/types.ts`, call via `api<T>()`.
7. Tests for any nontrivial service logic; run the smoke test if the flow is user-facing.

### Add a DB column
New migration pair → extend the module's column-list constant + scanner +
model struct (+ DTO if exposed) → `make migrate-up` → tests.

### Add a bot command
Handler method on `tgbot.Bot`, register it in `tgbot.New` with
`api.RegisterHandler(...)`. Escape user content with `escape()`. Callback data
convention: `"<verb>:<id>"` with `MatchTypePrefix`.

### Add a frontend page
File under `src/app/[locale]/<route>/page.tsx`. Client pages needing auth
follow the guard pattern in `profile/page.tsx` (`useAuth()` + redirect).
Server pages that fetch API data follow `events/[id]/page.tsx` (async
component + `generateMetadata`, `params` is a Promise — and now also
carries `locale`). Import `Link`/`useRouter`/`redirect`/`usePathname` from
`@/i18n/navigation`, never `next/link`/`next/navigation`. Add every UI
string to all three `messages/*.json` catalogs (same key, same nesting) —
use `useTranslations()` in Client Components, `getTranslations()` in
Server Components. Always `npm run build` before committing — it's the
typecheck, and it also statically renders all three locales for every
static route so a missing message key can show up there too.

## Debugging

- API request log: one JSON line per request (`method`, `path`, `status`, `duration_ms`).
- 500s log the underlying error server-side; clients only see the envelope.
- DB shell: `docker exec -it meetus-postgres-1 psql -U meetus`
- Redis shell: `docker exec -it meetus-redis-1 redis-cli`
- Reset dev DB completely: `docker compose down -v && make infra && make migrate-up`
