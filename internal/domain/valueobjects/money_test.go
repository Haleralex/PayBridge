// Package valueobjects_test demonstrates domain layer testing.
// Domain tests have NO external dependencies - pure unit tests.
//
// Testing Principles:
// - Test business rules and invariants
// - Test value object immutability
// - Test error conditions
// - No mocks needed (pure domain logic)
package valueobjects_test

import (
	"math/big"
	"testing"

	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// TestNewMoney_Success tests successful money creation.
func TestNewMoney_Success(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency valueobjects.Currency
		wantErr  bool
	}{
		{
			name:     "Valid USD amount",
			amount:   "100.50",
			currency: valueobjects.USD,
			wantErr:  false,
		},
		{
			name:     "Zero amount",
			amount:   "0",
			currency: valueobjects.EUR,
			wantErr:  false,
		},
		{
			name:     "Crypto with 8 decimals",
			amount:   "0.00000001",
			currency: valueobjects.BTC,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := valueobjects.NewMoney(tt.amount, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMoney() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && money.Currency().Code() != tt.currency.Code() {
				t.Errorf("Currency mismatch: got %v, want %v", money.Currency(), tt.currency)
			}
		})
	}
}

// TestNewMoney_NegativeAmount tests that negative amounts are rejected.
// Business Rule: Money cannot be negative.
func TestNewMoney_NegativeAmount(t *testing.T) {
	_, err := valueobjects.NewMoney("-100.50", valueobjects.USD)
	if err == nil {
		t.Error("Expected error for negative amount, got nil")
	}
}

// TestNewMoney_InvalidFormat tests invalid amount formats.
func TestNewMoney_InvalidFormat(t *testing.T) {
	invalidAmounts := []string{"abc", "12.34.56", "", "not-a-number"}

	for _, amount := range invalidAmounts {
		t.Run(amount, func(t *testing.T) {
			_, err := valueobjects.NewMoney(amount, valueobjects.USD)
			if err == nil {
				t.Errorf("Expected error for invalid amount %q, got nil", amount)
			}
		})
	}
}

// TestMoney_Add tests addition operation.
// Business Rule: Can only add same currency.
func TestMoney_Add(t *testing.T) {
	t.Run("Same currency addition", func(t *testing.T) {
		m1, _ := valueobjects.NewMoney("100.50", valueobjects.USD)
		m2, _ := valueobjects.NewMoney("50.25", valueobjects.USD)

		result, err := m1.Add(m2)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected, _ := valueobjects.NewMoney("150.75", valueobjects.USD)
		if !result.Equals(expected) {
			t.Errorf("Add result incorrect: got %v, want %v", result, expected)
		}
	})

	t.Run("Different currency addition fails", func(t *testing.T) {
		m1, _ := valueobjects.NewMoney("100", valueobjects.USD)
		m2, _ := valueobjects.NewMoney("100", valueobjects.EUR)

		_, err := m1.Add(m2)
		if err == nil {
			t.Error("Expected error when adding different currencies")
		}
	})
}

// TestMoney_Subtract tests subtraction with insufficient balance check.
func TestMoney_Subtract(t *testing.T) {
	t.Run("Valid subtraction", func(t *testing.T) {
		m1, _ := valueobjects.NewMoney("100", valueobjects.USD)
		m2, _ := valueobjects.NewMoney("30", valueobjects.USD)

		result, err := m1.Subtract(m2)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected, _ := valueobjects.NewMoney("70", valueobjects.USD)
		if !result.Equals(expected) {
			t.Errorf("Subtract result incorrect: got %v, want %v", result, expected)
		}
	})

	t.Run("Insufficient amount", func(t *testing.T) {
		m1, _ := valueobjects.NewMoney("50", valueobjects.USD)
		m2, _ := valueobjects.NewMoney("100", valueobjects.USD)

		_, err := m1.Subtract(m2)
		if err == nil {
			t.Error("Expected error for insufficient amount")
		}
	})
}

// TestMoney_Multiply tests multiplication for fee calculation.
func TestMoney_Multiply(t *testing.T) {
	money, _ := valueobjects.NewMoney("100", valueobjects.USD)

	// 3% fee calculation
	feeRate := big.NewRat(3, 100) // 0.03
	fee := money.Multiply(feeRate)

	expected, _ := valueobjects.NewMoney("3", valueobjects.USD)
	if !fee.Equals(expected) {
		t.Errorf("Fee calculation incorrect: got %v, want %v", fee, expected)
	}
}

