ALTER TABLE events ADD COLUMN series_id BIGINT;
CREATE INDEX idx_events_series ON events (series_id) WHERE series_id IS NOT NULL;
