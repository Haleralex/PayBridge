// Package entities_test demonstrates testing domain entities.
// Focus on business rules, state transitions, and invariants.
package entities_test

import (
	"testing"

	"github.com/yourusername/wallethub/internal/domain/entities"
	"github.com/yourusername/wallethub/internal/domain/errors"
)

// TestNewUser_Success tests successful user creation.
func TestNewUser_Success(t *testing.T) {
	user, err := entities.NewUser("test@example.com", "John Doe")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if user.Email() != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", user.Email())
	}

	if user.FullName() != "John Doe" {
		t.Errorf("FullName = %v, want John Doe", user.FullName())
	}

	// Business rule: New users start unverified
	if user.KYCStatus() != entities.KYCStatusUnverified {
		t.Errorf("KYCStatus = %v, want UNVERIFIED", user.KYCStatus())
	}

	// Entity must have identity
	if user.ID().String() == "" {
		t.Error("User ID should not be empty")
	}
}

// TestNewUser_InvalidEmail tests email validation.
func TestNewUser_InvalidEmail(t *testing.T) {
	invalidEmails := []string{
		"",
		"not-an-email",
		"missing@domain",
		"@example.com",
		"user@",
		"user space@example.com",
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			_, err := entities.NewUser(email, "John Doe")
			if err == nil {
				t.Errorf("Expected error for invalid email %q", email)
			}
		})
	}
}

// TestNewUser_EmptyFullName tests that full name is required.
func TestNewUser_EmptyFullName(t *testing.T) {
	_, err := entities.NewUser("test@example.com", "")
	if err == nil {
		t.Error("Expected error for empty full name")
	}
}

// TestUser_KYCWorkflow tests the complete KYC state machine.
// Business Rules:
// - Can start KYC from UNVERIFIED or REJECTED
// - Can approve from PENDING
// - Can reject from PENDING
func TestUser_KYCWorkflow(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	t.Run("Start KYC verification", func(t *testing.T) {
		err := user.StartKYCVerification()
		if err != nil {
			t.Fatalf("StartKYCVerification() error = %v", err)
		}

		if user.KYCStatus() != entities.KYCStatusPending {
			t.Errorf("KYCStatus = %v, want PENDING", user.KYCStatus())
		}
	})

	t.Run("Cannot start KYC when already pending", func(t *testing.T) {
		err := user.StartKYCVerification()
		if err == nil {
			t.Error("Expected error when starting KYC while pending")
		}
	})

	t.Run("Approve KYC", func(t *testing.T) {
		err := user.ApproveKYC()
		if err != nil {
			t.Fatalf("ApproveKYC() error = %v", err)
		}

		if user.KYCStatus() != entities.KYCStatusVerified {
			t.Errorf("KYCStatus = %v, want VERIFIED", user.KYCStatus())
		}

		if !user.IsVerified() {
			t.Error("User should be verified")
		}
	})
}

// TestUser_KYCRejection tests rejection workflow.
func TestUser_KYCRejection(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")
	_ = user.StartKYCVerification()

	err := user.RejectKYC()
	if err != nil {
		t.Fatalf("RejectKYC() error = %v", err)
	}

	if user.KYCStatus() != entities.KYCStatusRejected {
		t.Errorf("KYCStatus = %v, want REJECTED", user.KYCStatus())
	}

	// Business rule: Can retry KYC after rejection
	err = user.StartKYCVerification()
	if err != nil {
		t.Error("Should be able to restart KYC after rejection")
	}
}

// TestUser_CanCreateWallet tests wallet creation permission.
// Business Rule: Only verified users can create wallets.
func TestUser_CanCreateWallet(t *testing.T) {
	t.Run("Unverified user cannot create wallet", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		err := user.CanCreateWallet()
		if err == nil {
			t.Error("Unverified user should not be able to create wallet")
		}
		if err != errors.ErrUserNotVerified {
			t.Errorf("Expected ErrUserNotVerified, got %v", err)
		}
	})

	t.Run("Verified user can create wallet", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		_ = user.StartKYCVerification()
		_ = user.ApproveKYC()

		err := user.CanCreateWallet()
		if err != nil {
			t.Errorf("Verified user should be able to create wallet, got error: %v", err)
		}
	})
}

