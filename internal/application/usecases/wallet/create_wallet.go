// Package wallet содержит use cases для работы с кошельками.
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

// CreateWalletUseCase - use case для создания нового кошелька.
//
// Сценарий:
// 1. Загрузить пользователя и проверить KYC
// 2. Проверить, что у пользователя нет кошелька в этой валюте
// 3. Создать кошелёк через domain entity
// 4. Сохранить в БД
// 5. Опубликовать событие WalletCreated
//
// Бизнес-правила:
// - Только верифицированные пользователи могут создавать кошельки (domain rule)
// - У пользователя может быть только один кошелёк на валюту
type CreateWalletUseCase struct {
	userRepo       ports.UserRepository
	walletRepo     ports.WalletRepository
	eventPublisher ports.EventPublisher
	uow            ports.UnitOfWork
}

// NewCreateWalletUseCase создаёт новый use case.
func NewCreateWalletUseCase(
	userRepo ports.UserRepository,
	walletRepo ports.WalletRepository,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
) *CreateWalletUseCase {
	return &CreateWalletUseCase{
		userRepo:       userRepo,
		walletRepo:     walletRepo,
		eventPublisher: eventPublisher,
		uow:            uow,
	}
}

// Execute выполняет создание кошелька.
func (uc *CreateWalletUseCase) Execute(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error) {
	var result *dtos.WalletDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Парсим входные параметры
		userID, err := uuid.Parse(cmd.UserID)
		if err != nil {
			return errors.ValidationError{
				Field:   "user_id",
				Message: "invalid UUID format",
			}
		}

		currency, err := valueobjects.NewCurrency(cmd.CurrencyCode)
		if err != nil {
			return errors.ValidationError{
				Field:   "currency_code",
				Message: fmt.Sprintf("invalid currency: %v", err),
			}
		}

		// 2. Загружаем пользователя
		user, err := uc.userRepo.FindByID(txCtx, userID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewDomainError("USER_NOT_FOUND", "user not found", err)
			}
			return fmt.Errorf("failed to load user: %w", err)
		}

		// 3. Проверяем бизнес-правило: Только verified users могут создавать кошельки
		if err := user.CanCreateWallet(); err != nil {
			return err // Вернёт ErrUserNotVerified
		}

		// 4. Проверяем уникальность: Один кошелёк на валюту для пользователя
		exists, err := uc.walletRepo.ExistsByUserAndCurrency(txCtx, userID, currency)
		if err != nil {
			return fmt.Errorf("failed to check wallet existence: %w", err)
		}
		if exists {
			return errors.NewBusinessRuleViolation(
				"WALLET_ALREADY_EXISTS",
				fmt.Sprintf("wallet for currency %s already exists", currency.Code()),
				map[string]interface{}{
					"user_id":  userID.String(),
					"currency": currency.Code(),
				},
			)
		}

		// 5. Создаём domain entity Wallet
		wallet, err := entities.NewWallet(userID, currency)
		if err != nil {
			return fmt.Errorf("failed to create wallet entity: %w", err)
		}

		// 6. Сохраняем в repository
		if err := uc.walletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("failed to save wallet: %w", err)
		}

		// 7. Поднимаем domain event
		event := events.NewWalletCreated(wallet.ID(), userID, currency)

		// 8. Публикуем событие
		if err := uc.eventPublisher.Publish(txCtx, event); err != nil {
			return fmt.Errorf("failed to publish WalletCreated event: %w", err)
		}

		// 9. Конвертируем в DTO
		totalBalance, _ := wallet.TotalBalance()
		result = &dtos.WalletDTO{
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
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
