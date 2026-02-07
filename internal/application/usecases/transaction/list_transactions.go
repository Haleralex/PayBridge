// Package transaction - ListTransactions use case для получения списка транзакций.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/google/uuid"
)

// ListTransactionsUseCase - use case для получения списка транзакций с фильтрацией.
type ListTransactionsUseCase struct {
	transactionRepo ports.TransactionRepository
}

// NewListTransactionsUseCase создаёт новый use case.
func NewListTransactionsUseCase(transactionRepo ports.TransactionRepository) *ListTransactionsUseCase {
	return &ListTransactionsUseCase{
		transactionRepo: transactionRepo,
	}
}

// Execute возвращает список транзакций с фильтрацией и пагинацией.
func (uc *ListTransactionsUseCase) Execute(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
	filter := ports.TransactionFilter{}

	if query.WalletID != nil {
		walletID, err := uuid.Parse(*query.WalletID)
		if err != nil {
			return nil, fmt.Errorf("invalid wallet_id: %w", err)
		}
		filter.WalletID = &walletID
	}

	if query.UserID != nil {
		userID, err := uuid.Parse(*query.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id: %w", err)
		}
		filter.UserID = &userID
	}

	if query.Type != nil {
		txType := entities.TransactionType(*query.Type)
		filter.Type = &txType
	}

	if query.Status != nil {
		txStatus := entities.TransactionStatus(*query.Status)
		filter.Status = &txStatus
	}

	transactions, err := uc.transactionRepo.List(ctx, filter, query.Offset, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	return &dtos.TransactionListDTO{
		Transactions: dtos.ToTransactionDTOList(transactions),
		TotalCount:   len(transactions),
		Offset:       query.Offset,
		Limit:        query.Limit,
	}, nil
}
