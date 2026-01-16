package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// TestTransferBetweenWalletsUseCase_Success тестирует успешный трансфер между кошельками
func TestTransferBetweenWalletsUseCase_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	sourceWalletID := uuid.New()
	destinationWalletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")

	sourceWallet := createTestWallet(sourceWalletID, userID, currency)               // 1000 USD
	destinationWallet := createTestWallet(destinationWalletID, uuid.New(), currency) // 1000 USD

	var savedSourceWallet, savedDestinationWallet *entities.Wallet
	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == sourceWalletID {
				return sourceWallet, nil
			}
			if id == destinationWalletID {
				return destinationWallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, w *entities.Wallet) error {
			if w.ID() == sourceWalletID {
				savedSourceWallet = w
			} else if w.ID() == destinationWalletID {
				savedDestinationWallet = w
			}
			return nil
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWalletID.String(),
		DestinationWalletID: destinationWalletID.String(),
		Amount:              "250.00",
		IdempotencyKey:      idempotencyKey,
		Description:         "Test transfer",
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

	// Проверяем тип транзакции
	if savedTransaction.Type() != entities.TransactionTypeTransfer {
		t.Errorf("Expected transaction type = %s, got %s", entities.TransactionTypeTransfer, savedTransaction.Type())
	}

	// Проверяем статус
	if savedTransaction.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected transaction status = %s, got %s", entities.TransactionStatusCompleted, savedTransaction.Status())
	}

	// Проверяем баланс source wallet уменьшился
	expectedSourceBalance, _ := valueobjects.NewMoney("750.00", currency)
	if !savedSourceWallet.AvailableBalance().Equals(expectedSourceBalance) {
		t.Errorf("Expected source balance = %s, got %s", expectedSourceBalance.Amount(), savedSourceWallet.AvailableBalance().Amount())
	}

	// Проверяем баланс destination wallet увеличился
	expectedDestBalance, _ := valueobjects.NewMoney("1250.00", currency)
	if !savedDestinationWallet.AvailableBalance().Equals(expectedDestBalance) {
		t.Errorf("Expected destination balance = %s, got %s", expectedDestBalance.Amount(), savedDestinationWallet.AvailableBalance().Amount())
	}

	// Проверяем события (TransactionCreated, WalletDebited, WalletCredited, TransactionCompleted)
	if len(eventPublisher.publishedEvents) < 4 {
		t.Errorf("Expected at least 4 events, got %d", len(eventPublisher.publishedEvents))
	}
}

// TestTransferBetweenWalletsUseCase_CurrencyMismatch тестирует несовпадение валют
func TestTransferBetweenWalletsUseCase_CurrencyMismatch(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	sourceWalletID := uuid.New()
	destinationWalletID := uuid.New()
	idempotencyKey := uuid.New().String()

	sourceWallet := createTestWallet(sourceWalletID, userID, valueobjects.MustNewCurrency("USD"))
	destinationWallet := createTestWallet(destinationWalletID, uuid.New(), valueobjects.MustNewCurrency("EUR")) // Другая валюта!

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == sourceWalletID {
				return sourceWallet, nil
			}
			if id == destinationWalletID {
				return destinationWallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWalletID.String(),
		DestinationWalletID: destinationWalletID.String(),
		Amount:              "100.00",
		IdempotencyKey:      idempotencyKey,
		Description:         "Test transfer",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for currency mismatch, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это BusinessRuleViolation
	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Errorf("Expected BusinessRuleViolation error, got: %v", err)
	}
}

// TestTransferBetweenWalletsUseCase_InsufficientBalance тестирует недостаток средств
func TestTransferBetweenWalletsUseCase_InsufficientBalance(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	sourceWalletID := uuid.New()
	destinationWalletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")

	sourceWallet := createTestWallet(sourceWalletID, userID, currency) // 1000 USD
	destinationWallet := createTestWallet(destinationWalletID, uuid.New(), currency)

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == sourceWalletID {
				return sourceWallet, nil
			}
			if id == destinationWalletID {
				return destinationWallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWalletID.String(),
		DestinationWalletID: destinationWalletID.String(),
		Amount:              "5000.00", // Больше чем есть
		IdempotencyKey:      idempotencyKey,
		Description:         "Test transfer",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for insufficient balance, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это ошибка insufficient balance
	if !errors.Is(err, domainErrors.ErrInsufficientBalance) {
		t.Errorf("Expected ErrInsufficientBalance, got: %v", err)
	}
}

// TestTransferBetweenWalletsUseCase_SameWallet тестирует трансфер на тот же кошелёк
func TestTransferBetweenWalletsUseCase_SameWallet(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")

	wallet := createTestWallet(walletID, userID, currency)

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == walletID {
				return wallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      walletID.String(),
		DestinationWalletID: walletID.String(), // Тот же кошелёк!
		Amount:              "100.00",
		IdempotencyKey:      idempotencyKey,
		Description:         "Test transfer",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for same wallet transfer, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это BusinessRuleViolation (self-transfer)
	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Errorf("Expected BusinessRuleViolation for self-transfer, got: %v", err)
	}
}

// TestTransferBetweenWalletsUseCase_Idempotency тестирует идемпотентность
func TestTransferBetweenWalletsUseCase_Idempotency(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	sourceWalletID := uuid.New()
	destinationWalletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")

	sourceWallet := createTestWallet(sourceWalletID, userID, currency)
	destinationWallet := createTestWallet(destinationWalletID, uuid.New(), currency)

	// Существующая транзакция
	amountMoney, _ := valueobjects.NewMoney("250.00", currency)
	existingTx, _ := entities.NewTransaction(sourceWalletID, idempotencyKey, entities.TransactionTypeTransfer, amountMoney, "Test transfer")

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			if id == sourceWalletID {
				return sourceWallet, nil
			}
			if id == destinationWalletID {
				return destinationWallet, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepo{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			if key == idempotencyKey {
				return existingTx, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWalletID.String(),
		DestinationWalletID: destinationWalletID.String(),
		Amount:              "250.00",
		IdempotencyKey:      idempotencyKey,
		Description:         "Test transfer",
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

	// Идемпотентность: не должны публиковаться новые события
	if len(eventPublisher.publishedEvents) != 0 {
		t.Errorf("Expected no new events (idempotent), got %d", len(eventPublisher.publishedEvents))
	}
}
