// Package errors defines domain-specific error types.
// Using typed errors (instead of strings) allows clients to handle specific cases.
//
// SOLID Principles:
// - ISP: Clients can check for specific errors they care about
// - DIP: Error types are abstractions that don't depend on infrastructure
//
// Pattern: Sentinel Errors + Custom Error Types
package errors

import (
	"errors"
	"fmt"
)

// Common sentinel errors for domain validation
var (
	// Entity validation errors
	ErrInvalidEntityID     = errors.New("invalid entity ID")
	ErrEntityNotFound      = errors.New("entity not found")
	ErrEntityAlreadyExists = errors.New("entity already exists")

	// User errors
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrInvalidKYCStatus = errors.New("invalid KYC status")
	ErrUserNotVerified  = errors.New("user not verified")

	// Wallet errors
	ErrInvalidWalletType = errors.New("invalid wallet type")
	ErrWalletNotActive   = errors.New("wallet is not active")
	ErrWalletSuspended   = errors.New("wallet is suspended")
	ErrWalletLocked      = errors.New("wallet is locked")

	// Transaction errors
	ErrInsufficientBalance         = errors.New("insufficient balance")
	ErrInvalidTransactionType      = errors.New("invalid transaction type")
	ErrTransactionNotPending       = errors.New("transaction is not in pending state")
	ErrTransactionAlreadyProcessed = errors.New("transaction already processed")
	ErrDuplicateTransaction        = errors.New("duplicate transaction detected")

	// Business rule errors
	ErrTransactionLimitExceeded = errors.New("transaction limit exceeded")
	ErrDailyLimitExceeded       = errors.New("daily limit exceeded")
	ErrMonthlyLimitExceeded     = errors.New("monthly limit exceeded")
	ErrRiskCheckFailed          = errors.New("risk check failed")
	ErrBlacklistedAddress       = errors.New("address is blacklisted")
)

// DomainError is a custom error type that wraps errors with additional context.
// This allows us to add domain-specific information while maintaining the error chain.
//
// Pattern: Error Wrapping with Context
type DomainError struct {
	Code    string // Machine-readable error code (e.g., "INSUFFICIENT_BALANCE")
	Message string // Human-readable message
	Err     error  // Underlying error (for error chains)
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements error unwrapping for errors.Is and errors.As.
func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new domain error.
func NewDomainError(code, message string, err error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents validation failures with field-level details.
// Useful for returning multiple validation errors at once.
//
// Pattern: Composite Error for Multiple Validations
type ValidationError struct {
	Field   string // Field name that failed validation
	Message string // What went wrong
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %d error(s)", len(e))
}

// Add appends a validation error.
func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// BusinessRuleViolation represents a violation of a business rule.
// Unlike validation errors (which are about data format), these are about business logic.
//
// Example: "Cannot withdraw more than daily limit" is a business rule, not a validation.
type BusinessRuleViolation struct {
	Rule    string                 // Rule that was violated (e.g., "DAILY_LIMIT")
	Message string                 // Human-readable explanation
	Context map[string]interface{} // Additional context (e.g., {"limit": 1000, "attempted": 1500})
}

// Error implements the error interface.
func (e BusinessRuleViolation) Error() string {
	return fmt.Sprintf("business rule violation [%s]: %s", e.Rule, e.Message)
}

// NewBusinessRuleViolation creates a new business rule violation error.
func NewBusinessRuleViolation(rule, message string, context map[string]interface{}) *BusinessRuleViolation {
	return &BusinessRuleViolation{
		Rule:    rule,
		Message: message,
		Context: context,
	}
}

// ConcurrencyError represents errors from concurrent access (optimistic locking).
// This will be important when we implement balance updates with version checking.
type ConcurrencyError struct {
	EntityType string // e.g., "Wallet", "Balance"
	EntityID   string // ID of the entity
	Message    string
}

// Error implements the error interface.
func (e ConcurrencyError) Error() string {
	return fmt.Sprintf("concurrency error on %s [%s]: %s", e.EntityType, e.EntityID, e.Message)
}

// NewConcurrencyError creates a new concurrency error.
func NewConcurrencyError(entityType, entityID, message string) *ConcurrencyError {
	return &ConcurrencyError{
		EntityType: entityType,
		EntityID:   entityID,
		Message:    message,
	}
}

// Helper functions for common error checking

// IsNotFound checks if an error is an "entity not found" error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrEntityNotFound)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	var valErr ValidationError
	var valErrs ValidationErrors
	return errors.As(err, &valErr) || errors.As(err, &valErrs)
}

// IsValidation is an alias for IsValidationError (для совместимости).
func IsValidation(err error) bool {
	return IsValidationError(err)
}

// IsBusinessRuleViolation checks if an error is a business rule violation.
func IsBusinessRuleViolation(err error) bool {
	var brv *BusinessRuleViolation
	return errors.As(err, &brv)
}

// IsConcurrencyError checks if an error is a concurrency error.
func IsConcurrencyError(err error) bool {
	var ce *ConcurrencyError
	return errors.As(err, &ce)
}
