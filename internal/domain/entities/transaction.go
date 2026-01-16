// Package entities - Transaction represents a financial operation in the system.
// This is a critical entity with complex state management and business rules.
package entities

import (
	"encoding/json"
	"time"

	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// TransactionType represents the type of transaction.
type TransactionType string

const (
	TransactionTypeDeposit    TransactionType = "DEPOSIT"    // External deposit to wallet
	TransactionTypeWithdraw   TransactionType = "WITHDRAW"   // Withdrawal to external account
	TransactionTypePayout     TransactionType = "PAYOUT"     // Payout to merchant/user
	TransactionTypeTransfer   TransactionType = "TRANSFER"   // Internal transfer between wallets
	TransactionTypeFee        TransactionType = "FEE"        // System fee
	TransactionTypeRefund     TransactionType = "REFUND"     // Refund of previous transaction
	TransactionTypeAdjustment TransactionType = "ADJUSTMENT" // Manual adjustment (admin)
)

// IsValid checks if the transaction type is valid.
func (t TransactionType) IsValid() bool {
	switch t {
	case TransactionTypeDeposit, TransactionTypeWithdraw, TransactionTypePayout,
		TransactionTypeTransfer, TransactionTypeFee, TransactionTypeRefund, TransactionTypeAdjustment:
		return true
	default:
		return false
	}
}

// TransactionStatus represents the current state of a transaction.
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "PENDING"    // Created, not yet processed
	TransactionStatusProcessing TransactionStatus = "PROCESSING" // Currently being processed
	TransactionStatusCompleted  TransactionStatus = "COMPLETED"  // Successfully completed
	TransactionStatusFailed     TransactionStatus = "FAILED"     // Processing failed
	TransactionStatusCancelled  TransactionStatus = "CANCELLED"  // Cancelled by user/system
)

// IsValid checks if the transaction status is valid.
func (s TransactionStatus) IsValid() bool {
	switch s {
	case TransactionStatusPending, TransactionStatusProcessing, TransactionStatusCompleted,
		TransactionStatusFailed, TransactionStatusCancelled:
		return true
	default:
		return false
	}
}

// IsFinal returns true if the status is terminal (no further transitions).
func (s TransactionStatus) IsFinal() bool {
	return s == TransactionStatusCompleted || s == TransactionStatusFailed || s == TransactionStatusCancelled
}

// Transaction represents a financial transaction in the system.
// This is an Entity with complex state machine and business rules.
//
// Entity Pattern:
// - Has identity (ID + idempotency key)
// - Complex state machine (status transitions)
// - Rich domain behavior
// - Immutable after completion (status terminal)
//
// SOLID:
// - SRP: Manages transaction lifecycle and validation
// - OCP: New transaction types can be added via enum extension
// - LSP: All transaction types follow same state machine
//
// Patterns Applied:
// - State Machine: Status transitions with validation
// - Idempotency: Each transaction has unique idempotency key
type Transaction struct {
	id              uuid.UUID
	walletID        uuid.UUID // Source wallet
	idempotencyKey  string    // Unique key for idempotency (client-provided)
	transactionType TransactionType
	status          TransactionStatus
	amount          valueobjects.Money

	// Optional fields depending on transaction type
	destinationWalletID *uuid.UUID // For transfers
	externalReference   string     // External system reference (e.g., Stripe payment ID)
	description         string
	metadata            map[string]interface{} // Flexible metadata (JSON)

	// Failure information
	failureReason string
	retryCount    int // Number of retry attempts

	// Timestamps
	createdAt   time.Time
	updatedAt   time.Time
	processedAt *time.Time // When processing started
	completedAt *time.Time // When finalized (completed/failed/cancelled)
}

