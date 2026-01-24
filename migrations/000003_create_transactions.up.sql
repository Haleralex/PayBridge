-- Create transactions table with idempotency
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(255) NOT NULL,
    transaction_type VARCHAR(20) NOT NULL
        CHECK (transaction_type IN ('DEPOSIT', 'WITHDRAW', 'PAYOUT', 'TRANSFER', 'FEE', 'REFUND', 'ADJUSTMENT')),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(10) NOT NULL,
    destination_wallet_id UUID REFERENCES wallets(id) ON DELETE RESTRICT,
    external_reference VARCHAR(255),
    description TEXT,
    metadata JSONB DEFAULT '{}',
    failure_reason VARCHAR(255),
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    CONSTRAINT transactions_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions (transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_status ON transactions (wallet_id, status);
CREATE INDEX IF NOT EXISTS idx_transactions_wallet_created ON transactions (wallet_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_transactions_pending
    ON transactions (wallet_id, created_at)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_transactions_failed_retryable
    ON transactions (created_at)
    WHERE status = 'FAILED' AND retry_count < 3;

CREATE INDEX IF NOT EXISTS idx_transactions_external_ref
    ON transactions (external_reference)
    WHERE external_reference IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_metadata ON transactions USING GIN (metadata);

CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE transactions IS 'Financial transactions';
COMMENT ON COLUMN transactions.idempotency_key IS 'Unique key to prevent duplicate operations';
COMMENT ON COLUMN transactions.amount IS 'Amount in minor currency units (cents/satoshis)';
