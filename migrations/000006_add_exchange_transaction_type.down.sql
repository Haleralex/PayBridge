-- Revert: remove EXCHANGE from the allowed transaction types.
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_transaction_type_check;

ALTER TABLE transactions ADD CONSTRAINT transactions_transaction_type_check
    CHECK (transaction_type IN ('DEPOSIT', 'WITHDRAW', 'PAYOUT', 'TRANSFER', 'FEE', 'REFUND', 'ADJUSTMENT'));
