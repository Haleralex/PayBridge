package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	natsadapter "github.com/Haleralex/wallethub/internal/adapters/nats"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/events"
)

// Handler processes domain events and sends Telegram notifications.
type Handler struct {
	telegram   *TelegramSender
	walletRepo ports.WalletRepository
	userRepo   ports.UserRepository
	logger     *slog.Logger
}

// NewHandler creates a new notification handler.
func NewHandler(
	telegram *TelegramSender,
	walletRepo ports.WalletRepository,
	userRepo ports.UserRepository,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		telegram:   telegram,
		walletRepo: walletRepo,
		userRepo:   userRepo,
		logger:     logger,
	}
}

// HandleWalletCredited handles wallet.credited events — notifies the wallet owner.
func (h *Handler) HandleWalletCredited(ctx context.Context, msg *natsadapter.EventMessage) error {
	var payload struct {
		WalletID      string `json:"wallet_id"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		TransactionID string `json:"transaction_id"`
		BalanceAfter  string `json:"balance_after"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal wallet.credited payload: %w", err)
	}

	walletID, err := uuid.Parse(payload.WalletID)
	if err != nil {
		walletID, err = uuid.Parse(msg.AggregateID)
		if err != nil {
			return fmt.Errorf("invalid wallet ID: %w", err)
		}
	}

	// Lookup wallet owner
	wallet, err := h.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		return fmt.Errorf("failed to find wallet %s: %w", walletID, err)
	}

	// Lookup user to get Telegram ID
	user, err := h.userRepo.FindByID(ctx, wallet.UserID())
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", wallet.UserID(), err)
	}

	telegramID := user.TelegramID()
	if telegramID == nil {
		h.logger.Debug("User has no Telegram ID, skipping notification",
			slog.String("user_id", wallet.UserID().String()),
		)
		return nil
	}

	text := fmt.Sprintf("💰 <b>Получен перевод</b>\n+%s %s", payload.Amount, payload.Currency)

	if err := h.telegram.SendMessage(*telegramID, text); err != nil {
		return fmt.Errorf("failed to send telegram notification: %w", err)
	}

	h.logger.Info("Notification sent",
		slog.String("event", events.EventTypeWalletCredited),
		slog.Int64("telegram_id", *telegramID),
	)

	return nil
}

// HandleTransactionCompleted handles transaction.completed events — notifies the sender.
func (h *Handler) HandleTransactionCompleted(ctx context.Context, msg *natsadapter.EventMessage) error {
	var payload struct {
		TransactionID   string `json:"transaction_id"`
		WalletID        string `json:"wallet_id"`
		TransactionType string `json:"transaction_type"`
		Amount          string `json:"amount"`
		Currency        string `json:"currency"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal transaction.completed payload: %w", err)
	}

	// Only notify for transfers (deposits/debits are initiated by the user)
	if payload.TransactionType != "TRANSFER" {
		return nil
	}

	walletID, err := uuid.Parse(payload.WalletID)
	if err != nil {
		walletID, err = uuid.Parse(msg.AggregateID)
		if err != nil {
			return fmt.Errorf("invalid wallet ID: %w", err)
		}
	}

	wallet, err := h.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		return fmt.Errorf("failed to find wallet %s: %w", walletID, err)
	}

	user, err := h.userRepo.FindByID(ctx, wallet.UserID())
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", wallet.UserID(), err)
	}

	telegramID := user.TelegramID()
	if telegramID == nil {
		return nil
	}

	text := fmt.Sprintf("✅ <b>Перевод выполнен</b>\n%s %s", payload.Amount, payload.Currency)

	if err := h.telegram.SendMessage(*telegramID, text); err != nil {
		return fmt.Errorf("failed to send telegram notification: %w", err)
	}

	h.logger.Info("Notification sent",
		slog.String("event", events.EventTypeTransactionCompleted),
		slog.Int64("telegram_id", *telegramID),
	)

	return nil
}
