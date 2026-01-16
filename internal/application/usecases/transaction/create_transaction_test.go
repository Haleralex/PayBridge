package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	domainErrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// Mock Repositories
type mockTransactionRepo struct {
	saveFunc                 func(ctx context.Context, tx *entities.Transaction) error
	findByIdempotencyKeyFunc func(ctx context.Context, key string) (*entities.Transaction, error)
	findByIDFunc             func(ctx context.Context, id uuid.UUID) (*entities.Transaction, error)
}

func (m *mockTransactionRepo) Save(ctx context.Context, tx *entities.Transaction) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, tx)
	}
	return nil
}

func (m *mockTransactionRepo) FindByID(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockTransactionRepo) FindByIdempotencyKey(ctx context.Context, key string) (*entities.Transaction, error) {
	if m.findByIdempotencyKeyFunc != nil {
		return m.findByIdempotencyKeyFunc(ctx, key)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockTransactionRepo) FindByWalletID(ctx context.Context, walletID uuid.UUID, offset, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) FindPendingByWallet(ctx context.Context, walletID uuid.UUID) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) FindFailedRetryable(ctx context.Context, maxRetries int, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) List(ctx context.Context, filter ports.TransactionFilter, offset, limit int) ([]*entities.Transaction, error) {
	return nil, nil
}

type mockWalletRepo struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error)
	saveFunc     func(ctx context.Context, wallet *entities.Wallet) error
}

func (m *mockWalletRepo) Save(ctx context.Context, wallet *entities.Wallet) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, wallet)
	}
	return nil
}

func (m *mockWalletRepo) FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepo) FindByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Wallet, error) {
	return nil, nil
}

func (m *mockWalletRepo) ExistsByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error) {
	return false, nil
}

func (m *mockWalletRepo) List(ctx context.Context, filter ports.WalletFilter, offset, limit int) ([]*entities.Wallet, error) {
	return nil, nil
}

// mockEventPublisher moved to test_helpers.go as EnhancedMockEventPublisher

// Simple mockEventPublisher for old tests (backward compatibility)
type mockEventPublisher struct {
	publishedEvents []events.DomainEvent
	publishFunc     func(ctx context.Context, event events.DomainEvent) error
}

func (m *mockEventPublisher) Publish(ctx context.Context, event events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, event)
	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}
	return nil
}

func (m *mockEventPublisher) PublishBatch(ctx context.Context, evts []events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, evts...)
	return nil
}

type mockUnitOfWork struct {
	executeFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockUnitOfWork) Execute(ctx context.Context, fn func(context.Context) error) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, fn)
	}
	return fn(ctx)
}

func (m *mockUnitOfWork) ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	result, err := fn(ctx)
	return result, err
}

// Helper function to create a test wallet
func createTestWallet(walletID, userID uuid.UUID, currency valueobjects.Currency) *entities.Wallet {
	initialBalance, _ := valueobjects.NewMoney("1000", currency)
	dailyLimit, _ := valueobjects.NewMoney("10000", currency)
	monthlyLimit, _ := valueobjects.NewMoney("100000", currency)
	return entities.ReconstructWallet(walletID, userID, currency, entities.WalletTypeFiat, entities.WalletStatusActive,
		initialBalance, initialBalance, 0, dailyLimit, monthlyLimit, time.Now(), time.Now())
}

// TestCreateTransactionUseCase_Deposit_Success тестирует успешное создание транзакции DEPOSIT
func TestCreateTransactionUseCase_Deposit_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency)

	var savedWallet *entities.Wallet
	var savedTransaction *entities.Transaction

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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: idempotencyKey,
		Type:           "DEPOSIT",
		Amount:         "100.50",
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

	// Проверяем тип транзакции
	if savedTransaction.Type() != entities.TransactionTypeDeposit {
		t.Errorf("Expected transaction type = %s, got %s", entities.TransactionTypeDeposit, savedTransaction.Type())
	}

	// Проверяем события (TransactionCreated, TransactionCompleted, WalletCredited)
	if len(eventPublisher.publishedEvents) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(eventPublisher.publishedEvents))
	}

	// Проверяем баланс кошелька увеличился
	expectedBalance, _ := valueobjects.NewMoney("1100.50", currency)
	if !savedWallet.AvailableBalance().Equals(expectedBalance) {
		t.Errorf("Expected balance = %s, got %s", expectedBalance.Amount(), savedWallet.AvailableBalance().Amount())
	}
}

