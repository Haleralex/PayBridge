package dtos

import (
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/domain/entities"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToUserDTO(t *testing.T) {
	user, err := entities.NewUser("test@example.com", "Test User")
	require.NoError(t, err)

	dto := ToUserDTO(user)

	assert.Equal(t, user.ID().String(), dto.ID)
	assert.Equal(t, "test@example.com", dto.Email)
	assert.Equal(t, "Test User", dto.FullName)
	assert.Equal(t, "UNVERIFIED", dto.KYCStatus)
	assert.False(t, dto.CreatedAt.IsZero())
	assert.False(t, dto.UpdatedAt.IsZero())
}

func TestToUserDTO_WithVerifiedStatus(t *testing.T) {
	user, err := entities.NewUser("verified@example.com", "Verified User")
	require.NoError(t, err)

	err = user.StartKYCVerification()
	require.NoError(t, err)

	err = user.ApproveKYC()
	require.NoError(t, err)

	dto := ToUserDTO(user)

	assert.Equal(t, "VERIFIED", dto.KYCStatus)
}

func TestToUserDTOList(t *testing.T) {
	user1, _ := entities.NewUser("user1@example.com", "User One")
	user2, _ := entities.NewUser("user2@example.com", "User Two")
	user3, _ := entities.NewUser("user3@example.com", "User Three")

	users := []*entities.User{user1, user2, user3}

	dtos := ToUserDTOList(users)

	assert.Len(t, dtos, 3)
	assert.Equal(t, "user1@example.com", dtos[0].Email)
	assert.Equal(t, "user2@example.com", dtos[1].Email)
	assert.Equal(t, "user3@example.com", dtos[2].Email)
}

func TestToUserDTOList_Empty(t *testing.T) {
	var users []*entities.User

	dtos := ToUserDTOList(users)

	assert.Len(t, dtos, 0)
	assert.NotNil(t, dtos)
}

func TestToWalletDTO(t *testing.T) {
	userID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	wallet, err := entities.NewWallet(userID, currency)
	require.NoError(t, err)

	dto := ToWalletDTO(wallet)

	assert.Equal(t, wallet.ID().String(), dto.ID)
	assert.Equal(t, userID.String(), dto.UserID)
	assert.Equal(t, "USD", dto.CurrencyCode)
	assert.Equal(t, "FIAT", dto.WalletType)
	assert.Equal(t, "ACTIVE", dto.Status)
	assert.Equal(t, "0.00 USD", dto.AvailableBalance)
	assert.Equal(t, "0.00 USD", dto.PendingBalance)
	assert.Equal(t, "0.00 USD", dto.TotalBalance)
	assert.False(t, dto.CreatedAt.IsZero())
}

func TestToWalletDTO_WithBalance(t *testing.T) {
	userID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	wallet, err := entities.NewWallet(userID, currency)
	require.NoError(t, err)

	// Credit wallet
	amount, err := valueobjects.NewMoneyFromCents(10000, currency) // $100.00
	require.NoError(t, err)

	err = wallet.Credit(amount)
	require.NoError(t, err)

	dto := ToWalletDTO(wallet)

	assert.Contains(t, dto.AvailableBalance, "100.00")
	assert.Contains(t, dto.TotalBalance, "100.00")
}

func TestToWalletDTO_CryptoWallet(t *testing.T) {
	userID := uuid.New()
	currency, err := valueobjects.NewCurrency("BTC")
	require.NoError(t, err)

	wallet, err := entities.NewWallet(userID, currency)
	require.NoError(t, err)

	dto := ToWalletDTO(wallet)

	assert.Equal(t, "BTC", dto.CurrencyCode)
	assert.Equal(t, "CRYPTO", dto.WalletType)
}

func TestToWalletDTOList(t *testing.T) {
	userID := uuid.New()
	usd, _ := valueobjects.NewCurrency("USD")
	eur, _ := valueobjects.NewCurrency("EUR")

	wallet1, _ := entities.NewWallet(userID, usd)
	wallet2, _ := entities.NewWallet(userID, eur)

	wallets := []*entities.Wallet{wallet1, wallet2}

	dtos := ToWalletDTOList(wallets)

	assert.Len(t, dtos, 2)
	assert.Equal(t, "USD", dtos[0].CurrencyCode)
	assert.Equal(t, "EUR", dtos[1].CurrencyCode)
}

func TestToWalletDTOList_Empty(t *testing.T) {
	var wallets []*entities.Wallet

	dtos := ToWalletDTOList(wallets)

	assert.Len(t, dtos, 0)
	assert.NotNil(t, dtos)
}

func TestToTransactionDTO(t *testing.T) {
	walletID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	amount, err := valueobjects.NewMoneyFromCents(5000, currency) // $50.00
	require.NoError(t, err)

	tx, err := entities.NewTransaction(
		walletID,
		"idem-key-123",
		entities.TransactionTypeDeposit,
		amount,
		"Test deposit",
	)
	require.NoError(t, err)

	dto := ToTransactionDTO(tx)

	assert.Equal(t, tx.ID().String(), dto.ID)
	assert.Equal(t, walletID.String(), dto.WalletID)
	assert.Equal(t, "idem-key-123", dto.IdempotencyKey)
	assert.Equal(t, "DEPOSIT", dto.Type)
	assert.Equal(t, "PENDING", dto.Status)
	assert.Contains(t, dto.Amount, "50.00")
	assert.Equal(t, "USD", dto.CurrencyCode)
	assert.Equal(t, "Test deposit", dto.Description)
	assert.Equal(t, 0, dto.RetryCount)
	assert.Nil(t, dto.DestinationWalletID)
	assert.Nil(t, dto.ProcessedAt)
	assert.Nil(t, dto.CompletedAt)
}

