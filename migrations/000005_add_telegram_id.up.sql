-- Add telegram_id column to users table for Telegram Mini App authentication
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_id BIGINT UNIQUE;

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users (telegram_id) WHERE telegram_id IS NOT NULL;

COMMENT ON COLUMN users.telegram_id IS 'Telegram user ID for Mini App authentication';
