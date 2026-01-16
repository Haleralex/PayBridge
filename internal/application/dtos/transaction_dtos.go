// Package dtos - Transaction DTOs для передачи данных о транзакциях.
package dtos

import "time"

// ============================================
// Commands (Write операции)
// ============================================

// CreateTransactionCommand - команда для создания транзакции.
type CreateTransactionCommand struct {
	WalletID            string                 `json:"wallet_id" validate:"required,uuid"`
	IdempotencyKey      string                 `json:"idempotency_key" validate:"required"`
	Type                string                 `json:"type" validate:"required,oneof=DEPOSIT WITHDRAW PAYOUT TRANSFER FEE REFUND ADJUSTMENT"`
	Amount              string                 `json:"amount" validate:"required"`
	DestinationWalletID string                 `json:"destination_wallet_id,omitempty" validate:"omitempty,uuid"`
	Description         string                 `json:"description" validate:"required"`
	ExternalReference   string                 `json:"external_reference,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// TransferBetweenWalletsCommand - команда для перевода между кошельками.
type TransferBetweenWalletsCommand struct {
	SourceWalletID      string                 `json:"source_wallet_id" validate:"required,uuid"`
	DestinationWalletID string                 `json:"destination_wallet_id" validate:"required,uuid"`
	Amount              string                 `json:"amount" validate:"required"`
	IdempotencyKey      string                 `json:"idempotency_key" validate:"required"`
	Description         string                 `json:"description" validate:"required"`
	ExternalReference   string                 `json:"external_reference,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessTransactionCommand - команда для обработки транзакции.
type ProcessTransactionCommand struct {
	TransactionID string `json:"transaction_id" validate:"required,uuid"`
	Success       bool   `json:"success"`                  // Результат обработки (mock для примера)
	FailureReason string `json:"failure_reason,omitempty"` // Причина провала
}

// RetryTransactionCommand - команда для повтора failed транзакции.
type RetryTransactionCommand struct {
	TransactionID string `json:"transaction_id" validate:"required,uuid"`
}

// CancelTransactionCommand - команда для отмены транзакции.
type CancelTransactionCommand struct {
	TransactionID string `json:"transaction_id" validate:"required,uuid"`
	Reason        string `json:"reason" validate:"required"`
}

// ============================================
// Queries (Read операции)
// ============================================

// GetTransactionQuery - запрос транзакции по ID.
type GetTransactionQuery struct {
	TransactionID string `json:"transaction_id" validate:"required,uuid"`
}

// GetTransactionByIdempotencyKeyQuery - запрос по ключу идемпотентности.
type GetTransactionByIdempotencyKeyQuery struct {
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// ListTransactionsQuery - запрос списка транзакций с фильтрацией.
type ListTransactionsQuery struct {
	WalletID *string `json:"wallet_id,omitempty" validate:"omitempty,uuid"`
	UserID   *string `json:"user_id,omitempty" validate:"omitempty,uuid"`
	Type     *string `json:"type,omitempty" validate:"omitempty,oneof=DEPOSIT WITHDRAW PAYOUT TRANSFER FEE REFUND ADJUSTMENT"`
	Status   *string `json:"status,omitempty" validate:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED CANCELLED"`
	Offset   int     `json:"offset" validate:"min=0"`
	Limit    int     `json:"limit" validate:"min=1,max=100"`
}

// ============================================
// Response DTOs
// ============================================

// TransactionDTO - представление транзакции для API.
type TransactionDTO struct {
	ID                  string            `json:"id"`
	WalletID            string            `json:"wallet_id"`
	IdempotencyKey      string            `json:"idempotency_key"`
	Type                string            `json:"type"`
	Status              string            `json:"status"`
	Amount              string            `json:"amount"`
	CurrencyCode        string            `json:"currency_code"`
	DestinationWalletID *string           `json:"destination_wallet_id,omitempty"`
	ExternalReference   string            `json:"external_reference,omitempty"`
	Description         string            `json:"description"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	FailureReason       string            `json:"failure_reason,omitempty"`
	RetryCount          int               `json:"retry_count"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	ProcessedAt         *time.Time        `json:"processed_at,omitempty"`
	CompletedAt         *time.Time        `json:"completed_at,omitempty"`
}

// TransactionListDTO - результат для списка транзакций.
type TransactionListDTO struct {
	Transactions []TransactionDTO `json:"transactions"`
	TotalCount   int              `json:"total_count"`
	Offset       int              `json:"offset"`
	Limit        int              `json:"limit"`
}

// TransactionCreatedDTO - результат создания транзакции.
type TransactionCreatedDTO struct {
	Transaction TransactionDTO `json:"transaction"`
	Message     string         `json:"message"`
}