// TestMoney_Immutability tests that money operations don't modify the original.
// Value Object Pattern: Immutability is critical.
func TestMoney_Immutability(t *testing.T) {
	original, _ := valueobjects.NewMoney("100", valueobjects.USD)
	originalCents := original.Cents()

	// Perform operations
	addend, _ := valueobjects.NewMoney("50", valueobjects.USD)
	_, _ = original.Add(addend)

	// Original should be unchanged
	if original.Cents() != originalCents {
		t.Error("Money was mutated by Add operation (immutability violated)")
	}
}

// TestMoney_Comparison tests comparison operations.
func TestMoney_Comparison(t *testing.T) {
	m1, _ := valueobjects.NewMoney("100", valueobjects.USD)
	m2, _ := valueobjects.NewMoney("50", valueobjects.USD)
	m3, _ := valueobjects.NewMoney("100", valueobjects.USD)

	t.Run("GreaterThan", func(t *testing.T) {
		gt, err := m1.GreaterThan(m2)
		if err != nil || !gt {
			t.Error("100 should be greater than 50")
		}
	})

	t.Run("Equals", func(t *testing.T) {
		if !m1.Equals(m3) {
			t.Error("100 should equal 100")
		}
	})

	t.Run("LessThan", func(t *testing.T) {
		lt, err := m2.LessThan(m1)
		if err != nil || !lt {
			t.Error("50 should be less than 100")
		}
	})
}

// TestMoney_Cents tests the cents conversion (database storage format).
func TestMoney_Cents(t *testing.T) {
	tests := []struct {
		name      string
		amount    string
		currency  valueobjects.Currency
		wantCents int64
	}{
		{
			name:      "USD with cents",
			amount:    "100.50",
			currency:  valueobjects.USD,
			wantCents: 10050,
		},
		{
			name:      "Whole USD amount",
			amount:    "100",
			currency:  valueobjects.USD,
			wantCents: 10000,
		},
		{
			name:      "BTC (8 decimals)",
			amount:    "1",
			currency:  valueobjects.BTC,
			wantCents: 100000000, // 1 BTC = 100M satoshis
		},
		{
			name:      "Small BTC amount",
			amount:    "0.00000001",
			currency:  valueobjects.BTC,
			wantCents: 1, // 1 satoshi
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, _ := valueobjects.NewMoney(tt.amount, tt.currency)
			if money.Cents() != tt.wantCents {
				t.Errorf("Cents() = %v, want %v", money.Cents(), tt.wantCents)
			}
		})
	}
}

// TestNewMoneyFromCents tests creating money from cents (DB -> domain).
func TestNewMoneyFromCents(t *testing.T) {
	tests := []struct {
		name       string
		cents      int64
		currency   valueobjects.Currency
		wantAmount string
	}{
		{
			name:       "USD cents to dollars",
			cents:      10050,
			currency:   valueobjects.USD,
			wantAmount: "100.50",
		},
		{
			name:       "Satoshis to BTC",
			cents:      100000000,
			currency:   valueobjects.BTC,
			wantAmount: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := valueobjects.NewMoneyFromCents(tt.cents, tt.currency)
			if err != nil {
				t.Fatalf("NewMoneyFromCents() error = %v", err)
			}

			expected, _ := valueobjects.NewMoney(tt.wantAmount, tt.currency)
			if !money.Equals(expected) {
				t.Errorf("Amount mismatch: got %v, want %v", money, expected)
			}
		})
	}
}

// TestNewMoneyFromInt tests creating money from integer amounts.
func TestNewMoneyFromInt(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		currency valueobjects.Currency
		wantErr  bool
	}{
		{
			name:     "Positive integer",
			amount:   100,
			currency: valueobjects.USD,
			wantErr:  false,
		},
		{
			name:     "Zero",
			amount:   0,
			currency: valueobjects.EUR,
			wantErr:  false,
		},
		{
			name:     "Negative integer",
			amount:   -50,
			currency: valueobjects.USD,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := valueobjects.NewMoneyFromInt(tt.amount, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMoneyFromInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && money.Currency().Code() != tt.currency.Code() {
				t.Errorf("Currency mismatch: got %v, want %v", money.Currency(), tt.currency)
			}
		})
	}
}

