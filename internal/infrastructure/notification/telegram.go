// Package notification provides Telegram Bot API integration for sending notifications.
package notification

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// TelegramSender sends messages via Telegram Bot API.
type TelegramSender struct {
	botToken string
	client   *http.Client
	logger   *slog.Logger
}

// NewTelegramSender creates a new Telegram message sender.
func NewTelegramSender(botToken string, logger *slog.Logger) *TelegramSender {
	return &TelegramSender{
		botToken: botToken,
		client:   &http.Client{Timeout: 10 * time.Second},
		logger:   logger,
	}
}

// SendMessage sends a text message to a Telegram chat.
func (s *TelegramSender) SendMessage(chatID int64, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	params := url.Values{}
	params.Set("chat_id", fmt.Sprintf("%d", chatID))
	params.Set("text", text)
	params.Set("parse_mode", "HTML")

	resp, err := s.client.PostForm(apiURL, params)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode telegram response: %w", err)
	}

	s.logger.Debug("Telegram message sent", slog.Int64("chat_id", chatID))
	return nil
}
