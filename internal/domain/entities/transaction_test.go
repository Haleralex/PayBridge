package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// TestTransactionType_IsValid tests transaction type validation
func TestTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		expected bool
	}{
		{"DEPOSIT is valid", TransactionTypeDeposit, true},
		{"WITHDRAW is valid", TransactionTypeWithdraw, true},
		{"PAYOUT is valid", TransactionTypePayout, true},
		{"TRANSFER is valid", TransactionTypeTransfer, true},
		{"FEE is valid", TransactionTypeFee, true},
		{"REFUND is valid", TransactionTypeRefund, true},
		{"ADJUSTMENT is valid", TransactionTypeAdjustment, true},
		{"Invalid type", TransactionType("INVALID"), false},
		{"Empty type", TransactionType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.txType.IsValid(); got != tt.expected {
				t.Errorf("TransactionType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTransactionStatus_IsValid tests transaction status validation
func TestTransactionStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   TransactionStatus
		expected bool
	}{
		{"PENDING is valid", TransactionStatusPending, true},
		{"PROCESSING is valid", TransactionStatusProcessing, true},
		{"COMPLETED is valid", TransactionStatusCompleted, true},
		{"FAILED is valid", TransactionStatusFailed, true},
		{"CANCELLED is valid", TransactionStatusCancelled, true},
		{"Invalid status", TransactionStatus("INVALID"), false},
		{"Empty status", TransactionStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.expected {
				t.Errorf("TransactionStatus.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTransactionStatus_IsFinal tests final status checks
func TestTransactionStatus_IsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   TransactionStatus
		expected bool
	}{
		{"PENDING is not final", TransactionStatusPending, false},
		{"PROCESSING is not final", TransactionStatusProcessing, false},
		{"COMPLETED is final", TransactionStatusCompleted, true},
		{"FAILED is final", TransactionStatusFailed, true},
		{"CANCELLED is final", TransactionStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsFinal(); got != tt.expected {
				t.Errorf("TransactionStatus.IsFinal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNewTransaction_Success tests successful transaction creation
func TestNewTransaction_Success(t *testing.T) {
	walletID := uuid.New()
	idempotencyKey := "test-key-123"
	txType := TransactionTypeDeposit
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	description := "Test deposit"

	tx, err := NewTransaction(walletID, idempotencyKey, txType, amount, description)

	if err != nil {
		t.Fatalf("NewTransaction() error = %v, want nil", err)
	}

	if tx.ID() == uuid.Nil {
		t.Error("Transaction ID should not be nil")
	}

	if tx.WalletID() != walletID {
		t.Errorf("WalletID = %v, want %v", tx.WalletID(), walletID)
	}

	if tx.IdempotencyKey() != idempotencyKey {
		t.Errorf("IdempotencyKey = %v, want %v", tx.IdempotencyKey(), idempotencyKey)
	}

	if tx.Type() != txType {
		t.Errorf("Type = %v, want %v", tx.Type(), txType)
	}

	if tx.Status() != TransactionStatusPending {
		t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusPending)
	}

	if !tx.Amount().Equals(amount) {
		t.Errorf("Amount = %v, want %v", tx.Amount(), amount)
	}

	if tx.Description() != description {
		t.Errorf("Description = %v, want %v", tx.Description(), description)
	}

	if tx.RetryCount() != 0 {
		t.Errorf("RetryCount = %v, want 0", tx.RetryCount())
	}

	if len(tx.Metadata()) != 0 {
		t.Errorf("Metadata should be empty, got %v", tx.Metadata())
	}
}

// TestNewTransaction_EmptyIdempotencyKey tests validation
func TestNewTransaction_EmptyIdempotencyKey(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	_, err := NewTransaction(walletID, "", TransactionTypeDeposit, amount, "test")

	if err == nil {
		t.Fatal("NewTransaction() with empty idempotency key should return error")
	}

	if _, ok := err.(errors.ValidationError); !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

// TestNewTransaction_InvalidTransactionType tests validation
func TestNewTransaction_InvalidTransactionType(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	_, err := NewTransaction(walletID, "key-123", TransactionType("INVALID"), amount, "test")

	if err == nil {
		t.Fatal("NewTransaction() with invalid type should return error")
	}
}

// TestNewTransaction_NonPositiveAmount tests validation
func TestNewTransaction_NonPositiveAmount(t *testing.T) {
	walletID := uuid.New()
	zeroAmount := valueobjects.Zero(valueobjects.USD)

	_, err := NewTransaction(walletID, "key-123", TransactionTypeDeposit, zeroAmount, "test")

	if err == nil {
		t.Fatal("NewTransaction() with zero amount should return error")
	}
}

// TestReconstructTransaction tests transaction reconstruction
func TestReconstructTransaction(t *testing.T) {
	id := uuid.New()
	walletID := uuid.New()
	destWalletID := uuid.New()
	idempotencyKey := "key-123"
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	metadata := map[string]interface{}{"source": "app", "userId": "123"}
	metadataJSON, _ := json.Marshal(metadata)
	now := time.Now()
	processedAt := now.Add(1 * time.Minute)
	completedAt := now.Add(2 * time.Minute)

	tx, err := ReconstructTransaction(
		id, walletID,
		idempotencyKey,
		TransactionTypeTransfer,
		TransactionStatusCompleted,
		amount,
		&destWalletID,
		"ext-ref-123",
		"Test transfer",
		metadataJSON,
		"",
		2,
		now, now,
		&processedAt, &completedAt,
	)

	if err != nil {
		t.Fatalf("ReconstructTransaction() error = %v", err)
	}

	if tx.ID() != id {
		t.Errorf("ID = %v, want %v", tx.ID(), id)
	}

	if tx.RetryCount() != 2 {
		t.Errorf("RetryCount = %v, want 2", tx.RetryCount())
	}

	if tx.DestinationWalletID() == nil || *tx.DestinationWalletID() != destWalletID {
		t.Errorf("DestinationWalletID incorrect")
	}

	if tx.ExternalReference() != "ext-ref-123" {
		t.Errorf("ExternalReference = %v, want ext-ref-123", tx.ExternalReference())
	}

	if tx.Metadata()["source"] != "app" {
		t.Errorf("Metadata not reconstructed correctly")
	}
}

// TestReconstructTransaction_InvalidMetadata tests reconstruction with bad metadata
func TestReconstructTransaction_InvalidMetadata(t *testing.T) {
	id := uuid.New()
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	invalidJSON := []byte("invalid json {")
	now := time.Now()

	_, err := ReconstructTransaction(
		id, walletID,
		"key-123",
		TransactionTypeDeposit,
		TransactionStatusPending,
		amount,
		nil,
		"",
		"Test",
		invalidJSON,
		"",
		0,
		now, now,
		nil, nil,
	)

	if err == nil {
		t.Fatal("ReconstructTransaction() with invalid JSON should return error")
	}
}

// TestReconstructTransaction_EmptyMetadata tests reconstruction with no metadata
func TestReconstructTransaction_EmptyMetadata(t *testing.T) {
	id := uuid.New()
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	now := time.Now()

	tx, err := ReconstructTransaction(
		id, walletID,
		"key-123",
		TransactionTypeDeposit,
		TransactionStatusPending,
		amount,
		nil,
		"",
		"Test",
		nil,
		"",
		0,
		now, now,
		nil, nil,
	)

	if err != nil {
		t.Fatalf("ReconstructTransaction() error = %v", err)
	}

	if len(tx.Metadata()) != 0 {
		t.Errorf("Metadata should be empty")
	}
}

// TestTransaction_StatusChecks tests all status check methods
func TestTransaction_StatusChecks(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	tests := []struct {
		name         string
		status       TransactionStatus
		isPending    bool
		isProcessing bool
		isCompleted  bool
		isFailed     bool
		isFinal      bool
	}{
		{"Pending", TransactionStatusPending, true, false, false, false, false},
		{"Processing", TransactionStatusProcessing, false, true, false, false, false},
		{"Completed", TransactionStatusCompleted, false, false, true, false, true},
		{"Failed", TransactionStatusFailed, false, false, false, true, true},
		{"Cancelled", TransactionStatusCancelled, false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				id:              uuid.New(),
				walletID:        walletID,
				idempotencyKey:  "key-123",
				transactionType: TransactionTypeDeposit,
				status:          tt.status,
				amount:          amount,
			}

			if tx.IsPending() != tt.isPending {
				t.Errorf("IsPending() = %v, want %v", tx.IsPending(), tt.isPending)
			}
			if tx.IsProcessing() != tt.isProcessing {
				t.Errorf("IsProcessing() = %v, want %v", tx.IsProcessing(), tt.isProcessing)
			}
			if tx.IsCompleted() != tt.isCompleted {
				t.Errorf("IsCompleted() = %v, want %v", tx.IsCompleted(), tt.isCompleted)
			}
			if tx.IsFailed() != tt.isFailed {
				t.Errorf("IsFailed() = %v, want %v", tx.IsFailed(), tt.isFailed)
			}
			if tx.IsFinal() != tt.isFinal {
				t.Errorf("IsFinal() = %v, want %v", tx.IsFinal(), tt.isFinal)
			}
		})
	}
}

// TestTransaction_SetDestinationWallet tests setting destination wallet
func TestTransaction_SetDestinationWallet(t *testing.T) {
	walletID := uuid.New()
	destWalletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Set destination for transfer", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeTransfer, amount, "Transfer")

		err := tx.SetDestinationWallet(destWalletID)
		if err != nil {
			t.Fatalf("SetDestinationWallet() error = %v", err)
		}

		if tx.DestinationWalletID() == nil || *tx.DestinationWalletID() != destWalletID {
			t.Error("Destination wallet not set correctly")
		}
	})

	t.Run("Cannot set destination for non-transfer", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.SetDestinationWallet(destWalletID)
		if err == nil {
			t.Fatal("SetDestinationWallet() on non-transfer should return error")
		}
	})

	t.Run("Cannot set destination on final transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeTransfer, amount, "Transfer")
		tx.status = TransactionStatusCompleted

		err := tx.SetDestinationWallet(destWalletID)
		if err == nil {
			t.Fatal("SetDestinationWallet() on final transaction should return error")
		}
	})
}

// TestTransaction_SetExternalReference tests setting external reference
func TestTransaction_SetExternalReference(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Set external reference", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		reference := "stripe_123"

		err := tx.SetExternalReference(reference)
		if err != nil {
			t.Fatalf("SetExternalReference() error = %v", err)
		}

		if tx.ExternalReference() != reference {
			t.Errorf("ExternalReference = %v, want %v", tx.ExternalReference(), reference)
		}
	})

	t.Run("Cannot set reference on final transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.status = TransactionStatusCompleted

		err := tx.SetExternalReference("ref-123")
		if err == nil {
			t.Fatal("SetExternalReference() on final transaction should return error")
		}
	})
}

// TestTransaction_AddMetadata tests adding metadata
func TestTransaction_AddMetadata(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Add metadata", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.AddMetadata("userId", "user-123")
		if err != nil {
			t.Fatalf("AddMetadata() error = %v", err)
		}

		if tx.Metadata()["userId"] != "user-123" {
			t.Error("Metadata not added correctly")
		}
	})

	t.Run("Add multiple metadata fields", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		tx.AddMetadata("userId", "user-123")
		tx.AddMetadata("source", "app")
		tx.AddMetadata("version", 1)

		if len(tx.Metadata()) != 3 {
			t.Errorf("Metadata count = %v, want 3", len(tx.Metadata()))
		}
	})

	t.Run("Cannot add metadata to final transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.status = TransactionStatusCompleted

		err := tx.AddMetadata("key", "value")
		if err == nil {
			t.Fatal("AddMetadata() on final transaction should return error")
		}
	})
}

