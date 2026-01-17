// Package dtos - Wallet DTOs для передачи данных о кошельках.
package dtos

import "time"

// ============================================
// Commands (Write операции)
// ============================================

// CreateWalletCommand - команда для создания кошелька.
type CreateWalletCommand struct {
	UserID       string `json:"user_id" validate:"required,uuid"`
	CurrencyCode string `json:"currency_code" validate:"required,len=3"` // USD, EUR, BTC
}

// CreditWalletCommand - команда для пополнения кошелька.
type CreditWalletCommand struct {
	WalletID          string `json:"wallet_id" validate:"required,uuid"`
	Amount            string `json:"amount" validate:"required"` // Decimal string: "100.50"
	IdempotencyKey    string `json:"idempotency_key" validate:"required,uuid"`
	Description       string `json:"description" validate:"required"`
	ExternalReference string `json:"external_reference,omitempty"` // Например, Stripe payment_intent_id
}

// DebitWalletCommand - команда для списания с кошелька.
type DebitWalletCommand struct {
	WalletID          string `json:"wallet_id" validate:"required,uuid"`
	Amount            string `json:"amount" validate:"required"`
	IdempotencyKey    string `json:"idempotency_key" validate:"required,uuid"`
	Description       string `json:"description" validate:"required"`
	ExternalReference string `json:"external_reference,omitempty"`
}

// TransferFundsCommand - команда для перевода между кошельками.
type TransferFundsCommand struct {
	SourceWalletID      string `json:"source_wallet_id" validate:"required,uuid"`
	DestinationWalletID string `json:"destination_wallet_id" validate:"required,uuid"`
	Amount              string `json:"amount" validate:"required"`
	IdempotencyKey      string `json:"idempotency_key" validate:"required,uuid"`
	Description         string `json:"description" validate:"required"`
}

// UpdateWalletStatusCommand - команда для изменения статуса кошелька.
type UpdateWalletStatusCommand struct {
	WalletID string `json:"wallet_id" validate:"required,uuid"`
	Status   string `json:"status" validate:"required,oneof=ACTIVE SUSPENDED LOCKED CLOSED"`
	Reason   string `json:"reason,omitempty"` // Причина изменения статуса
}

// UpdateWalletLimitsCommand - команда для обновления лимитов.
type UpdateWalletLimitsCommand struct {
	WalletID     string `json:"wallet_id" validate:"required,uuid"`
	DailyLimit   string `json:"daily_limit" validate:"required"`
	MonthlyLimit string `json:"monthly_limit" validate:"required"`
}

// ============================================
// Queries (Read операции)
// ============================================

// GetWalletQuery - запрос для получения кошелька по ID.
type GetWalletQuery struct {
	WalletID string `json:"wallet_id" validate:"required,uuid"`
}

// GetWalletByUserAndCurrencyQuery - запрос кошелька пользователя по валюте.
type GetWalletByUserAndCurrencyQuery struct {
	UserID       string `json:"user_id" validate:"required,uuid"`
	CurrencyCode string `json:"currency_code" validate:"required,len=3"`
}

// ListWalletsQuery - запрос списка кошельков с фильтрацией.
type ListWalletsQuery struct {
	UserID       *string `json:"user_id,omitempty" validate:"omitempty,uuid"`
	CurrencyCode *string `json:"currency_code,omitempty" validate:"omitempty,len=3"`
	Status       *string `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE SUSPENDED LOCKED CLOSED"`
	Offset       int     `json:"offset" validate:"min=0"`
	Limit        int     `json:"limit" validate:"min=1,max=100"`
}

// ============================================
// Response DTOs
// ============================================

// WalletDTO - представление кошелька для API.
type WalletDTO struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	CurrencyCode     string    `json:"currency_code"`
	WalletType       string    `json:"wallet_type"` // "FIAT" or "CRYPTO"
	Status           string    `json:"status"`
	AvailableBalance string    `json:"available_balance"` // Decimal string: "100.50"
	PendingBalance   string    `json:"pending_balance"`
	TotalBalance     string    `json:"total_balance"`
	DailyLimit       string    `json:"daily_limit"`
	MonthlyLimit     string    `json:"monthly_limit"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WalletListDTO - результат для списка кошельков.
type WalletListDTO struct {
	Wallets    []WalletDTO `json:"wallets"`
	TotalCount int         `json:"total_count"`
	Offset     int         `json:"offset"`
	Limit      int         `json:"limit"`
}

// WalletOperationDTO - результат операции с кошельком (credit/debit).
type WalletOperationDTO struct {
	Wallet        WalletDTO `json:"wallet"`
	TransactionID string    `json:"transaction_id"`
	Message       string    `json:"message"` // Например: "Wallet credited successfully"
}

// TransferResultDTO - результат перевода между кошельками.
type TransferResultDTO struct {
	SourceWallet      WalletDTO `json:"source_wallet"`
	DestinationWallet WalletDTO `json:"destination_wallet"`
	TransactionID     string    `json:"transaction_id"`
	Amount            string    `json:"amount"`
	Status            string    `json:"status"`
}