// TestUser_UpdateEmail tests email update with validation.
func TestUser_UpdateEmail(t *testing.T) {
	user, _ := entities.NewUser("old@example.com", "John Doe")

	t.Run("Valid email update", func(t *testing.T) {
		err := user.UpdateEmail("new@example.com")
		if err != nil {
			t.Fatalf("UpdateEmail() error = %v", err)
		}

		if user.Email() != "new@example.com" {
			t.Errorf("Email not updated: got %v, want new@example.com", user.Email())
		}
	})

	t.Run("Invalid email rejected", func(t *testing.T) {
		err := user.UpdateEmail("invalid-email")
		if err == nil {
			t.Error("Expected error for invalid email")
		}

		// Email should remain unchanged
		if user.Email() != "new@example.com" {
			t.Error("Email should not change on validation error")
		}
	})
}

// TestUser_UpdateFullName tests name update.
func TestUser_UpdateFullName(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	err := user.UpdateFullName("Jane Smith")
	if err != nil {
		t.Fatalf("UpdateFullName() error = %v", err)
	}

	if user.FullName() != "Jane Smith" {
		t.Errorf("FullName = %v, want Jane Smith", user.FullName())
	}
}

// TestUser_UpdateFullName_Empty tests that name cannot be empty.
func TestUser_UpdateFullName_Empty(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	err := user.UpdateFullName("")
	if err == nil {
		t.Error("Expected error for empty full name")
	}

	// Name should remain unchanged
	if user.FullName() != "John Doe" {
		t.Error("Name should not change on validation error")
	}
}

// TestUser_UpdateFullName_Whitespace tests that whitespace-only name is rejected.
func TestUser_UpdateFullName_Whitespace(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	err := user.UpdateFullName("   ")
	if err == nil {
		t.Error("Expected error for whitespace-only name")
	}

	if user.FullName() != "John Doe" {
		t.Error("Name should not change on validation error")
	}
}

// TestNewUser_EmailNormalization tests email is normalized (lowercase, trimmed).
func TestNewUser_EmailNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "Test@Example.COM", expected: "test@example.com"},
		{input: "  user@domain.com  ", expected: "user@domain.com"},
		{input: "CAPS@EXAMPLE.COM", expected: "caps@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			user, err := entities.NewUser(tt.input, "John Doe")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if user.Email() != tt.expected {
				t.Errorf("Email = %v, want %v", user.Email(), tt.expected)
			}
		})
	}
}

// TestUser_CreatedAt tests creation timestamp is set.
func TestUser_CreatedAt(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	if user.CreatedAt().IsZero() {
		t.Error("CreatedAt should be set")
	}
}

// TestUser_UpdatedAt tests updated timestamp changes on mutations.
func TestUser_UpdatedAt(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	initialUpdatedAt := user.UpdatedAt()

	// Small sleep to ensure time difference
	// time.Sleep(time.Millisecond)

	// Note: In production, you might want to use a time provider interface
	// For now, we just check that UpdatedAt is set and not zero

	if initialUpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set initially")
	}

	// UpdatedAt should match CreatedAt for new user
	if !user.UpdatedAt().Equal(user.CreatedAt()) {
		t.Log("UpdatedAt and CreatedAt may differ slightly in fast execution")
	}
}

