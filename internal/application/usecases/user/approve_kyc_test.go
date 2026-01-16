package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	domainErrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/google/uuid"
)

// Mock UserRepository для тестирования
type mockUserRepoForKYC struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entities.User, error)
	saveFunc     func(ctx context.Context, user *entities.User) error
}

func (m *mockUserRepoForKYC) Save(ctx context.Context, user *entities.User) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, user)
	}
	return nil
}

func (m *mockUserRepoForKYC) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockUserRepoForKYC) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	return nil, domainErrors.ErrEntityNotFound
}

func (m *mockUserRepoForKYC) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (m *mockUserRepoForKYC) List(ctx context.Context, offset, limit int) ([]*entities.User, error) {
	return nil, nil
}

// Mock EventPublisher для тестирования
type mockEventPublisherForKYC struct {
	publishedEvents []events.DomainEvent
	publishFunc     func(ctx context.Context, event events.DomainEvent) error
}

func (m *mockEventPublisherForKYC) Publish(ctx context.Context, event events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, event)
	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}
	return nil
}

func (m *mockEventPublisherForKYC) PublishBatch(ctx context.Context, events []events.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, events...)
	return nil
}

// Mock UnitOfWork для тестирования
type mockUoWForKYC struct {
	executeFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockUoWForKYC) Execute(ctx context.Context, fn func(context.Context) error) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, fn)
	}
	// По умолчанию просто выполняем функцию
	return fn(ctx)
}

func (m *mockUoWForKYC) ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	result, err := fn(ctx)
	return result, err
}

// TestApproveKYCUseCase_Success_Approved тестирует успешное одобрение KYC
func TestApproveKYCUseCase_Success_Approved(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	// Создаем пользователя в статусе PENDING
	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification() // Переводим в PENDING

	var savedUser *entities.User
	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			if id == userID {
				return user, nil
			}
			return nil, domainErrors.ErrEntityNotFound
		},
		saveFunc: func(ctx context.Context, u *entities.User) error {
			savedUser = u
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

	if result.KYCStatus != string(entities.KYCStatusVerified) {
		t.Errorf("Expected KYCStatus = %s, got %s", entities.KYCStatusVerified, result.KYCStatus)
	}

	if savedUser == nil {
		t.Fatal("Expected user to be saved")
	}

	if savedUser.KYCStatus() != entities.KYCStatusVerified {
		t.Errorf("Expected saved user KYCStatus = %s, got %s", entities.KYCStatusVerified, savedUser.KYCStatus())
	}

	// Проверяем событие UserKYCApproved
	if len(eventPublisher.publishedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(eventPublisher.publishedEvents))
	}

	if eventPublisher.publishedEvents[0].EventType() != events.EventTypeUserKYCApproved {
		t.Errorf("Expected event type %s, got %s", events.EventTypeUserKYCApproved, eventPublisher.publishedEvents[0].EventType())
	}
}

// TestApproveKYCUseCase_Success_Rejected тестирует успешное отклонение KYC
func TestApproveKYCUseCase_Success_Rejected(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()

	var savedUser *entities.User
	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
		saveFunc: func(ctx context.Context, u *entities.User) error {
			savedUser = u
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: false,
		Reason:   "Documents expired",
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.KYCStatus != string(entities.KYCStatusRejected) {
		t.Errorf("Expected KYCStatus = %s, got %s", entities.KYCStatusRejected, result.KYCStatus)
	}

	if savedUser.KYCStatus() != entities.KYCStatusRejected {
		t.Errorf("Expected saved user KYCStatus = %s, got %s", entities.KYCStatusRejected, savedUser.KYCStatus())
	}

	// Проверяем событие UserKYCRejected
	if len(eventPublisher.publishedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(eventPublisher.publishedEvents))
	}

	if eventPublisher.publishedEvents[0].EventType() != events.EventTypeUserKYCRejected {
		t.Errorf("Expected event type %s, got %s", events.EventTypeUserKYCRejected, eventPublisher.publishedEvents[0].EventType())
	}
}

// TestApproveKYCUseCase_InvalidUUID тестирует ошибку валидации UUID
func TestApproveKYCUseCase_InvalidUUID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := &mockUserRepoForKYC{}
	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   "invalid-uuid",
		Verified: true,
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

	// Проверяем тип ошибки
	if !domainErrors.IsValidationError(err) {
		t.Errorf("Expected ValidationError, got %T: %v", err, err)
	}
}

// TestApproveKYCUseCase_UserNotFound тестирует случай, когда пользователь не найден
func TestApproveKYCUseCase_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return nil, domainErrors.ErrEntityNotFound
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

	// Проверяем, что это ошибка "not found"
	if !domainErrors.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