func TestToTransactionDTO_WithDestinationWallet(t *testing.T) {
	walletID := uuid.New()
	destWalletID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	amount, err := valueobjects.NewMoneyFromCents(1000, currency)
	require.NoError(t, err)

	tx, err := entities.NewTransaction(
		walletID,
		"transfer-key",
		entities.TransactionTypeTransfer,
		amount,
		"Transfer to another wallet",
	)
	require.NoError(t, err)

	err = tx.SetDestinationWallet(destWalletID)
	require.NoError(t, err)

	dto := ToTransactionDTO(tx)

	assert.NotNil(t, dto.DestinationWalletID)
	assert.Equal(t, destWalletID.String(), *dto.DestinationWalletID)
}

func TestToTransactionDTO_CompletedTransaction(t *testing.T) {
	walletID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	amount, err := valueobjects.NewMoneyFromCents(1000, currency)
	require.NoError(t, err)

	tx, err := entities.NewTransaction(
		walletID,
		"complete-key",
		entities.TransactionTypeDeposit,
		amount,
		"",
	)
	require.NoError(t, err)

	// Process and complete
	err = tx.StartProcessing()
	require.NoError(t, err)

	err = tx.MarkCompleted()
	require.NoError(t, err)

	dto := ToTransactionDTO(tx)

	assert.Equal(t, "COMPLETED", dto.Status)
	assert.NotNil(t, dto.ProcessedAt)
	assert.NotNil(t, dto.CompletedAt)
}

func TestToTransactionDTO_FailedTransaction(t *testing.T) {
	walletID := uuid.New()
	currency, err := valueobjects.NewCurrency("USD")
	require.NoError(t, err)

	amount, err := valueobjects.NewMoneyFromCents(1000, currency)
	require.NoError(t, err)

	tx, err := entities.NewTransaction(
		walletID,
		"fail-key",
		entities.TransactionTypeDeposit,
		amount,
		"",
	)
	require.NoError(t, err)

	err = tx.StartProcessing()
	require.NoError(t, err)

	err = tx.MarkFailed("Insufficient funds")
	require.NoError(t, err)

	dto := ToTransactionDTO(tx)

	assert.Equal(t, "FAILED", dto.Status)
	assert.Equal(t, "Insufficient funds", dto.FailureReason)
}

func TestToTransactionDTOList(t *testing.T) {
	walletID := uuid.New()
	currency, _ := valueobjects.NewCurrency("USD")
	amount, _ := valueobjects.NewMoneyFromCents(1000, currency)

	tx1, _ := entities.NewTransaction(walletID, "key1", entities.TransactionTypeDeposit, amount, "")
	tx2, _ := entities.NewTransaction(walletID, "key2", entities.TransactionTypeWithdraw, amount, "")
	tx3, _ := entities.NewTransaction(walletID, "key3", entities.TransactionTypeTransfer, amount, "")

	transactions := []*entities.Transaction{tx1, tx2, tx3}

	dtos := ToTransactionDTOList(transactions)

	assert.Len(t, dtos, 3)
	assert.Equal(t, "DEPOSIT", dtos[0].Type)
	assert.Equal(t, "WITHDRAW", dtos[1].Type)
	assert.Equal(t, "TRANSFER", dtos[2].Type)
}

func TestToTransactionDTOList_Empty(t *testing.T) {
	var transactions []*entities.Transaction

	dtos := ToTransactionDTOList(transactions)

	assert.Len(t, dtos, 0)
	assert.NotNil(t, dtos)
}

func TestMapTransactionToDTO(t *testing.T) {
	walletID := uuid.New()
	currency, _ := valueobjects.NewCurrency("USD")
	amount, _ := valueobjects.NewMoneyFromCents(1000, currency)

	tx, err := entities.NewTransaction(
		walletID,
		"map-key",
		entities.TransactionTypeDeposit,
		amount,
		"",
	)
	require.NoError(t, err)

	dto := MapTransactionToDTO(tx)

	assert.NotNil(t, dto)
	assert.Equal(t, tx.ID().String(), dto.ID)
}

func TestConvertMetadataToStringMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name: "string values",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"string": "hello",
				"int":    42,
				"float":  3.14,
				"bool":   true,
				"nil":    nil,
				"time":   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: map[string]string{
				"string": "hello",
				"int":    "42",
				"float":  "3.14",
				"bool":   "true",
				"nil":    "",
				"time":   "2024-01-01 00:00:00 +0000 UTC",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMetadataToStringMap(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, len(tt.expected), len(result))
				for k, v := range tt.expected {
					assert.Equal(t, v, result[k])
				}
			}
		})
	}
}

func TestAllTransactionTypes(t *testing.T) {
	walletID := uuid.New()
	currency, _ := valueobjects.NewCurrency("USD")
	amount, _ := valueobjects.NewMoneyFromCents(1000, currency)

	types := []struct {
		txType   entities.TransactionType
		expected string
	}{
		{entities.TransactionTypeDeposit, "DEPOSIT"},
		{entities.TransactionTypeWithdraw, "WITHDRAW"},
		{entities.TransactionTypePayout, "PAYOUT"},
		{entities.TransactionTypeTransfer, "TRANSFER"},
		{entities.TransactionTypeFee, "FEE"},
		{entities.TransactionTypeRefund, "REFUND"},
		{entities.TransactionTypeAdjustment, "ADJUSTMENT"},
	}

	for _, tt := range types {
		t.Run(tt.expected, func(t *testing.T) {
			tx, err := entities.NewTransaction(
				walletID,
				"key-"+tt.expected,
				tt.txType,
				amount,
				"",
			)
			require.NoError(t, err)

			dto := ToTransactionDTO(tx)
			assert.Equal(t, tt.expected, dto.Type)
		})
	}
}
