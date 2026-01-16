-- Migration: 004_create_outbox
-- Description: Создание таблицы для Transactional Outbox Pattern

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    partition_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox_events (status);
CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON outbox_events (created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox_events (aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox_events (status, created_at) WHERE status = 'PENDING';

COMMENT ON TABLE outbox_events IS 'Transactional Outbox для гарантированной доставки событий';
COMMENT ON COLUMN outbox_events.aggregate_type IS 'Тип агрегата (User, Wallet, Transaction)';
COMMENT ON COLUMN outbox_events.aggregate_id IS 'ID агрегата, породившего событие';
COMMENT ON COLUMN outbox_events.event_type IS 'Тип события (user.created, wallet.credited, etc.)';
COMMENT ON COLUMN outbox_events.payload IS 'Полные данные события в JSON';
COMMENT ON COLUMN outbox_events.partition_key IS 'Ключ для партиционирования в Kafka (обычно aggregate_id)';
COMMENT ON COLUMN outbox_events.status IS 'Статус публикации: PENDING, PUBLISHED, FAILED';
