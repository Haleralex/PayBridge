// Package user содержит use cases для работы с пользователями.
//
// SOLID Principles:
// - SRP: Каждый use case отвечает за один сценарий
// - DIP: Зависит от интерфейсов (ports), не от конкретных реализаций
// - OCP: Новые use cases добавляются без изменения существующих
//
// Pattern: Use Case (Interactor)
// - Оркестрирует domain entities
// - Управляет транзакциями
// - Публикует события
package user

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
)

// CreateUserUseCase - use case для создания нового пользователя.
//
// Сценарий:
// 1. Проверить уникальность email
// 2. Создать domain entity User
// 3. Сохранить в БД
// 4. Опубликовать событие UserCreated
// 5. Вернуть DTO
//
// Транзакция: Весь use case выполняется в одной БД-транзакции
type CreateUserUseCase struct {
	userRepo       ports.UserRepository
	eventPublisher ports.EventPublisher
	uow            ports.UnitOfWork
}

// NewCreateUserUseCase создаёт новый use case.
// Dependency Injection через конструктор (DIP).
func NewCreateUserUseCase(
	userRepo ports.UserRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *CreateUserUseCase {
	return &CreateUserUseCase{
		userRepo:       userRepo,
		eventPublisher: eventPublisher,
		uow:            uow,
	}
}

// Execute выполняет use case.
//
// Parameters:
//   - ctx: Context для отмены и передачи транзакции
//   - cmd: Команда с параметрами
//
// Returns:
//   - *dtos.UserCreatedDTO: Результат операции
//   - error: Ошибка если что-то пошло не так
//
// Errors:
//   - ErrEntityAlreadyExists: Email уже используется
//   - ValidationError: Невалидные данные
//   - InfrastructureError: Проблемы с БД/сетью
func (uc *CreateUserUseCase) Execute(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error) {
	var result *dtos.UserCreatedDTO

	// Выполняем всё в транзакции
	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Проверка бизнес-правила: Email должен быть уникальным
		exists, err := uc.userRepo.ExistsByEmail(txCtx, cmd.Email)
		if err != nil {
			return fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if exists {
			return errors.NewBusinessRuleViolation(
				"EMAIL_ALREADY_EXISTS",
				fmt.Sprintf("user with email %s already exists", cmd.Email),
				map[string]interface{}{"email": cmd.Email},
			)
		}

		// 2. Создаём domain entity (валидация внутри)
		user, err := entities.NewUser(cmd.Email, cmd.FullName)
		if err != nil {
			return fmt.Errorf("failed to create user entity: %w", err)
		}

		// 3. Сохраняем в repository
		if err := uc.userRepo.Save(txCtx, user); err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		// 4. Поднимаем domain event
		event := events.NewUserCreated(user.ID(), user.Email(), user.FullName())

		// 5. Публикуем событие (в той же транзакции через Outbox pattern)
		if err := uc.eventPublisher.Publish(txCtx, event); err != nil {
			return fmt.Errorf("failed to publish UserCreated event: %w", err)
		}

		// 6. Конвертируем domain entity в DTO
		result = &dtos.UserCreatedDTO{
			User: dtos.UserDTO{
				ID:        user.ID().String(),
				Email:     user.Email(),
				FullName:  user.FullName(),
				KYCStatus: string(user.KYCStatus()),
				CreatedAt: user.CreatedAt(),
				UpdatedAt: user.UpdatedAt(),
			},
			Message: "User created successfully. Please complete KYC verification.",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
