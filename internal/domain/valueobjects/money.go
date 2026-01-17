// Package valueobjects - Money is one of the most critical value objects in financial systems.
// It combines amount and currency to prevent common bugs like mixing currencies.
//
// SOLID Principles:
// - SRP: Money knows how to be Money (arithmetic, comparison, validation)
// - OCP: Can extend with new operations without modifying existing code
// - LSP: All Money instances follow the same contract
package valueobjects

import (
	"errors"
	"fmt"
	"math/big"
)

// Money represents a monetary amount with its currency.
// Uses big.Rat for arbitrary precision to avoid floating-point errors.
//
// Value Object Pattern:
// - Immutable: All operations return new Money instances
// - Self-validating: Cannot create invalid Money
// - Type-safe: Prevents mixing currencies
//
// Why big.Rat?
// - Avoids floating-point precision issues (0.1 + 0.2 != 0.3)
// - Handles large amounts common in crypto
// - Exact decimal representation
type Money struct {
	amount   *big.Rat // Arbitrary precision rational number
	currency Currency
}

// Common domain errors for Money operations
var (
	ErrNegativeAmount     = errors.New("amount cannot be negative")
	ErrCurrencyMismatch   = errors.New("cannot operate on different currencies")
	ErrInsufficientAmount = errors.New("insufficient amount")
	ErrInvalidAmount      = errors.New("invalid amount format")
)

// NewMoney creates a Money instance from a string amount.
// The amount is parsed as a decimal (e.g., "100.50", "0.001").
//
// Parameters:
//   - amountStr: Decimal string (e.g., "123.45")
//   - currency: Currency instance
//
// Returns error if:
//   - Amount is negative
//   - Amount cannot be parsed
//
// Example:
//
//	money, err := NewMoney("100.50", USD)
func NewMoney(amountStr string, currency Currency) (Money, error) {
	// Parse string to big.Rat
	amount := new(big.Rat)
	if _, ok := amount.SetString(amountStr); !ok {
		return Money{}, fmt.Errorf("%w: %s", ErrInvalidAmount, amountStr)
	}

	// Business rule: Money cannot be negative (use different types for debits/credits)
	if amount.Sign() < 0 {
		return Money{}, ErrNegativeAmount
	}

	return Money{
		amount:   amount,
		currency: currency,
	}, nil
}

// NewMoneyFromInt creates Money from an integer amount.
// Useful for whole unit amounts.
func NewMoneyFromInt(amount int64, currency Currency) (Money, error) {
	if amount < 0 {
		return Money{}, ErrNegativeAmount
	}

	return Money{
		amount:   big.NewRat(amount, 1),
		currency: currency,
	}, nil
}

// NewMoneyFromCents creates Money from the smallest currency unit (cents, satoshis, wei).
// This is the preferred way to store money in databases (as integer cents).
//
// Example:
//
//	NewMoneyFromCents(10050, USD) // $100.50
//	NewMoneyFromCents(100000000, BTC) // 1 BTC (100M satoshis)
func NewMoneyFromCents(cents int64, currency Currency) (Money, error) {
	if cents < 0 {
		return Money{}, ErrNegativeAmount
	}

	// For crypto, use 8 decimals; for fiat, use 2
	divisor := int64(100) // Default: 2 decimals
	if currency.IsCrypto() {
		divisor = 100000000 // 8 decimals for crypto
	}

	return Money{
		amount:   big.NewRat(cents, divisor),
		currency: currency,
	}, nil
}

// Zero creates a zero money amount for the given currency.
func Zero(currency Currency) Money {
	return Money{
		amount:   big.NewRat(0, 1),
		currency: currency,
	}
}

// Currency returns the currency of this money.
func (m Money) Currency() Currency {
	return m.currency
}

// Amount returns the amount as a big.Rat.
// Returns a copy to maintain immutability.
func (m Money) Amount() *big.Rat {
	return new(big.Rat).Set(m.amount)
}

// String returns a human-readable representation.
// Example: "100.50 USD"
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.amount.FloatString(m.decimalPlaces()), m.currency.Code())
}

// Float64 returns the amount as float64.
// WARNING: Use only for display purposes, not for calculations!
func (m Money) Float64() float64 {
	f, _ := m.amount.Float64()
	return f
}

// Cents returns the amount in the smallest currency unit (cents, satoshis).
// This is the preferred storage format in databases.
func (m Money) Cents() int64 {
	multiplier := int64(100)
	if m.currency.IsCrypto() {
		multiplier = 100000000
	}

	// Multiply by 10^decimals and convert to int
	scaled := new(big.Rat).Mul(m.amount, big.NewRat(multiplier, 1))
	return scaled.Num().Int64() / scaled.Denom().Int64()
}

// Add returns a new Money with the sum of two amounts.
// IMMUTABLE: Returns new instance, doesn't modify receiver.
//
// Business rule: Cannot add different currencies.
func (m Money) Add(other Money) (Money, error) {
	if !m.currency.Equals(other.currency) {
		return Money{}, ErrCurrencyMismatch
	}

	sum := new(big.Rat).Add(m.amount, other.amount)
	return Money{amount: sum, currency: m.currency}, nil
}

// Subtract returns a new Money with the difference.
// Returns error if result would be negative.
func (m Money) Subtract(other Money) (Money, error) {
	if !m.currency.Equals(other.currency) {
		return Money{}, ErrCurrencyMismatch
	}

	diff := new(big.Rat).Sub(m.amount, other.amount)
	if diff.Sign() < 0 {
		return Money{}, ErrInsufficientAmount
	}

	return Money{amount: diff, currency: m.currency}, nil
}

// Multiply returns a new Money multiplied by a factor.
// Use for calculations like fees (e.g., amount * 0.03 for 3% fee).
func (m Money) Multiply(factor *big.Rat) Money {
	product := new(big.Rat).Mul(m.amount, factor)
	return Money{amount: product, currency: m.currency}
}

// IsZero returns true if the amount is zero.
func (m Money) IsZero() bool {
	return m.amount.Sign() == 0
}

// IsPositive returns true if the amount is greater than zero.
func (m Money) IsPositive() bool {
	return m.amount.Sign() > 0
}

// GreaterThan checks if this money is greater than another.
func (m Money) GreaterThan(other Money) (bool, error) {
	if !m.currency.Equals(other.currency) {
		return false, ErrCurrencyMismatch
	}
	return m.amount.Cmp(other.amount) > 0, nil
}

// GreaterThanOrEqual checks if this money is >= another.
func (m Money) GreaterThanOrEqual(other Money) (bool, error) {
	if !m.currency.Equals(other.currency) {
		return false, ErrCurrencyMismatch
	}
	return m.amount.Cmp(other.amount) >= 0, nil
}

// LessThan checks if this money is less than another.
func (m Money) LessThan(other Money) (bool, error) {
	if !m.currency.Equals(other.currency) {
		return false, ErrCurrencyMismatch
	}
	return m.amount.Cmp(other.amount) < 0, nil
}

// Equals checks if two money values are equal (amount and currency).
func (m Money) Equals(other Money) bool {
	return m.currency.Equals(other.currency) && m.amount.Cmp(other.amount) == 0
}

// decimalPlaces returns the number of decimal places for display.
func (m Money) decimalPlaces() int {
	if m.currency.IsCrypto() {
		return 8
	}
	return 2
}
