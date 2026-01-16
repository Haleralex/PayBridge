-- Migration: 002_create_wallets
-- Description: Создание таблицы кошельков с поддержкой optimistic locking
--
-- Design Decisions:
-- - Balance хранится как BIGINT (cents/satoshis) для точности
-- - Optimistic locking через поле balance_version
-- - UNIQUE constraint на (user_id, currency) - один кошелёк на валюту
-- - Foreign key на users с CASCADE DELETE

-- +goose Up
CREATE TABLE IF NOT EXISTS wallets (
    -- Primary Key
    id UUID PRIMARY KEY,

    -- Foreign Key: Владелец кошелька
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Currency: ISO 4217 код валюты (USD, EUR, BTC, ETH, etc.)
    currency VARCHAR(10) NOT NULL,

    -- Wallet Type: FIAT или CRYPTO
    wallet_type VARCHAR(10) NOT NULL DEFAULT 'FIAT'
        CHECK (wallet_type IN ('FIAT', 'CRYPTO')),

    -- Status: Состояние кошелька
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'SUSPENDED', 'LOCKED', 'CLOSED')),

    -- Balance: Хранится в минимальных единицах (cents для fiat, satoshis для crypto)
    -- BIGINT вмещает до 9.2 quintillion единиц
    available_balance BIGINT NOT NULL DEFAULT 0
        CHECK (available_balance >= 0),

    pending_balance BIGINT NOT NULL DEFAULT 0
        CHECK (pending_balance >= 0),

    -- Optimistic Locking: Версия для конкурентного доступа
    -- При каждом изменении баланса увеличивается на 1
    balance_version BIGINT NOT NULL DEFAULT 0,

    -- Limits: Лимиты транзакций (в минимальных единицах)
    daily_limit BIGINT NOT NULL DEFAULT 1000000,   -- $10,000 для fiat
    monthly_limit BIGINT NOT NULL DEFAULT 10000000, -- $100,000 для fiat

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    -- У пользователя может быть только один кошелёк на каждую валюту
    CONSTRAINT wallets_user_currency_unique UNIQUE (user_id, currency)
);

-- Indexes для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_currency ON wallets (currency);
CREATE INDEX IF NOT EXISTS idx_wallets_status ON wallets (status);
CREATE INDEX IF NOT EXISTS idx_wallets_user_currency ON wallets (user_id, currency);

-- Trigger для обновления updated_at
CREATE TRIGGER update_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE wallets IS 'Кошельки пользователей для хранения средств';
COMMENT ON COLUMN wallets.available_balance IS 'Доступный баланс в минимальных единицах (cents/satoshis)';
COMMENT ON COLUMN wallets.pending_balance IS 'Заблокированный баланс (pending транзакции)';
COMMENT ON COLUMN wallets.balance_version IS 'Версия для optimistic locking';
COMMENT ON COLUMN wallets.daily_limit IS 'Дневной лимит транзакций в минимальных единицах';
COMMENT ON COLUMN wallets.monthly_limit IS 'Месячный лимит транзакций в минимальных единицах';

-- +goose Down
DROP TRIGGER IF EXISTS update_wallets_updated_at ON wallets;
DROP TABLE IF EXISTS wallets;
