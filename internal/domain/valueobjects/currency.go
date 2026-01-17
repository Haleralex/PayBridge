// Package valueobjects contains immutable value objects that represent domain concepts
// without identity. They are compared by their values, not by identity.
//
// SOLID Principles Applied:
// - SRP: Currency only handles currency validation and representation
// - OCP: Can extend supported currencies without modifying existing code
// - LSP: All currencies are interchangeable as Currency type
package valueobjects

import (
	"errors"
	"strings"
)

// Currency represents a monetary currency code (ISO 4217).
// It's a value object - immutable and validated on creation.
//
// Value Object Pattern: No identity, compared by value, immutable.
// This prevents invalid currency codes from entering the domain.
type Currency struct {
	code string // Private field ensures immutability
}

// Predefined supported currencies (can be extended)
var (
	USD  = Currency{code: "USD"}
	EUR  = Currency{code: "EUR"}
	GBP  = Currency{code: "GBP"}
	BTC  = Currency{code: "BTC"} // Crypto support
	ETH  = Currency{code: "ETH"}
	USDT = Currency{code: "USDT"}
)

// supportedCurrencies defines the whitelist of allowed currencies.
// This demonstrates OCP - we can add currencies here without changing the validation logic.
var supportedCurrencies = map[string]bool{
	"USD":  true,
	"EUR":  true,
	"GBP":  true,
	"BTC":  true,
	"ETH":  true,
	"USDT": true,
	"USDC": true,
}

// ErrInvalidCurrency is returned when an invalid currency code is provided.
// Using typed errors (instead of strings) allows callers to handle specific error cases.
var ErrInvalidCurrency = errors.New("invalid currency code")

// NewCurrency creates a new Currency value object with validation.
// Factory function pattern ensures all Currency instances are valid.
//
// Returns:
//   - Currency: Valid currency instance
//   - error: ErrInvalidCurrency if code is not supported
//
// Example:
//
//	curr, err := NewCurrency("USD")
//	if err != nil {
//	    // handle error
//	}
func NewCurrency(code string) (Currency, error) {
	// Normalize to uppercase for case-insensitive comparison
	code = strings.ToUpper(strings.TrimSpace(code))

	// Validate: Business rule - only supported currencies are allowed
	if !supportedCurrencies[code] {
		return Currency{}, ErrInvalidCurrency
	}

	return Currency{code: code}, nil
}

// MustNewCurrency is a convenience function that panics on invalid input.
// Use only in initialization code where invalid input indicates a programming error.
func MustNewCurrency(code string) Currency {
	curr, err := NewCurrency(code)
	if err != nil {
		panic(err)
	}
	return curr
}

// Code returns the ISO 4217 currency code.
// Read-only access maintains immutability.
func (c Currency) Code() string {
	return c.code
}

// Equals checks if two currencies are the same.
// Value objects are compared by value, not by reference.
func (c Currency) Equals(other Currency) bool {
	return c.code == other.code
}

// String implements fmt.Stringer interface for readable output.
func (c Currency) String() string {
	return c.code
}

// IsCrypto returns true if this is a cryptocurrency.
// Business logic: Different rules apply to crypto vs fiat.
func (c Currency) IsCrypto() bool {
	cryptos := map[string]bool{
		"BTC":  true,
		"ETH":  true,
		"USDT": true,
		"USDC": true,
	}
	return cryptos[c.code]
}

// IsFiat returns true if this is a fiat currency.
func (c Currency) IsFiat() bool {
	return !c.IsCrypto()
}

// IsZero checks if this is an uninitialized currency.
// Useful for optional currency fields.
func (c Currency) IsZero() bool {
	return c.code == ""
}
