package entities

import (
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// TestWalletType_IsValid tests the WalletType validation
func TestWalletType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		wType    WalletType
		expected bool
	}{
		{"FIAT is valid", WalletTypeFiat, true},
		{"CRYPTO is valid", WalletTypeCrypto, true},
		{"Invalid type", WalletType("INVALID"), false},
		{"Empty type", WalletType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.wType.IsValid(); got != tt.expected {
				t.Errorf("WalletType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestWalletStatus_IsValid tests the WalletStatus validation
func TestWalletStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   WalletStatus
		expected bool
	}{
		{"ACTIVE is valid", WalletStatusActive, true},
		{"SUSPENDED is valid", WalletStatusSuspended, true},
		{"LOCKED is valid", WalletStatusLocked, true},
		{"CLOSED is valid", WalletStatusClosed, true},
		{"Invalid status", WalletStatus("INVALID"), false},
		{"Empty status", WalletStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.expected {
				t.Errorf("WalletStatus.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNewWallet_Success tests successful wallet creation
func TestNewWallet_Success(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	wallet, err := NewWallet(userID, currency)

	if err != nil {
		t.Fatalf("NewWallet() error = %v, want nil", err)
	}

	if wallet.ID() == uuid.Nil {
		t.Error("Wallet ID should not be nil")
	}

	if wallet.UserID() != userID {
		t.Errorf("Wallet UserID = %v, want %v", wallet.UserID(), userID)
	}

	if !wallet.Currency().Equals(currency) {
		t.Errorf("Wallet Currency = %v, want %v", wallet.Currency(), currency)
	}

	if wallet.Status() != WalletStatusActive {
		t.Errorf("Wallet Status = %v, want %v", wallet.Status(), WalletStatusActive)
	}

	if wallet.WalletType() != WalletTypeFiat {
		t.Errorf("Wallet Type = %v, want %v", wallet.WalletType(), WalletTypeFiat)
	}

	if !wallet.AvailableBalance().IsZero() {
		t.Errorf("AvailableBalance should be zero, got %v", wallet.AvailableBalance())
	}

	if !wallet.PendingBalance().IsZero() {
		t.Errorf("PendingBalance should be zero, got %v", wallet.PendingBalance())
	}

	if wallet.BalanceVersion() != 0 {
		t.Errorf("BalanceVersion = %v, want 0", wallet.BalanceVersion())
	}
}

// TestNewWallet_CryptoWallet tests crypto wallet creation
func TestNewWallet_CryptoWallet(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.BTC

	wallet, err := NewWallet(userID, currency)

	if err != nil {
		t.Fatalf("NewWallet() error = %v, want nil", err)
	}

	if wallet.WalletType() != WalletTypeCrypto {
		t.Errorf("Wallet Type = %v, want %v", wallet.WalletType(), WalletTypeCrypto)
	}
}

// TestNewWallet_InvalidCurrency tests wallet creation with invalid currency
func TestNewWallet_InvalidCurrency(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.Currency{}

	_, err := NewWallet(userID, currency)

	if err == nil {
		t.Fatal("NewWallet() with zero currency should return error")
	}

	if _, ok := err.(errors.ValidationError); !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

// TestReconstructWallet tests wallet reconstruction from storage
func TestReconstructWallet(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	currency := valueobjects.USD
	available, _ := valueobjects.NewMoneyFromInt(100, currency)
	pending, _ := valueobjects.NewMoneyFromInt(20, currency)
	dailyLimit, _ := valueobjects.NewMoneyFromInt(1000, currency)
	monthlyLimit, _ := valueobjects.NewMoneyFromInt(5000, currency)
	now := time.Now()

	wallet := ReconstructWallet(
		id, userID,
		currency,
		WalletTypeFiat,
		WalletStatusActive,
		available, pending,
		5,
		dailyLimit, monthlyLimit,
		now, now,
	)

	if wallet.ID() != id {
		t.Errorf("ID = %v, want %v", wallet.ID(), id)
	}
	if wallet.UserID() != userID {
		t.Errorf("UserID = %v, want %v", wallet.UserID(), userID)
	}
	if wallet.BalanceVersion() != 5 {
		t.Errorf("BalanceVersion = %v, want 5", wallet.BalanceVersion())
	}
}

// TestWallet_IsActive tests the IsActive method
func TestWallet_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   WalletStatus
		expected bool
	}{
		{"Active wallet", WalletStatusActive, true},
		{"Suspended wallet", WalletStatusSuspended, false},
		{"Locked wallet", WalletStatusLocked, false},
		{"Closed wallet", WalletStatusClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := &Wallet{status: tt.status}
			if got := wallet.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestWallet_CanDebit tests debit permission checks
func TestWallet_CanDebit(t *testing.T) {
	tests := []struct {
		name      string
		status    WalletStatus
		wantError bool
	}{
		{"Active wallet can debit", WalletStatusActive, false},
		{"Suspended wallet cannot debit", WalletStatusSuspended, true},
		{"Locked wallet cannot debit", WalletStatusLocked, true},
		{"Closed wallet cannot debit", WalletStatusClosed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := &Wallet{status: tt.status}
			err := wallet.CanDebit()
			if (err != nil) != tt.wantError {
				t.Errorf("CanDebit() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestWallet_CanCredit tests credit permission checks
func TestWallet_CanCredit(t *testing.T) {
	tests := []struct {
		name      string
		status    WalletStatus
		wantError bool
	}{
		{"Active wallet can credit", WalletStatusActive, false},
		{"Suspended wallet can credit", WalletStatusSuspended, false},
		{"Locked wallet can credit", WalletStatusLocked, false},
		{"Closed wallet cannot credit", WalletStatusClosed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := &Wallet{
				id:     uuid.New(),
				status: tt.status,
			}
			err := wallet.CanCredit()
			if (err != nil) != tt.wantError {
				t.Errorf("CanCredit() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestWallet_HasSufficientBalance tests balance checks
func TestWallet_HasSufficientBalance(t *testing.T) {
	currency := valueobjects.USD
	available, _ := valueobjects.NewMoneyFromInt(100, currency)

	wallet := &Wallet{
		currency: currency,
		balance: Balance{
			available: available,
			pending:   valueobjects.Zero(currency),
			version:   0,
		},
	}

	tests := []struct {
		name     string
		amount   valueobjects.Money
		expected bool
	}{
		{
			name:     "Sufficient balance",
			amount:   mustMoney(valueobjects.NewMoneyFromInt(50, currency)),
			expected: true,
		},
		{
			name:     "Exact balance",
			amount:   available,
			expected: true,
		},
		{
			name:     "Insufficient balance",
			amount:   mustMoney(valueobjects.NewMoneyFromInt(150, currency)),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wallet.HasSufficientBalance(tt.amount)
			if err != nil {
				t.Fatalf("HasSufficientBalance() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("HasSufficientBalance() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestWallet_Credit tests crediting wallet
func TestWallet_Credit(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Successful credit", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)

		err := wallet.Credit(amount)
		if err != nil {
			t.Fatalf("Credit() error = %v, want nil", err)
		}

		if !wallet.AvailableBalance().Equals(amount) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), amount)
		}

		if wallet.BalanceVersion() != 1 {
			t.Errorf("BalanceVersion = %v, want 1", wallet.BalanceVersion())
		}
	})

	t.Run("Credit closed wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusClosed
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)

		err := wallet.Credit(amount)
		if err == nil {
			t.Fatal("Credit() on closed wallet should return error")
		}
	})

	t.Run("Currency mismatch", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.EUR)

		err := wallet.Credit(amount)
		if err == nil {
			t.Fatal("Credit() with different currency should return error")
		}
	})

	t.Run("Credit multiple times increases balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount1, _ := valueobjects.NewMoneyFromInt(100, currency)
		amount2, _ := valueobjects.NewMoneyFromInt(50, currency)

		_ = wallet.Credit(amount1)
		_ = wallet.Credit(amount2)

		expected, _ := valueobjects.NewMoneyFromInt(150, currency)
		if !wallet.AvailableBalance().Equals(expected) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), expected)
		}

		if wallet.BalanceVersion() != 2 {
			t.Errorf("BalanceVersion = %v, want 2", wallet.BalanceVersion())
		}
	})

	t.Run("Credit zero amount", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		zeroAmount := valueobjects.Zero(currency)

		err := wallet.Credit(zeroAmount)
		if err != nil {
			t.Fatalf("Credit() with zero amount error = %v", err)
		}

		if !wallet.AvailableBalance().IsZero() {
			t.Errorf("Balance should remain zero")
		}
	})

	t.Run("Credit suspended wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusSuspended
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)

		// Suspended wallets CAN receive credits
		err := wallet.Credit(amount)
		if err != nil {
			t.Fatalf("Credit() on suspended wallet should succeed, got error: %v", err)
		}
	})
}

// TestWallet_Debit tests debiting wallet
func TestWallet_Debit(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Successful debit", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		initialAmount, _ := valueobjects.NewMoneyFromInt(100, currency)
		debitAmount, _ := valueobjects.NewMoneyFromInt(30, currency)

		_ = wallet.Credit(initialAmount)
		initialVersion := wallet.BalanceVersion()

		err := wallet.Debit(debitAmount)
		if err != nil {
			t.Fatalf("Debit() error = %v, want nil", err)
		}

		expected, _ := valueobjects.NewMoneyFromInt(70, currency)
		if !wallet.AvailableBalance().Equals(expected) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), expected)
		}

		if wallet.BalanceVersion() != initialVersion+1 {
			t.Errorf("BalanceVersion not incremented")
		}
	})

	t.Run("Debit suspended wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusSuspended
		amount, _ := valueobjects.NewMoneyFromInt(10, currency)

		err := wallet.Debit(amount)
		if err == nil {
			t.Fatal("Debit() on suspended wallet should return error")
		}
	})

	t.Run("Insufficient balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)

		err := wallet.Debit(amount)
		if err == nil {
			t.Fatal("Debit() with insufficient balance should return error")
		}
	})

	t.Run("Currency mismatch", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		amount, _ := valueobjects.NewMoneyFromInt(10, valueobjects.EUR)

		err := wallet.Debit(amount)
		if err == nil {
			t.Fatal("Debit() with different currency should return error")
		}
	})

	t.Run("Debit exact balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)
		_ = wallet.Credit(amount)

		err := wallet.Debit(amount)
		if err != nil {
			t.Fatalf("Debit() exact balance error = %v", err)
		}

		if !wallet.AvailableBalance().IsZero() {
			t.Errorf("Balance should be zero, got %v", wallet.AvailableBalance())
		}
	})

	t.Run("Debit zero amount", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		zeroAmount := valueobjects.Zero(currency)

		err := wallet.Debit(zeroAmount)
		if err != nil {
			t.Fatalf("Debit() zero amount error = %v", err)
		}
	})
}

