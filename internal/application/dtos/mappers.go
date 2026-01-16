// Package dtos - Mappers для конвертации domain entities в DTOs.
//
// SOLID Principles:
// - SRP: Mappers отвечают только за конвертацию
// - OCP: Новые мапперы добавляются без изменения существующих
//
// Pattern: Mapper/Converter
// Отделяет domain representation от API representation
package dtos

import (
	"fmt"

	"github.com/Haleralex/wallethub/internal/domain/entities"
)

// ============================================
// User Mappers
// ============================================

// ToUserDTO конвертирует domain entity User в DTO.
func ToUserDTO(user *entities.User) UserDTO {
	return UserDTO{
		ID:        user.ID().String(),
		Email:     user.Email(),
		FullName:  user.FullName(),
		KYCStatus: string(user.KYCStatus()),
		CreatedAt: user.CreatedAt(),
		UpdatedAt: user.UpdatedAt(),
	}
}

// ToUserDTOList конвертирует список users.
func ToUserDTOList(users []*entities.User) []UserDTO {
	result := make([]UserDTO, len(users))
	for i, user := range users {
		result[i] = ToUserDTO(user)
	}
	return result
}

// ============================================
// Wallet Mappers
// ============================================

// ToWalletDTO конвертирует domain entity Wallet в DTO.
func ToWalletDTO(wallet *entities.Wallet) WalletDTO {
	totalBalance, _ := wallet.TotalBalance()

	return WalletDTO{
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
}

// ToWalletDTOList конвертирует список wallets.
func ToWalletDTOList(wallets []*entities.Wallet) []WalletDTO {
	result := make([]WalletDTO, len(wallets))
	for i, wallet := range wallets {
		result[i] = ToWalletDTO(wallet)
	}
	return result
}

// ============================================
// Transaction Mappers
// ============================================

// ToTransactionDTO конвертирует domain entity Transaction в DTO.
func ToTransactionDTO(tx *entities.Transaction) TransactionDTO {
	dto := TransactionDTO{
		ID:                tx.ID().String(),
		WalletID:          tx.WalletID().String(),
		IdempotencyKey:    tx.IdempotencyKey(),
		Type:              string(tx.Type()),
		Status:            string(tx.Status()),
		Amount:            tx.Amount().String(),
		CurrencyCode:      tx.Amount().Currency().Code(),
		ExternalReference: tx.ExternalReference(),
		Description:       tx.Description(),
		Metadata:          convertMetadataToStringMap(tx.Metadata()),
		FailureReason:     tx.FailureReason(),
		RetryCount:        tx.RetryCount(),
		CreatedAt:         tx.CreatedAt(),
		UpdatedAt:         tx.UpdatedAt(),
	}

	// Optional fields
	if destWalletID := tx.DestinationWalletID(); destWalletID != nil {
		destStr := destWalletID.String()
		dto.DestinationWalletID = &destStr
	}

	if processedAt := tx.ProcessedAt(); processedAt != nil {
		dto.ProcessedAt = processedAt
	}

	if completedAt := tx.CompletedAt(); completedAt != nil {
		dto.CompletedAt = completedAt
	}

	return dto
}

// ToTransactionDTOList конвертирует список transactions.
func ToTransactionDTOList(transactions []*entities.Transaction) []TransactionDTO {
	result := make([]TransactionDTO, len(transactions))
	for i, tx := range transactions {
		result[i] = ToTransactionDTO(tx)
	}
	return result
}

// MapTransactionToDTO - alias для ToTransactionDTO (для совместимости с use cases).
func MapTransactionToDTO(tx *entities.Transaction) *TransactionDTO {
	dto := ToTransactionDTO(tx)
	return &dto
}

// ============================================
// Helper functions
// ============================================

// convertMetadataToStringMap конвертирует map[string]interface{} в map[string]string.
// Для упрощения JSON сериализации.
func convertMetadataToStringMap(metadata map[string]interface{}) map[string]string {
	if metadata == nil {
		return nil
	}

	result := make(map[string]string, len(metadata))
	for k, v := range metadata {
		// Конвертируем значения в string
		switch val := v.(type) {
		case string:
			result[k] = val
		case nil:
			result[k] = ""
		default:
			// Для других типов используем fmt.Sprintf
			result[k] = fmt.Sprintf("%v", val)
		}
	}

	return result
}
