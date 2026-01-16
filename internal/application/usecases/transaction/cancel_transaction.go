// Package transaction - CancelTransaction use case для отмены транзакций.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/google/uuid"
)

// CancelTransactionUseCase - use case для отмены транзакции.
//
// Сценарий:
// 1. Загрузить транзакцию
// 2. Проверить что она в отменяемом статусе (PENDING/PROCESSING)
// 3. Отменить транзакцию
// 4. Если wallet был изменён - откатить изменения
// 5. Сохранить изменения
// 6. Опубликовать событие TransactionCancelled
//
// Бизнес-правила:
// - Можно отменить только PENDING или PROCESSING транзакции
// - COMPLETED/FAILED транзакции нельзя отменить (нужен REFUND)
// - Rollback изменений wallet
type CancelTransactionUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
}

// NewCancelTransactionUseCase создаёт новый use case.
func NewCancelTransactionUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *CancelTransactionUseCase {
	return &CancelTransactionUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет отмену транзакции.
func (uc *CancelTransactionUseCase) Execute(ctx context.Context, cmd dtos.CancelTransactionCommand) (*dtos.TransactionDTO, error) {
	var result *dtos.TransactionDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Парсим transaction ID
		transactionID, err := uuid.Parse(cmd.TransactionID)
		if err != nil {
			return errors.ValidationError{
				Field:   "transaction_id",
				Message: "invalid transaction ID format",
			}
		}

		// 2. Загружаем транзакцию
		transaction, err := uc.transactionRepo.FindByID(txCtx, transactionID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: transaction %s", errors.ErrEntityNotFound, cmd.TransactionID)
			}
			return fmt.Errorf("failed to load transaction: %w", err)
		}

		// 3. Проверяем что транзакцию можно отменить
		if transaction.Status() == entities.TransactionStatusCancelled {
			// Idempotent: уже отменена
			result = dtos.MapTransactionToDTO(transaction)
			return nil
		}

		if transaction.Status() == entities.TransactionStatusCompleted {
			return errors.NewBusinessRuleViolation(
				"CancelCompleted",
				"cannot cancel completed transaction, use refund instead",
				nil,
			)
		}

		if transaction.Status() == entities.TransactionStatusFailed {
			return errors.NewBusinessRuleViolation(
				"CancelFailed",
				"cannot cancel failed transaction",
				nil,
			)
		}

		// Сохраняем текущий статус для rollback логики
		wasProcessing := transaction.Status() == entities.TransactionStatusProcessing

		// 4. Отменяем транзакцию
		if err := transaction.Cancel(); err != nil {
			return fmt.Errorf("failed to cancel transaction: %w", err)
		}

		// 5. Rollback изменений wallet (если транзакция уже была applied)
		// Предполагаем что для PENDING транзакций wallet НЕ был изменён
		// Для PROCESSING - возможно был изменён, откатываем
		if wasProcessing {
			wallet, err := uc.walletRepo.FindByID(txCtx, transaction.WalletID())
			if err != nil {
				return fmt.Errorf("failed to load wallet: %w", err)
			}

			// Откатываем операцию
			switch transaction.Type() {
			case entities.TransactionTypeDeposit, entities.TransactionTypeRefund:
				// Было Credit - делаем Debit
				if err := wallet.Debit(transaction.Amount()); err != nil {
					return fmt.Errorf("failed to rollback credit: %w", err)
				}

			case entities.TransactionTypeWithdraw, entities.TransactionTypePayout:
				// Было Debit - делаем Credit
				if err := wallet.Credit(transaction.Amount()); err != nil {
					return fmt.Errorf("failed to rollback debit: %w", err)
				}

			case entities.TransactionTypeTransfer:
				// Для TRANSFER нужно откатить оба кошелька
				// TODO: реализовать rollback для transfer
				return errors.BusinessRuleViolation{
					Rule:    "CancelTransfer",
					Message: "canceling TRANSFER transactions not yet implemented",
				}
			}

			// Сохраняем wallet
			if err := uc.walletRepo.Save(txCtx, wallet); err != nil {
				return fmt.Errorf("failed to save wallet: %w", err)
			}
		}

		// 6. Сохраняем транзакцию
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		// 7. Публикуем события
		eventList := []events.DomainEvent{
			events.NewTransactionFailed(
				transaction.ID(),
				transaction.WalletID(),
				string(transaction.Type()),
				transaction.Amount(),
				"transaction cancelled by user",
				false, // not retryable
			),
		}

		if err := uc.eventPublisher.PublishBatch(txCtx, eventList); err != nil {
			return fmt.Errorf("failed to publish events: %w", err)
		}

		result = dtos.MapTransactionToDTO(transaction)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
