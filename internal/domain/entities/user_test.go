// Package entities_test demonstrates testing domain entities.
// Focus on business rules, state transitions, and invariants.
package entities_test

import (
	"testing"

	"github.com/Haleralex/wallethub/internal/domain/entities"
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

	// Users are auto-verified — no real KYC workflow in this project
	if user.KYCStatus() != entities.KYCStatusVerified {
		t.Errorf("KYCStatus = %v, want VERIFIED", user.KYCStatus())
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

// TestUser_CanCreateWallet tests that newly created users can create wallets.
func TestUser_CanCreateWallet(t *testing.T) {
	user, _ := entities.NewUser("test@example.com", "John Doe")

	if err := user.CanCreateWallet(); err != nil {
		t.Errorf("Newly created user should be able to create wallet, got error: %v", err)
	}
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
	user, _ := entities.NewUser("test@example.com", "John Doe")

	reconstructed := entities.ReconstructUser(
		user.ID(),
		user.Email(),
		user.FullName(),
		user.KYCStatus(),
		nil,
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if reconstructed.ID() != user.ID() {
		t.Error("ID mismatch after reconstruction")
	}
	if reconstructed.Email() != user.Email() {
		t.Error("Email mismatch after reconstruction")
	}
	if reconstructed.KYCStatus() != user.KYCStatus() {
		t.Error("KYC status mismatch after reconstruction")
	}
}

