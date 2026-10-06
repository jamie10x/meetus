CREATE TABLE delivery_jobs (
 id BIGSERIAL PRIMARY KEY,
 dedupe_key TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL,
 payload JSONB NOT NULL,
 attempts INT NOT NULL DEFAULT 0,
 available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 lease_until TIMESTAMPTZ,
 done_at TIMESTAMPTZ,
 failed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_pending ON delivery_jobs(available_at,id) WHERE done_at IS NULL AND failed_at IS NULL;
