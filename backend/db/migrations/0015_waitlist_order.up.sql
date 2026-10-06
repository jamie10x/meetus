ALTER TABLE rsvps ADD COLUMN waitlisted_at TIMESTAMPTZ;
UPDATE rsvps SET waitlisted_at=created_at WHERE status='waitlisted';
CREATE INDEX idx_waitlist_order ON rsvps(event_id,waitlisted_at,id) WHERE status='waitlisted';
