package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	domainerrors "github.com/yourusername/wallethub/internal/domain/errors"
)

func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(RequestIDKey, "test-request-123")
	return c, w
}

// ============================================
// Test Request ID Functions
// ============================================

func TestGetRequestID(t *testing.T) {
	t.Run("ReturnsRequestID", func(t *testing.T) {
		c, _ := setupTestContext()
		id := GetRequestID(c)
		assert.Equal(t, "test-request-123", id)
	})

	t.Run("ReturnsEmptyWhenNotSet", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		id := GetRequestID(c)
		assert.Empty(t, id)
	})
}

func TestSetRequestID(t *testing.T) {
	c, w := setupTestContext()
	SetRequestID(c, "new-id-456")

	assert.Equal(t, "new-id-456", GetRequestID(c))
	assert.Equal(t, "new-id-456", w.Header().Get(RequestIDKey))
}

// ============================================
// Test Success Responses
// ============================================

func TestSuccess(t *testing.T) {
	c, w := setupTestContext()

	data := map[string]string{"status": "ok", "message": "success"}
	Success(c, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)
	assert.Equal(t, "test-request-123", response.RequestID)
	assert.False(t, response.Timestamp.IsZero())
}

func TestSuccessWithMeta(t *testing.T) {
	c, w := setupTestContext()

	data := []string{"item1", "item2"}
	meta := &APIMeta{
		Page:       1,
		PerPage:    20,
		Total:      100,
		TotalPages: 5,
	}

	SuccessWithMeta(c, http.StatusOK, data, meta)

	assert.Equal(t, http.StatusOK, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.True(t, response.Success)
	assert.NotNil(t, response.Meta)
	assert.Equal(t, 1, response.Meta.Page)
	assert.Equal(t, 100, response.Meta.Total)
}

// ============================================
// Test Error Responses
// ============================================

func TestError(t *testing.T) {
	c, w := setupTestContext()

	apiError := &APIError{
		Code:    ErrCodeValidation,
		Message: "Validation failed",
	}

	Error(c, http.StatusBadRequest, apiError)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, ErrCodeValidation, response.Error.Code)
}

func TestValidationErrorResponse(t *testing.T) {
	c, w := setupTestContext()

	fields := []FieldError{
		{Field: "email", Message: "Invalid format", Code: "email"},
		{Field: "name", Message: "Required", Code: "required"},
	}

	ValidationErrorResponse(c, fields)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.False(t, response.Success)
	assert.Equal(t, ErrCodeValidation, response.Error.Code)
	assert.Len(t, response.Error.Fields, 2)
}

func TestNotFoundResponse(t *testing.T) {
	c, w := setupTestContext()

	NotFoundResponse(c, "User")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.False(t, response.Success)
	assert.Equal(t, ErrCodeNotFound, response.Error.Code)
	assert.Contains(t, response.Error.Message, "User")
}

func TestBadRequestResponse(t *testing.T) {
	c, w := setupTestContext()

	BadRequestResponse(c, "Invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeBadRequest, response.Error.Code)
}

func TestUnauthorizedResponse(t *testing.T) {
	c, w := setupTestContext()

	UnauthorizedResponse(c, "Token expired")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeUnauthorized, response.Error.Code)
}

func TestForbiddenResponse(t *testing.T) {
	c, w := setupTestContext()

	ForbiddenResponse(c, "Access denied")

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeForbidden, response.Error.Code)
}

func TestConflictResponse(t *testing.T) {
	c, w := setupTestContext()

	ConflictResponse(c, "Resource already exists")

	assert.Equal(t, http.StatusConflict, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeConflict, response.Error.Code)
}

func TestTooManyRequestsResponse(t *testing.T) {
	c, w := setupTestContext()

	TooManyRequestsResponse(c, 60)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeTooManyRequests, response.Error.Code)
	assert.Equal(t, 60, response.Error.RetryAfter)
}

func TestInternalErrorResponse(t *testing.T) {
	c, w := setupTestContext()

	InternalErrorResponse(c, "Database error")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, ErrCodeInternal, response.Error.Code)
}

// ============================================
// Test HandleDomainError
// ============================================

func TestHandleDomainError(t *testing.T) {
	t.Run("ValidationError", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.ValidationError{
			Field:   "email",
			Message: "invalid format",
		}

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("BusinessRuleViolation", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewBusinessRuleViolation(
			"INSUFFICIENT_BALANCE",
			"Not enough funds",
			map[string]interface{}{"balance": 100, "required": 200},
		)

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		var response APIResponse
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, ErrCodeBusinessRule, response.Error.Code)
		assert.NotNil(t, response.Error.Details)
	})

	t.Run("ConcurrencyError", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewConcurrencyError("Wallet", "123", "Version mismatch")

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusConflict, w.Code)

		var response APIResponse
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, ErrCodeConcurrency, response.Error.Code)
	})

	t.Run("NotFoundError", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewDomainError("USER_NOT_FOUND", "User not found", domainerrors.ErrEntityNotFound)

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("DomainError_UserNotFound", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewDomainError("USER_NOT_FOUND", "User not found", nil)

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response APIResponse
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, "USER_NOT_FOUND", response.Error.Code)
	})

	t.Run("DomainError_InsufficientBalance", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewDomainError("INSUFFICIENT_BALANCE", "Not enough balance", nil)

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("GenericError", func(t *testing.T) {
		c, w := setupTestContext()

		err := domainerrors.NewDomainError("UNKNOWN_ERROR", "Something went wrong", nil)

		HandleDomainError(c, err)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================
// Test Error Extractors
// ============================================

func TestExtractValidationError(t *testing.T) {
	valErr := domainerrors.ValidationError{Field: "email", Message: "invalid"}
	extracted := extractValidationError(valErr)
	assert.NotNil(t, extracted)
	assert.Equal(t, "email", extracted.Field)
}

func TestExtractBusinessRuleViolation(t *testing.T) {
	brv := domainerrors.NewBusinessRuleViolation("RULE", "message", nil)
	extracted := extractBusinessRuleViolation(brv)
	assert.NotNil(t, extracted)
	assert.Equal(t, "RULE", extracted.Rule)
}

func TestExtractDomainError(t *testing.T) {
	domainErr := domainerrors.NewDomainError("CODE", "message", nil)
	extracted := extractDomainError(domainErr)
	assert.NotNil(t, extracted)
	assert.Equal(t, "CODE", extracted.Code)
}
