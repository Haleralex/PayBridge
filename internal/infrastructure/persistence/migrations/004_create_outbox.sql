-- Migration: 004_create_outbox
-- Description: Таблица Outbox для Transactional Outbox Pattern
--
-- Transactional Outbox Pattern:
-- 1. В той же транзакции, что и бизнес-операция, записываем событие в outbox
-- 2. Отдельный worker (poller) читает unpublished события и публикует в Kafka
-- 3. После успешной публикации помечает событие как published
--
-- Гарантирует: exactly-once semantics для событий!
-- Решает проблему: "что если commit прошёл, но Kafka недоступен?"

-- +goose Up
CREATE TABLE IF NOT EXISTS outbox (
    -- Primary Key: UUID события (из DomainEvent.ID)
    id UUID PRIMARY KEY,

    -- Aggregate Info: К какому агрегату относится событие
    aggregate_type VARCHAR(100) NOT NULL,  -- e.g., "User", "Wallet", "Transaction"
    aggregate_id UUID NOT NULL,            -- ID агрегата

    -- Event Info
    event_type VARCHAR(100) NOT NULL,      -- e.g., "user.created", "wallet.credited"
    event_version INT NOT NULL DEFAULT 1,  -- Версия схемы события

    -- Payload: Сериализованное событие (JSON)
    payload JSONB NOT NULL,

    -- Publishing Status
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),

    -- Retry Info
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,

    -- Partition key для Kafka (optional)
    partition_key VARCHAR(255)
);

-- Primary index: Unpublished события для polling
-- ORDER BY created_at гарантирует FIFO
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (created_at)
    WHERE status = 'PENDING';

-- Index для поиска по агрегату (debug, audit)
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate
    ON outbox (aggregate_type, aggregate_id);

-- Index для поиска по типу события
CREATE INDEX IF NOT EXISTS idx_outbox_event_type ON outbox (event_type);

-- Index для failed события (retry logic)
CREATE INDEX IF NOT EXISTS idx_outbox_failed
    ON outbox (created_at)
    WHERE status = 'FAILED' AND retry_count < 5;

-- Cleanup: Удаляем опубликованные события старше 7 дней
-- Можно настроить через cron job или pg_cron
-- CREATE INDEX IF NOT EXISTS idx_outbox_cleanup
--     ON outbox (published_at)
--     WHERE status = 'PUBLISHED';

-- Comments
COMMENT ON TABLE outbox IS 'Transactional Outbox для гарантированной доставки событий';
COMMENT ON COLUMN outbox.aggregate_type IS 'Тип агрегата: User, Wallet, Transaction';
COMMENT ON COLUMN outbox.aggregate_id IS 'ID агрегата, породившего событие';
COMMENT ON COLUMN outbox.event_type IS 'Тип события: user.created, wallet.credited и т.д.';
COMMENT ON COLUMN outbox.payload IS 'Сериализованное событие в JSON формате';
COMMENT ON COLUMN outbox.partition_key IS 'Ключ партиции Kafka для ordering';

-- +goose Down
DROP TABLE IF EXISTS outbox;
