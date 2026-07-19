CREATE TABLE channel_connections (
    id           BIGSERIAL PRIMARY KEY,
    organizer_id BIGINT NOT NULL REFERENCES organizers (id),
    chat_id      BIGINT NOT NULL UNIQUE,
    chat_title   TEXT NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_channel_connections_organizer ON channel_connections (organizer_id);