// TestWallet_Reserve tests reserving funds
func TestWallet_Reserve(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Successful reserve", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		initialAmount, _ := valueobjects.NewMoneyFromInt(100, currency)
		reserveAmount, _ := valueobjects.NewMoneyFromInt(30, currency)

		_ = wallet.Credit(initialAmount)

		err := wallet.Reserve(reserveAmount)
		if err != nil {
			t.Fatalf("Reserve() error = %v, want nil", err)
		}

		expectedAvailable, _ := valueobjects.NewMoneyFromInt(70, currency)
		if !wallet.AvailableBalance().Equals(expectedAvailable) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), expectedAvailable)
		}

		if !wallet.PendingBalance().Equals(reserveAmount) {
			t.Errorf("PendingBalance = %v, want %v", wallet.PendingBalance(), reserveAmount)
		}
	})

	t.Run("Reserve insufficient balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)

		err := wallet.Reserve(amount)
		if err == nil {
			t.Fatal("Reserve() with insufficient balance should return error")
		}
	})

	t.Run("Reserve on inactive wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusSuspended
		amount, _ := valueobjects.NewMoneyFromInt(10, currency)

		err := wallet.Reserve(amount)
		if err == nil {
			t.Fatal("Reserve() on inactive wallet should return error")
		}
	})

	t.Run("Multiple reserves accumulate pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))

		reserve1, _ := valueobjects.NewMoneyFromInt(20, currency)
		reserve2, _ := valueobjects.NewMoneyFromInt(15, currency)

		_ = wallet.Reserve(reserve1)
		_ = wallet.Reserve(reserve2)

		expectedPending, _ := valueobjects.NewMoneyFromInt(35, currency)
		if !wallet.PendingBalance().Equals(expectedPending) {
			t.Errorf("PendingBalance = %v, want %v", wallet.PendingBalance(), expectedPending)
		}

		expectedAvailable, _ := valueobjects.NewMoneyFromInt(65, currency)
		if !wallet.AvailableBalance().Equals(expectedAvailable) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), expectedAvailable)
		}
	})

	t.Run("Reserve exact balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		amount, _ := valueobjects.NewMoneyFromInt(100, currency)
		_ = wallet.Credit(amount)

		err := wallet.Reserve(amount)
		if err != nil {
			t.Fatalf("Reserve() exact balance error = %v", err)
		}

		if !wallet.AvailableBalance().IsZero() {
			t.Errorf("Available should be zero")
		}

		if !wallet.PendingBalance().Equals(amount) {
			t.Errorf("Pending = %v, want %v", wallet.PendingBalance(), amount)
		}
	})
}