// TestCreateTransactionUseCase_Withdraw_Success тестирует успешное списание
func TestCreateTransactionUseCase_Withdraw_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency) // 1000 USD initial

	var savedWallet *entities.Wallet
	var savedTransaction *entities.Transaction

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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: idempotencyKey,
		Type:           "WITHDRAW",
		Amount:         "50.25",
		Description:    "Test withdrawal",
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
	if savedTransaction.Type() != entities.TransactionTypeWithdraw {
		t.Errorf("Expected transaction type = %s, got %s", entities.TransactionTypeWithdraw, savedTransaction.Type())
	}

	// Проверяем баланс кошелька уменьшился
	expectedBalance, _ := valueobjects.NewMoney("949.75", currency)
	if !savedWallet.AvailableBalance().Equals(expectedBalance) {
		t.Errorf("Expected balance = %s, got %s", expectedBalance.Amount(), savedWallet.AvailableBalance().Amount())
	}
}

// TestCreateTransactionUseCase_Idempotency тестирует идемпотентность
func TestCreateTransactionUseCase_Idempotency(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency)

	// Существующая транзакция
	amountMoney, _ := valueobjects.NewMoney("100.50", currency)
	existingTx, _ := entities.NewTransaction(walletID, idempotencyKey, entities.TransactionTypeDeposit, amountMoney, "Test deposit")

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
			return wallet, nil
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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: idempotencyKey,
		Type:           "DEPOSIT",
		Amount:         "100.50",
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

// TestCreateTransactionUseCase_InsufficientBalance тестирует недостаток средств
func TestCreateTransactionUseCase_InsufficientBalance(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
	idempotencyKey := uuid.New().String()
	currency := valueobjects.MustNewCurrency("USD")
	wallet := createTestWallet(walletID, userID, currency) // 1000 USD

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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: idempotencyKey,
		Type:           "WITHDRAW",
		Amount:         "5000.00", // Больше чем есть на балансе
		Description:    "Test withdrawal",
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

// TestCreateTransactionUseCase_InvalidWalletID тестирует валидацию UUID
func TestCreateTransactionUseCase_InvalidWalletID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	walletRepo := &mockWalletRepo{}
	transactionRepo := &mockTransactionRepo{}
	eventPublisher := &mockEventPublisher{}
	uow := &mockUnitOfWork{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       "invalid-uuid",
		IdempotencyKey: uuid.New().String(),
		Type:           "DEPOSIT",
		Amount:         "100.50",
		Description:    "Test",
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

// TestCreateTransactionUseCase_WalletNotFound тестирует несуществующий кошелёк
func TestCreateTransactionUseCase_WalletNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	walletID := uuid.New()

	walletRepo := &mockWalletRepo{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: uuid.New().String(),
		Type:           "DEPOSIT",
		Amount:         "100.50",
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for wallet not found, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}
}

// TestCreateTransactionUseCase_InvalidAmount тестирует некорректную сумму
func TestCreateTransactionUseCase_InvalidAmount(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	walletID := uuid.New()
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

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	cmd := dtos.CreateTransactionCommand{
		WalletID:       walletID.String(),
		IdempotencyKey: uuid.New().String(),
		Type:           "DEPOSIT",
		Amount:         "invalid-amount",
		Description:    "Test",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error for invalid amount, got nil")
	}

	if result != nil {
		t.Errorf("Expected no result on error, got: %v", result)
	}

	// Проверяем что это ValidationError
	if !domainErrors.IsValidation(err) {
		t.Errorf("Expected ValidationError, got: %v", err)
	}
}
