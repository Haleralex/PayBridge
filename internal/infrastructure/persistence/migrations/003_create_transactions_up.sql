-- Migration: 003_create_transactions
-- Description: Создание таблицы транзакций

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(255) NOT NULL,
    
    -- Transaction Type: расширенная схема в соответствии с domain model
    transaction_type VARCHAR(20) NOT NULL
        CHECK (transaction_type IN ('DEPOSIT', 'WITHDRAW', 'PAYOUT', 'TRANSFER', 'FEE', 'REFUND', 'ADJUSTMENT')),
    
    -- Status: жизненный цикл транзакции
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    
    -- Amount & Currency
    amount BIGINT NOT NULL
        CHECK (amount > 0),
    currency VARCHAR(10) NOT NULL,
    
    -- Optional: для TRANSFER типа
    destination_wallet_id UUID REFERENCES wallets(id),
    
    -- External system reference (Stripe, PayPal, etc)
    external_reference VARCHAR(255),
    
    -- Description & Metadata
    description TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Failure tracking
    failure_reason TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- Constraints
    CONSTRAINT transactions_idempotency_unique UNIQUE (wallet_id, idempotency_key)
);

-- Indexes для производительности
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_transaction_type ON transactions (transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_idempotency ON transactions (wallet_id, idempotency_key);
-- Trigger для автоматического обновления updated_at
CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Документация схемы
COMMENT ON TABLE transactions IS 'Транзакции по кошелькам - все финансовые операции';
COMMENT ON COLUMN transactions.transaction_type IS 'Тип транзакции: DEPOSIT, WITHDRAW, PAYOUT, TRANSFER, FEE, REFUND, ADJUSTMENT';
COMMENT ON COLUMN transactions.status IS 'Статус: PENDING → PROCESSING → COMPLETED/FAILED/CANCELLED';
COMMENT ON COLUMN transactions.amount IS 'Сумма транзакции в минимальных единицах (cents/satoshis)';
COMMENT ON COLUMN transactions.idempotency_key IS 'Ключ идемпотентности для предотвращения дубликатов';
COMMENT ON COLUMN transactions.destination_wallet_id IS 'ID кошелька получателя (только для TRANSFER)';
COMMENT ON COLUMN transactions.external_reference IS 'Ссылка на внешнюю систему (Stripe, PayPal, etc)';
COMMENT ON COLUMN transactions.metadata IS 'Дополнительные данные транзакции в JSON формате';
COMMENT ON COLUMN transactions.retry_count IS 'Количество попыток повтора при ошибке';
