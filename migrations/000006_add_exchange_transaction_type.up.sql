-- Add EXCHANGE to the allowed transaction types so currency exchange
-- between a user's own wallets can be persisted.
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_transaction_type_check;

ALTER TABLE transactions ADD CONSTRAINT transactions_transaction_type_check
    CHECK (transaction_type IN ('DEPOSIT', 'WITHDRAW', 'PAYOUT', 'TRANSFER', 'FEE', 'REFUND', 'ADJUSTMENT', 'EXCHANGE'));
