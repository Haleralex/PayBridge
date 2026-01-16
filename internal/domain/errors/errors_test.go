package errors

import (
	"errors"
	"testing"
)

// TestSentinelErrors tests that all sentinel errors are defined
func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidEntityID", ErrInvalidEntityID},
		{"ErrEntityNotFound", ErrEntityNotFound},
		{"ErrEntityAlreadyExists", ErrEntityAlreadyExists},
		{"ErrInvalidEmail", ErrInvalidEmail},
		{"ErrInvalidKYCStatus", ErrInvalidKYCStatus},
		{"ErrUserNotVerified", ErrUserNotVerified},
		{"ErrInvalidWalletType", ErrInvalidWalletType},
		{"ErrWalletNotActive", ErrWalletNotActive},
		{"ErrWalletSuspended", ErrWalletSuspended},
		{"ErrWalletLocked", ErrWalletLocked},
		{"ErrInsufficientBalance", ErrInsufficientBalance},
		{"ErrInvalidTransactionType", ErrInvalidTransactionType},
		{"ErrTransactionNotPending", ErrTransactionNotPending},
		{"ErrTransactionAlreadyProcessed", ErrTransactionAlreadyProcessed},
		{"ErrDuplicateTransaction", ErrDuplicateTransaction},
		{"ErrTransactionLimitExceeded", ErrTransactionLimitExceeded},
		{"ErrDailyLimitExceeded", ErrDailyLimitExceeded},
		{"ErrMonthlyLimitExceeded", ErrMonthlyLimitExceeded},
		{"ErrRiskCheckFailed", ErrRiskCheckFailed},
		{"ErrBlacklistedAddress", ErrBlacklistedAddress},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s should have an error message", tt.name)
			}
		})
	}
}

// TestDomainError_Error tests DomainError error message formatting
func TestDomainError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DomainError
		contains []string
	}{
		{
			name: "Error with underlying error",
			err: &DomainError{
				Code:    "TEST_ERROR",
				Message: "Test message",
				Err:     errors.New("underlying error"),
			},
			contains: []string{"TEST_ERROR", "Test message", "underlying error"},
		},
		{
			name: "Error without underlying error",
			err: &DomainError{
				Code:    "TEST_ERROR",
				Message: "Test message",
				Err:     nil,
			},
			contains: []string{"TEST_ERROR", "Test message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, substr := range tt.contains {
				if !contains(errMsg, substr) {
					t.Errorf("Error message %q should contain %q", errMsg, substr)
				}
			}
		})
	}
}

// TestDomainError_Unwrap tests error unwrapping
func TestDomainError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	domainErr := &DomainError{
		Code:    "TEST",
		Message: "Test",
		Err:     underlyingErr,
	}

	unwrapped := domainErr.Unwrap()
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}
}

// TestDomainError_Unwrap_Nil tests unwrapping with no underlying error
func TestDomainError_Unwrap_Nil(t *testing.T) {
	domainErr := &DomainError{
		Code:    "TEST",
		Message: "Test",
		Err:     nil,
	}

	unwrapped := domainErr.Unwrap()
	if unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

// TestNewDomainError tests DomainError creation
func TestNewDomainError(t *testing.T) {
	underlyingErr := errors.New("test error")
	domainErr := NewDomainError("TEST_CODE", "Test message", underlyingErr)

	if domainErr.Code != "TEST_CODE" {
		t.Errorf("Code = %q, want %q", domainErr.Code, "TEST_CODE")
	}

	if domainErr.Message != "Test message" {
		t.Errorf("Message = %q, want %q", domainErr.Message, "Test message")
	}

	if domainErr.Err != underlyingErr {
		t.Errorf("Err = %v, want %v", domainErr.Err, underlyingErr)
	}
}

// TestNewDomainError_NoUnderlyingError tests creation without underlying error
func TestNewDomainError_NoUnderlyingError(t *testing.T) {
	domainErr := NewDomainError("TEST_CODE", "Test message", nil)

	if domainErr.Err != nil {
		t.Errorf("Err should be nil, got %v", domainErr.Err)
	}
}

// TestValidationError_Error tests ValidationError error message
func TestValidationError_Error(t *testing.T) {
	valErr := ValidationError{
		Field:   "email",
		Message: "invalid format",
	}

	errMsg := valErr.Error()
	if !contains(errMsg, "email") || !contains(errMsg, "invalid format") {
		t.Errorf("Error() = %q, should contain field and message", errMsg)
	}
}

// TestValidationErrors_Error tests ValidationErrors error message
func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   ValidationErrors
		contains string
	}{
		{
			name:     "Empty validation errors",
			errors:   ValidationErrors{},
			contains: "validation failed",
		},
		{
			name: "Single validation error",
			errors: ValidationErrors{
				{Field: "email", Message: "invalid"},
			},
			contains: "1 error",
		},
		{
			name: "Multiple validation errors",
			errors: ValidationErrors{
				{Field: "email", Message: "invalid"},
				{Field: "name", Message: "required"},
			},
			contains: "2 error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.errors.Error()
			if !contains(errMsg, tt.contains) {
				t.Errorf("Error() = %q, should contain %q", errMsg, tt.contains)
			}
		})
	}
}

// TestValidationErrors_Add tests adding validation errors
func TestValidationErrors_Add(t *testing.T) {
	var errs ValidationErrors

	errs.Add("email", "invalid format")
	errs.Add("name", "required")

	if len(errs) != 2 {
		t.Errorf("len(errs) = %d, want 2", len(errs))
	}

	if errs[0].Field != "email" {
		t.Errorf("First error field = %q, want %q", errs[0].Field, "email")
	}

	if errs[1].Field != "name" {
		t.Errorf("Second error field = %q, want %q", errs[1].Field, "name")
	}
}

