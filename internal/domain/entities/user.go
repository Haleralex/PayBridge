// Package entities contains domain entities with identity and lifecycle.
// Entities are mutable and compared by their ID, not by their attributes.
//
// SOLID Principles:
// - SRP: User entity manages user-related business rules
// - OCP: Can add new methods without modifying existing code
// - DIP: Doesn't depend on infrastructure (no DB, no HTTP)
package entities

import (
	"regexp"
	"strings"
	"time"

	"github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/google/uuid"
)

// KYCStatus represents the Know Your Customer verification status.
// Using enum pattern for type safety.
type KYCStatus string

const (
	KYCStatusUnverified KYCStatus = "UNVERIFIED" // No verification attempted
	KYCStatusPending    KYCStatus = "PENDING"    // Verification in progress
	KYCStatusVerified   KYCStatus = "VERIFIED"   // Successfully verified
	KYCStatusRejected   KYCStatus = "REJECTED"   // Verification failed
)

// IsValid checks if the KYC status is valid.
func (s KYCStatus) IsValid() bool {
	switch s {
	case KYCStatusUnverified, KYCStatusPending, KYCStatusVerified, KYCStatusRejected:
		return true
	default:
		return false
	}
}

// User represents a user of the wallet system.
// This is an Entity (has identity via ID, has lifecycle).
//
// Entity Pattern:
// - Has unique identity (ID)
// - Mutable state over time
// - Business logic encapsulated in methods
// - Self-validating (maintains invariants)
type User struct {
	id        uuid.UUID // Identity - never changes
	email     string
	fullName  string
	kycStatus KYCStatus
	createdAt time.Time
	updatedAt time.Time
}

// Email validation regex (simplified - real systems use more complex validation)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// NewUser creates a new User with validation.
// Factory function ensures all User instances satisfy business invariants.
//
// Business Rules:
// - Email must be valid format and unique (uniqueness checked by repository)
// - Full name is required
// - New users start as UNVERIFIED
//
// Parameters:
//   - email: User's email address
//   - fullName: User's full name
//
// Returns:
//   - *User: Valid user instance
//   - error: Validation error if any rule is violated
func NewUser(email, fullName string) (*User, error) {
	// Generate new identity
	id := uuid.New()

	// Validate email
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRegex.MatchString(email) {
		return nil, errors.ErrInvalidEmail
	}

	// Validate full name
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, errors.ValidationError{
			Field:   "fullName",
			Message: "full name is required",
		}
	}

	now := time.Now()
	return &User{
		id:        id,
		email:     email,
		fullName:  fullName,
		kycStatus: KYCStatusUnverified, // Business rule: Start unverified
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstructUser reconstructs a User from stored data (e.g., from database).
// Used by repository layer to hydrate entities.
// No validation - assumes data is already valid.
func ReconstructUser(id uuid.UUID, email, fullName string, kycStatus KYCStatus, createdAt, updatedAt time.Time) *User {
	return &User{
		id:        id,
		email:     email,
		fullName:  fullName,
		kycStatus: kycStatus,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID returns the user's unique identifier.
// Identity is immutable.
func (u *User) ID() uuid.UUID {
	return u.id
}

// Email returns the user's email.
func (u *User) Email() string {
	return u.email
}

// FullName returns the user's full name.
func (u *User) FullName() string {
	return u.fullName
}

// KYCStatus returns the user's KYC verification status.
func (u *User) KYCStatus() KYCStatus {
	return u.kycStatus
}

// CreatedAt returns when the user was created.
func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

// UpdatedAt returns when the user was last updated.
func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}

// UpdateEmail changes the user's email with validation.
// Business method that encapsulates the business rule.
func (u *User) UpdateEmail(newEmail string) error {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	if !emailRegex.MatchString(newEmail) {
		return errors.ErrInvalidEmail
	}

	u.email = newEmail
	u.updatedAt = time.Now()
	return nil
}

// UpdateFullName changes the user's full name.
func (u *User) UpdateFullName(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.ValidationError{
			Field:   "fullName",
			Message: "full name cannot be empty",
		}
	}

	u.fullName = newName
	u.updatedAt = time.Now()
	return nil
}

// StartKYCVerification initiates the KYC verification process.
// Business rule: Can only start if currently UNVERIFIED or REJECTED.
func (u *User) StartKYCVerification() error {
	if u.kycStatus != KYCStatusUnverified && u.kycStatus != KYCStatusRejected {
		return errors.NewBusinessRuleViolation(
			"KYC_ALREADY_IN_PROGRESS",
			"KYC verification already completed or in progress",
			map[string]interface{}{"currentStatus": u.kycStatus},
		)
	}

	u.kycStatus = KYCStatusPending
	u.updatedAt = time.Now()
	return nil
}

// ApproveKYC marks the user as verified.
// Business rule: Can only approve if PENDING.
func (u *User) ApproveKYC() error {
	if u.kycStatus != KYCStatusPending {
		return errors.NewBusinessRuleViolation(
			"KYC_NOT_PENDING",
			"KYC verification is not in pending state",
			map[string]interface{}{"currentStatus": u.kycStatus},
		)
	}

	u.kycStatus = KYCStatusVerified
	u.updatedAt = time.Now()
	return nil
}

// RejectKYC marks the KYC verification as rejected.
func (u *User) RejectKYC() error {
	if u.kycStatus != KYCStatusPending {
		return errors.NewBusinessRuleViolation(
			"KYC_NOT_PENDING",
			"KYC verification is not in pending state",
			map[string]interface{}{"currentStatus": u.kycStatus},
		)
	}

	u.kycStatus = KYCStatusRejected
	u.updatedAt = time.Now()
	return nil
}

// IsVerified returns true if the user has completed KYC verification.
// Convenience method for business rules that require verification.
func (u *User) IsVerified() bool {
	return u.kycStatus == KYCStatusVerified
}

// CanCreateWallet checks if the user can create a wallet.
// Business rule: Only verified users can create wallets (risk control).
func (u *User) CanCreateWallet() error {
	if !u.IsVerified() {
		return errors.ErrUserNotVerified
	}
	return nil
}

// CanPerformTransaction checks if the user can perform transactions.
// Business rule: Only verified users can transact.
func (u *User) CanPerformTransaction() error {
	if !u.IsVerified() {
		return errors.ErrUserNotVerified
	}
	return nil
}
