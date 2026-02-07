package transaction

import (
	"context"
	"testing"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	domainErrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// TestProcessTransactionUseCase_Success тестирует успешную обработку транзакции
func TestProcessTransactionUseCase_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию в статусе PENDING
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	// Проверяем статус транзакции изменился на COMPLETED
	if savedTransaction.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected transaction status = %s, got %s", entities.TransactionStatusCompleted, savedTransaction.Status())
	}

	// Проверяем событие TransactionCompleted опубликовано
	if len(eventPublisher.publishedEvents) == 0 {
		t.Error("Expected at least 1 event to be published")
	}
}

// TestProcessTransactionUseCase_Failure тестирует провал обработки с rollback
func TestProcessTransactionUseCase_Failure(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	userID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию в статусе PENDING типа DEPOSIT
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")

	// Кошелёк который нужно откатить
	wallet := createTestWallet(walletID, userID, currency)
	// Предположим wallet был зачислен при создании транзакции
	_ = wallet.Credit(amountMoney)

	var savedTransaction *entities.Transaction
	var savedWallet *entities.Wallet

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == walletID {
				return wallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, w *entities.Wallet) error {
			savedWallet = w
			return nil
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       false,
		FailureReason: "Payment gateway error",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	// Проверяем статус транзакции изменился на FAILED
	if savedTransaction.Status() != entities.TransactionStatusFailed {
		t.Errorf("Expected transaction status = %s, got %s", entities.TransactionStatusFailed, savedTransaction.Status())
	}

	// Проверяем причину провала
	if savedTransaction.FailureReason() != "Payment gateway error" {
		t.Errorf("Expected failure reason = 'Payment gateway error', got '%s'", savedTransaction.FailureReason())
	}

	// Проверяем что wallet был откачен (rollback)
	if savedWallet == nil {
		t.Error("Expected wallet to be saved after rollback")
	}

	// Проверяем событие TransactionFailed опубликовано
	if len(eventPublisher.publishedEvents) == 0 {
		t.Error("Expected at least 1 event to be published")
	}
}

// TestProcessTransactionUseCase_InvalidTransactionID тестирует валидацию UUID
func TestProcessTransactionUseCase_InvalidTransactionID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	walletRepo := &mockWalletRepo{}
	transactionRepo := &mockTransactionRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: "invalid-uuid",
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for invalid UUID, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это ValidationError
	if !domainErrors.IsValidation(err) {
		t.Errorf("Expected ValidationError, got: %v", err)
	}
}

// TestProcessTransactionUseCase_TransactionNotFound тестирует несуществующую транзакцию
func TestProcessTransactionUseCase_TransactionNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for transaction not found, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestCancelTransactionUseCase_Success тестирует успешную отмену транзакции
func TestCancelTransactionUseCase_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	userID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию в статусе PENDING типа DEPOSIT
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")

	// Кошелёк
	wallet := createTestWallet(walletID, userID, currency)

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == walletID {
				return wallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	// Проверяем статус транзакции изменился на CANCELLED
	if savedTransaction.Status() != entities.TransactionStatusCancelled {
		t.Errorf("Expected transaction status = %s, got %s", entities.TransactionStatusCancelled, savedTransaction.Status())
	}

	// Проверяем событие опубликовано
	if len(eventPublisher.publishedEvents) == 0 {
		t.Error("Expected at least 1 event to be published")
	}
}

// TestCancelTransactionUseCase_AlreadyCompleted тестирует отмену завершённой транзакции
func TestCancelTransactionUseCase_AlreadyCompleted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию в статусе COMPLETED
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.StartProcessing()
	_ = transaction.MarkCompleted() // Уже завершена!

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for cancelling completed transaction, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestCancelTransactionUseCase_InvalidTransactionID тестирует валидацию UUID
func TestCancelTransactionUseCase_InvalidTransactionID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	walletRepo := &mockWalletRepo{}
	transactionRepo := &mockTransactionRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: "invalid-uuid",
		Reason:        "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for invalid UUID, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это ValidationError
	if !domainErrors.IsValidation(err) {
		t.Errorf("Expected ValidationError, got: %v", err)
	}
}

// TestProcessTransactionUseCase_IdempotentCompleted tests that completed transactions return without error
func TestProcessTransactionUseCase_IdempotentCompleted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию уже в статусе COMPLETED
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.StartProcessing()
	_ = transaction.MarkCompleted()

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error for idempotent request, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

