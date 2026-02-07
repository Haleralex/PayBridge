package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReExportedTypes(t *testing.T) {
	// Test that re-exported types work correctly
	response := &APIResponse{
		Success: true,
		Data:    "test",
	}
	assert.True(t, response.Success)
	assert.Equal(t, "test", response.Data)

	meta := &APIMeta{
		Page:       1,
		PerPage:    10,
		Total:      100,
		TotalPages: 10,
	}
	assert.Equal(t, 1, meta.Page)
	assert.Equal(t, 10, meta.PerPage)
	assert.Equal(t, 100, meta.Total)
	assert.Equal(t, 10, meta.TotalPages)

	apiErr := &APIError{
		Code:    "TEST_ERROR",
		Message: "test message",
	}
	assert.Equal(t, "TEST_ERROR", apiErr.Code)
	assert.Equal(t, "test message", apiErr.Message)

	fieldErr := &FieldError{
		Field:   "email",
		Message: "invalid email",
	}
	assert.Equal(t, "email", fieldErr.Field)
	assert.Equal(t, "invalid email", fieldErr.Message)
}

func TestReExportedErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"validation", ErrCodeValidation, "VALIDATION_ERROR"},
		{"not found", ErrCodeNotFound, "NOT_FOUND"},
		{"bad request", ErrCodeBadRequest, "BAD_REQUEST"},
		{"unauthorized", ErrCodeUnauthorized, "UNAUTHORIZED"},
		{"forbidden", ErrCodeForbidden, "FORBIDDEN"},
		{"conflict", ErrCodeConflict, "CONFLICT"},
		{"too many requests", ErrCodeTooManyRequests, "TOO_MANY_REQUESTS"},
		{"business rule", ErrCodeBusinessRule, "BUSINESS_RULE_VIOLATION"},
		{"duplicate request", ErrCodeDuplicateRequest, "DUPLICATE_REQUEST"},
		{"internal", ErrCodeInternal, "INTERNAL_ERROR"},
		{"concurrency", ErrCodeConcurrency, "CONCURRENCY_ERROR"},
		{"timeout", ErrCodeTimeout, "TIMEOUT"},
		{"unavailable", ErrCodeUnavailable, "SERVICE_UNAVAILABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code)
		})
	}
}

func TestReExportedFunctions(t *testing.T) {
	// Just verify that functions are accessible
	assert.NotNil(t, GetRequestID)
	assert.NotNil(t, SetRequestID)
	assert.NotNil(t, Success)
	assert.NotNil(t, SuccessWithMeta)
	assert.NotNil(t, Error)
	assert.NotNil(t, ValidationErrorResponse)
	assert.NotNil(t, NotFoundResponse)
	assert.NotNil(t, BadRequestResponse)
	assert.NotNil(t, UnauthorizedResponse)
	assert.NotNil(t, ForbiddenResponse)
	assert.NotNil(t, ConflictResponse)
	assert.NotNil(t, TooManyRequestsResponse)
	assert.NotNil(t, InternalErrorResponse)
	assert.NotNil(t, HandleDomainError)
}