// TestNewMoneyFromCents_NegativeAmount tests that negative cents are rejected.
func TestNewMoneyFromCents_NegativeAmount(t *testing.T) {
	_, err := valueobjects.NewMoneyFromCents(-100, valueobjects.USD)
	if err == nil {
		t.Error("Expected error for negative cents, got nil")
	}
}

// TestZero tests the Zero constructor.
func TestZero(t *testing.T) {
	zero := valueobjects.Zero(valueobjects.USD)

	if !zero.IsZero() {
		t.Error("Zero() should create a zero amount")
	}

	if zero.Currency().Code() != valueobjects.USD.Code() {
		t.Errorf("Currency mismatch: got %v, want USD", zero.Currency())
	}

	if zero.Cents() != 0 {
		t.Errorf("Zero cents should be 0, got %d", zero.Cents())
	}
}

// TestMoney_Currency tests the Currency accessor.
func TestMoney_Currency(t *testing.T) {
	money, _ := valueobjects.NewMoney("100", valueobjects.EUR)

	if money.Currency().Code() != "EUR" {
		t.Errorf("Currency() = %v, want EUR", money.Currency())
	}
}

// TestMoney_Amount tests the Amount accessor returns a copy.
func TestMoney_Amount(t *testing.T) {
	money, _ := valueobjects.NewMoney("100.50", valueobjects.USD)

	amount := money.Amount()
	// Modify the returned amount
	amount.Add(amount, big.NewRat(50, 1))

	// Original money should be unchanged
	if money.Float64() != 100.50 {
		t.Error("Amount() should return a copy, not the original (immutability violated)")
	}
}

// TestMoney_String tests the string representation.
func TestMoney_String(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency valueobjects.Currency
		want     string
	}{
		{
			name:     "USD with cents",
			amount:   "100.50",
			currency: valueobjects.USD,
			want:     "100.50 USD",
		},
		{
			name:     "BTC with 8 decimals",
			amount:   "0.00000001",
			currency: valueobjects.BTC,
			want:     "0.00000001 BTC",
		},
		{
			name:     "Whole number",
			amount:   "1000",
			currency: valueobjects.EUR,
			want:     "1000.00 EUR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, _ := valueobjects.NewMoney(tt.amount, tt.currency)
			if money.String() != tt.want {
				t.Errorf("String() = %v, want %v", money.String(), tt.want)
			}
		})
	}
}

// TestMoney_Float64 tests float64 conversion.
func TestMoney_Float64(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency valueobjects.Currency
		want     float64
	}{
		{
			name:     "USD amount",
			amount:   "100.50",
			currency: valueobjects.USD,
			want:     100.50,
		},
		{
			name:     "Zero",
			amount:   "0",
			currency: valueobjects.USD,
			want:     0.0,
		},
		{
			name:     "Large amount",
			amount:   "999999.99",
			currency: valueobjects.EUR,
			want:     999999.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, _ := valueobjects.NewMoney(tt.amount, tt.currency)
			got := money.Float64()
			// Use approximate comparison for floats
			if got < tt.want-0.01 || got > tt.want+0.01 {
				t.Errorf("Float64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMoney_IsZero tests zero checking.
func TestMoney_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		want   bool
	}{
		{name: "Zero", amount: "0", want: true},
		{name: "Non-zero", amount: "100", want: false},
		{name: "Small amount", amount: "0.01", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, _ := valueobjects.NewMoney(tt.amount, valueobjects.USD)
			if money.IsZero() != tt.want {
				t.Errorf("IsZero() = %v, want %v", money.IsZero(), tt.want)
			}
		})
	}
}

// TestMoney_IsPositive tests positive checking.
func TestMoney_IsPositive(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		want   bool
	}{
		{name: "Positive", amount: "100", want: true},
		{name: "Zero", amount: "0", want: false},
		{name: "Small positive", amount: "0.01", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, _ := valueobjects.NewMoney(tt.amount, valueobjects.USD)
			if money.IsPositive() != tt.want {
				t.Errorf("IsPositive() = %v, want %v", money.IsPositive(), tt.want)
			}
		})
	}
}

