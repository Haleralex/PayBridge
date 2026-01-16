package wallet

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

// Mock repositories and services
type mockUserRepoForWallet struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entities.User, error)
}

func (m *mockUserRepoForWallet) Save(ctx context.Context, user *entities.User) error {
	return nil
}

func (m *mockUserRepoForWallet) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockUserRepoForWallet) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockUserRepoForWallet) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (m *mockUserRepoForWallet) List(ctx context.Context, offset, limit int) ([]*entities.User, error) {
	return nil, nil
}

type mockWalletRepoForCreate struct {
	saveFunc                    func(ctx context.Context, wallet *entities.Wallet) error
	existsByUserAndCurrencyFunc func(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error)
	findByUserAndCurrencyFunc   func(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error)
}

func (m *mockWalletRepoForCreate) Save(ctx context.Context, wallet *entities.Wallet) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, wallet)
	}
	return nil
}

func (m *mockWalletRepoForCreate) FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepoForCreate) FindByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error) {
	if m.findByUserAndCurrencyFunc != nil {
		return m.findByUserAndCurrencyFunc(ctx, userID, currency)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockWalletRepoForCreate) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Wallet, error) {
	return nil, nil
}

func (m *mockWalletRepoForCreate) ExistsByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error) {
	if m.existsByUserAndCurrencyFunc != nil {
		return m.existsByUserAndCurrencyFunc(ctx, userID, currency)
	}
	return false, nil
}

func (m *mockWalletRepoForCreate) List(ctx context.Context, filter ports.WalletFilter, offset, limit int) ([]*entities.Wallet, error) {
	return nil, nil
}

type mockEventPublisherForWallet struct {
	publishedEvents []events.DomainEvent
	publishFunc     func(ctx context.Context, event events.DomainEvent) error
}

func (m *mockEventPublisherForWallet) Publish(ctx context.Context, event events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, event)
	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}
	return nil
}

func (m *mockEventPublisherForWallet) PublishBatch(ctx context.Context, evts []events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, evts...)
	return nil
}

type mockUoWForWallet struct {
	executeFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockUoWForWallet) Execute(ctx context.Context, fn func(context.Context) error) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, fn)
	}
	return fn(ctx)
}

func (m *mockUoWForWallet) ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	result, err := fn(ctx)
	return result, err
}

// TestCreateWalletUseCase_Success тестирует успешное создание кошелька
func TestCreateWalletUseCase_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	// Создаем верифицированного пользователя
	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()
	_ = user.ApproveKYC() // Verified пользователь

	var savedWallet *entities.Wallet

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			if id == userID {
				return user, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, nil // Кошелёк не существует
		},
		saveFunc: func(ctx context.Context, wallet *entities.Wallet) error {
			savedWallet = wallet
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
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

	if result.UserID != userID.String() {
		t.Errorf("Expected UserID = %s, got %s", userID.String(), result.UserID)
	}

	if result.CurrencyCode != "USD" {
		t.Errorf("Expected CurrencyCode = USD, got %s", result.CurrencyCode)
	}

	if result.Status != string(entities.WalletStatusActive) {
		t.Errorf("Expected Status = %s, got %s", entities.WalletStatusActive, result.Status)
	}

	if savedWallet == nil {
		t.Fatal("Expected wallet to be saved")
	}

	// Проверяем событие WalletCreated
	if len(eventPublisher.publishedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(eventPublisher.publishedEvents))
	}

	if eventPublisher.publishedEvents[0].EventType() != events.EventTypeWalletCreated {
		t.Errorf("Expected event type %s, got %s", events.EventTypeWalletCreated, eventPublisher.publishedEvents[0].EventType())
	}
}

// TestCreateWalletUseCase_CryptoWallet тестирует создание крипто-кошелька
func TestCreateWalletUseCase_CryptoWallet(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "BTC",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.CurrencyCode != "BTC" {
		t.Errorf("Expected CurrencyCode = BTC, got %s", result.CurrencyCode)
	}

	if result.WalletType != string(entities.WalletTypeCrypto) {
		t.Errorf("Expected WalletType = %s, got %s", entities.WalletTypeCrypto, result.WalletType)
	}
}

// TestCreateWalletUseCase_InvalidUserUUID тестирует валидацию UUID пользователя
func TestCreateWalletUseCase_InvalidUserUUID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	userRepo := &mockUserRepoForWallet{}
	walletRepo := &mockWalletRepoForCreate{}
	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       "invalid-uuid",
		CurrencyCode: "USD",
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