// TestWallet_Release tests releasing reserved funds
func TestWallet_Release(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Successful release", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		initialAmount, _ := valueobjects.NewMoneyFromInt(100, currency)
		reserveAmount, _ := valueobjects.NewMoneyFromInt(30, currency)

		_ = wallet.Credit(initialAmount)
		_ = wallet.Reserve(reserveAmount)

		err := wallet.Release(reserveAmount)
		if err != nil {
			t.Fatalf("Release() error = %v, want nil", err)
		}

		if !wallet.AvailableBalance().Equals(initialAmount) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), initialAmount)
		}

		if !wallet.PendingBalance().IsZero() {
			t.Errorf("PendingBalance should be zero, got %v", wallet.PendingBalance())
		}
	})

	t.Run("Release more than pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(30, currency)))

		releaseAmount, _ := valueobjects.NewMoneyFromInt(50, currency)
		err := wallet.Release(releaseAmount)
		if err == nil {
			t.Fatal("Release() more than pending should return error")
		}
	})

	t.Run("Release partial pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(30, currency)))

		releaseAmount, _ := valueobjects.NewMoneyFromInt(10, currency)
		err := wallet.Release(releaseAmount)
		if err != nil {
			t.Fatalf("Release() error = %v", err)
		}

		expectedPending, _ := valueobjects.NewMoneyFromInt(20, currency)
		if !wallet.PendingBalance().Equals(expectedPending) {
			t.Errorf("PendingBalance = %v, want %v", wallet.PendingBalance(), expectedPending)
		}
	})

	t.Run("Release exact pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		reserveAmount, _ := valueobjects.NewMoneyFromInt(30, currency)
		_ = wallet.Reserve(reserveAmount)

		err := wallet.Release(reserveAmount)
		if err != nil {
			t.Fatalf("Release() exact pending error = %v", err)
		}

		if !wallet.PendingBalance().IsZero() {
			t.Errorf("Pending should be zero")
		}
	})

	t.Run("Release with zero pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))

		releaseAmount, _ := valueobjects.NewMoneyFromInt(10, currency)
		err := wallet.Release(releaseAmount)
		if err == nil {
			t.Fatal("Release() with zero pending should return error")
		}
	})
}

