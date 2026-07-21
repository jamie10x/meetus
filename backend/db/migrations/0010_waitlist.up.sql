ALTER TABLE rsvps DROP CONSTRAINT rsvps_status_check;
ALTER TABLE rsvps ADD CONSTRAINT rsvps_status_check
    CHECK (status IN ('going', 'canceled', 'waitlisted'));
