-- Create outbox table for Transactional Outbox Pattern
CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    partition_key VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (created_at)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_outbox_aggregate
    ON outbox (aggregate_type, aggregate_id);

CREATE INDEX IF NOT EXISTS idx_outbox_event_type ON outbox (event_type);

CREATE INDEX IF NOT EXISTS idx_outbox_failed
    ON outbox (created_at)
    WHERE status = 'FAILED' AND retry_count < 5;

COMMENT ON TABLE outbox IS 'Transactional Outbox for guaranteed event delivery';
COMMENT ON COLUMN outbox.aggregate_type IS 'Aggregate type: User, Wallet, Transaction';
COMMENT ON COLUMN outbox.payload IS 'Serialized event in JSON format';