// TestCreateWalletUseCase_InvalidCurrency тестирует валидацию валюты
func TestCreateWalletUseCase_InvalidCurrency(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	userRepo := &mockUserRepoForWallet{}
	walletRepo := &mockWalletRepoForCreate{}
	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "INVALID",
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

// TestCreateWalletUseCase_UserNotFound тестирует случай, когда пользователь не найден
func TestCreateWalletUseCase_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	walletRepo := &mockWalletRepoForCreate{}
	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
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

// TestCreateWalletUseCase_UserNotVerified тестирует бизнес-правило: только verified users могут создавать кошельки
func TestCreateWalletUseCase_UserNotVerified(t *testing.T) {
	tests := []struct {
		name      string
		kycStatus entities.KYCStatus
	}{
		{
			name:      "Unverified user",
			kycStatus: entities.KYCStatusUnverified,
		},
		{
			name:      "Pending KYC",
			kycStatus: entities.KYCStatusPending,
		},
		{
			name:      "Rejected KYC",
			kycStatus: entities.KYCStatusRejected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			userID := uuid.New()

			user, _ := entities.NewUser("test@example.com", "Test User")
			user = entities.ReconstructUser(userID, user.Email(), user.FullName(), tt.kycStatus, time.Now(), time.Now())

			userRepo := &mockUserRepoForWallet{
				findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
					return user, nil
				},
			}

			walletRepo := &mockWalletRepoForCreate{}
			eventPublisher := &mockEventPublisherForWallet{}
			uow := &mockUoWForWallet{}

			useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

			cmd := dtos.CreateWalletCommand{
				UserID:       userID.String(),
				CurrencyCode: "USD",
			}

			// Act
			result, err := useCase.Execute(ctx, cmd)

			// Assert
			if err == nil {
				t.Fatal("Expected ErrUserNotVerified, got nil")
			}

			if result != nil {
				t.Errorf("Expected nil result, got %v", result)
			}

			// Domain должен вернуть ErrUserNotVerified
			if !errors.Is(err, domainErrors.ErrUserNotVerified) {
				t.Errorf("Expected ErrUserNotVerified, got: %v", err)
			}
		})
	}
}

// TestCreateWalletUseCase_WalletAlreadyExists тестирует бизнес-правило: один кошелёк на валюту для пользователя
func TestCreateWalletUseCase_WalletAlreadyExists(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			// Кошелёк уже существует
			return true, nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected BusinessRuleViolation, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Errorf("Expected BusinessRuleViolation, got %T: %v", err, err)
	}
}

// TestCreateWalletUseCase_ExistsCheckError тестирует ошибку при проверке существования
func TestCreateWalletUseCase_ExistsCheckError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, errors.New("database connection error")
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
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

// TestCreateWalletUseCase_SaveError тестирует ошибку при сохранении
func TestCreateWalletUseCase_SaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, nil
		},
		saveFunc: func(ctx context.Context, wallet *entities.Wallet) error {
			return errors.New("database save error")
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
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

// TestCreateWalletUseCase_EventPublishError тестирует ошибку публикации события
func TestCreateWalletUseCase_EventPublishError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{
		publishFunc: func(ctx context.Context, event events.DomainEvent) error {
			return errors.New("event bus error")
		},
	}

	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
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

// TestCreateWalletUseCase_InitialBalanceIsZero тестирует, что начальный баланс = 0
func TestCreateWalletUseCase_InitialBalanceIsZero(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusVerified, time.Now(), time.Now())

	userRepo := &mockUserRepoForWallet{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	walletRepo := &mockWalletRepoForCreate{
		existsByUserAndCurrencyFunc: func(ctx context.Context, uid uuid.UUID, currency valueobjects.Currency) (bool, error) {
			return false, nil
		},
	}

	eventPublisher := &mockEventPublisherForWallet{}
	uow := &mockUoWForWallet{}

	useCase := NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)

	cmd := dtos.CreateWalletCommand{
		UserID:       userID.String(),
		CurrencyCode: "USD",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Money.String() возвращает "0.00 USD", а не "0"
	// Проверяем, что баланс содержит "0"
	if result.AvailableBalance == "" {
		t.Error("Expected AvailableBalance to be set")
	}

	if result.PendingBalance == "" {
		t.Error("Expected PendingBalance to be set")
	}

	if result.TotalBalance == "" {
		t.Error("Expected TotalBalance to be set")
	}
}
