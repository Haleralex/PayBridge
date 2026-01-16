package wallet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/application/ports"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// Mock TransactionRepository
type mockTransactionRepoForCredit struct {
	saveFunc                 func(ctx context.Context, tx *entities.Transaction) error
	findByIdempotencyKeyFunc func(ctx context.Context, key string) (*entities.Transaction, error)
}

func (m *mockTransactionRepoForCredit) Save(ctx context.Context, tx *entities.Transaction) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, tx)
	}
	return nil
}

func (m *mockTransactionRepoForCredit) FindByID(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockTransactionRepoForCredit) FindByIdempotencyKey(ctx context.Context, key string) (*entities.Transaction, error) {
	if m.findByIdempotencyKeyFunc != nil {
		return m.findByIdempotencyKeyFunc(ctx, key)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockTransactionRepoForCredit) FindByWalletID(ctx context.Context, walletID uuid.UUID, offset, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepoForCredit) FindPendingByWallet(ctx context.Context, walletID uuid.UUID) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepoForCredit) FindFailedRetryable(ctx context.Context, maxRetries int, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepoForCredit) List(ctx context.Context, filter ports.TransactionFilter, offset, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

type mockWalletRepoForCredit struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error)
	saveFunc     func(ctx context.Context, wallet *entities.Wallet) error
}

func (m *mockWalletRepoForCredit) Save(ctx context.Context, wallet *entities.Wallet) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, wallet)
	}
	return nil
}

func (m *mockWalletRepoForCredit) FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepoForCredit) FindByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepoForCredit) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Wallet, error) {
	return nil, nil
}

func (m *mockWalletRepoForCredit) ExistsByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error) {
	return false, nil
}

func (m *mockWalletRepoForCredit) List(ctx context.Context, filter ports.WalletFilter, offset, limit int) ([]*entities.Wallet, error) {
	return nil, nil
}

// Helper function to create a test wallet
func createTestWallet(walletID, userID uuid.UUID, currency valueobjects.Currency) *entities.Wallet {
	initialBalance, _ := valueobjects.NewMoney("0", currency)
	dailyLimit, _ := valueobjects.NewMoney("10000", currency)
	monthlyLimit, _ := valueobjects.NewMoney("100000", currency)
	return entities.ReconstructWallet(walletID, userID, currency, entities.WalletTypeFiat, entities.WalletStatusActive,
		initialBalance, initialBalance, 0, dailyLimit, monthlyLimit, time.Now(), time.Now())
}

// TestCreditWalletUseCase_Success тестирует успешное пополнение кошелька
func TestCreditWalletUseCase_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency)

	var savedWallet *entities.Wallet
	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepoForCredit{
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

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       walletID.String(),
		Amount:         "100.50",
		IdempotencyKey: idempotencyKey,
		Description:    "Test deposit",
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

	if savedWallet == nil {
		t.Fatal("Expected wallet to be saved")
	}

	if savedTransaction == nil {
		t.Fatal("Expected transaction to be saved")
	}

	// Проверяем статус транзакции
	if savedTransaction.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected transaction status = %s, got %s", entities.TransactionStatusCompleted, savedTransaction.Status())
	}

	// Проверяем события (3: TransactionCreated, WalletCredited, TransactionCompleted)
	if len(eventPublisher.publishedEvents) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(eventPublisher.publishedEvents))
	}
}

