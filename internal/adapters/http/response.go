// Package http содержит HTTP адаптеры (REST API).
//
// Структура пакета:
// - common/: Общие типы и helpers (вынесены для избежания циклических импортов)
// - middleware/: HTTP middleware (auth, logging, recovery)
// - handlers/: HTTP handlers для каждого ресурса
// - router.go: Конфигурация маршрутов
// - server.go: HTTP server lifecycle
//
// Pattern: Adapter (Hexagonal Architecture)
// - HTTP - внешний адаптер, который преобразует HTTP запросы в вызовы Use Cases
// - Не содержит бизнес-логики
// - Занимается только преобразованием данных и HTTP семантикой
package http

import (
	"github.com/Haleralex/wallethub/internal/adapters/http/common"
)

// Re-export types from common package for convenience
type (
	// APIResponse - стандартный формат ответа API.
	APIResponse = common.APIResponse
	// APIMeta - мета-информация для пагинации.
	APIMeta = common.APIMeta
	// APIError - структура ошибки API.
	APIError = common.APIError
	// FieldError - ошибка конкретного поля.
	FieldError = common.FieldError
)

// Re-export error codes
const (
	ErrCodeValidation       = common.ErrCodeValidation
	ErrCodeNotFound         = common.ErrCodeNotFound
	ErrCodeBadRequest       = common.ErrCodeBadRequest
	ErrCodeUnauthorized     = common.ErrCodeUnauthorized
	ErrCodeForbidden        = common.ErrCodeForbidden
	ErrCodeConflict         = common.ErrCodeConflict
	ErrCodeTooManyRequests  = common.ErrCodeTooManyRequests
	ErrCodeBusinessRule     = common.ErrCodeBusinessRule
	ErrCodeDuplicateRequest = common.ErrCodeDuplicateRequest
	ErrCodeInternal         = common.ErrCodeInternal
	ErrCodeConcurrency      = common.ErrCodeConcurrency
	ErrCodeTimeout          = common.ErrCodeTimeout
	ErrCodeUnavailable      = common.ErrCodeUnavailable
)

// Re-export functions
var (
	// GetRequestID возвращает Request ID из контекста.
	GetRequestID = common.GetRequestID
	// SetRequestID устанавливает Request ID в контекст.
	SetRequestID = common.SetRequestID
	// Success отправляет успешный ответ.
	Success = common.Success
	// SuccessWithMeta отправляет успешный ответ с мета-информацией.
	SuccessWithMeta = common.SuccessWithMeta
	// Error отправляет ответ с ошибкой.
	Error = common.Error
	// ValidationErrorResponse создаёт ответ для ошибок валидации.
	ValidationErrorResponse = common.ValidationErrorResponse
	// NotFoundResponse создаёт ответ для 404.
	NotFoundResponse = common.NotFoundResponse
	// BadRequestResponse создаёт ответ для некорректного запроса.
	BadRequestResponse = common.BadRequestResponse
	// UnauthorizedResponse создаёт ответ для 401.
	UnauthorizedResponse = common.UnauthorizedResponse
	// ForbiddenResponse создаёт ответ для 403.
	ForbiddenResponse = common.ForbiddenResponse
	// ConflictResponse создаёт ответ для 409.
	ConflictResponse = common.ConflictResponse
	// TooManyRequestsResponse создаёт ответ для rate limiting.
	TooManyRequestsResponse = common.TooManyRequestsResponse
	// InternalErrorResponse создаёт ответ для внутренней ошибки.
	InternalErrorResponse = common.InternalErrorResponse
	// HandleDomainError преобразует domain error в HTTP response.
	HandleDomainError = common.HandleDomainError
)
