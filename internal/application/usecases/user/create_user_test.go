// Package user_test демонстрирует тестирование use cases с использованием mocks.
//
// Тестирование Application Layer:
// - Используем mocks для ports (repositories, event publisher)
// - Проверяем оркестрацию logic
// - Проверяем обработку ошибок
// - Проверяем публикацию событий
//
// Pattern: Test Doubles (Mocks, Stubs)
package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/application/usecases/user"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/events"
)

// ============================================
// Mock Implementations (Test Doubles)
// ============================================

// MockUserRepository - mock реализация UserRepository для тестов.
type MockUserRepository struct {
	SaveFunc          func(ctx context.Context, user *entities.User) error
	FindByIDFunc      func(ctx context.Context, id uuid.UUID) (*entities.User, error)
	FindByEmailFunc   func(ctx context.Context, email string) (*entities.User, error)
	ExistsByEmailFunc func(ctx context.Context, email string) (bool, error)
	ListFunc          func(ctx context.Context, offset, limit int) ([]*entities.User, error)
}

func (m *MockUserRepository) Save(ctx context.Context, user *entities.User) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, user)
	}
	return nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	if m.FindByEmailFunc != nil {
		return m.FindByEmailFunc(ctx, email)
	}
	return nil, domainErrors.ErrEntityNotFound
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if m.ExistsByEmailFunc != nil {
		return m.ExistsByEmailFunc(ctx, email)
	}
	return false, nil
}

func (m *MockUserRepository) List(ctx context.Context, offset, limit int) ([]*entities.User, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, offset, limit)
	}
	return nil, nil
}

// MockEventPublisher - mock для event publisher.
type MockEventPublisher struct {
	PublishFunc      func(ctx context.Context, event events.DomainEvent) error
	PublishBatchFunc func(ctx context.Context, events []events.DomainEvent) error
	PublishedEvents  []events.DomainEvent // Для проверки публикации
}

func (m *MockEventPublisher) Publish(ctx context.Context, event events.DomainEvent) error {
	m.PublishedEvents = append(m.PublishedEvents, event)
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, event)
	}
	return nil
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, events []events.DomainEvent) error {
	m.PublishedEvents = append(m.PublishedEvents, events...)
	if m.PublishBatchFunc != nil {
		return m.PublishBatchFunc(ctx, events)
	}
	return nil
}

// MockUnitOfWork - mock для unit of work.
type MockUnitOfWork struct {
	ExecuteFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *MockUnitOfWork) Execute(ctx context.Context, fn func(context.Context) error) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, fn)
	}
	// Default: просто выполняем функцию без реальной транзакции
	return fn(ctx)
}

func (m *MockUnitOfWork) ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	result, err := fn(ctx)
	return result, err
}

// ============================================
// Tests
// ============================================

// TestCreateUserUseCase_Success тестирует успешное создание пользователя.
func TestCreateUserUseCase_Success(t *testing.T) {
	// Arrange: Подготовка mocks
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil // Email не существует
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil // Успешное сохранение
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	// Создаём use case
	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	// Act: Выполняем use case
	result, err := useCase.Execute(context.Background(), cmd)

	// Assert: Проверяем результат
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Проверяем DTO
	if result.User.Email != cmd.Email {
		t.Errorf("Expected email %s, got %s", cmd.Email, result.User.Email)
	}

	if result.User.FullName != cmd.FullName {
		t.Errorf("Expected full name %s, got %s", cmd.FullName, result.User.FullName)
	}

	if result.User.KYCStatus != string(entities.KYCStatusUnverified) {
		t.Errorf("Expected KYC status UNVERIFIED, got %s", result.User.KYCStatus)
	}

	// Проверяем, что событие было опубликовано
	if len(eventPublisher.PublishedEvents) != 1 {
		t.Fatalf("Expected 1 event published, got %d", len(eventPublisher.PublishedEvents))
	}

	publishedEvent := eventPublisher.PublishedEvents[0]
	if publishedEvent.EventType() != events.EventTypeUserCreated {
		t.Errorf("Expected event type %s, got %s", events.EventTypeUserCreated, publishedEvent.EventType())
	}
}

// TestCreateUserUseCase_EmailAlreadyExists тестирует ошибку при дублировании email.
func TestCreateUserUseCase_EmailAlreadyExists(t *testing.T) {
	// Arrange: Email уже существует
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return true, nil // Email существует!
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "existing@example.com",
		FullName: "John Doe",
	}

	// Act
	result, err := useCase.Execute(context.Background(), cmd)

	// Assert: Ожидаем ошибку
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Проверяем тип ошибки
	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Errorf("Expected BusinessRuleViolation error, got %T", err)
	}

	// Проверяем, что событие НЕ было опубликовано
	if len(eventPublisher.PublishedEvents) != 0 {
		t.Errorf("Expected 0 events published, got %d", len(eventPublisher.PublishedEvents))
	}
}

