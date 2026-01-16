// Package transaction - ProcessTransaction use case для обработки асинхронных транзакций.
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

// ProcessTransactionUseCase - use case для обработки pending транзакций.
//
// Сценарий:
// 1. Загрузить транзакцию по ID
// 2. Проверить что статус PENDING или PROCESSING
// 3. Выполнить внешний вызов (payment gateway, bank API, etc.)
// 4. В зависимости от результата: Complete или Fail
// 5. Применить изменения к wallet (если ещё не применены)
// 6. Сохранить изменения
// 7. Опубликовать события
//
// Бизнес-правила:
// - Можно обработать только PENDING/PROCESSING транзакции
// - Retry logic для failed external calls
// - Idempotent: повторная обработка completed транзакции - no-op
type ProcessTransactionUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
	// В реальной системе здесь будет PaymentGatewayClient
}

// NewProcessTransactionUseCase создаёт новый use case.
func NewProcessTransactionUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *ProcessTransactionUseCase {
	return &ProcessTransactionUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет обработку транзакции.
func (uc *ProcessTransactionUseCase) Execute(ctx context.Context, cmd dtos.ProcessTransactionCommand) (*dtos.TransactionDTO, error) {
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

		// 3. Проверяем что транзакция в обрабатываемом статусе
		if transaction.Status() == entities.TransactionStatusCompleted {
			// Idempotent: уже обработана
			result = dtos.MapTransactionToDTO(transaction)
			return nil
		}

		if transaction.Status() != entities.TransactionStatusPending &&
			transaction.Status() != entities.TransactionStatusProcessing {
			return errors.BusinessRuleViolation{
				Rule: "TransactionStatus",
				Message: fmt.Sprintf(
					"cannot process transaction in status: %s",
					transaction.Status(),
				),
			}
		}

		// 4. Помечаем как PROCESSING (если ещё не)
		if transaction.Status() == entities.TransactionStatusPending {
			if err := transaction.StartProcessing(); err != nil {
				return fmt.Errorf("failed to start processing: %w", err)
			}
		}

		// 5. Выполняем внешний вызов (мок для примера)
		// В реальной системе здесь будет вызов payment gateway:
		// success, err := uc.paymentGateway.Process(transaction)
		success := cmd.Success // Для примера берём из command

		if !success {
			// Обработка провалилась
			failureReason := cmd.FailureReason
			if failureReason == "" {
				failureReason = "external service error"
			}

			if err := transaction.MarkFailed(failureReason); err != nil {
				return fmt.Errorf("failed to mark transaction as failed: %w", err)
			}

			// Для failed транзакций нужен rollback изменений wallet
			// Если wallet уже был изменён при создании транзакции
			wallet, err := uc.walletRepo.FindByID(txCtx, transaction.WalletID())
			if err != nil {
				return fmt.Errorf("failed to load wallet for rollback: %w", err)
			}

			// Rollback: обратная операция
			switch transaction.Type() {
			case entities.TransactionTypeDeposit, entities.TransactionTypeRefund:
				// Было Credit - делаем Debit
				if err := wallet.Debit(transaction.Amount()); err != nil {
					// Если не можем откатить - это критическая ошибка
					return fmt.Errorf("CRITICAL: failed to rollback credit: %w", err)
				}

			case entities.TransactionTypeWithdraw, entities.TransactionTypePayout:
				// Было Debit - делаем Credit
				if err := wallet.Credit(transaction.Amount()); err != nil {
					return fmt.Errorf("CRITICAL: failed to rollback debit: %w", err)
				}
			}

			// Сохраняем wallet с rollback
			if err := uc.walletRepo.Save(txCtx, wallet); err != nil {
				return fmt.Errorf("failed to save wallet after rollback: %w", err)
			}

		} else {
			// Обработка успешна
			if err := transaction.MarkCompleted(); err != nil {
				return fmt.Errorf("failed to complete transaction: %w", err)
			}
		}

		// 6. Сохраняем транзакцию
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		// 7. Публикуем события
		var eventList []events.DomainEvent

		if transaction.IsCompleted() {
			eventList = []events.DomainEvent{
				events.NewTransactionCompleted(
					transaction.ID(),
					transaction.WalletID(),
					string(transaction.Type()),
					transaction.Amount(),
				),
			}
		} else if transaction.IsFailed() {
			eventList = []events.DomainEvent{
				events.NewTransactionFailed(
					transaction.ID(),
					transaction.WalletID(),
					string(transaction.Type()),
					transaction.Amount(),
					transaction.FailureReason(),
					false, // isRetryable - можно настроить по логике
				),
			}
		}

		if len(eventList) > 0 {
			if err := uc.eventPublisher.PublishBatch(txCtx, eventList); err != nil {
				return fmt.Errorf("failed to publish events: %w", err)
			}
		}

		result = dtos.MapTransactionToDTO(transaction)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
