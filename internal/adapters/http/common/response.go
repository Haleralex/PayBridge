// Package common содержит общие типы для HTTP слоя.
//
// Вынесен в отдельный пакет чтобы избежать циклических импортов
// между handlers и основным http пакетом.
package common

import (
	"net/http"
	"time"

	domainerrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/gin-gonic/gin"
)

// ============================================
// Standard API Response Format
// ============================================

// APIResponse - стандартный формат ответа API.
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *APIMeta    `json:"meta,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIMeta - мета-информация для пагинации.
type APIMeta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// APIError - структура ошибки API.
type APIError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Fields     []FieldError           `json:"fields,omitempty"`
	RetryAfter int                    `json:"retry_after,omitempty"`
}

// FieldError - ошибка конкретного поля.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ============================================
// Error Codes
// ============================================

const (
	ErrCodeValidation       = "VALIDATION_ERROR"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeTooManyRequests  = "TOO_MANY_REQUESTS"
	ErrCodeBusinessRule     = "BUSINESS_RULE_VIOLATION"
	ErrCodeDuplicateRequest = "DUPLICATE_REQUEST"
	ErrCodeInternal         = "INTERNAL_ERROR"
	ErrCodeConcurrency      = "CONCURRENCY_ERROR"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeUnavailable      = "SERVICE_UNAVAILABLE"
)

// ============================================
// Request ID
// ============================================

const RequestIDKey = "X-Request-ID"

// GetRequestID возвращает Request ID из контекста.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDKey); exists {
		return id.(string)
	}
	return ""
}

// SetRequestID устанавливает Request ID в контекст.
func SetRequestID(c *gin.Context, id string) {
	c.Set(RequestIDKey, id)
	c.Header(RequestIDKey, id)
}

// ============================================
// Response Helpers
// ============================================

// Success отправляет успешный ответ.
func Success(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, APIResponse{
		Success:   true,
		Data:      data,
		RequestID: GetRequestID(c),
		Timestamp: time.Now().UTC(),
	})
}

// SuccessWithMeta отправляет успешный ответ с мета-информацией.
func SuccessWithMeta(c *gin.Context, statusCode int, data interface{}, meta *APIMeta) {
	c.JSON(statusCode, APIResponse{
		Success:   true,
		Data:      data,
		Meta:      meta,
		RequestID: GetRequestID(c),
		Timestamp: time.Now().UTC(),
	})
}

// Error отправляет ответ с ошибкой.
func Error(c *gin.Context, statusCode int, apiError *APIError) {
	c.JSON(statusCode, APIResponse{
		Success:   false,
		Error:     apiError,
		RequestID: GetRequestID(c),
		Timestamp: time.Now().UTC(),
	})
}

// ============================================
// Error Response Helpers
// ============================================

// ValidationErrorResponse создаёт ответ для ошибок валидации.
func ValidationErrorResponse(c *gin.Context, fields []FieldError) {
	Error(c, http.StatusBadRequest, &APIError{
		Code:    ErrCodeValidation,
		Message: "Request validation failed",
		Fields:  fields,
	})
}

// NotFoundResponse создаёт ответ для 404.
func NotFoundResponse(c *gin.Context, resource string) {
	Error(c, http.StatusNotFound, &APIError{
		Code:    ErrCodeNotFound,
		Message: resource + " not found",
		Details: map[string]interface{}{
			"resource": resource,
		},
	})
}

// BadRequestResponse создаёт ответ для некорректного запроса.
func BadRequestResponse(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, &APIError{
		Code:    ErrCodeBadRequest,
		Message: message,
	})
}

// UnauthorizedResponse создаёт ответ для 401.
func UnauthorizedResponse(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, &APIError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	})
}

// ForbiddenResponse создаёт ответ для 403.
func ForbiddenResponse(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, &APIError{
		Code:    ErrCodeForbidden,
		Message: message,
	})
}

// ConflictResponse создаёт ответ для 409.
func ConflictResponse(c *gin.Context, message string) {
	Error(c, http.StatusConflict, &APIError{
		Code:    ErrCodeConflict,
		Message: message,
	})
}

// TooManyRequestsResponse создаёт ответ для rate limiting.
func TooManyRequestsResponse(c *gin.Context, retryAfter int) {
	Error(c, http.StatusTooManyRequests, &APIError{
		Code:       ErrCodeTooManyRequests,
		Message:    "Too many requests, please try again later",
		RetryAfter: retryAfter,
	})
}

// InternalErrorResponse создаёт ответ для внутренней ошибки.
func InternalErrorResponse(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, &APIError{
		Code:    ErrCodeInternal,
		Message: message,
	})
}

// ============================================
// Domain Error to HTTP Error Mapper
// ============================================

// HandleDomainError преобразует domain error в HTTP response.
func HandleDomainError(c *gin.Context, err error) {
	// 1. Проверяем ValidationError
	if domainerrors.IsValidationError(err) {
		if valErr := extractValidationError(err); valErr != nil {
			ValidationErrorResponse(c, []FieldError{
				{Field: valErr.Field, Message: valErr.Message, Code: "invalid"},
			})
			return
		}
		BadRequestResponse(c, err.Error())
		return
	}

	// 2. Проверяем BusinessRuleViolation
	if domainerrors.IsBusinessRuleViolation(err) {
		if brv := extractBusinessRuleViolation(err); brv != nil {
			Error(c, http.StatusUnprocessableEntity, &APIError{
				Code:    ErrCodeBusinessRule,
				Message: brv.Message,
				Details: map[string]interface{}{
					"rule":    brv.Rule,
					"context": brv.Context,
				},
			})
			return
		}
	}

	// 3. Проверяем ConcurrencyError
	if domainerrors.IsConcurrencyError(err) {
		Error(c, http.StatusConflict, &APIError{
			Code:    ErrCodeConcurrency,
			Message: "Resource was modified by another request, please retry",
			Details: map[string]interface{}{
				"retryable": true,
			},
		})
		return
	}

	// 4. Проверяем NotFound
	if domainerrors.IsNotFound(err) {
		NotFoundResponse(c, "Resource")
		return
	}

	// 5. Проверяем DomainError
	if domainErr := extractDomainError(err); domainErr != nil {
		statusCode := http.StatusBadRequest

		switch domainErr.Code {
		case "USER_NOT_FOUND", "WALLET_NOT_FOUND", "TRANSACTION_NOT_FOUND":
			statusCode = http.StatusNotFound
		case "INSUFFICIENT_BALANCE", "USER_NOT_VERIFIED":
			statusCode = http.StatusUnprocessableEntity
		}

		Error(c, statusCode, &APIError{
			Code:    domainErr.Code,
			Message: domainErr.Message,
		})
		return
	}

	// 6. Default: Internal Server Error
	InternalErrorResponse(c, "An unexpected error occurred")
}

// extractValidationError извлекает ValidationError из цепочки ошибок.
func extractValidationError(err error) *domainerrors.ValidationError {
	for e := err; e != nil; e = unwrap(e) {
		if v, ok := e.(domainerrors.ValidationError); ok {
			return &v
		}
	}
	return nil
}

// extractBusinessRuleViolation извлекает BusinessRuleViolation из цепочки ошибок.
func extractBusinessRuleViolation(err error) *domainerrors.BusinessRuleViolation {
	for e := err; e != nil; e = unwrap(e) {
		if v, ok := e.(*domainerrors.BusinessRuleViolation); ok {
			return v
		}
	}
	return nil
}

// extractDomainError извлекает DomainError из цепочки ошибок.
func extractDomainError(err error) *domainerrors.DomainError {
	for e := err; e != nil; e = unwrap(e) {
		if v, ok := e.(*domainerrors.DomainError); ok {
			return v
		}
	}
	return nil
}

// unwrap получает wrapped error
func unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}
