-- Remove telegram_id column from users table
DROP INDEX IF EXISTS idx_users_telegram_id;
ALTER TABLE users DROP COLUMN IF EXISTS telegram_id;