// TestTransaction_StartProcessing tests starting processing
func TestTransaction_StartProcessing(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Start processing pending transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.StartProcessing()
		if err != nil {
			t.Fatalf("StartProcessing() error = %v", err)
		}

		if tx.Status() != TransactionStatusProcessing {
			t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusProcessing)
		}

		if tx.ProcessedAt() == nil {
			t.Error("ProcessedAt should be set")
		}
	})

	t.Run("Cannot start processing non-pending transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.status = TransactionStatusProcessing

		err := tx.StartProcessing()
		if err == nil {
			t.Fatal("StartProcessing() on non-pending should return error")
		}
	})
}

// TestTransaction_MarkCompleted tests marking as completed
func TestTransaction_MarkCompleted(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Mark processing transaction as completed", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()

		err := tx.MarkCompleted()
		if err != nil {
			t.Fatalf("MarkCompleted() error = %v", err)
		}

		if tx.Status() != TransactionStatusCompleted {
			t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusCompleted)
		}

		if tx.CompletedAt() == nil {
			t.Error("CompletedAt should be set")
		}
	})

	t.Run("Cannot complete non-processing transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.MarkCompleted()
		if err == nil {
			t.Fatal("MarkCompleted() on pending should return error")
		}
	})
}

// TestTransaction_MarkFailed tests marking as failed
func TestTransaction_MarkFailed(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Mark pending transaction as failed", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		reason := "Network timeout"

		err := tx.MarkFailed(reason)
		if err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}

		if tx.Status() != TransactionStatusFailed {
			t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusFailed)
		}

		if tx.FailureReason() != reason {
			t.Errorf("FailureReason = %v, want %v", tx.FailureReason(), reason)
		}

		if tx.CompletedAt() == nil {
			t.Error("CompletedAt should be set")
		}
	})

	t.Run("Mark processing transaction as failed", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()

		err := tx.MarkFailed("Error occurred")
		if err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}

		if tx.Status() != TransactionStatusFailed {
			t.Error("Status should be FAILED")
		}
	})

	t.Run("Cannot fail already final transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()
		tx.MarkCompleted()

		err := tx.MarkFailed("Reason")
		if err == nil {
			t.Fatal("MarkFailed() on completed should return error")
		}
	})
}

