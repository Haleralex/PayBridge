// Package user - ApproveKYC use case для одобрения KYC верификации.
package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/application/ports"
	"github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/events"
)

// ApproveKYCUseCase - use case для одобрения/отклонения KYC.
//
// Сценарий:
// 1. Загрузить пользователя
// 2. Вызвать domain метод ApproveKYC() или RejectKYC()
// 3. Сохранить изменения
// 4. Опубликовать событие
//
// Бизнес-правила (из domain):
// - KYC можно одобрить только в статусе PENDING
// - После одобрения пользователь может создавать кошельки
type ApproveKYCUseCase struct {
	userRepo       ports.UserRepository
	eventPublisher ports.EventPublisher
	uow            ports.UnitOfWork
}

// NewApproveKYCUseCase создаёт новый use case.
func NewApproveKYCUseCase(
	userRepo ports.UserRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *ApproveKYCUseCase {
	return &ApproveKYCUseCase{
		userRepo:       userRepo,
		eventPublisher: eventPublisher,
		uow:            uow,
	}
}

// Execute выполняет одобрение или отклонение KYC.
func (uc *ApproveKYCUseCase) Execute(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error) {
	var result *dtos.UserDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Парсим UUID
		userID, err := uuid.Parse(cmd.UserID)
		if err != nil {
			return errors.ValidationError{
				Field:   "user_id",
				Message: "invalid UUID format",
			}
		}

		// 2. Загружаем пользователя
		user, err := uc.userRepo.FindByID(txCtx, userID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewDomainError("USER_NOT_FOUND", "user not found", err)
			}
			return fmt.Errorf("failed to load user: %w", err)
		}

		// 3. Применяем бизнес-логику через domain entity
		var event events.DomainEvent
		if cmd.Verified {
			// Одобрить KYC
			if err := user.ApproveKYC(); err != nil {
				return err // Domain вернёт BusinessRuleViolation если статус не PENDING
			}
			event = events.NewUserKYCApproved(user.ID())
		} else {
			// Отклонить KYC
			if err := user.RejectKYC(); err != nil {
				return err
			}
			event = events.NewUserKYCRejected(user.ID(), cmd.Reason)
		}

		// 4. Сохраняем изменения
		if err := uc.userRepo.Save(txCtx, user); err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		// 5. Публикуем событие
		if err := uc.eventPublisher.Publish(txCtx, event); err != nil {
			return fmt.Errorf("failed to publish KYC event: %w", err)
		}

		// 6. Формируем DTO
		result = &dtos.UserDTO{
			ID:        user.ID().String(),
			Email:     user.Email(),
			FullName:  user.FullName(),
			KYCStatus: string(user.KYCStatus()),
			CreatedAt: user.CreatedAt(),
			UpdatedAt: user.UpdatedAt(),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
