package ports

import (
	"context"
	"math/big"
)

// ExchangeRateProvider provides currency exchange rates.
type ExchangeRateProvider interface {
	// GetRate returns the exchange rate from one currency to another.
	// The rate represents how much of 'to' currency you get for 1 unit of 'from' currency.
	// Example: GetRate(ctx, "USD", "EUR") might return 0.92 (1 USD = 0.92 EUR).
	GetRate(ctx context.Context, from, to string) (*big.Rat, error)
}