// TestTransaction_Cancel tests canceling transaction
func TestTransaction_Cancel(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	t.Run("Cancel pending transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.Cancel()
		if err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}

		if tx.Status() != TransactionStatusCancelled {
			t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusCancelled)
		}

		if tx.CompletedAt() == nil {
			t.Error("CompletedAt should be set")
		}
	})

	t.Run("Cannot cancel processing transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()

		err := tx.Cancel()
		if err == nil {
			t.Fatal("Cancel() on processing should return error")
		}
	})

	t.Run("Cannot cancel completed transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()
		tx.MarkCompleted()

		err := tx.Cancel()
		if err == nil {
			t.Fatal("Cancel() on completed should return error")
		}
	})
}

// TestTransaction_Retry tests retry logic
func TestTransaction_Retry(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	maxRetries := 3

	t.Run("Retry failed transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.StartProcessing()
		tx.MarkFailed("Network error")

		err := tx.Retry(maxRetries)
		if err != nil {
			t.Fatalf("Retry() error = %v", err)
		}

		if tx.Status() != TransactionStatusPending {
			t.Errorf("Status = %v, want %v", tx.Status(), TransactionStatusPending)
		}

		if tx.RetryCount() != 1 {
			t.Errorf("RetryCount = %v, want 1", tx.RetryCount())
		}

		if tx.FailureReason() != "" {
			t.Error("FailureReason should be cleared")
		}

		if tx.CompletedAt() != nil {
			t.Error("CompletedAt should be cleared")
		}
	})

	t.Run("Cannot retry non-failed transaction", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		err := tx.Retry(maxRetries)
		if err == nil {
			t.Fatal("Retry() on pending should return error")
		}
	})

	t.Run("Cannot retry beyond max retries", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
		tx.retryCount = 3
		tx.status = TransactionStatusFailed

		err := tx.Retry(maxRetries)
		if err == nil {
			t.Fatal("Retry() beyond max should return error")
		}
	})

	t.Run("Multiple retries increment count", func(t *testing.T) {
		tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")

		tx.StartProcessing()
		tx.MarkFailed("Error 1")
		tx.Retry(maxRetries)

		if tx.RetryCount() != 1 {
			t.Errorf("RetryCount = %v, want 1", tx.RetryCount())
		}

		tx.StartProcessing()
		tx.MarkFailed("Error 2")
		tx.Retry(maxRetries)

		if tx.RetryCount() != 2 {
			t.Errorf("RetryCount = %v, want 2", tx.RetryCount())
		}
	})
}

