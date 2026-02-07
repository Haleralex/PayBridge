// Package transaction - RetryTransaction use case для повтора failed транзакции.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/google/uuid"
)

const defaultMaxRetries = 3

// RetryTransactionUseCase - use case для повтора failed транзакции.
type RetryTransactionUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
}

// NewRetryTransactionUseCase создаёт новый use case.
func NewRetryTransactionUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *RetryTransactionUseCase {
	return &RetryTransactionUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет повтор failed транзакции.
func (uc *RetryTransactionUseCase) Execute(ctx context.Context, cmd dtos.RetryTransactionCommand) (*dtos.TransactionDTO, error) {
	var result *dtos.TransactionDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		txID, err := uuid.Parse(cmd.TransactionID)
		if err != nil {
			return errors.ValidationError{Field: "transaction_id", Message: "invalid UUID"}
		}

		transaction, err := uc.transactionRepo.FindByID(txCtx, txID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewDomainError("TRANSACTION_NOT_FOUND", "transaction not found", err)
			}
			return fmt.Errorf("failed to load transaction: %w", err)
		}

		// Domain entity проверяет бизнес-правила (статус FAILED, retryCount < maxRetries)
		if err := transaction.Retry(defaultMaxRetries); err != nil {
			return err
		}

		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		event := events.NewTransactionCreated(
			transaction.ID(),
			transaction.WalletID(),
			string(transaction.Type()),
			transaction.Amount(),
			transaction.IdempotencyKey(),
		)

		if err := uc.eventPublisher.Publish(txCtx, event); err != nil {
			return fmt.Errorf("failed to publish event: %w", err)
		}

		dto := dtos.ToTransactionDTO(transaction)
		result = &dto
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