// TestApproveKYCUseCase_UserNotInPendingStatus тестирует бизнес-правило: KYC можно одобрить только в статусе PENDING
func TestApproveKYCUseCase_UserNotInPendingStatus(t *testing.T) {
	tests := []struct {
		name      string
		kycStatus entities.KYCStatus
	}{
		{
			name:      "Unverified status",
			kycStatus: entities.KYCStatusUnverified,
		},
		{
			name:      "Already verified",
			kycStatus: entities.KYCStatusVerified,
		},
		{
			name:      "Already rejected",
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

			userRepo := &mockUserRepoForKYC{
				findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
					return user, nil
				},
			}

			eventPublisher := &mockEventPublisherForKYC{}
			uow := &mockUoWForKYC{}

			useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

			cmd := dtos.ApproveKYCCommand{
				UserID:   userID.String(),
				Verified: true,
			}

			// Act
			result, err := useCase.Execute(ctx, cmd)

			// Assert
			if err == nil {
				t.Fatal("Expected BusinessRuleViolation error, got nil")
			}

			if result != nil {
				t.Errorf("Expected nil result, got %v", result)
			}

			// Domain должен вернуть ошибку бизнес-правила
			if !domainErrors.IsBusinessRuleViolation(err) {
				t.Errorf("Expected BusinessRuleViolation, got %T: %v", err, err)
			}
		})
	}
}

// TestApproveKYCUseCase_RepositoryError тестирует ошибку репозитория при поиске
func TestApproveKYCUseCase_RepositoryError_FindByID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return nil, errors.New("database connection error")
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

// TestApproveKYCUseCase_SaveError тестирует ошибку при сохранении
func TestApproveKYCUseCase_SaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
		saveFunc: func(ctx context.Context, u *entities.User) error {
			return errors.New("database save error")
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

// TestApproveKYCUseCase_EventPublishError тестирует ошибку публикации события
func TestApproveKYCUseCase_EventPublishError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
		saveFunc: func(ctx context.Context, u *entities.User) error {
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForKYC{
		publishFunc: func(ctx context.Context, event events.DomainEvent) error {
			return errors.New("event bus connection error")
		},
	}

	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

// TestApproveKYCUseCase_TransactionRollback тестирует rollback при ошибке в UoW
func TestApproveKYCUseCase_TransactionRollback(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()

	user, _ := entities.NewUser("test@example.com", "Test User")
	user = entities.ReconstructUser(userID, user.Email(), user.FullName(), entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}

	// UoW возвращает ошибку - симулируем rollback
	uow := &mockUoWForKYC{
		executeFunc: func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			if err != nil {
				// В реальной реализации здесь был бы tx.Rollback()
				return err
			}
			// Симулируем commit error
			return errors.New("transaction commit failed")
		},
	}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
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

// TestApproveKYCUseCase_ResultContainsCorrectData тестирует корректность данных в результате
func TestApproveKYCUseCase_ResultContainsCorrectData(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := uuid.New()
	email := "test@example.com"
	fullName := "Test User"

	user, _ := entities.NewUser(email, fullName)
	user = entities.ReconstructUser(userID, email, fullName, entities.KYCStatusUnverified, time.Now(), time.Now())
	_ = user.StartKYCVerification()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return user, nil
		},
		saveFunc: func(ctx context.Context, u *entities.User) error {
			return nil
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ID != userID.String() {
		t.Errorf("Expected ID = %s, got %s", userID.String(), result.ID)
	}

	if result.Email != email {
		t.Errorf("Expected Email = %s, got %s", email, result.Email)
	}

	if result.FullName != fullName {
		t.Errorf("Expected FullName = %s, got %s", fullName, result.FullName)
	}

	if result.KYCStatus != string(entities.KYCStatusVerified) {
		t.Errorf("Expected KYCStatus = %s, got %s", entities.KYCStatusVerified, result.KYCStatus)
	}
}

// TestApproveKYCUseCase_ContextCancellation тестирует отмену контекста
func TestApproveKYCUseCase_ContextCancellation(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем контекст немедленно

	userID := uuid.New()

	userRepo := &mockUserRepoForKYC{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			// Проверяем отмену контекста
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return nil, domainErrors.ErrEntityNotFound
			}
		},
	}

	eventPublisher := &mockEventPublisherForKYC{}
	uow := &mockUoWForKYC{}

	useCase := NewApproveKYCUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.ApproveKYCCommand{
		UserID:   userID.String(),
		Verified: true,
	}

	// Act
	result, err := useCase.Execute(ctx, cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}
