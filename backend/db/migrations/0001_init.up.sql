CREATE TABLE cities (
    id      SERIAL PRIMARY KEY,
    slug    TEXT NOT NULL UNIQUE,
    name_uz TEXT NOT NULL,
    name_ru TEXT NOT NULL,
    name_en TEXT NOT NULL
);

CREATE TABLE categories (
    id      SERIAL PRIMARY KEY,
    slug    TEXT NOT NULL UNIQUE,
    name_uz TEXT NOT NULL,
    name_ru TEXT NOT NULL,
    name_en TEXT NOT NULL
);

CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    username    TEXT,
    avatar_url  TEXT,
    city_id     INT REFERENCES cities (id),
    district    TEXT,
    language    TEXT NOT NULL DEFAULT 'uz',
    is_banned   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organizers (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL UNIQUE REFERENCES users (id),
    display_name TEXT NOT NULL,
    bio          TEXT,
    avatar_url   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE event_status AS ENUM ('draft', 'published', 'canceled', 'finished');
CREATE TYPE event_visibility AS ENUM ('public', 'unlisted');

CREATE TABLE events (
    id            BIGSERIAL PRIMARY KEY,
    organizer_id  BIGINT NOT NULL REFERENCES organizers (id),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    category_id   INT NOT NULL REFERENCES categories (id),
    city_id       INT REFERENCES cities (id),
    district      TEXT,
    location_name TEXT,
    address       TEXT,
    lat           DOUBLE PRECISION,
    lng           DOUBLE PRECISION,
    is_online     BOOLEAN NOT NULL DEFAULT FALSE,
    starts_at     TIMESTAMPTZ NOT NULL,
    ends_at       TIMESTAMPTZ,
    capacity      INT CHECK (capacity IS NULL OR capacity > 0),
    cover_url     TEXT,
    status        event_status NOT NULL DEFAULT 'draft',
    visibility    event_visibility NOT NULL DEFAULT 'public',
    search        tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(description, '')), 'B')
    ) STORED,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_search ON events USING gin (search);
CREATE INDEX idx_events_explore ON events (status, city_id, starts_at);
CREATE INDEX idx_events_organizer ON events (organizer_id, starts_at);

CREATE TABLE rsvps (
    id         BIGSERIAL PRIMARY KEY,
    event_id   BIGINT NOT NULL REFERENCES events (id),
    user_id    BIGINT NOT NULL REFERENCES users (id),
    status     TEXT NOT NULL DEFAULT 'going' CHECK (status IN ('going', 'canceled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id)
);

CREATE INDEX idx_rsvps_user ON rsvps (user_id, created_at);

CREATE TABLE tickets (
    id            BIGSERIAL PRIMARY KEY,
    rsvp_id       BIGINT NOT NULL UNIQUE REFERENCES rsvps (id),
    code          TEXT NOT NULL UNIQUE,
    checked_in_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);

CREATE TABLE notification_log (
    id       BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES events (id),
    user_id  BIGINT NOT NULL REFERENCES users (id),
    kind     TEXT NOT NULL,
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id, kind)
);