// TestWallet_CompletePending tests completing pending transactions
func TestWallet_CompletePending(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Successful complete", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		reserveAmount, _ := valueobjects.NewMoneyFromInt(30, currency)
		_ = wallet.Reserve(reserveAmount)

		err := wallet.CompletePending(reserveAmount)
		if err != nil {
			t.Fatalf("CompletePending() error = %v, want nil", err)
		}

		if !wallet.PendingBalance().IsZero() {
			t.Errorf("PendingBalance should be zero, got %v", wallet.PendingBalance())
		}

		expectedAvailable, _ := valueobjects.NewMoneyFromInt(70, currency)
		if !wallet.AvailableBalance().Equals(expectedAvailable) {
			t.Errorf("AvailableBalance = %v, want %v", wallet.AvailableBalance(), expectedAvailable)
		}
	})

	t.Run("Complete more than pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(30, currency)))

		completeAmount, _ := valueobjects.NewMoneyFromInt(50, currency)
		err := wallet.CompletePending(completeAmount)
		if err == nil {
			t.Fatal("CompletePending() more than pending should return error")
		}
	})

	t.Run("Complete exact pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		reserveAmount, _ := valueobjects.NewMoneyFromInt(30, currency)
		_ = wallet.Reserve(reserveAmount)

		err := wallet.CompletePending(reserveAmount)
		if err != nil {
			t.Fatalf("CompletePending() exact amount error = %v", err)
		}

		if !wallet.PendingBalance().IsZero() {
			t.Errorf("Pending should be zero")
		}
	})

	t.Run("Complete partial pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(30, currency)))

		completeAmount, _ := valueobjects.NewMoneyFromInt(10, currency)
		err := wallet.CompletePending(completeAmount)
		if err != nil {
			t.Fatalf("CompletePending() partial error = %v", err)
		}

		expectedPending, _ := valueobjects.NewMoneyFromInt(20, currency)
		if !wallet.PendingBalance().Equals(expectedPending) {
			t.Errorf("Pending = %v, want %v", wallet.PendingBalance(), expectedPending)
		}
	})
}

