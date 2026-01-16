// Package valueobjects_test demonstrates testing value objects.
package valueobjects_test

import (
	"testing"

	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// TestNewCurrency_Success tests successful currency creation.
func TestNewCurrency_Success(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "USD", code: "USD", want: "USD"},
		{name: "EUR", code: "EUR", want: "EUR"},
		{name: "GBP", code: "GBP", want: "GBP"},
		{name: "BTC", code: "BTC", want: "BTC"},
		{name: "ETH", code: "ETH", want: "ETH"},
		{name: "USDT", code: "USDT", want: "USDT"},
		{name: "USDC", code: "USDC", want: "USDC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curr, err := valueobjects.NewCurrency(tt.code)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if curr.Code() != tt.want {
				t.Errorf("Code() = %v, want %v", curr.Code(), tt.want)
			}
		})
	}
}

// TestNewCurrency_Invalid tests invalid currency codes.
func TestNewCurrency_Invalid(t *testing.T) {
	invalidCodes := []string{
		"XXX",
		"INVALID",
		"",
		"RUB",
		"JPY",
		"123",
	}

	for _, code := range invalidCodes {
		t.Run(code, func(t *testing.T) {
			_, err := valueobjects.NewCurrency(code)
			if err == nil {
				t.Errorf("Expected error for invalid code %q, got nil", code)
			}
			if err != valueobjects.ErrInvalidCurrency {
				t.Errorf("Expected ErrInvalidCurrency, got %v", err)
			}
		})
	}
}

// TestNewCurrency_CaseInsensitive tests normalization.
func TestNewCurrency_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "usd", want: "USD"},
		{input: "Usd", want: "USD"},
		{input: "USD", want: "USD"},
		{input: "btc", want: "BTC"},
		{input: "BtC", want: "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			curr, err := valueobjects.NewCurrency(tt.input)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if curr.Code() != tt.want {
				t.Errorf("Code() = %v, want %v", curr.Code(), tt.want)
			}
		})
	}
}

// TestNewCurrency_Whitespace tests trimming.
func TestNewCurrency_Whitespace(t *testing.T) {
	tests := []string{
		" USD ",
		"  EUR  ",
		"\tBTC\t",
		"\nETH\n",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			curr, err := valueobjects.NewCurrency(input)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			// Should have no whitespace in code
			if len(curr.Code()) != 3 && len(curr.Code()) != 4 {
				t.Errorf("Code length unexpected: %d", len(curr.Code()))
			}
		})
	}
}

// TestMustNewCurrency_Success tests MustNewCurrency with valid code.
func TestMustNewCurrency_Success(t *testing.T) {
	// Should not panic
	curr := valueobjects.MustNewCurrency("USD")
	if curr.Code() != "USD" {
		t.Errorf("Code() = %v, want USD", curr.Code())
	}
}

// TestMustNewCurrency_Panic tests MustNewCurrency panics on invalid code.
func TestMustNewCurrency_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, but didn't panic")
		}
	}()

	// Should panic
	valueobjects.MustNewCurrency("INVALID")
}

// TestCurrency_Code tests the Code accessor.
func TestCurrency_Code(t *testing.T) {
	curr := valueobjects.USD
	if curr.Code() != "USD" {
		t.Errorf("Code() = %v, want USD", curr.Code())
	}
}

// TestCurrency_Equals tests equality comparison.
func TestCurrency_Equals(t *testing.T) {
	usd1 := valueobjects.USD
	usd2, _ := valueobjects.NewCurrency("USD")
	eur := valueobjects.EUR

	if !usd1.Equals(usd2) {
		t.Error("Expected USD to equal USD")
	}

	if usd1.Equals(eur) {
		t.Error("Expected USD not to equal EUR")
	}
}

// TestCurrency_String tests string representation.
func TestCurrency_String(t *testing.T) {
	tests := []struct {
		curr valueobjects.Currency
		want string
	}{
		{curr: valueobjects.USD, want: "USD"},
		{curr: valueobjects.EUR, want: "EUR"},
		{curr: valueobjects.BTC, want: "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.curr.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCurrency_IsCrypto tests crypto detection.
func TestCurrency_IsCrypto(t *testing.T) {
	tests := []struct {
		curr     valueobjects.Currency
		isCrypto bool
	}{
		{curr: valueobjects.USD, isCrypto: false},
		{curr: valueobjects.EUR, isCrypto: false},
		{curr: valueobjects.GBP, isCrypto: false},
		{curr: valueobjects.BTC, isCrypto: true},
		{curr: valueobjects.ETH, isCrypto: true},
		{curr: valueobjects.USDT, isCrypto: true},
	}

	for _, tt := range tests {
		t.Run(tt.curr.Code(), func(t *testing.T) {
			if got := tt.curr.IsCrypto(); got != tt.isCrypto {
				t.Errorf("IsCrypto() = %v, want %v for %s", got, tt.isCrypto, tt.curr.Code())
			}
		})
	}
}

// TestCurrency_IsFiat tests fiat detection.
func TestCurrency_IsFiat(t *testing.T) {
	tests := []struct {
		curr   valueobjects.Currency
		isFiat bool
	}{
		{curr: valueobjects.USD, isFiat: true},
		{curr: valueobjects.EUR, isFiat: true},
		{curr: valueobjects.GBP, isFiat: true},
		{curr: valueobjects.BTC, isFiat: false},
		{curr: valueobjects.ETH, isFiat: false},
		{curr: valueobjects.USDT, isFiat: false},
	}

	for _, tt := range tests {
		t.Run(tt.curr.Code(), func(t *testing.T) {
			if got := tt.curr.IsFiat(); got != tt.isFiat {
				t.Errorf("IsFiat() = %v, want %v for %s", got, tt.isFiat, tt.curr.Code())
			}
		})
	}
}

// TestCurrency_IsZero tests zero value detection.
func TestCurrency_IsZero(t *testing.T) {
	t.Run("Initialized currency is not zero", func(t *testing.T) {
		curr := valueobjects.USD
		if curr.IsZero() {
			t.Error("Expected initialized currency not to be zero")
		}
	})

	t.Run("Default currency is zero", func(t *testing.T) {
		var curr valueobjects.Currency
		if !curr.IsZero() {
			t.Error("Expected default currency to be zero")
		}
	})
}

// TestCurrency_Predefined tests predefined currency constants.
func TestCurrency_Predefined(t *testing.T) {
	tests := []struct {
		curr valueobjects.Currency
		code string
	}{
		{curr: valueobjects.USD, code: "USD"},
		{curr: valueobjects.EUR, code: "EUR"},
		{curr: valueobjects.GBP, code: "GBP"},
		{curr: valueobjects.BTC, code: "BTC"},
		{curr: valueobjects.ETH, code: "ETH"},
		{curr: valueobjects.USDT, code: "USDT"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if tt.curr.Code() != tt.code {
				t.Errorf("Predefined %s has code %v", tt.code, tt.curr.Code())
			}
		})
	}
}

// TestCurrency_ImmutabilityThroughEquals tests value objects are compared by value.
func TestCurrency_ImmutabilityThroughEquals(t *testing.T) {
	curr1, _ := valueobjects.NewCurrency("USD")
	curr2, _ := valueobjects.NewCurrency("USD")

	// Different instances but should be equal
	if !curr1.Equals(curr2) {
		t.Error("Currencies with same code should be equal")
	}

	// Different codes should not be equal
	curr3, _ := valueobjects.NewCurrency("EUR")
	if curr1.Equals(curr3) {
		t.Error("Currencies with different codes should not be equal")
	}
}
