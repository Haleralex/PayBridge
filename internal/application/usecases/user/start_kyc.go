// Package user - StartKYC use case для запуска KYC верификации.
package user

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/google/uuid"
)

// StartKYCUseCase - use case для запуска процесса KYC верификации.
type StartKYCUseCase struct {
	userRepo       ports.UserRepository
	eventPublisher ports.EventPublisher
	uow            ports.UnitOfWork
}

// NewStartKYCUseCase создаёт новый use case.
func NewStartKYCUseCase(
	userRepo ports.UserRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *StartKYCUseCase {
	return &StartKYCUseCase{
		userRepo:       userRepo,
		eventPublisher: eventPublisher,
		uow:            uow,
	}
}

// Execute запускает процесс KYC верификации для пользователя.
func (uc *StartKYCUseCase) Execute(ctx context.Context, cmd dtos.StartKYCVerificationCommand) (*dtos.UserDTO, error) {
	var result *dtos.UserDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		userID, err := uuid.Parse(cmd.UserID)
		if err != nil {
			return errors.ValidationError{Field: "user_id", Message: "invalid UUID"}
		}

		user, err := uc.userRepo.FindByID(txCtx, userID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewDomainError("USER_NOT_FOUND", "user not found", err)
			}
			return fmt.Errorf("failed to load user: %w", err)
		}

		if err := user.StartKYCVerification(); err != nil {
			return err
		}

		if err := uc.userRepo.Save(txCtx, user); err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		dto := dtos.ToUserDTO(user)
		result = &dto
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
