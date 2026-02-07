// Package wallet - ListWallets use case для получения списка кошельков.
package wallet

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// ListWalletsUseCase - use case для получения списка кошельков с фильтрацией.
type ListWalletsUseCase struct {
	walletRepo ports.WalletRepository
}

// NewListWalletsUseCase создаёт новый use case.
func NewListWalletsUseCase(walletRepo ports.WalletRepository) *ListWalletsUseCase {
	return &ListWalletsUseCase{
		walletRepo: walletRepo,
	}
}

// Execute возвращает список кошельков с фильтрацией и пагинацией.
func (uc *ListWalletsUseCase) Execute(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error) {
	filter := ports.WalletFilter{}

	if query.UserID != nil {
		userID, err := uuid.Parse(*query.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id: %w", err)
		}
		filter.UserID = &userID
	}

	if query.CurrencyCode != nil {
		currency, err := valueobjects.NewCurrency(*query.CurrencyCode)
		if err != nil {
			return nil, fmt.Errorf("invalid currency_code: %w", err)
		}
		filter.Currency = &currency
	}

	if query.Status != nil {
		status := entities.WalletStatus(*query.Status)
		filter.Status = &status
	}

	wallets, err := uc.walletRepo.List(ctx, filter, query.Offset, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}

	return &dtos.WalletListDTO{
		Wallets:    dtos.ToWalletDTOList(wallets),
		TotalCount: len(wallets),
		Offset:     query.Offset,
		Limit:      query.Limit,
	}, nil
}
