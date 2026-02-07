// Package wallet - GetWallet use case для получения кошелька по ID.
package wallet

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/google/uuid"
)

// GetWalletUseCase - use case для получения кошелька по ID.
type GetWalletUseCase struct {
	walletRepo ports.WalletRepository
}

// NewGetWalletUseCase создаёт новый use case.
func NewGetWalletUseCase(walletRepo ports.WalletRepository) *GetWalletUseCase {
	return &GetWalletUseCase{
		walletRepo: walletRepo,
	}
}

// Execute возвращает кошелёк по ID.
func (uc *GetWalletUseCase) Execute(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error) {
	walletID, err := uuid.Parse(query.WalletID)
	if err != nil {
		return nil, errors.ValidationError{Field: "wallet_id", Message: "invalid UUID"}
	}

	wallet, err := uc.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: wallet %s", errors.ErrEntityNotFound, query.WalletID)
		}
		return nil, fmt.Errorf("failed to load wallet: %w", err)
	}

	dto := dtos.ToWalletDTO(wallet)
	return &dto, nil
}
