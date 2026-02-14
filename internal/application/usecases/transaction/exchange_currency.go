package transaction

import (
	"context"
	"fmt"
	"math/big"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// ExchangeCurrencyUseCase handles currency exchange between user's own wallets.
type ExchangeCurrencyUseCase struct {
	walletRepo     ports.WalletRepository
	transactionRepo ports.TransactionRepository
	rateProvider   ports.ExchangeRateProvider
	eventPublisher ports.EventPublisher
	uow            ports.UnitOfWork
	spreadPercent  float64
}

// NewExchangeCurrencyUseCase creates a new use case.
func NewExchangeCurrencyUseCase(
	walletRepo ports.WalletRepository,
	transactionRepo ports.TransactionRepository,
	rateProvider ports.ExchangeRateProvider,
	eventPublisher ports.EventPublisher,
	uow ports.UnitOfWork,
	spreadPercent float64,
) *ExchangeCurrencyUseCase {
	return &ExchangeCurrencyUseCase{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		rateProvider:    rateProvider,
		eventPublisher:  eventPublisher,
		uow:            uow,
		spreadPercent:   spreadPercent,
	}
}

// Execute performs the currency exchange.
func (uc *ExchangeCurrencyUseCase) Execute(ctx context.Context, cmd dtos.ExchangeCurrencyCommand) (*dtos.ExchangeResultDTO, error) {
	var result *dtos.ExchangeResultDTO

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		// 1. Idempotency check
		if cmd.IdempotencyKey != "" {
			existingTx, err := uc.transactionRepo.FindByIdempotencyKey(txCtx, cmd.IdempotencyKey)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to check idempotency: %w", err)
			}
			if existingTx != nil {
				sourceWallet, err := uc.walletRepo.FindByID(txCtx, existingTx.WalletID())
				if err != nil {
					return fmt.Errorf("failed to load source wallet: %w", err)
				}
				destID := existingTx.DestinationWalletID()
				if destID == nil {
					return fmt.Errorf("existing exchange transaction has no destination wallet")
				}
				destWallet, err := uc.walletRepo.FindByID(txCtx, *destID)
				if err != nil {
					return fmt.Errorf("failed to load destination wallet: %w", err)
				}
				result = uc.buildResult(sourceWallet, destWallet, existingTx, "", "", "")
				return nil
			}
		}

		// 2. Parse IDs
		sourceWalletID, err := uuid.Parse(cmd.SourceWalletID)
		if err != nil {
			return errors.ValidationError{Field: "source_wallet_id", Message: "invalid source wallet ID format"}
		}
		destWalletID, err := uuid.Parse(cmd.DestinationWalletID)
		if err != nil {
			return errors.ValidationError{Field: "destination_wallet_id", Message: "invalid destination wallet ID format"}
		}
		if sourceWalletID == destWalletID {
			return errors.NewBusinessRuleViolation("SelfExchange", "cannot exchange to the same wallet", nil)
		}

		// 3. Load wallets
		sourceWallet, err := uc.walletRepo.FindByID(txCtx, sourceWalletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: source wallet %s", errors.ErrEntityNotFound, cmd.SourceWalletID)
			}
			return fmt.Errorf("failed to load source wallet: %w", err)
		}
		destWallet, err := uc.walletRepo.FindByID(txCtx, destWalletID)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("%w: destination wallet %s", errors.ErrEntityNotFound, cmd.DestinationWalletID)
			}
			return fmt.Errorf("failed to load destination wallet: %w", err)
		}

		// 4. Both wallets must belong to the same user
		if sourceWallet.UserID() != destWallet.UserID() {
			return errors.NewBusinessRuleViolation("OwnerMismatch", "exchange is only allowed between your own wallets", nil)
		}

		// 5. Currencies must be different (otherwise use transfer)
		if sourceWallet.Currency().Code() == destWallet.Currency().Code() {
			return errors.NewBusinessRuleViolation("SameCurrency", "wallets have the same currency, use transfer instead", nil)
		}

		// 6. Parse source amount
		sourceAmount, err := valueobjects.NewMoney(cmd.Amount, sourceWallet.Currency())
		if err != nil {
			return errors.ValidationError{Field: "amount", Message: fmt.Sprintf("invalid amount: %v", err)}
		}

		// 7. Get exchange rate
		rate, err := uc.rateProvider.GetRate(txCtx, sourceWallet.Currency().Code(), destWallet.Currency().Code())
		if err != nil {
			return fmt.Errorf("failed to get exchange rate: %w", err)
		}

		// 8. Apply spread: effectiveRate = rate * (1 - spread/100)
		spreadFactor := new(big.Rat).SetFloat64(1.0 - uc.spreadPercent/100.0)
		effectiveRate := new(big.Rat).Mul(rate, spreadFactor)

		// 9. Calculate destination amount
		destAmountRat := new(big.Rat).Mul(sourceAmount.Amount(), effectiveRate)
		destAmountMoney, err := valueobjects.NewMoney(destAmountRat.FloatString(8), destWallet.Currency())
		if err != nil {
			return fmt.Errorf("failed to create destination amount: %w", err)
		}

		// 10. Create EXCHANGE transaction
		transaction, err := entities.NewTransaction(
			sourceWalletID,
			cmd.IdempotencyKey,
			entities.TransactionTypeExchange,
			sourceAmount,
			fmt.Sprintf("Exchange %s → %s", sourceWallet.Currency().Code(), destWallet.Currency().Code()),
		)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		if err := transaction.SetDestinationWallet(destWalletID); err != nil {
			return fmt.Errorf("failed to set destination wallet: %w", err)
		}

		// Store exchange metadata
		_ = transaction.AddMetadata("exchange_rate", rate.FloatString(8))
		_ = transaction.AddMetadata("effective_rate", effectiveRate.FloatString(8))
		_ = transaction.AddMetadata("spread_percent", fmt.Sprintf("%.2f", uc.spreadPercent))
		_ = transaction.AddMetadata("source_currency", sourceWallet.Currency().Code())
		_ = transaction.AddMetadata("dest_currency", destWallet.Currency().Code())
		_ = transaction.AddMetadata("dest_amount", destAmountMoney.String())

		// 11. Debit source wallet
		if err := sourceWallet.Debit(sourceAmount); err != nil {
			return fmt.Errorf("failed to debit source wallet: %w", err)
		}

		// 12. Credit destination wallet
		if err := destWallet.Credit(destAmountMoney); err != nil {
			return fmt.Errorf("failed to credit destination wallet: %w", err)
		}

		// 13. Complete transaction
		if err := transaction.StartProcessing(); err != nil {
			return fmt.Errorf("failed to start processing: %w", err)
		}
		if err := transaction.MarkCompleted(); err != nil {
			return fmt.Errorf("failed to complete transaction: %w", err)
		}

		// 14. Save atomically
		if err := uc.transactionRepo.Save(txCtx, transaction); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}
		if err := uc.walletRepo.Save(txCtx, sourceWallet); err != nil {
			return fmt.Errorf("failed to save source wallet: %w", err)
		}
		if err := uc.walletRepo.Save(txCtx, destWallet); err != nil {
			return fmt.Errorf("failed to save destination wallet: %w", err)
		}

		// 15. Publish events
		rateStr := effectiveRate.FloatString(8)
		spreadStr := fmt.Sprintf("%.2f%%", uc.spreadPercent)

		eventList := []events.DomainEvent{
			events.NewWalletDebited(sourceWalletID, sourceAmount, transaction.ID(), sourceWallet.AvailableBalance()),
			events.NewWalletCredited(destWalletID, destAmountMoney, transaction.ID(), destWallet.AvailableBalance()),
			events.NewCurrencyExchanged(
				transaction.ID(), sourceWalletID, destWalletID,
				sourceAmount, destAmountMoney,
				rateStr, sourceWallet.Currency().Code(), destWallet.Currency().Code(),
			),
		}
		if err := uc.eventPublisher.PublishBatch(txCtx, eventList); err != nil {
			return fmt.Errorf("failed to publish events: %w", err)
		}

		result = uc.buildResult(sourceWallet, destWallet, transaction, rateStr, spreadStr, destAmountMoney.String())
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (uc *ExchangeCurrencyUseCase) buildResult(
	source, dest *entities.Wallet,
	tx *entities.Transaction,
	rate, spread, destAmount string,
) *dtos.ExchangeResultDTO {
	srcTotal, _ := source.TotalBalance()
	dstTotal, _ := dest.TotalBalance()

	return &dtos.ExchangeResultDTO{
		SourceWallet: dtos.WalletDTO{
			ID:               source.ID().String(),
			UserID:           source.UserID().String(),
			CurrencyCode:     source.Currency().Code(),
			WalletType:       string(source.WalletType()),
			Status:           string(source.Status()),
			AvailableBalance: source.AvailableBalance().String(),
			PendingBalance:   source.PendingBalance().String(),
			TotalBalance:     srcTotal.String(),
			DailyLimit:       source.DailyLimit().String(),
			MonthlyLimit:     source.MonthlyLimit().String(),
			CreatedAt:        source.CreatedAt(),
			UpdatedAt:        source.UpdatedAt(),
		},
		DestinationWallet: dtos.WalletDTO{
			ID:               dest.ID().String(),
			UserID:           dest.UserID().String(),
			CurrencyCode:     dest.Currency().Code(),
			WalletType:       string(dest.WalletType()),
			Status:           string(dest.Status()),
			AvailableBalance: dest.AvailableBalance().String(),
			PendingBalance:   dest.PendingBalance().String(),
			TotalBalance:     dstTotal.String(),
			DailyLimit:       dest.DailyLimit().String(),
			MonthlyLimit:     dest.MonthlyLimit().String(),
			CreatedAt:        dest.CreatedAt(),
			UpdatedAt:        dest.UpdatedAt(),
		},
		TransactionID:     tx.ID().String(),
		SourceAmount:      tx.Amount().String(),
		DestinationAmount: destAmount,
		ExchangeRate:      rate,
		Spread:            spread,
		SourceCurrency:    source.Currency().Code(),
		DestCurrency:      dest.Currency().Code(),
		Status:            string(tx.Status()),
	}
}