// TestCreditWalletUseCase_Idempotency тестирует идемпотентность
func TestCreditWalletUseCase_Idempotency(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")

	// Кошелёк уже пополнен
	creditedBalance, _ := valueobjects.NewMoney("100.50", currency)
	zeroBalance, _ := valueobjects.NewMoney("0", currency)
	dailyLimit, _ := valueobjects.NewMoney("10000", currency)
	monthlyLimit, _ := valueobjects.NewMoney("100000", currency)
	wallet := entities.ReconstructWallet(walletID, userID, currency, entities.WalletTypeFiat, entities.WalletStatusActive,
		creditedBalance, zeroBalance, 0, dailyLimit, monthlyLimit, time.Now(), time.Now())

	// Существующая транзакция
	amountMoney, _ := valueobjects.NewMoney("100.50", currency)
	existingTx, _ := entities.NewTransaction(walletID, idempotencyKey, entities.TransactionTypeDeposit, amountMoney, "Test deposit")

	walletRepo := &mockWalletRepoForCredit{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
		},
	}

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			if key == idempotencyKey {
				return existingTx, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       walletID.String(),
		Amount:         "100.50",
		IdempotencyKey: idempotencyKey,
		Description:    "Test deposit",
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

// TestCreditWalletUseCase_InvalidWalletUUID тестирует валидацию UUID
func TestCreditWalletUseCase_InvalidWalletUUID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	walletRepo := &mockWalletRepoForCredit{}
	transactionRepo := &mockTransactionRepoForCredit{}
	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       "invalid-uuid",
		Amount:         "100.50",
		IdempotencyKey: uuid.New().String(),
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if !domainErrors.IsValidationError(err) {
		t.Errorf("Expected ValidationError, got %T: %v", err, err)
	}
}

// TestCreditWalletUseCase_WalletNotFound тестирует ошибку "кошелёк не найден"
func TestCreditWalletUseCase_WalletNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	walletID := uuid.New()

	walletRepo := &mockWalletRepoForCredit{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       walletID.String(),
		Amount:         "100.50",
		IdempotencyKey: uuid.New().String(),
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if !domainErrors.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %T: %v", err, err)
	}
}

// TestCreditWalletUseCase_ClosedWallet тестирует попытку пополнения закрытого кошелька
func TestCreditWalletUseCase_ClosedWallet(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")

	zeroBalance, _ := valueobjects.NewMoney("0", currency)
	dailyLimit, _ := valueobjects.NewMoney("10000", currency)
	monthlyLimit, _ := valueobjects.NewMoney("100000", currency)
	wallet := entities.ReconstructWallet(walletID, userID, currency, entities.WalletTypeFiat, entities.WalletStatusClosed,
		zeroBalance, zeroBalance, 0, dailyLimit, monthlyLimit, time.Now(), time.Now())

	walletRepo := &mockWalletRepoForCredit{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
		},
	}

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       walletID.String(),
		Amount:         "100.50",
		IdempotencyKey: uuid.New().String(),
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestCreditWalletUseCase_ConcurrencyError тестирует optimistic locking
func TestCreditWalletUseCase_ConcurrencyError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency)

	walletRepo := &mockWalletRepoForCredit{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
		},
		saveFunc: func(ctx context.Context, w *entities.Wallet) error {
			return domainErrors.NewConcurrencyError("Wallet", walletID.String(), "wallet was modified")
		},
	}

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:       walletID.String(),
		Amount:         "100.50",
		IdempotencyKey: uuid.New().String(),
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected ConcurrencyError, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if !domainErrors.IsConcurrencyError(err) {
		t.Errorf("Expected ConcurrencyError, got %T: %v", err, err)
	}
}

// TestCreditWalletUseCase_ExternalReference тестирует установку external reference
func TestCreditWalletUseCase_ExternalReference(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency)

	var savedTransaction *entities.Transaction

	walletRepo := &mockWalletRepoForCredit{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
		},
	}

	transactionRepo := &mockTransactionRepoForCredit{
		findByIdempotencyKeyFunc: func(ctx context.Context, key string) (*entities.Transaction, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, tx *entities.Transaction) error {
			savedTransaction = tx
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreditWalletCommand{
		WalletID:          walletID.String(),
		Amount:            "100.50",
		IdempotencyKey:    uuid.New().String(),
		Description:       "Test",
		ExternalReference: "stripe_pi_123456",
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

	if savedTransaction.ExternalReference() != "stripe_pi_123456" {
		t.Errorf("Expected ExternalReference = stripe_pi_123456, got %s", savedTransaction.ExternalReference())
	}
}
