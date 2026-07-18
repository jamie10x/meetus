# Data Model

Source of truth: `backend/db/migrations/`. Migrations are **append-only** —
never edit an applied file; add a new `NNNN_name.up.sql` / `.down.sql` pair.
They are embedded into the `migrate` binary (`backend/db/embed.go`).

## Entity relationships

```text
cities ─┐                            categories ─┐
        ▼                                        ▼
      users ──1:1── organizers ──1:N── events ──1:N── rsvps ──1:1── tickets
        │                                │               │
        │                                └──── 1:N ──────┤
        ├──1:N── refresh_tokens                          │
        └──────────────── 1:N ── notification_log ◄──────┘ (by event+user)
```

## Tables

### users
Identity = Telegram account. Created lazily on first login (web or bot).

| Column | Type | Notes |
|---|---|---|
| id | bigserial PK | internal ID (JWT `sub`) |
| telegram_id | bigint UNIQUE | the real identity key |
| name | text | first + last name from Telegram, refreshed on login |
| username, avatar_url | text NULL | refreshed on login (COALESCE keeps old value if Telegram omits) |
| city_id | int FK cities NULL | user-chosen, drives future personalization |
| district | text NULL | free text |
| language | text default 'uz' | `uz` \| `ru` \| `en` (validated in handler) |
| is_banned | bool | checked at login **and** refresh |

### organizers
1:1 with users (UNIQUE user_id). Existence of a row == organizer role;
`organizer.RequireOrganizer` middleware loads it per request.

### cities, categories
Reference data, seeded in `0002_seed.up.sql` (14 cities, 10 categories) with
`slug` + `name_uz/ru/en`. Served by `GET /api/meta/*`. Add rows via a new
migration, not by hand.

### events

| Column | Type | Notes |
|---|---|---|
| organizer_id | FK organizers | owner; ownership checked in `event.Service.getOwned` |
| title, description | text | description defaults `''` |
| category_id | FK categories NOT NULL | FK violation → mapped to 400 |
| city_id | FK cities NULL | required by service when `is_online = false` |
| district, location_name, address | text NULL | |
| lat, lng | double NULL | reserved for maps (no UI yet) |
| is_online | bool | |
| starts_at | timestamptz NOT NULL | must be future to **publish** (not to save a draft) |
| ends_at | timestamptz NULL | service enforces `> starts_at` |
| capacity | int NULL, CHECK > 0 | NULL = unlimited |
| cover_url | text NULL | from `POST /api/uploads` |
| status | enum `event_status` | `draft → published ⇄ draft`, `→ canceled`; `finished` reserved (nothing sets it yet) |
| visibility | enum `event_visibility` | `public` (listed) \| `unlisted` (direct link only) |
| search | tsvector **GENERATED** | title weight A + description weight B, `simple` config; **never write to it** |

Indexes: GIN on `search`; `(status, city_id, starts_at)` for explore;
`(organizer_id, starts_at)` for dashboards.

Lifecycle rules (in `event.Service`, keep them there):
- edit: draft or published only (canceled/finished are read-only)
- publish: draft only + future `starts_at`
- delete: **draft only** — published events must be canceled instead (RSVPs exist)

### rsvps
UNIQUE `(event_id, user_id)` — one row per user per event, ever.
`status`: `going` | `canceled` (CHECK). Cancel + re-join flips status back and
**keeps the same ticket**. Capacity counts `status = 'going'` only.
All creation goes through the `rsvp.Repository.Join` transaction (row lock on
the event) — never insert an RSVP another way.

### tickets
1:1 with rsvps (UNIQUE rsvp_id). `code` UNIQUE, 16 random bytes hex.
`checked_in_at` NULL until scanned; the update is guarded by
`WHERE checked_in_at IS NULL` so double check-in loses the race cleanly.
QR value = `code.HMAC-SHA256(code, TICKET_SECRET)` — computed, never stored.

### refresh_tokens
`token_hash` = SHA-256 hex of the raw token (raw value never stored).
`revoked_at` set on rotation/logout. Rows accumulate; a periodic
`DELETE WHERE expires_at < now()` is a fine future housekeeping job.

### notification_log
Dedup ledger: UNIQUE `(event_id, user_id, kind)`,
kind ∈ `reminder_24h` | `reminder_1h`. Inserted with `ON CONFLICT DO NOTHING`
after every send **attempt** (success or failure) — see architecture.md.

## Query conventions

- Repositories use explicit column lists + a `scanX(row)` helper per entity
  (`userColumns`/`scanUser`, `eventSelect`/`scanEvent`). Extend the constant
  and the scanner together.
- `eventSelect` joins organizer name, category slug, city slug, and computes
  `goingCount` as a subquery — every event read returns a fully-hydrated model.
- Nullable columns ↔ Go pointers. No `sql.NullX`.
- Dynamic filters (explore) build numbered args via a local `arg()` closure —
  copy that pattern; never fmt.Sprintf user input into SQL.
- Multi-step writes (RSVP join) use a transaction + `FOR UPDATE`; single
  UPDATEs rely on `RowsAffected()` to detect missing rows.

## Working with migrations

```bash
# create
$EDITOR backend/db/migrations/0003_add_thing.up.sql
$EDITOR backend/db/migrations/0003_add_thing.down.sql
# apply / inspect / roll back one
make migrate-up
cd backend && go run ./cmd/migrate version
cd backend && go run ./cmd/migrate down 1
```

Down migrations must actually reverse the up (they're used in dev). In
production only `up` runs (compose `migrate` service before the API).