// TestCreateUserUseCase_RepositoryError тестирует обработку ошибок repository.
func TestCreateUserUseCase_RepositoryError(t *testing.T) {
	// Arrange: Repository возвращает ошибку
	expectedError := errors.New("database connection failed")

	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, expectedError // Ошибка БД
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	// Act
	result, err := useCase.Execute(context.Background(), cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Проверяем, что событие НЕ было опубликовано
	if len(eventPublisher.PublishedEvents) != 0 {
		t.Errorf("Expected 0 events published, got %d", len(eventPublisher.PublishedEvents))
	}
}

// TestCreateUserUseCase_InvalidEmail тестирует валидацию email.
func TestCreateUserUseCase_InvalidEmail(t *testing.T) {
	// Arrange
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	// Invalid email
	cmd := dtos.CreateUserCommand{
		Email:    "invalid-email", // Нет @
		FullName: "John Doe",
	}

	// Act
	result, err := useCase.Execute(context.Background(), cmd)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Domain entity должна отклонить invalid email
	if !errors.Is(err, domainErrors.ErrInvalidEmail) {
		t.Errorf("Expected ErrInvalidEmail, got %v", err)
	}
}

// TestCreateUserUseCase_TransactionRollback тестирует rollback при ошибке.
func TestCreateUserUseCase_TransactionRollback(t *testing.T) {
	// Arrange: Save успешен, но event publish fails
	saveWasCalled := false

	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			saveWasCalled = true
			return nil
		},
	}

	eventPublishError := errors.New("kafka unavailable")
	eventPublisher := &MockEventPublisher{
		PublishFunc: func(ctx context.Context, event events.DomainEvent) error {
			return eventPublishError // Publish fails!
		},
	}

	uow := &MockUnitOfWork{
		// Симулируем rollback при ошибке
		ExecuteFunc: func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			if err != nil {
				// В реальной БД здесь был бы ROLLBACK
				return err
			}
			return nil
		},
	}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	// Act
	result, err := useCase.Execute(context.Background(), cmd)

	// Assert: Use case должен вернуть ошибку
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Save был вызван, но транзакция должна быть отменена
	if !saveWasCalled {
		t.Error("Expected Save to be called")
	}

	// В реальной системе изменения были бы откачены
	// Здесь мы просто проверяем, что ошибка распространилась
}

// BenchmarkCreateUserUseCase_Success - бенчмарк для измерения производительности.
func BenchmarkCreateUserUseCase_Success(b *testing.B) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = useCase.Execute(context.Background(), cmd)
	}
}

// TestCreateUserUseCase_SaveError tests error when saving user fails.
func TestCreateUserUseCase_SaveError(t *testing.T) {
	saveError := errors.New("failed to insert into database")

	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return saveError
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Event should not be published
	if len(eventPublisher.PublishedEvents) != 0 {
		t.Errorf("Expected 0 events, got %d", len(eventPublisher.PublishedEvents))
	}
}

// TestCreateUserUseCase_EmptyFullName tests validation of full name.
func TestCreateUserUseCase_EmptyFullName(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "", // Empty name
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestCreateUserUseCase_EmailNormalization tests email is normalized.
func TestCreateUserUseCase_EmailNormalization(t *testing.T) {
	var savedEmail string

	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			savedEmail = user.Email()
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "Test@EXAMPLE.COM", // Mixed case
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.User.Email != "test@example.com" {
		t.Errorf("Expected normalized email test@example.com, got %s", result.User.Email)
	}

	if savedEmail != "test@example.com" {
		t.Errorf("Expected saved email test@example.com, got %s", savedEmail)
	}
}

// TestCreateUserUseCase_ResultContainsCorrectMessage tests the success message.
func TestCreateUserUseCase_ResultContainsCorrectMessage(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Message == "" {
		t.Error("Expected non-empty message")
	}

	expectedMsg := "User created successfully. Please complete KYC verification."
	if result.Message != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, result.Message)
	}
}

// TestCreateUserUseCase_UserIDIsGenerated tests that user gets a unique ID.
func TestCreateUserUseCase_UserIDIsGenerated(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.User.ID == "" {
		t.Error("Expected user ID to be generated")
	}

	// ID should be valid UUID
	_, err = uuid.Parse(result.User.ID)
	if err != nil {
		t.Errorf("Expected valid UUID, got %s: %v", result.User.ID, err)
	}
}

// TestCreateUserUseCase_TimestampsAreSet tests CreatedAt and UpdatedAt are set.
func TestCreateUserUseCase_TimestampsAreSet(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.User.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if result.User.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

// TestCreateUserUseCase_PublishedEventContainsCorrectData tests event payload.
func TestCreateUserUseCase_PublishedEventContainsCorrectData(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		SaveFunc: func(ctx context.Context, user *entities.User) error {
			return nil
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	result, err := useCase.Execute(context.Background(), cmd)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(eventPublisher.PublishedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(eventPublisher.PublishedEvents))
	}

	event := eventPublisher.PublishedEvents[0]

	// Check event type
	if event.EventType() != events.EventTypeUserCreated {
		t.Errorf("Expected event type %s, got %s", events.EventTypeUserCreated, event.EventType())
	}

	// Check aggregate ID matches user ID
	expectedID := uuid.MustParse(result.User.ID)
	if event.AggregateID() != expectedID {
		t.Errorf("Expected aggregate ID %s, got %s", expectedID, event.AggregateID())
	}

	// Event should have occurred at timestamp
	if event.OccurredAt().IsZero() {
		t.Error("Expected event OccurredAt to be set")
	}
}

// TestCreateUserUseCase_ContextCancellation tests context cancellation handling.
func TestCreateUserUseCase_ContextCancellation(t *testing.T) {
	userRepo := &MockUserRepository{
		ExistsByEmailFunc: func(ctx context.Context, email string) (bool, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			default:
				return false, nil
			}
		},
	}

	eventPublisher := &MockEventPublisher{}
	uow := &MockUnitOfWork{}

	useCase := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)

	cmd := dtos.CreateUserCommand{
		Email:    "test@example.com",
		FullName: "John Doe",
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := useCase.Execute(ctx, cmd)

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}