// NewTransaction creates a new transaction.
// Factory function with validation.
//
// Business Rules:
// - Idempotency key must be unique (checked by repository)
// - Amount must be positive
// - Wallet must exist
// - Transaction type must be valid
// - New transactions start in PENDING status
func NewTransaction(
	walletID uuid.UUID,
	idempotencyKey string,
	transactionType TransactionType,
	amount valueobjects.Money,
	description string,
) (*Transaction, error) {
	// Validate inputs
	if idempotencyKey == "" {
		return nil, errors.ValidationError{
			Field:   "idempotencyKey",
			Message: "idempotency key is required",
		}
	}

	if !transactionType.IsValid() {
		return nil, errors.ErrInvalidTransactionType
	}

	if !amount.IsPositive() {
		return nil, errors.NewBusinessRuleViolation(
			"INVALID_AMOUNT",
			"transaction amount must be positive",
			map[string]interface{}{"amount": amount.String()},
		)
	}

	now := time.Now()
	return &Transaction{
		id:              uuid.New(),
		walletID:        walletID,
		idempotencyKey:  idempotencyKey,
		transactionType: transactionType,
		status:          TransactionStatusPending,
		amount:          amount,
		description:     description,
		metadata:        make(map[string]interface{}),
		retryCount:      0,
		createdAt:       now,
		updatedAt:       now,
	}, nil
}

// ReconstructTransaction reconstructs a Transaction from stored data.
func ReconstructTransaction(
	id, walletID uuid.UUID,
	idempotencyKey string,
	transactionType TransactionType,
	status TransactionStatus,
	amount valueobjects.Money,
	destinationWalletID *uuid.UUID,
	externalReference string,
	description string,
	metadataJSON []byte,
	failureReason string,
	retryCount int,
	createdAt, updatedAt time.Time,
	processedAt, completedAt *time.Time,
) (*Transaction, error) {
	var metadata map[string]interface{}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return nil, err
		}
	} else {
		metadata = make(map[string]interface{})
	}

	return &Transaction{
		id:                  id,
		walletID:            walletID,
		idempotencyKey:      idempotencyKey,
		transactionType:     transactionType,
		status:              status,
		amount:              amount,
		destinationWalletID: destinationWalletID,
		externalReference:   externalReference,
		description:         description,
		metadata:            metadata,
		failureReason:       failureReason,
		retryCount:          retryCount,
		createdAt:           createdAt,
		updatedAt:           updatedAt,
		processedAt:         processedAt,
		completedAt:         completedAt,
	}, nil
}

// Getters

func (t *Transaction) ID() uuid.UUID {
	return t.id
}

func (t *Transaction) WalletID() uuid.UUID {
	return t.walletID
}

func (t *Transaction) IdempotencyKey() string {
	return t.idempotencyKey
}

func (t *Transaction) Type() TransactionType {
	return t.transactionType
}

func (t *Transaction) Status() TransactionStatus {
	return t.status
}

func (t *Transaction) Amount() valueobjects.Money {
	return t.amount
}

func (t *Transaction) DestinationWalletID() *uuid.UUID {
	return t.destinationWalletID
}

func (t *Transaction) ExternalReference() string {
	return t.externalReference
}

func (t *Transaction) Description() string {
	return t.description
}

func (t *Transaction) Metadata() map[string]interface{} {
	return t.metadata
}

func (t *Transaction) FailureReason() string {
	return t.failureReason
}

func (t *Transaction) RetryCount() int {
	return t.retryCount
}

func (t *Transaction) CreatedAt() time.Time {
	return t.createdAt
}

func (t *Transaction) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t *Transaction) ProcessedAt() *time.Time {
	return t.processedAt
}

func (t *Transaction) CompletedAt() *time.Time {
	return t.completedAt
}

// Business Methods

// IsPending returns true if the transaction is in pending state.
func (t *Transaction) IsPending() bool {
	return t.status == TransactionStatusPending
}

// IsProcessing returns true if the transaction is being processed.
func (t *Transaction) IsProcessing() bool {
	return t.status == TransactionStatusProcessing
}

// IsCompleted returns true if the transaction completed successfully.
func (t *Transaction) IsCompleted() bool {
	return t.status == TransactionStatusCompleted
}

// IsFailed returns true if the transaction failed.
func (t *Transaction) IsFailed() bool {
	return t.status == TransactionStatusFailed
}

// IsFinal returns true if the transaction is in a terminal state.
func (t *Transaction) IsFinal() bool {
	return t.status.IsFinal()
}

// SetDestinationWallet sets the destination wallet for transfers.
func (t *Transaction) SetDestinationWallet(walletID uuid.UUID) error {
	if t.transactionType != TransactionTypeTransfer {
		return errors.NewBusinessRuleViolation(
			"INVALID_TRANSACTION_TYPE",
			"destination wallet only applies to transfer transactions",
			map[string]interface{}{"type": t.transactionType},
		)
	}

	if t.IsFinal() {
		return errors.ErrTransactionAlreadyProcessed
	}

	t.destinationWalletID = &walletID
	t.updatedAt = time.Now()
	return nil
}