// TestProcessTransactionUseCase_WithdrawRollback tests rollback for withdraw transactions
func TestProcessTransactionUseCase_WithdrawRollback(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	userID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// Создаём транзакцию типа WITHDRAW
	amountMoney, _ := valueobjects.NewMoney("50.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeWithdraw, amountMoney, "Withdraw Test")

	// Кошелёк с балансом после debit
	wallet := createTestWallet(walletID, userID, currency)
	initialBalance, _ := valueobjects.NewMoney("100.00", currency)
	_ = wallet.Credit(initialBalance) // Start with 100
	_ = wallet.Debit(amountMoney)     // After withdraw: 50

	var savedWallet *entities.Wallet

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == walletID {
				return wallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, w *entities.Wallet) error {
			savedWallet = w
			return nil
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			if id == transactionID {
				return transaction, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       false,
		FailureReason: "External service timeout",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Wallet should be saved with rollback
	if savedWallet == nil {
		t.Error("Expected wallet to be saved after rollback")
	}
}

// TestProcessTransactionUseCase_FailureDefaultReason tests default failure reason
func TestProcessTransactionUseCase_FailureDefaultReason(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	userID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")

	wallet := createTestWallet(walletID, userID, currency)
	_ = wallet.Credit(amountMoney)

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
		},
		saveFunc: func(ctx context.Context, w *entities.Wallet) error {
			return nil
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       false,
		FailureReason: "", // Empty - should use default
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	// Should use default failure reason
	if savedTransaction.FailureReason() != "external service error" {
		t.Errorf("Expected default failure reason, got: %s", savedTransaction.FailureReason())
	}
}

// TestCancelTransactionUseCase_AlreadyCancelled tests idempotent cancel
func TestCancelTransactionUseCase_AlreadyCancelled(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.Cancel() // Already cancelled

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert - should succeed (idempotent)
	if err != nil {
		t.Fatalf("Expected no error for idempotent cancel, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

// TestCancelTransactionUseCase_AlreadyFailed tests cancelling failed transaction
func TestCancelTransactionUseCase_AlreadyFailed(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.StartProcessing()
	_ = transaction.MarkFailed("Some error")

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for cancelling failed transaction, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestCancelTransactionUseCase_TransactionNotFound tests non-existent transaction
func TestCancelTransactionUseCase_TransactionNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for transaction not found, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestCancelTransactionUseCase_PendingDeposit tests cancelling a pending deposit (no wallet rollback needed)
func TestCancelTransactionUseCase_PendingDeposit(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	// PENDING transaction - wallet was NOT modified yet
	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	if savedTransaction.Status() != entities.TransactionStatusCancelled {
		t.Errorf("Expected CANCELLED status, got %s", savedTransaction.Status())
	}
}

// TestCancelTransactionUseCase_PendingWithdraw tests cancel of a pending withdraw
func TestCancelTransactionUseCase_PendingWithdraw(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("50.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeWithdraw, amountMoney, "Withdraw")
	// PENDING - no wallet rollback needed

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	if savedTransaction.Status() != entities.TransactionStatusCancelled {
		t.Errorf("Expected CANCELLED status, got %s", savedTransaction.Status())
	}
}

// TestCancelTransactionUseCase_TransferNotImplemented tests transfer cancel returns error
func TestCancelTransactionUseCase_TransferNotImplemented(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("50.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeTransfer, amountMoney, "Transfer")
	// PENDING transfer - cancel should succeed since no rollback needed for PENDING

	walletRepo := &mockWalletRepo{}

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CancelTransactionCommand{
		TransactionID: transactionID.String(),
		Reason:        "User cancelled",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert - PENDING transfer cancel should succeed (no rollback needed)
	if err != nil {
		t.Fatalf("Expected no error for pending transfer cancel, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

// TestProcessTransactionUseCase_InvalidStatus tests processing transaction with invalid status
func TestProcessTransactionUseCase_InvalidStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.Cancel() // CANCELLED status

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for invalid status, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestProcessTransactionUseCase_AlreadyProcessing tests processing already processing transaction
func TestProcessTransactionUseCase_AlreadyProcessing(t *testing.T) {
	// Arrange
	ctx := context.Background()
	transactionID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	amountMoney, _ := valueobjects.NewMoney("100.00", currency)
	transaction, _ := entities.NewTransaction(walletID, uuid.New().String(), entities.TransactionTypeDeposit, amountMoney, "Test")
	_ = transaction.StartProcessing() // Already PROCESSING

	var savedTransaction *entities.Transaction

	transactionRepo := &mockTransactionRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
			return transaction, nil
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	walletRepo := &mockWalletRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transactionID.String(),
		Success:       true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	if savedTransaction.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected COMPLETED status, got %s", savedTransaction.Status())
	}
}
