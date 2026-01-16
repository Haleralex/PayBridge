-- Migration: 001_create_users
-- Description: Создание таблицы пользователей
--
-- Design Decisions:
-- - UUID как primary key (распределённость, безопасность)
-- - email с UNIQUE constraint (бизнес-правило уникальности)
-- - kyc_status как VARCHAR с CHECK constraint (валидация на уровне БД)
-- - Timestamps с timezone (UTC consistency)
-- - Index на email для быстрого поиска

-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    -- Primary Key: UUID для распределённых систем
    id UUID PRIMARY KEY,

    -- Email: Уникальный идентификатор пользователя
    -- Lower case для case-insensitive поиска
    email VARCHAR(255) NOT NULL,

    -- Full Name: Имя пользователя
    full_name VARCHAR(255) NOT NULL,

    -- KYC Status: State machine с ограниченными значениями
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'UNVERIFIED'
        CHECK (kyc_status IN ('UNVERIFIED', 'PENDING', 'VERIFIED', 'REJECTED')),

    -- Timestamps: Всегда с timezone для consistency
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT users_email_unique UNIQUE (email)
);

-- Index для быстрого поиска по email
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- Index для фильтрации по статусу KYC
CREATE INDEX IF NOT EXISTS idx_users_kyc_status ON users (kyc_status);

-- Trigger для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Comments для документации схемы
COMMENT ON TABLE users IS 'Пользователи системы WalletHub';
COMMENT ON COLUMN users.id IS 'Уникальный идентификатор пользователя (UUID v4)';
COMMENT ON COLUMN users.email IS 'Email пользователя (уникальный, lowercase)';
COMMENT ON COLUMN users.full_name IS 'Полное имя пользователя';
COMMENT ON COLUMN users.kyc_status IS 'Статус KYC верификации: UNVERIFIED, PENDING, VERIFIED, REJECTED';

-- +goose Down
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TABLE IF EXISTS users;
