-- Migration: 003_create_transactions
-- Description: Создание таблицы транзакций с идемпотентностью
--
-- Design Decisions:
-- - idempotency_key с UNIQUE constraint для предотвращения дубликатов
-- - Amount как BIGINT (в минимальных единицах валюты)
-- - Status как state machine с CHECK constraint
-- - JSONB для flexible metadata
-- - Partial indexes для оптимизации типичных запросов

-- +goose Up
CREATE TABLE IF NOT EXISTS transactions (
    -- Primary Key
    id UUID PRIMARY KEY,

    -- Foreign Key: Исходный кошелёк
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,

    -- Idempotency Key: Уникальный ключ для предотвращения дубликатов
    -- Критично для финансовых операций!
    idempotency_key VARCHAR(255) NOT NULL,

    -- Transaction Type
    transaction_type VARCHAR(20) NOT NULL
        CHECK (transaction_type IN ('DEPOSIT', 'WITHDRAW', 'PAYOUT', 'TRANSFER', 'FEE', 'REFUND', 'ADJUSTMENT')),

    -- Status: State machine
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED')),

    -- Amount: В минимальных единицах валюты кошелька
    amount BIGINT NOT NULL CHECK (amount > 0),

    -- Currency: Сохраняем для денормализации (быстрые отчёты)
    currency VARCHAR(10) NOT NULL,

    -- Optional: Кошелёк-получатель (для TRANSFER)
    destination_wallet_id UUID REFERENCES wallets(id) ON DELETE RESTRICT,

    -- External Reference: ID из внешней системы (Stripe, bank, etc.)
    external_reference VARCHAR(255),

    -- Description: Человекочитаемое описание
    description TEXT,

    -- Metadata: Гибкие дополнительные данные (JSON)
    metadata JSONB DEFAULT '{}',

    -- Failure Info
    failure_reason VARCHAR(255),
    retry_count INT NOT NULL DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,  -- Когда начали обработку
    completed_at TIMESTAMPTZ,  -- Когда завершили (успех/неудача/отмена)

    -- Constraints
    CONSTRAINT transactions_idempotency_key_unique UNIQUE (idempotency_key)
);

-- Primary indexes
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions (transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at DESC);

-- Composite indexes для типичных запросов
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_status ON transactions (wallet_id, status);
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_created ON transactions (wallet_id, created_at DESC);

-- Partial indexes для оптимизации (только активные записи)
-- Pending транзакции - часто запрашиваются для обработки
CREATE INDEX IF NOT EXISTS idx_transactions_pending
    ON transactions (wallet_id, created_at)
    WHERE status = 'PENDING';

-- Failed транзакции для retry - фоновая обработка
CREATE INDEX IF NOT EXISTS idx_transactions_failed_retryable
    ON transactions (created_at)
    WHERE status = 'FAILED' AND retry_count < 3;

-- Index для поиска по external reference
CREATE INDEX IF NOT EXISTS idx_transactions_external_ref
    ON transactions (external_reference)
    WHERE external_reference IS NOT NULL;

-- JSONB index для поиска по metadata
CREATE INDEX IF NOT EXISTS idx_transactions_metadata ON transactions USING GIN (metadata);

-- Trigger для обновления updated_at
CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE transactions IS 'Финансовые транзакции в системе';
COMMENT ON COLUMN transactions.idempotency_key IS 'Уникальный ключ для предотвращения дублирования операций';
COMMENT ON COLUMN transactions.amount IS 'Сумма в минимальных единицах валюты (cents/satoshis)';
COMMENT ON COLUMN transactions.metadata IS 'Дополнительные данные в формате JSON';
COMMENT ON COLUMN transactions.failure_reason IS 'Причина неудачи (для status=FAILED)';
COMMENT ON COLUMN transactions.retry_count IS 'Количество попыток повторной обработки';

-- +goose Down
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
DROP TABLE IF EXISTS transactions;
