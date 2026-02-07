// Package transaction - GetTransactionByIdempotencyKey use case.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
)

// GetTransactionByIdempotencyKeyUseCase - use case для поиска транзакции по ключу идемпотентности.
type GetTransactionByIdempotencyKeyUseCase struct {
	transactionRepo ports.TransactionRepository
}

// NewGetTransactionByIdempotencyKeyUseCase создаёт новый use case.
func NewGetTransactionByIdempotencyKeyUseCase(transactionRepo ports.TransactionRepository) *GetTransactionByIdempotencyKeyUseCase {
	return &GetTransactionByIdempotencyKeyUseCase{
		transactionRepo: transactionRepo,
	}
}

// Execute возвращает транзакцию по ключу идемпотентности.
func (uc *GetTransactionByIdempotencyKeyUseCase) Execute(ctx context.Context, query dtos.GetTransactionByIdempotencyKeyQuery) (*dtos.TransactionDTO, error) {
	if query.IdempotencyKey == "" {
		return nil, errors.ValidationError{Field: "idempotency_key", Message: "idempotency key is required"}
	}

	tx, err := uc.transactionRepo.FindByIdempotencyKey(ctx, query.IdempotencyKey)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewDomainError("TRANSACTION_NOT_FOUND", "transaction not found", err)
		}
		return nil, fmt.Errorf("failed to find transaction by idempotency key: %w", err)
	}

	result := dtos.ToTransactionDTO(tx)
	return &result, nil
}