// SetExternalReference sets an external system reference.
func (t *Transaction) SetExternalReference(reference string) error {
	if t.IsFinal() {
		return errors.ErrTransactionAlreadyProcessed
	}

	t.externalReference = reference
	t.updatedAt = time.Now()
	return nil
}

// AddMetadata adds custom metadata to the transaction.
func (t *Transaction) AddMetadata(key string, value interface{}) error {
	if t.IsFinal() {
		return errors.ErrTransactionAlreadyProcessed
	}

	t.metadata[key] = value
	t.updatedAt = time.Now()
	return nil
}

// State Machine Transitions

// StartProcessing transitions the transaction to PROCESSING status.
// Business rule: Can only process PENDING transactions.
func (t *Transaction) StartProcessing() error {
	if !t.IsPending() {
		return errors.ErrTransactionNotPending
	}

	now := time.Now()
	t.status = TransactionStatusProcessing
	t.processedAt = &now
	t.updatedAt = now
	return nil
}

// MarkCompleted transitions the transaction to COMPLETED status.
// Business rule: Can only complete PROCESSING transactions.
func (t *Transaction) MarkCompleted() error {
	if !t.IsProcessing() {
		return errors.NewBusinessRuleViolation(
			"CANNOT_COMPLETE_NON_PROCESSING_TRANSACTION",
			"only processing transactions can be completed",
			map[string]interface{}{"currentStatus": t.status},
		)
	}

	now := time.Now()
	t.status = TransactionStatusCompleted
	t.completedAt = &now
	t.updatedAt = now
	return nil
}

// MarkFailed transitions the transaction to FAILED status with reason.
// Business rule: Can fail from PENDING or PROCESSING states.
func (t *Transaction) MarkFailed(reason string) error {
	if t.IsFinal() {
		return errors.ErrTransactionAlreadyProcessed
	}

	now := time.Now()
	t.status = TransactionStatusFailed
	t.failureReason = reason
	t.completedAt = &now
	t.updatedAt = now
	return nil
}

// Cancel transitions the transaction to CANCELLED status.
// Business rule: Can only cancel PENDING transactions.
func (t *Transaction) Cancel() error {
	if !t.IsPending() {
		return errors.NewBusinessRuleViolation(
			"CANNOT_CANCEL_NON_PENDING_TRANSACTION",
			"only pending transactions can be cancelled",
			map[string]interface{}{"currentStatus": t.status},
		)
	}

	now := time.Now()
	t.status = TransactionStatusCancelled
	t.completedAt = &now
	t.updatedAt = now
	return nil
}

// Retry attempts to retry a failed transaction.
// Business rule: Only FAILED transactions can be retried, with max retry limit.
func (t *Transaction) Retry(maxRetries int) error {
	if !t.IsFailed() {
		return errors.NewBusinessRuleViolation(
			"CANNOT_RETRY_NON_FAILED_TRANSACTION",
			"only failed transactions can be retried",
			map[string]interface{}{"currentStatus": t.status},
		)
	}

	if t.retryCount >= maxRetries {
		return errors.NewBusinessRuleViolation(
			"MAX_RETRIES_EXCEEDED",
			"maximum retry attempts exceeded",
			map[string]interface{}{
				"retryCount": t.retryCount,
				"maxRetries": maxRetries,
			},
		)
	}

	t.status = TransactionStatusPending
	t.retryCount++
	t.failureReason = ""
	t.completedAt = nil
	t.updatedAt = time.Now()
	return nil
}

// CanRetry checks if the transaction can be retried.
func (t *Transaction) CanRetry(maxRetries int) bool {
	return t.IsFailed() && t.retryCount < maxRetries
}

// IsRetryable returns true if the failure reason indicates the transaction can be retried.
// Business logic: Some errors are permanent (invalid account), others are transient (network timeout).
func (t *Transaction) IsRetryable() bool {
	// List of non-retryable failure reasons
	nonRetryableReasons := []string{
		"INVALID_ACCOUNT",
		"ACCOUNT_CLOSED",
		"INSUFFICIENT_BALANCE",
		"FRAUD_DETECTED",
		"BLACKLISTED",
	}

	for _, reason := range nonRetryableReasons {
		if t.failureReason == reason {
			return false
		}
	}

	return true
}