// TestWallet_Suspend tests suspending wallet
func TestWallet_Suspend(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Suspend active wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)

		err := wallet.Suspend()
		if err != nil {
			t.Fatalf("Suspend() error = %v, want nil", err)
		}

		if wallet.Status() != WalletStatusSuspended {
			t.Errorf("Status = %v, want %v", wallet.Status(), WalletStatusSuspended)
		}
	})

	t.Run("Suspend closed wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusClosed

		err := wallet.Suspend()
		if err == nil {
			t.Fatal("Suspend() closed wallet should return error")
		}
	})
}

// TestWallet_Activate tests activating wallet
func TestWallet_Activate(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Activate suspended wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Suspend()

		err := wallet.Activate()
		if err != nil {
			t.Fatalf("Activate() error = %v, want nil", err)
		}

		if wallet.Status() != WalletStatusActive {
			t.Errorf("Status = %v, want %v", wallet.Status(), WalletStatusActive)
		}
	})

	t.Run("Activate closed wallet", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		wallet.status = WalletStatusClosed

		err := wallet.Activate()
		if err == nil {
			t.Fatal("Activate() closed wallet should return error")
		}
	})
}

// TestWallet_Lock tests locking wallet
func TestWallet_Lock(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	wallet, _ := NewWallet(userID, currency)

	err := wallet.Lock()
	if err != nil {
		t.Fatalf("Lock() error = %v, want nil", err)
	}

	if wallet.Status() != WalletStatusLocked {
		t.Errorf("Status = %v, want %v", wallet.Status(), WalletStatusLocked)
	}
}

// TestWallet_Close tests closing wallet
func TestWallet_Close(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Close wallet with zero balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)

		err := wallet.Close()
		if err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}

		if wallet.Status() != WalletStatusClosed {
			t.Errorf("Status = %v, want %v", wallet.Status(), WalletStatusClosed)
		}
	})

	t.Run("Close wallet with non-zero available balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))

		err := wallet.Close()
		if err == nil {
			t.Fatal("Close() with non-zero balance should return error")
		}
	})

	t.Run("Close wallet with non-zero pending balance", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
		_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))

		err := wallet.Close()
		if err == nil {
			t.Fatal("Close() with pending balance should return error")
		}
	})

	t.Run("Close wallet with available but no pending", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(50, currency)))

		err := wallet.Close()
		if err == nil {
			t.Fatal("Close() with available balance should return error")
		}
	})
}

// TestWallet_UpdateLimits tests updating transaction limits
func TestWallet_UpdateLimits(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	t.Run("Update limits successfully", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		newDaily, _ := valueobjects.NewMoneyFromInt(5000, currency)
		newMonthly, _ := valueobjects.NewMoneyFromInt(20000, currency)

		err := wallet.UpdateLimits(newDaily, newMonthly)
		if err != nil {
			t.Fatalf("UpdateLimits() error = %v, want nil", err)
		}

		if !wallet.DailyLimit().Equals(newDaily) {
			t.Errorf("DailyLimit = %v, want %v", wallet.DailyLimit(), newDaily)
		}

		if !wallet.MonthlyLimit().Equals(newMonthly) {
			t.Errorf("MonthlyLimit = %v, want %v", wallet.MonthlyLimit(), newMonthly)
		}
	})

	t.Run("Update limits with wrong currency", func(t *testing.T) {
		wallet, _ := NewWallet(userID, currency)
		newDaily, _ := valueobjects.NewMoneyFromInt(5000, valueobjects.EUR)
		newMonthly, _ := valueobjects.NewMoneyFromInt(20000, currency)

		err := wallet.UpdateLimits(newDaily, newMonthly)
		if err == nil {
			t.Fatal("UpdateLimits() with wrong currency should return error")
		}
	})
}