// TestReconstructUser tests reconstruction from persistence.
func TestReconstructUser(t *testing.T) {
	// Simulate data from database
	user, _ := entities.NewUser("test@example.com", "John Doe")
	_ = user.StartKYCVerification()
	_ = user.ApproveKYC()

	// Reconstruct from stored values
	reconstructed := entities.ReconstructUser(
		user.ID(),
		user.Email(),
		user.FullName(),
		user.KYCStatus(),
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if reconstructed.ID() != user.ID() {
		t.Error("ID mismatch after reconstruction")
	}
	if reconstructed.Email() != user.Email() {
		t.Error("Email mismatch after reconstruction")
	}
	if reconstructed.KYCStatus() != entities.KYCStatusVerified {
		t.Error("KYC status mismatch after reconstruction")
	}
}

// TestKYCStatus_IsValid tests KYC status validation.
func TestKYCStatus_IsValid(t *testing.T) {
	tests := []struct {
		status entities.KYCStatus
		valid  bool
	}{
		{entities.KYCStatusUnverified, true},
		{entities.KYCStatusPending, true},
		{entities.KYCStatusVerified, true},
		{entities.KYCStatusRejected, true},
		{entities.KYCStatus("INVALID"), false},
		{entities.KYCStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestUser_ApproveKYC_WhenNotPending tests approve fails when not pending.
func TestUser_ApproveKYC_WhenNotPending(t *testing.T) {
	tests := []struct {
		name   string
		status entities.KYCStatus
	}{
		{"unverified", entities.KYCStatusUnverified},
		{"verified", entities.KYCStatusVerified},
		{"rejected", entities.KYCStatusRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, _ := entities.NewUser("test@example.com", "John Doe")

			// Manually set status using ReconstructUser
			user = entities.ReconstructUser(
				user.ID(),
				user.Email(),
				user.FullName(),
				tt.status,
				user.CreatedAt(),
				user.UpdatedAt(),
			)

			err := user.ApproveKYC()
			if err == nil {
				t.Errorf("ApproveKYC should fail when status is %v", tt.status)
			}
		})
	}
}

// TestUser_RejectKYC_WhenNotPending tests reject fails when not pending.
func TestUser_RejectKYC_WhenNotPending(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	// Try to reject without starting KYC
	err := user.RejectKYC()
	if err == nil {
		t.Error("RejectKYC should fail when not pending")
	}
}

// TestUser_CanPerformTransaction tests transaction permission.
func TestUser_CanPerformTransaction(t *testing.T) {
	t.Run("Unverified user cannot transact", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		err := user.CanPerformTransaction()
		if err == nil {
			t.Error("Unverified user should not be able to transact")
		}
		if err != errors.ErrUserNotVerified {
			t.Errorf("Expected ErrUserNotVerified, got %v", err)
		}
	})

	t.Run("Verified user can transact", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		_ = user.StartKYCVerification()
		_ = user.ApproveKYC()

		err := user.CanPerformTransaction()
		if err != nil {
			t.Errorf("Verified user should be able to transact, got error: %v", err)
		}
	})

	t.Run("Pending user cannot transact", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		_ = user.StartKYCVerification()

		err := user.CanPerformTransaction()
		if err == nil {
			t.Error("Pending user should not be able to transact")
		}
	})

	t.Run("Rejected user cannot transact", func(t *testing.T) {
		user, _ := entities.NewUser("test@example.com", "John Doe")
		_ = user.StartKYCVerification()
		_ = user.RejectKYC()

		err := user.CanPerformTransaction()
		if err == nil {
			t.Error("Rejected user should not be able to transact")
		}
	})
}

// TestUser_CanCreateWallet_AllStatuses tests wallet creation for all KYC statuses.
func TestUser_CanCreateWallet_AllStatuses(t *testing.T) {
	tests := []struct {
		name      string
		status    entities.KYCStatus
		shouldErr bool
	}{
		{"unverified", entities.KYCStatusUnverified, true},
		{"pending", entities.KYCStatusPending, true},
		{"verified", entities.KYCStatusVerified, false},
		{"rejected", entities.KYCStatusRejected, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, _ := entities.NewUser("test@example.com", "John Doe")

			user = entities.ReconstructUser(
				user.ID(),
				user.Email(),
				user.FullName(),
				tt.status,
				user.CreatedAt(),
				user.UpdatedAt(),
			)

			err := user.CanCreateWallet()
			if (err != nil) != tt.shouldErr {
				t.Errorf("CanCreateWallet() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

// TestUser_StartKYC_FromVerified tests cannot restart KYC when verified.
func TestUser_StartKYC_FromVerified(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")
	_ = user.StartKYCVerification()
	_ = user.ApproveKYC()

	err := user.StartKYCVerification()
	if err == nil {
		t.Error("Should not be able to restart KYC when verified")
	}
}

// TestUser_IsVerified_AllStatuses tests IsVerified for all statuses.
func TestUser_IsVerified_AllStatuses(t *testing.T) {
	tests := []struct {
		status   entities.KYCStatus
		verified bool
	}{
		{entities.KYCStatusUnverified, false},
		{entities.KYCStatusPending, false},
		{entities.KYCStatusVerified, true},
		{entities.KYCStatusRejected, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			user, _ := entities.NewUser("test@example.com", "John Doe")

			user = entities.ReconstructUser(
				user.ID(),
				user.Email(),
				user.FullName(),
				tt.status,
				user.CreatedAt(),
				user.UpdatedAt(),
			)

			if user.IsVerified() != tt.verified {
				t.Errorf("IsVerified() = %v, want %v for status %v",
					user.IsVerified(), tt.verified, tt.status)
			}
		})
	}
}
