CREATE TABLE event_feedback (
    id         BIGSERIAL PRIMARY KEY,
    event_id   BIGINT NOT NULL REFERENCES events (id),
    user_id    BIGINT NOT NULL REFERENCES users (id),
    rating     SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id)
);

CREATE INDEX idx_event_feedback_event ON event_feedback (event_id);