// TestValidationErrors_HasErrors tests error detection
func TestValidationErrors_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   ValidationErrors
		expected bool
	}{
		{"Empty errors", ValidationErrors{}, false},
		{"With errors", ValidationErrors{{Field: "test", Message: "error"}}, true},
		{"Multiple errors", ValidationErrors{{Field: "a", Message: "1"}, {Field: "b", Message: "2"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.errors.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestBusinessRuleViolation_Error tests BusinessRuleViolation error message
func TestBusinessRuleViolation_Error(t *testing.T) {
	brv := BusinessRuleViolation{
		Rule:    "DAILY_LIMIT",
		Message: "Daily transaction limit exceeded",
		Context: map[string]interface{}{
			"limit":     1000,
			"attempted": 1500,
		},
	}

	errMsg := brv.Error()
	if !contains(errMsg, "DAILY_LIMIT") || !contains(errMsg, "Daily transaction limit exceeded") {
		t.Errorf("Error() = %q, should contain rule and message", errMsg)
	}
}

// TestNewBusinessRuleViolation tests BusinessRuleViolation creation
func TestNewBusinessRuleViolation(t *testing.T) {
	context := map[string]interface{}{
		"limit": 1000,
	}

	brv := NewBusinessRuleViolation("TEST_RULE", "Test message", context)

	if brv.Rule != "TEST_RULE" {
		t.Errorf("Rule = %q, want %q", brv.Rule, "TEST_RULE")
	}

	if brv.Message != "Test message" {
		t.Errorf("Message = %q, want %q", brv.Message, "Test message")
	}

	if brv.Context["limit"] != 1000 {
		t.Errorf("Context[limit] = %v, want 1000", brv.Context["limit"])
	}
}

// TestNewBusinessRuleViolation_NilContext tests creation with nil context
func TestNewBusinessRuleViolation_NilContext(t *testing.T) {
	brv := NewBusinessRuleViolation("TEST_RULE", "Test message", nil)

	if brv.Context != nil {
		t.Errorf("Context should be nil, got %v", brv.Context)
	}
}

// TestConcurrencyError_Error tests ConcurrencyError error message
func TestConcurrencyError_Error(t *testing.T) {
	ce := ConcurrencyError{
		EntityType: "Wallet",
		EntityID:   "wallet-123",
		Message:    "Version mismatch",
	}

	errMsg := ce.Error()
	if !contains(errMsg, "Wallet") || !contains(errMsg, "wallet-123") || !contains(errMsg, "Version mismatch") {
		t.Errorf("Error() = %q, should contain entity type, ID, and message", errMsg)
	}
}

// TestNewConcurrencyError tests ConcurrencyError creation
func TestNewConcurrencyError(t *testing.T) {
	ce := NewConcurrencyError("Wallet", "wallet-123", "Version mismatch")

	if ce.EntityType != "Wallet" {
		t.Errorf("EntityType = %q, want %q", ce.EntityType, "Wallet")
	}

	if ce.EntityID != "wallet-123" {
		t.Errorf("EntityID = %q, want %q", ce.EntityID, "wallet-123")
	}

	if ce.Message != "Version mismatch" {
		t.Errorf("Message = %q, want %q", ce.Message, "Version mismatch")
	}
}

// TestIsNotFound tests IsNotFound helper
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Sentinel ErrEntityNotFound", ErrEntityNotFound, true},
		{"Wrapped ErrEntityNotFound", NewDomainError("NOT_FOUND", "Not found", ErrEntityNotFound), true},
		{"Different error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.expected {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsValidationError tests IsValidationError helper
func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ValidationError", ValidationError{Field: "test", Message: "error"}, true},
		{"ValidationErrors", ValidationErrors{{Field: "test", Message: "error"}}, true},
		{"Different error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidationError(tt.err); got != tt.expected {
				t.Errorf("IsValidationError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsBusinessRuleViolation tests IsBusinessRuleViolation helper
func TestIsBusinessRuleViolation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"BusinessRuleViolation", NewBusinessRuleViolation("RULE", "message", nil), true},
		{"Different error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBusinessRuleViolation(tt.err); got != tt.expected {
				t.Errorf("IsBusinessRuleViolation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsConcurrencyError tests IsConcurrencyError helper
func TestIsConcurrencyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ConcurrencyError", NewConcurrencyError("Wallet", "123", "conflict"), true},
		{"Different error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConcurrencyError(tt.err); got != tt.expected {
				t.Errorf("IsConcurrencyError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestErrorWrapping tests that errors.Is works with wrapped domain errors
func TestErrorWrapping(t *testing.T) {
	baseErr := ErrInsufficientBalance
	wrappedErr := NewDomainError("INSUFFICIENT_BALANCE", "Not enough funds", baseErr)

	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is should recognize wrapped error")
	}
}

// TestErrorAs tests that errors.As works with custom error types
func TestErrorAs(t *testing.T) {
	brv := NewBusinessRuleViolation("TEST", "Test", nil)

	var target *BusinessRuleViolation
	if !errors.As(brv, &target) {
		t.Error("errors.As should work with BusinessRuleViolation")
	}

	if target.Rule != "TEST" {
		t.Errorf("Target rule = %q, want %q", target.Rule, "TEST")
	}
}

// Helper function for string containment checks
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
