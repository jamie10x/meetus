-- The existing (status, city_id, starts_at) index cannot serve the
-- unfiltered discovery order or the (starts_at,id) keyset directly.
CREATE INDEX idx_events_public_cursor ON events(starts_at,id)
WHERE status='published' AND visibility='public';
