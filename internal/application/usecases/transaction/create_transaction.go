// Package transaction содержит use cases для работы с транзакциями.
package transaction

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/application/ports"
	"github.com/yourusername/wallethub/internal/domain/entities"
	"github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/events"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// CreateTransactionUseCase - use case для создания транзакции.
//
// Сценарий:
// 1. Проверить idempotency_key (защита от дубликатов)
// 2. Загрузить кошелёк и проверить его
// 3. Создать Transaction entity
// 4. Применить операцию к кошельку (Credit/Debit в зависимости от типа)
// 5. Сохранить транзакцию и кошелёк
// 6. Опубликовать события
//
// Бизнес-правила:
// - Idempotency: повторный запрос с тем же ключом возвращает существующую транзакцию
// - Кошелёк должен существовать и быть активным
// - Для WITHDRAW/PAYOUT достаточно средств
// - Для DEPOSIT/REFUND лимиты не превышены
type CreateTransactionUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
}

// NewCreateTransactionUseCase создаёт новый use case.
func NewCreateTransactionUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *CreateTransactionUseCase {
	return &CreateTransactionUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет создание транзакции.
func (uc *CreateTransactionUseCase) Execute(ctx context.Context, cmd dtos.CreateTransactionCommand) (*dtos.TransactionDTO, error) {
	var result *dtos.TransactionDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Проверка idempotency
		if cmd.IdempotencyKey != "" {
			existingTx, err := uc.transactionRepo.FindByIdempotencyKey(txCtx, cmd.IdempotencyKey)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to check idempotency: %w", err)
			}
			if existingTx != nil {
				// Идемпотентный запрос - возвращаем существующую транзакцию
				result = dtos.MapTransactionToDTO(existingTx)
				return nil
			}
		}

		// 2. Парсим wallet ID
		walletID, err := uuid.Parse(cmd.WalletID)
		if err != nil {
			return errors.ValidationError{
				Field:   "wallet_id",
				Message: "invalid wallet ID format",
			}
		}

		// 3. Загружаем кошелёк
		wallet, err := uc.walletRepo.FindByID(txCtx, walletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: wallet %s", errors.ErrEntityNotFound, cmd.WalletID)
			}
			return fmt.Errorf("failed to load wallet: %w", err)
		}

		// 4. Парсим сумму
		amount, err := valueobjects.NewMoney(cmd.Amount, wallet.Currency())
		if err != nil {
			return errors.ValidationError{
				Field:   "amount",
				Message: fmt.Sprintf("invalid amount: %v", err),
			}
		}

		// 6. Создаём транзакцию через domain entity
		transaction, err := entities.NewTransaction(
			walletID,
			cmd.IdempotencyKey,
			entities.TransactionType(cmd.Type),
			amount,
			cmd.Description,
		)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		// Устанавливаем опциональные поля
		if cmd.DestinationWalletID != "" {
			destID, err := uuid.Parse(cmd.DestinationWalletID)
			if err != nil {
				return errors.ValidationError{
					Field:   "destination_wallet_id",
					Message: "invalid destination wallet ID format",
				}
			}
			if err := transaction.SetDestinationWallet(destID); err != nil {
				return fmt.Errorf("failed to set destination wallet: %w", err)
			}
		}

		if cmd.ExternalReference != "" {
			if err := transaction.SetExternalReference(cmd.ExternalReference); err != nil {
				return fmt.Errorf("failed to set external reference: %w", err)
			}
		}

		if len(cmd.Metadata) > 0 {
			for key, value := range cmd.Metadata {
				if err := transaction.AddMetadata(key, value); err != nil {
					return fmt.Errorf("failed to add metadata: %w", err)
				}
			}
		}

		// 7. Применяем операцию к кошельку в зависимости от типа транзакции
		switch entities.TransactionType(cmd.Type) {
		case entities.TransactionTypeDeposit, entities.TransactionTypeRefund:
			// Пополнение кошелька
			if err := wallet.Credit(amount); err != nil {
				return fmt.Errorf("failed to credit wallet: %w", err)
			}

		case entities.TransactionTypeWithdraw, entities.TransactionTypePayout, entities.TransactionTypeFee:
			// Списание с кошелька
			if err := wallet.Debit(amount); err != nil {
				return fmt.Errorf("failed to debit wallet: %w", err)
			}

		case entities.TransactionTypeTransfer:
			// Для TRANSFER нужен отдельный use case (TransferBetweenWallets)
			return errors.BusinessRuleViolation{
				Rule:    "TransferType",
				Message: "use TransferBetweenWallets use case for TRANSFER transactions",
			}

		case entities.TransactionTypeAdjustment:
			// Adjustment может быть и Credit, и Debit - определяется знаком amount
			// Для простоты считаем что это всегда Credit
			// TODO: добавить поле direction в command
			if err := wallet.Credit(amount); err != nil {
				return fmt.Errorf("failed to adjust wallet: %w", err)
			}

		default:
			return errors.ValidationError{
				Field:   "type",
				Message: fmt.Sprintf("unsupported transaction type: %s", cmd.Type),
			}
		}

		// 8. Переводим в PROCESSING и затем в COMPLETED (для синхронных операций)
		if err := transaction.StartProcessing(); err != nil {
			return fmt.Errorf("failed to start processing transaction: %w", err)
		}

		if err := transaction.MarkCompleted(); err != nil {
			return fmt.Errorf("failed to complete transaction: %w", err)
		}

		// 9. Сохраняем транзакцию
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		// 10. Сохраняем обновлённый кошелёк
		if err := uc.walletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("failed to save wallet: %w", err)
		}

		// 11. Публикуем события
		eventList := []events.DomainEvent{
			events.NewTransactionCreated(
				transaction.ID(),
				walletID,
				string(transaction.Type()),
				amount,
				cmd.IdempotencyKey,
			),
			events.NewTransactionCompleted(
				transaction.ID(),
				walletID,
				string(transaction.Type()),
				amount,
			),
		}

		// Добавляем события в зависимости от типа транзакции
		switch transaction.Type() {
		case entities.TransactionTypeDeposit, entities.TransactionTypeRefund:
			eventList = append(eventList, events.NewWalletCredited(
				walletID,
				amount,
				transaction.ID(),
				wallet.AvailableBalance(),
			))
		case entities.TransactionTypeWithdraw, entities.TransactionTypePayout, entities.TransactionTypeFee:
			eventList = append(eventList, events.NewWalletDebited(
				walletID,
				amount,
				transaction.ID(),
				wallet.AvailableBalance(),
			))
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
