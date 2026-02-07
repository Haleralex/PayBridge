// Package transaction - GetTransaction use case для получения транзакции по ID.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/google/uuid"
)

// GetTransactionUseCase - use case для получения транзакции по ID.
type GetTransactionUseCase struct {
	transactionRepo ports.TransactionRepository
}

// NewGetTransactionUseCase создаёт новый use case.
func NewGetTransactionUseCase(transactionRepo ports.TransactionRepository) *GetTransactionUseCase {
	return &GetTransactionUseCase{
		transactionRepo: transactionRepo,
	}
}

// Execute возвращает транзакцию по ID.
func (uc *GetTransactionUseCase) Execute(ctx context.Context, query dtos.GetTransactionQuery) (*dtos.TransactionDTO, error) {
	txID, err := uuid.Parse(query.TransactionID)
	if err != nil {
		return nil, errors.ValidationError{Field: "transaction_id", Message: "invalid UUID"}
	}

	tx, err := uc.transactionRepo.FindByID(ctx, txID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewDomainError("TRANSACTION_NOT_FOUND", "transaction not found", err)
		}
		return nil, fmt.Errorf("failed to load transaction: %w", err)
	}

	result := dtos.ToTransactionDTO(tx)
	return &result, nil
}
