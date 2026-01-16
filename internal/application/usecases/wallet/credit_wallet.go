// Package wallet - CreditWallet use case для пополнения кошелька.
//
// Этот use case демонстрирует:
// - Идемпотентность через idempotency_key
// - Создание Transaction entity
// - Обновление баланса Wallet
// - Публикацию множественных событий
// - Координацию двух aggregates (Wallet + Transaction)
package wallet

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

// CreditWalletUseCase - use case для пополнения кошелька.
//
// Сценарий:
// 1. Проверить idempotency_key (защита от дубликатов)
// 2. Создать Transaction entity
// 3. Загрузить Wallet
// 4. Применить Credit operation
// 5. Сохранить оба aggregate
// 6. Опубликовать события (TransactionCreated, WalletCredited)
//
// Idempotency:
// Если запрос с таким же idempotency_key уже обработан,
// возвращаем существующую транзакцию без изменений.
type CreditWalletUseCase struct {
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	eventPublisher  ports.EventPublisher
	uow             ports.UnitOfWork
}

// NewCreditWalletUseCase создаёт новый use case.
func NewCreditWalletUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *CreditWalletUseCase {
	return &CreditWalletUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		uow:             uow,
	}
}

// Execute выполняет пополнение кошелька.
func (uc *CreditWalletUseCase) Execute(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error) {
	var result *dtos.WalletOperationDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Проверка идемпотентности
		// Если транзакция с таким ключом уже существует, возвращаем её
		existingTx, err := uc.transactionRepo.FindByIdempotencyKey(txCtx, cmd.IdempotencyKey)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check idempotency key: %w", err)
		}

		if existingTx != nil {
			// 🔑 Идемпотентность: Транзакция уже существует
			// Загружаем кошелёк и возвращаем текущее состояние
			wallet, err := uc.walletRepo.FindByID(txCtx, existingTx.WalletID())
			if err != nil {
				return fmt.Errorf("failed to load wallet: %w", err)
			}

			result = uc.buildResult(wallet, existingTx)
			return nil // Успешно, но без изменений (idempotent)
		}

		// 2. Парсим входные параметры
		walletID, err := uuid.Parse(cmd.WalletID)
		if err != nil {
			return errors.ValidationError{Field: "wallet_id", Message: "invalid UUID"}
		}

		// 3. Загружаем кошелёк
		wallet, err := uc.walletRepo.FindByID(txCtx, walletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewDomainError("WALLET_NOT_FOUND", "wallet not found", err)
			}
			return fmt.Errorf("failed to load wallet: %w", err)
		}

		// 4. Создаём Money с правильной валютой из кошелька
		amountMoney, err := valueobjects.NewMoney(cmd.Amount, wallet.Currency())
		if err != nil {
			return errors.ValidationError{Field: "amount", Message: fmt.Sprintf("invalid amount: %v", err)}
		}

		// 5. Создаём Transaction entity
		transaction, err := entities.NewTransaction(
			walletID,
			cmd.IdempotencyKey,
			entities.TransactionTypeDeposit,
			amountMoney,
			cmd.Description,
		)
		if err != nil {
			return fmt.Errorf("failed to create transaction entity: %w", err)
		}

		// Устанавливаем external reference если есть
		if cmd.ExternalReference != "" {
			if err := transaction.SetExternalReference(cmd.ExternalReference); err != nil {
				return fmt.Errorf("failed to set external reference: %w", err)
			}
		}

		// 6. Применяем бизнес-операцию Credit к кошельку
		// Domain entity Wallet выполнит валидацию и обновит баланс
		if err := wallet.Credit(amountMoney); err != nil {
			return fmt.Errorf("failed to credit wallet: %w", err)
		}

		// 7. Обновляем статус транзакции
		if err := transaction.StartProcessing(); err != nil {
			return fmt.Errorf("failed to start transaction processing: %w", err)
		}
		if err := transaction.MarkCompleted(); err != nil {
			return fmt.Errorf("failed to complete transaction: %w", err)
		}

		// 8. Сохраняем оба aggregate
		// Wallet с новым балансом (и incremented version)
		if err := uc.walletRepo.Save(txCtx, wallet); err != nil {
			if errors.IsConcurrencyError(err) {
				// Другая транзакция изменила wallet - нужен retry
				return errors.NewConcurrencyError(
					"Wallet",
					walletID.String(),
					"wallet was modified by another transaction",
				)
			}
			return fmt.Errorf("failed to save wallet: %w", err)
		}

		// Transaction (новая)
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		// 9. Публикуем события
		eventList := []events.DomainEvent{
			events.NewTransactionCreated(
				transaction.ID(),
				walletID,
				string(entities.TransactionTypeDeposit),
				amountMoney,
				cmd.IdempotencyKey,
			),
			events.NewWalletCredited(
				walletID,
				amountMoney,
				transaction.ID(),
				wallet.AvailableBalance(),
			),
			events.NewTransactionCompleted(
				transaction.ID(),
				walletID,
				string(entities.TransactionTypeDeposit),
				amountMoney,
			),
		}

		if err := uc.eventPublisher.PublishBatch(txCtx, eventList); err != nil {
			return fmt.Errorf("failed to publish events: %w", err)
		}

		// 10. Формируем результат
		result = uc.buildResult(wallet, transaction)

		return nil
	})

	if err != nil {
		// Если ошибка - concurrency error, может потребоваться retry
		// Обработка retry будет в HTTP layer или middleware
		return nil, err
	}

	return result, nil
}

// buildResult - вспомогательный метод для построения DTO
func (uc *CreditWalletUseCase) buildResult(wallet *entities.Wallet, tx *entities.Transaction) *dtos.WalletOperationDTO {
	totalBalance, _ := wallet.TotalBalance()

	return &dtos.WalletOperationDTO{
		Wallet: dtos.WalletDTO{
			ID:               wallet.ID().String(),
			UserID:           wallet.UserID().String(),
			CurrencyCode:     wallet.Currency().Code(),
			WalletType:       string(wallet.WalletType()),
			Status:           string(wallet.Status()),
			AvailableBalance: wallet.AvailableBalance().String(),
			PendingBalance:   wallet.PendingBalance().String(),
			TotalBalance:     totalBalance.String(),
			DailyLimit:       wallet.DailyLimit().String(),
			MonthlyLimit:     wallet.MonthlyLimit().String(),
			CreatedAt:        wallet.CreatedAt(),
			UpdatedAt:        wallet.UpdatedAt(),
		},
		TransactionID: tx.ID().String(),
		Message:       fmt.Sprintf("Wallet credited with %s successfully", tx.Amount().String()),
	}
}