// TestMoney_GreaterThanOrEqual tests >= comparison.
func TestMoney_GreaterThanOrEqual(t *testing.T) {
	m1, _ := valueobjects.NewMoney("100", valueobjects.USD)
	m2, _ := valueobjects.NewMoney("50", valueobjects.USD)
	m3, _ := valueobjects.NewMoney("100", valueobjects.USD)

	t.Run("Greater", func(t *testing.T) {
		gte, err := m1.GreaterThanOrEqual(m2)
		if err != nil || !gte {
			t.Error("100 should be >= 50")
		}
	})

	t.Run("Equal", func(t *testing.T) {
		gte, err := m1.GreaterThanOrEqual(m3)
		if err != nil || !gte {
			t.Error("100 should be >= 100")
		}
	})

	t.Run("Less", func(t *testing.T) {
		gte, err := m2.GreaterThanOrEqual(m1)
		if err != nil || gte {
			t.Error("50 should not be >= 100")
		}
	})

	t.Run("Different currencies", func(t *testing.T) {
		mEUR, _ := valueobjects.NewMoney("100", valueobjects.EUR)
		_, err := m1.GreaterThanOrEqual(mEUR)
		if err == nil {
			t.Error("Expected error when comparing different currencies")
		}
	})
}

// TestMoney_Subtract_DifferentCurrencies tests subtraction with currency mismatch.
func TestMoney_Subtract_DifferentCurrencies(t *testing.T) {
	m1, _ := valueobjects.NewMoney("100", valueobjects.USD)
	m2, _ := valueobjects.NewMoney("50", valueobjects.EUR)

	_, err := m1.Subtract(m2)
	if err == nil {
		t.Error("Expected error when subtracting different currencies")
	}
}

// TestMoney_Multiply_Precision tests multiplication preserves precision.
func TestMoney_Multiply_Precision(t *testing.T) {
	money, _ := valueobjects.NewMoney("100.33", valueobjects.USD)
	factor := big.NewRat(3, 1) // multiply by 3

	result := money.Multiply(factor)
	expected, _ := valueobjects.NewMoney("300.99", valueobjects.USD)

	if !result.Equals(expected) {
		t.Errorf("Multiply precision lost: got %v, want %v", result, expected)
	}
}

// TestMoney_Multiply_Zero tests multiplication by zero.
func TestMoney_Multiply_Zero(t *testing.T) {
	money, _ := valueobjects.NewMoney("100", valueobjects.USD)
	result := money.Multiply(big.NewRat(0, 1))

	if !result.IsZero() {
		t.Error("Multiplying by zero should result in zero")
	}
}

// TestMoney_Comparison_DifferentCurrencies tests comparison error handling.
func TestMoney_Comparison_DifferentCurrencies(t *testing.T) {
	mUSD, _ := valueobjects.NewMoney("100", valueobjects.USD)
	mEUR, _ := valueobjects.NewMoney("100", valueobjects.EUR)

	t.Run("GreaterThan different currencies", func(t *testing.T) {
		_, err := mUSD.GreaterThan(mEUR)
		if err == nil {
			t.Error("Expected error when comparing different currencies")
		}
	})

	t.Run("LessThan different currencies", func(t *testing.T) {
		_, err := mUSD.LessThan(mEUR)
		if err == nil {
			t.Error("Expected error when comparing different currencies")
		}
	})
}

// TestMoney_Equals_DifferentCurrencies tests equals with different currencies.
func TestMoney_Equals_DifferentCurrencies(t *testing.T) {
	mUSD, _ := valueobjects.NewMoney("100", valueobjects.USD)
	mEUR, _ := valueobjects.NewMoney("100", valueobjects.EUR)

	if mUSD.Equals(mEUR) {
		t.Error("Money with different currencies should not be equal")
	}
}

// TestMoney_Add_Zero tests adding zero.
func TestMoney_Add_Zero(t *testing.T) {
	money, _ := valueobjects.NewMoney("100.50", valueobjects.USD)
	zero := valueobjects.Zero(valueobjects.USD)

	result, err := money.Add(zero)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Equals(money) {
		t.Errorf("Adding zero should not change the amount: got %v, want %v", result, money)
	}
}

// TestMoney_Subtract_ToZero tests subtracting to exactly zero.
func TestMoney_Subtract_ToZero(t *testing.T) {
	money, _ := valueobjects.NewMoney("100", valueobjects.USD)
	same, _ := valueobjects.NewMoney("100", valueobjects.USD)

	result, err := money.Subtract(same)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.IsZero() {
		t.Errorf("Subtracting same amount should result in zero: got %v", result)
	}
}

// BenchmarkMoney_Add benchmarks addition performance.
func BenchmarkMoney_Add(b *testing.B) {
	m1, _ := valueobjects.NewMoney("100.50", valueobjects.USD)
	m2, _ := valueobjects.NewMoney("50.25", valueobjects.USD)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m1.Add(m2)
	}
}
