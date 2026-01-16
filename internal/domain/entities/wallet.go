// Package entities - Wallet is the core entity for managing user balances.
// It enforces business rules around balance operations, limits, and status.
package entities

import (
	"time"

	"github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// WalletType represents the type of wallet.
type WalletType string

const (
	WalletTypeFiat   WalletType = "FIAT"   // Fiat currency wallet (USD, EUR, etc.)
	WalletTypeCrypto WalletType = "CRYPTO" // Cryptocurrency wallet (BTC, ETH, etc.)
)

// IsValid checks if the wallet type is valid.
func (t WalletType) IsValid() bool {
	return t == WalletTypeFiat || t == WalletTypeCrypto
}

// WalletStatus represents the operational status of a wallet.
type WalletStatus string

const (
	WalletStatusActive    WalletStatus = "ACTIVE"    // Normal operations allowed
	WalletStatusSuspended WalletStatus = "SUSPENDED" // Temporarily disabled
	WalletStatusLocked    WalletStatus = "LOCKED"    // Locked due to security/compliance
	WalletStatusClosed    WalletStatus = "CLOSED"    // Permanently closed
)

// IsValid checks if the wallet status is valid.
func (s WalletStatus) IsValid() bool {
	switch s {
	case WalletStatusActive, WalletStatusSuspended, WalletStatusLocked, WalletStatusClosed:
		return true
	default:
		return false
	}
}

// Wallet represents a user's wallet for a specific currency.
// A user can have multiple wallets (one per currency).
//
// Entity Pattern:
// - Has identity (ID)
// - Aggregates Balance (separate entity, not exposed directly)
// - Enforces invariants (balance consistency, status rules)
// - Rich behavior (not just data)
//
// SOLID:
// - SRP: Manages wallet operations and rules
// - OCP: Can add new wallet types without modifying existing code
// - LSP: All wallets follow the same contract
type Wallet struct {
	id         uuid.UUID
	userID     uuid.UUID // Foreign key to User (aggregate boundary)
	currency   valueobjects.Currency
	walletType WalletType
	status     WalletStatus

	// Balance tracking (embedded aggregate)
	// In a real system, this might be a separate entity with optimistic locking
	balance Balance

	// Transaction limits (business rules)
	dailyLimit   valueobjects.Money // Max daily transaction volume
	monthlyLimit valueobjects.Money // Max monthly transaction volume

	createdAt time.Time
	updatedAt time.Time
}

// Balance represents the wallet's balance with version for optimistic locking.
// This is a separate entity within the Wallet aggregate.
type Balance struct {
	available valueobjects.Money // Available for transactions
	pending   valueobjects.Money // Pending transactions (reserved)
	version   int64              // Optimistic locking version
}

// NewWallet creates a new wallet for a user.
// Factory function with validation.
//
// Business Rules:
// - User must exist (checked by application layer)
// - Currency must be supported
// - Wallet type must match currency type (fiat/crypto)
// - New wallets start ACTIVE with zero balance
// - Default limits applied based on wallet type
func NewWallet(userID uuid.UUID, currency valueobjects.Currency) (*Wallet, error) {
	// Validate currency
	if currency.IsZero() {
		return nil, errors.ValidationError{
			Field:   "currency",
			Message: "currency is required",
		}
	}

	// Determine wallet type from currency
	walletType := WalletTypeFiat
	if currency.IsCrypto() {
		walletType = WalletTypeCrypto
	}

	// Set default limits (can be customized later)
	defaultLimit, _ := valueobjects.NewMoneyFromInt(10000, currency) // $10k or 10k crypto units
	if walletType == WalletTypeCrypto {
		defaultLimit, _ = valueobjects.NewMoneyFromInt(100, currency) // 100 crypto units
	}

	now := time.Now()
	wallet := &Wallet{
		id:         uuid.New(),
		userID:     userID,
		currency:   currency,
		walletType: walletType,
		status:     WalletStatusActive,
		balance: Balance{
			available: valueobjects.Zero(currency),
			pending:   valueobjects.Zero(currency),
			version:   0,
		},
		dailyLimit:   defaultLimit,
		monthlyLimit: defaultLimit, // TODO: Make this higher in real system
		createdAt:    now,
		updatedAt:    now,
	}

	return wallet, nil
}

// ReconstructWallet reconstructs a Wallet from stored data.
// Used by repository to hydrate entities from database.
func ReconstructWallet(
	id, userID uuid.UUID,
	currency valueobjects.Currency,
	walletType WalletType,
	status WalletStatus,
	available, pending valueobjects.Money,
	balanceVersion int64,
	dailyLimit, monthlyLimit valueobjects.Money,
	createdAt, updatedAt time.Time,
) *Wallet {
	return &Wallet{
		id:         id,
		userID:     userID,
		currency:   currency,
		walletType: walletType,
		status:     status,
		balance: Balance{
			available: available,
			pending:   pending,
			version:   balanceVersion,
		},
		dailyLimit:   dailyLimit,
		monthlyLimit: monthlyLimit,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// Getters

func (w *Wallet) ID() uuid.UUID {
	return w.id
}

func (w *Wallet) UserID() uuid.UUID {
	return w.userID
}

func (w *Wallet) Currency() valueobjects.Currency {
	return w.currency
}

func (w *Wallet) WalletType() WalletType {
	return w.walletType
}

func (w *Wallet) Status() WalletStatus {
	return w.status
}

func (w *Wallet) AvailableBalance() valueobjects.Money {
	return w.balance.available
}

func (w *Wallet) PendingBalance() valueobjects.Money {
	return w.balance.pending
}

func (w *Wallet) TotalBalance() (valueobjects.Money, error) {
	return w.balance.available.Add(w.balance.pending)
}

func (w *Wallet) BalanceVersion() int64 {
	return w.balance.version
}

func (w *Wallet) DailyLimit() valueobjects.Money {
	return w.dailyLimit
}

func (w *Wallet) MonthlyLimit() valueobjects.Money {
	return w.monthlyLimit
}

func (w *Wallet) CreatedAt() time.Time {
	return w.createdAt
}

func (w *Wallet) UpdatedAt() time.Time {
	return w.updatedAt
}

// Business Methods

// IsActive returns true if the wallet is active and can perform operations.
func (w *Wallet) IsActive() bool {
	return w.status == WalletStatusActive
}

// CanDebit checks if the wallet can be debited (withdrawn from).
// Business rule: Only active wallets can be debited.
func (w *Wallet) CanDebit() error {
	if w.status != WalletStatusActive {
		return errors.ErrWalletNotActive
	}
	return nil
}

// CanCredit checks if the wallet can be credited (deposited to).
// Business rule: Active and suspended wallets can receive credits.
func (w *Wallet) CanCredit() error {
	if w.status == WalletStatusClosed {
		return errors.NewBusinessRuleViolation(
			"WALLET_CLOSED",
			"cannot credit a closed wallet",
			map[string]interface{}{"walletID": w.id},
		)
	}
	return nil
}

// HasSufficientBalance checks if the wallet has enough available balance.
// Business rule: Cannot spend more than available balance.
func (w *Wallet) HasSufficientBalance(amount valueobjects.Money) (bool, error) {
	return w.balance.available.GreaterThanOrEqual(amount)
}

// Credit adds funds to the wallet.
// This is a domain operation that enforces business rules.
//
// Business Rules:
// - Wallet must accept credits
// - Amount must be in the same currency
// - Balance version is incremented (optimistic locking)
func (w *Wallet) Credit(amount valueobjects.Money) error {
	// Check if wallet can accept credits
	if err := w.CanCredit(); err != nil {
		return err
	}

	// Validate currency match
	if !w.currency.Equals(amount.Currency()) {
		return errors.NewBusinessRuleViolation(
			"CURRENCY_MISMATCH",
			"amount currency doesn't match wallet currency",
			map[string]interface{}{
				"walletCurrency": w.currency.Code(),
				"amountCurrency": amount.Currency().Code(),
			},
		)
	}

	// Update balance
	newBalance, err := w.balance.available.Add(amount)
	if err != nil {
		return err
	}

	w.balance.available = newBalance
	w.balance.version++ // Increment version for optimistic locking
	w.updatedAt = time.Now()

	return nil
}

// Debit subtracts funds from the wallet.
// Business Rules:
// - Wallet must be active
// - Sufficient balance must be available
// - Currency must match
func (w *Wallet) Debit(amount valueobjects.Money) error {
	// Check if wallet can be debited
	if err := w.CanDebit(); err != nil {
		return err
	}

	// Validate currency
	if !w.currency.Equals(amount.Currency()) {
		return errors.NewBusinessRuleViolation(
			"CURRENCY_MISMATCH",
			"amount currency doesn't match wallet currency",
			map[string]interface{}{
				"walletCurrency": w.currency.Code(),
				"amountCurrency": amount.Currency().Code(),
			},
		)
	}

	// Check sufficient balance
	hasSufficient, err := w.HasSufficientBalance(amount)
	if err != nil {
		return err
	}
	if !hasSufficient {
		return errors.ErrInsufficientBalance
	}

	// Update balance
	newBalance, err := w.balance.available.Subtract(amount)
	if err != nil {
		return err
	}

	w.balance.available = newBalance
	w.balance.version++
	w.updatedAt = time.Now()

	return nil
}

// Reserve moves funds from available to pending.
// Used for two-phase commits (reserve, then complete or release).
//
// Example: When initiating a payout, reserve the amount first.
func (w *Wallet) Reserve(amount valueobjects.Money) error {
	if err := w.CanDebit(); err != nil {
		return err
	}

	hasSufficient, err := w.HasSufficientBalance(amount)
	if err != nil {
		return err
	}
	if !hasSufficient {
		return errors.ErrInsufficientBalance
	}

	// Move from available to pending
	newAvailable, err := w.balance.available.Subtract(amount)
	if err != nil {
		return err
	}

	newPending, err := w.balance.pending.Add(amount)
	if err != nil {
		return err
	}

	w.balance.available = newAvailable
	w.balance.pending = newPending
	w.balance.version++
	w.updatedAt = time.Now()

	return nil
}

// Release moves funds from pending back to available.
// Used when a reserved transaction is cancelled.
func (w *Wallet) Release(amount valueobjects.Money) error {
	// Check if enough pending balance
	hasSufficient, err := w.balance.pending.GreaterThanOrEqual(amount)
	if err != nil {
		return err
	}
	if !hasSufficient {
		return errors.NewBusinessRuleViolation(
			"INSUFFICIENT_PENDING_BALANCE",
			"insufficient pending balance to release",
			map[string]interface{}{"amount": amount.String()},
		)
	}

	// Move from pending to available
	newPending, err := w.balance.pending.Subtract(amount)
	if err != nil {
		return err
	}

	newAvailable, err := w.balance.available.Add(amount)
	if err != nil {
		return err
	}

	w.balance.available = newAvailable
	w.balance.pending = newPending
	w.balance.version++
	w.updatedAt = time.Now()

	return nil
}

// CompletePending completes a pending transaction by removing it from pending.
// Used when a reserved transaction is finalized (e.g., payout completed).
func (w *Wallet) CompletePending(amount valueobjects.Money) error {
	hasSufficient, err := w.balance.pending.GreaterThanOrEqual(amount)
	if err != nil {
		return err
	}
	if !hasSufficient {
		return errors.NewBusinessRuleViolation(
			"INSUFFICIENT_PENDING_BALANCE",
			"insufficient pending balance to complete",
			map[string]interface{}{"amount": amount.String()},
		)
	}

	newPending, err := w.balance.pending.Subtract(amount)
	if err != nil {
		return err
	}

	w.balance.pending = newPending
	w.balance.version++
	w.updatedAt = time.Now()

	return nil
}

// Status Management

// Suspend temporarily disables the wallet.
func (w *Wallet) Suspend() error {
	if w.status == WalletStatusClosed {
		return errors.NewBusinessRuleViolation(
			"CANNOT_SUSPEND_CLOSED_WALLET",
			"cannot suspend a closed wallet",
			nil,
		)
	}

	w.status = WalletStatusSuspended
	w.updatedAt = time.Now()
	return nil
}

// Activate activates a suspended wallet.
func (w *Wallet) Activate() error {
	if w.status == WalletStatusClosed {
		return errors.NewBusinessRuleViolation(
			"CANNOT_ACTIVATE_CLOSED_WALLET",
			"cannot activate a closed wallet",
			nil,
		)
	}

	w.status = WalletStatusActive
	w.updatedAt = time.Now()
	return nil
}

// Lock locks the wallet (security/compliance).
func (w *Wallet) Lock() error {
	w.status = WalletStatusLocked
	w.updatedAt = time.Now()
	return nil
}

// Close permanently closes the wallet.
// Business rule: Can only close if balance is zero.
func (w *Wallet) Close() error {
	total, err := w.TotalBalance()
	if err != nil {
		return err
	}

	if !total.IsZero() {
		return errors.NewBusinessRuleViolation(
			"CANNOT_CLOSE_NON_ZERO_WALLET",
			"cannot close wallet with non-zero balance",
			map[string]interface{}{
				"balance": total.String(),
			},
		)
	}

	w.status = WalletStatusClosed
	w.updatedAt = time.Now()
	return nil
}

// UpdateLimits updates the daily and monthly transaction limits.
// This would typically be called by an admin or risk management system.
func (w *Wallet) UpdateLimits(dailyLimit, monthlyLimit valueobjects.Money) error {
	// Validate currency matches
	if !w.currency.Equals(dailyLimit.Currency()) || !w.currency.Equals(monthlyLimit.Currency()) {
		return errors.NewBusinessRuleViolation(
			"LIMIT_CURRENCY_MISMATCH",
			"limit currency must match wallet currency",
			nil,
		)
	}

	w.dailyLimit = dailyLimit
	w.monthlyLimit = monthlyLimit
	w.updatedAt = time.Now()
	return nil
}
