// Package user - GetUser use case для получения пользователя по ID.
package user

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/google/uuid"
)

// GetUserUseCase - use case для получения пользователя по ID.
type GetUserUseCase struct {
	userRepo ports.UserRepository
}

// NewGetUserUseCase создаёт новый use case.
func NewGetUserUseCase(userRepo ports.UserRepository) *GetUserUseCase {
	return &GetUserUseCase{
		userRepo: userRepo,
	}
}

// Execute возвращает пользователя по ID.
func (uc *GetUserUseCase) Execute(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error) {
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, errors.ValidationError{Field: "user_id", Message: "invalid UUID"}
	}

	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewDomainError("USER_NOT_FOUND", "user not found", err)
		}
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	result := dtos.ToUserDTO(user)
	return &result, nil
}
