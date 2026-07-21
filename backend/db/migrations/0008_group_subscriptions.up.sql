CREATE TABLE group_subscriptions (
    id            BIGSERIAL PRIMARY KEY,
    chat_id       BIGINT NOT NULL UNIQUE,
    chat_title    TEXT NOT NULL,
    subscribed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