// TestWallet_TotalBalance tests calculating total balance
func TestWallet_TotalBalance(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD

	wallet, _ := NewWallet(userID, currency)
	_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))
	_ = wallet.Reserve(mustMoney(valueobjects.NewMoneyFromInt(30, currency)))

	total, err := wallet.TotalBalance()
	if err != nil {
		t.Fatalf("TotalBalance() error = %v", err)
	}

	expected, _ := valueobjects.NewMoneyFromInt(100, currency)
	if !total.Equals(expected) {
		t.Errorf("TotalBalance = %v, want %v", total, expected)
	}
}

// TestWallet_Getters tests all getter methods
func TestWallet_Getters(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	currency := valueobjects.USD
	available, _ := valueobjects.NewMoneyFromInt(100, currency)
	pending, _ := valueobjects.NewMoneyFromInt(20, currency)
	dailyLimit, _ := valueobjects.NewMoneyFromInt(1000, currency)
	monthlyLimit, _ := valueobjects.NewMoneyFromInt(5000, currency)
	now := time.Now()

	wallet := ReconstructWallet(
		id, userID,
		currency,
		WalletTypeFiat,
		WalletStatusActive,
		available, pending,
		5,
		dailyLimit, monthlyLimit,
		now, now,
	)

	if wallet.ID() != id {
		t.Errorf("ID() = %v, want %v", wallet.ID(), id)
	}
	if wallet.UserID() != userID {
		t.Errorf("UserID() = %v, want %v", wallet.UserID(), userID)
	}
	if !wallet.Currency().Equals(currency) {
		t.Errorf("Currency() = %v, want %v", wallet.Currency(), currency)
	}
	if wallet.WalletType() != WalletTypeFiat {
		t.Errorf("WalletType() = %v, want %v", wallet.WalletType(), WalletTypeFiat)
	}
	if wallet.Status() != WalletStatusActive {
		t.Errorf("Status() = %v, want %v", wallet.Status(), WalletStatusActive)
	}
	if !wallet.AvailableBalance().Equals(available) {
		t.Errorf("AvailableBalance() = %v, want %v", wallet.AvailableBalance(), available)
	}
	if !wallet.PendingBalance().Equals(pending) {
		t.Errorf("PendingBalance() = %v, want %v", wallet.PendingBalance(), pending)
	}
	if wallet.BalanceVersion() != 5 {
		t.Errorf("BalanceVersion() = %v, want 5", wallet.BalanceVersion())
	}
	if !wallet.DailyLimit().Equals(dailyLimit) {
		t.Errorf("DailyLimit() = %v, want %v", wallet.DailyLimit(), dailyLimit)
	}
	if !wallet.MonthlyLimit().Equals(monthlyLimit) {
		t.Errorf("MonthlyLimit() = %v, want %v", wallet.MonthlyLimit(), monthlyLimit)
	}
	if !wallet.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt() = %v, want %v", wallet.CreatedAt(), now)
	}
	if !wallet.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() = %v, want %v", wallet.UpdatedAt(), now)
	}
}

// TestWallet_UpdatedAtChanges tests that UpdatedAt changes on operations
func TestWallet_UpdatedAtChanges(t *testing.T) {
	userID := uuid.New()
	currency := valueobjects.USD
	wallet, _ := NewWallet(userID, currency)

	initialUpdatedAt := wallet.UpdatedAt()
	time.Sleep(10 * time.Millisecond)

	_ = wallet.Credit(mustMoney(valueobjects.NewMoneyFromInt(100, currency)))

	if !wallet.UpdatedAt().After(initialUpdatedAt) {
		t.Error("UpdatedAt should change after Credit operation")
	}
}

// Helper function for tests
func mustMoney(m valueobjects.Money, err error) valueobjects.Money {
	if err != nil {
		panic(err)
	}
	return m
}
