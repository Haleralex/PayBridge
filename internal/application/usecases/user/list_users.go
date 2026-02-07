// Package user - ListUsers use case для получения списка пользователей.
package user

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
)

// ListUsersUseCase - use case для получения списка пользователей с пагинацией.
type ListUsersUseCase struct {
	userRepo ports.UserRepository
}

// NewListUsersUseCase создаёт новый use case.
func NewListUsersUseCase(userRepo ports.UserRepository) *ListUsersUseCase {
	return &ListUsersUseCase{
		userRepo: userRepo,
	}
}

// Execute возвращает список пользователей с пагинацией.
func (uc *ListUsersUseCase) Execute(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error) {
	users, err := uc.userRepo.List(ctx, query.Offset, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return &dtos.UserListDTO{
		Users:      dtos.ToUserDTOList(users),
		TotalCount: len(users),
		Offset:     query.Offset,
		Limit:      query.Limit,
	}, nil
}