// TestTransaction_CanRetry tests retry permission check
func TestTransaction_CanRetry(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	maxRetries := 3

	tests := []struct {
		name       string
		status     TransactionStatus
		retryCount int
		expected   bool
	}{
		{"Failed with retries left", TransactionStatusFailed, 0, true},
		{"Failed at max retries", TransactionStatusFailed, 3, false},
		{"Pending cannot retry", TransactionStatusPending, 0, false},
		{"Completed cannot retry", TransactionStatusCompleted, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				id:              uuid.New(),
				walletID:        walletID,
				idempotencyKey:  "key-123",
				transactionType: TransactionTypeDeposit,
				status:          tt.status,
				amount:          amount,
				retryCount:      tt.retryCount,
			}

			if got := tx.CanRetry(maxRetries); got != tt.expected {
				t.Errorf("CanRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTransaction_IsRetryable tests retryability logic
func TestTransaction_IsRetryable(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	tests := []struct {
		name          string
		failureReason string
		expected      bool
	}{
		{"Network error is retryable", "NETWORK_TIMEOUT", true},
		{"Unknown error is retryable", "UNKNOWN_ERROR", true},
		{"Invalid account not retryable", "INVALID_ACCOUNT", false},
		{"Account closed not retryable", "ACCOUNT_CLOSED", false},
		{"Insufficient balance not retryable", "INSUFFICIENT_BALANCE", false},
		{"Fraud detected not retryable", "FRAUD_DETECTED", false},
		{"Blacklisted not retryable", "BLACKLISTED", false},
		{"Empty reason is retryable", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				id:              uuid.New(),
				walletID:        walletID,
				idempotencyKey:  "key-123",
				transactionType: TransactionTypeDeposit,
				status:          TransactionStatusFailed,
				amount:          amount,
				failureReason:   tt.failureReason,
			}

			if got := tx.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTransaction_Getters tests all getter methods
func TestTransaction_Getters(t *testing.T) {
	id := uuid.New()
	walletID := uuid.New()
	destWalletID := uuid.New()
	idempotencyKey := "key-123"
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	description := "Test transaction"
	externalRef := "ext-123"
	failureReason := "Test failure"
	metadata := map[string]interface{}{"test": "value"}
	metadataJSON, _ := json.Marshal(metadata)
	now := time.Now()
	processedAt := now.Add(1 * time.Minute)
	completedAt := now.Add(2 * time.Minute)

	tx, _ := ReconstructTransaction(
		id, walletID,
		idempotencyKey,
		TransactionTypeTransfer,
		TransactionStatusFailed,
		amount,
		&destWalletID,
		externalRef,
		description,
		metadataJSON,
		failureReason,
		2,
		now, now,
		&processedAt, &completedAt,
	)

	if tx.ID() != id {
		t.Errorf("ID() = %v, want %v", tx.ID(), id)
	}
	if tx.WalletID() != walletID {
		t.Errorf("WalletID() = %v, want %v", tx.WalletID(), walletID)
	}
	if tx.IdempotencyKey() != idempotencyKey {
		t.Errorf("IdempotencyKey() = %v, want %v", tx.IdempotencyKey(), idempotencyKey)
	}
	if tx.Type() != TransactionTypeTransfer {
		t.Errorf("Type() = %v, want %v", tx.Type(), TransactionTypeTransfer)
	}
	if tx.Status() != TransactionStatusFailed {
		t.Errorf("Status() = %v, want %v", tx.Status(), TransactionStatusFailed)
	}
	if !tx.Amount().Equals(amount) {
		t.Errorf("Amount() = %v, want %v", tx.Amount(), amount)
	}
	if tx.DestinationWalletID() == nil || *tx.DestinationWalletID() != destWalletID {
		t.Error("DestinationWalletID() incorrect")
	}
	if tx.ExternalReference() != externalRef {
		t.Errorf("ExternalReference() = %v, want %v", tx.ExternalReference(), externalRef)
	}
	if tx.Description() != description {
		t.Errorf("Description() = %v, want %v", tx.Description(), description)
	}
	if tx.FailureReason() != failureReason {
		t.Errorf("FailureReason() = %v, want %v", tx.FailureReason(), failureReason)
	}
	if tx.RetryCount() != 2 {
		t.Errorf("RetryCount() = %v, want 2", tx.RetryCount())
	}
	if !tx.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt() = %v, want %v", tx.CreatedAt(), now)
	}
	if !tx.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() = %v, want %v", tx.UpdatedAt(), now)
	}
	if tx.ProcessedAt() == nil || !tx.ProcessedAt().Equal(processedAt) {
		t.Error("ProcessedAt() incorrect")
	}
	if tx.CompletedAt() == nil || !tx.CompletedAt().Equal(completedAt) {
		t.Error("CompletedAt() incorrect")
	}
	if tx.Metadata()["test"] != "value" {
		t.Error("Metadata() incorrect")
	}
}

// TestTransaction_UpdatedAtChanges tests that UpdatedAt changes on operations
func TestTransaction_UpdatedAtChanges(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	tx, _ := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Test")

	initialUpdatedAt := tx.UpdatedAt()
	time.Sleep(10 * time.Millisecond)

	tx.AddMetadata("test", "value")

	if !tx.UpdatedAt().After(initialUpdatedAt) {
		t.Error("UpdatedAt should change after metadata addition")
	}
}

// TestTransaction_FullLifecycle tests complete transaction lifecycle
func TestTransaction_FullLifecycle(t *testing.T) {
	walletID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	// Create
	tx, err := NewTransaction(walletID, "key-123", TransactionTypeDeposit, amount, "Deposit")
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	if !tx.IsPending() {
		t.Error("New transaction should be pending")
	}

	// Add metadata before processing
	tx.AddMetadata("source", "app")

	// Start processing
	err = tx.StartProcessing()
	if err != nil {
		t.Fatalf("StartProcessing() error = %v", err)
	}

	if !tx.IsProcessing() {
		t.Error("Transaction should be processing")
	}

	// Complete
	err = tx.MarkCompleted()
	if err != nil {
		t.Fatalf("MarkCompleted() error = %v", err)
	}

	if !tx.IsCompleted() || !tx.IsFinal() {
		t.Error("Transaction should be completed and final")
	}

	// Cannot modify after completion
	err = tx.AddMetadata("key", "value")
	if err == nil {
		t.Error("Should not be able to add metadata to completed transaction")
	}
}
