// Package transaction - TransferBetweenWallets use case для перевода между кошельками.
package transaction

import (
	"context"
	"fmt"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// TransferBetweenWalletsUseCase - use case для перевода между кошельками.
//
// Сценарий:
// 1. Проверить idempotency_key
// 2. Загрузить оба кошелька
// 3. Проверить валюты (должны совпадать)
// 4. Создать транзакцию TRANSFER
// 5. Debit с source wallet
// 6. Credit на destination wallet
// 7. Сохранить всё атомарно
// 8. Опубликовать события
//
// Бизнес-правила:
// - Валюты должны совпадать
// - Достаточно средств на source wallet
// - Оба кошелька должны быть активны
// - Атомарность: либо оба изменения, либо ничего
type TransferBetweenWalletsUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
}

// NewTransferBetweenWalletsUseCase создаёт новый use case.
func NewTransferBetweenWalletsUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *TransferBetweenWalletsUseCase {
	return &TransferBetweenWalletsUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет перевод между кошельками.
func (uc *TransferBetweenWalletsUseCase) Execute(ctx context.Context, cmd dtos.TransferBetweenWalletsCommand) (*dtos.TransactionDTO, error) {
	var result *dtos.TransactionDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Проверка idempotency
		if cmd.IdempotencyKey != "" {
			existingTx, err := uc.transactionRepo.FindByIdempotencyKey(txCtx, cmd.IdempotencyKey)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to check idempotency: %w", err)
			}
			if existingTx != nil {
				result = dtos.MapTransactionToDTO(existingTx)
				return nil
			}
		}

		// 2. Парсим IDs
		sourceWalletID, err := uuid.Parse(cmd.SourceWalletID)
		if err != nil {
			return errors.ValidationError{
				Field:   "source_wallet_id",
				Message: "invalid source wallet ID format",
			}
		}

		destinationWalletID, err := uuid.Parse(cmd.DestinationWalletID)
		if err != nil {
			return errors.ValidationError{
				Field:   "destination_wallet_id",
				Message: "invalid destination wallet ID format",
			}
		}

		// Проверка: нельзя переводить самому себе
		if sourceWalletID == destinationWalletID {
			return errors.NewBusinessRuleViolation(
				"SelfTransfer",
				"cannot transfer to the same wallet",
				nil,
			)
		}

		// 3. Загружаем оба кошелька
		sourceWallet, err := uc.walletRepo.FindByID(txCtx, sourceWalletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: source wallet %s", errors.ErrEntityNotFound, cmd.SourceWalletID)
			}
			return fmt.Errorf("failed to load source wallet: %w", err)
		}

		destinationWallet, err := uc.walletRepo.FindByID(txCtx, destinationWalletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: destination wallet %s", errors.ErrEntityNotFound, cmd.DestinationWalletID)
			}
			return fmt.Errorf("failed to load destination wallet: %w", err)
		}

		// 4. Проверка валют
		if sourceWallet.Currency().Code() != destinationWallet.Currency().Code() {
			return errors.NewBusinessRuleViolation(
				"CurrencyMismatch",
				fmt.Sprintf(
					"currency mismatch: source=%s, destination=%s",
					sourceWallet.Currency().Code(),
					destinationWallet.Currency().Code(),
				),
				nil,
			)
		}

		// 5. Парсим сумму
		amount, err := valueobjects.NewMoney(cmd.Amount, sourceWallet.Currency())
		if err != nil {
			return errors.ValidationError{
				Field:   "amount",
				Message: fmt.Sprintf("invalid amount: %v", err),
			}
		}

		// 6. Создаём транзакцию TRANSFER
		transaction, err := entities.NewTransaction(
			sourceWalletID,
			cmd.IdempotencyKey,
			entities.TransactionTypeTransfer,
			amount,
			cmd.Description,
		)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		// Устанавливаем destination wallet
		if err := transaction.SetDestinationWallet(destinationWalletID); err != nil {
			return fmt.Errorf("failed to set destination wallet: %w", err)
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

		// 7. Списываем с source wallet
		if err := sourceWallet.Debit(amount); err != nil {
			return fmt.Errorf("failed to debit source wallet: %w", err)
		}

		// 8. Зачисляем на destination wallet
		if err := destinationWallet.Credit(amount); err != nil {
			return fmt.Errorf("failed to credit destination wallet: %w", err)
		}

		// 9. Переводим в PROCESSING и затем в COMPLETED
		if err := transaction.StartProcessing(); err != nil {
			return fmt.Errorf("failed to start processing transaction: %w", err)
		}

		if err := transaction.MarkCompleted(); err != nil {
			return fmt.Errorf("failed to complete transaction: %w", err)
		}

		// 10. Сохраняем всё атомарно
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		if err := uc.walletRepo.Save(txCtx, sourceWallet); err != nil {
			return fmt.Errorf("failed to save source wallet: %w", err)
		}

		if err := uc.walletRepo.Save(txCtx, destinationWallet); err != nil {
			return fmt.Errorf("failed to save destination wallet: %w", err)
		}

		// 11. Публикуем события
		eventList := []events.DomainEvent{
			events.NewTransactionCreated(
				transaction.ID(),
				sourceWalletID,
				string(entities.TransactionTypeTransfer),
				amount,
				cmd.IdempotencyKey,
			),
			events.NewWalletDebited(
				sourceWalletID,
				amount,
				transaction.ID(),
				sourceWallet.AvailableBalance(),
			),
			events.NewWalletCredited(
				destinationWalletID,
				amount,
				transaction.ID(),
				destinationWallet.AvailableBalance(),
			),
			events.NewTransactionCompleted(
				transaction.ID(),
				sourceWalletID,
				string(entities.TransactionTypeTransfer),
				amount,
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
