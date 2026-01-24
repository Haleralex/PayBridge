-- Create wallets table with optimistic locking
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency VARCHAR(10) NOT NULL,
    wallet_type VARCHAR(10) NOT NULL DEFAULT 'FIAT'
        CHECK (wallet_type IN ('FIAT', 'CRYPTO')),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'SUSPENDED', 'LOCKED', 'CLOSED')),
    available_balance BIGINT NOT NULL DEFAULT 0
        CHECK (available_balance >= 0),
    pending_balance BIGINT NOT NULL DEFAULT 0
        CHECK (pending_balance >= 0),
    balance_version BIGINT NOT NULL DEFAULT 0,
    daily_limit BIGINT NOT NULL DEFAULT 1000000,
    monthly_limit BIGINT NOT NULL DEFAULT 10000000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallets_user_currency_unique UNIQUE (user_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_currency ON wallets (currency);
CREATE INDEX IF NOT EXISTS idx_wallets_status ON wallets (status);
CREATE INDEX IF NOT EXISTS idx_wallets_user_currency ON wallets (user_id, currency);

CREATE TRIGGER update_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE wallets IS 'User wallets for storing funds';
COMMENT ON COLUMN wallets.available_balance IS 'Available balance in minor units (cents/satoshis)';
COMMENT ON COLUMN wallets.balance_version IS 'Version for optimistic locking';
