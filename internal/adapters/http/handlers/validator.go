// Package handlers содержит HTTP handlers для REST API.
//
// Handler - это Adapter в терминах Clean Architecture:
// - Принимает HTTP запрос
// - Преобразует в Command/Query DTO
// - Вызывает Use Case
// - Преобразует результат в HTTP ответ
//
// SOLID:
// - SRP: Каждый handler отвечает за один endpoint
// - DIP: Handler зависит от интерфейса Use Case
package handlers

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/yourusername/wallethub/internal/adapters/http/common"
)

// ============================================
// Custom Validator Setup
// ============================================

var (
	setupOnce sync.Once
)

// SetupValidator настраивает кастомные валидаторы для Gin.
func SetupValidator() {
	setupOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			// Используем json tag для имён полей в ошибках
			v.RegisterTagNameFunc(func(fld reflect.StructField) string {
				name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
				if name == "-" {
					return ""
				}
				return name
			})

			// Регистрируем кастомные валидаторы
			_ = v.RegisterValidation("currency_code", validateCurrencyCode)
			_ = v.RegisterValidation("money_amount", validateMoneyAmount)
			_ = v.RegisterValidation("kyc_status", validateKYCStatus)
			_ = v.RegisterValidation("wallet_status", validateWalletStatus)
			_ = v.RegisterValidation("transaction_type", validateTransactionType)
		}
	})
}

// ============================================
// Custom Validators
// ============================================

// validateCurrencyCode проверяет код валюты (3 буквы).
func validateCurrencyCode(fl validator.FieldLevel) bool {
	code := fl.Field().String()
	if len(code) != 3 {
		return false
	}

	// Проверяем, что все символы - заглавные буквы
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}

	return true
}

// validateMoneyAmount проверяет формат суммы (decimal string).
var moneyPattern = regexp.MustCompile(`^\d+(\.\d{1,8})?$`)

func validateMoneyAmount(fl validator.FieldLevel) bool {
	amount := fl.Field().String()
	return moneyPattern.MatchString(amount)
}

// validateKYCStatus проверяет статус KYC.
func validateKYCStatus(fl validator.FieldLevel) bool {
	status := fl.Field().String()
	validStatuses := map[string]bool{
		"UNVERIFIED": true,
		"PENDING":    true,
		"VERIFIED":   true,
		"REJECTED":   true,
	}
	return validStatuses[status]
}

// validateWalletStatus проверяет статус кошелька.
func validateWalletStatus(fl validator.FieldLevel) bool {
	status := fl.Field().String()
	validStatuses := map[string]bool{
		"ACTIVE":    true,
		"SUSPENDED": true,
		"LOCKED":    true,
		"CLOSED":    true,
	}
	return validStatuses[status]
}

// validateTransactionType проверяет тип транзакции.
func validateTransactionType(fl validator.FieldLevel) bool {
	txType := fl.Field().String()
	validTypes := map[string]bool{
		"DEPOSIT":    true,
		"WITHDRAW":   true,
		"PAYOUT":     true,
		"TRANSFER":   true,
		"FEE":        true,
		"REFUND":     true,
		"ADJUSTMENT": true,
	}
	return validTypes[txType]
}

// ============================================
// Validation Error Handling
// ============================================

// HandleValidationErrors преобразует ошибки валидации в HTTP ответ.
func HandleValidationErrors(c *gin.Context, err error) {
	var fieldErrors []common.FieldError

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range validationErrors {
			fieldErrors = append(fieldErrors, common.FieldError{
				Field:   fieldErr.Field(),
				Message: getValidationMessage(fieldErr),
				Code:    fieldErr.Tag(),
			})
		}
	}

	if len(fieldErrors) == 0 {
		// Если не удалось распарсить - общая ошибка
		common.BadRequestResponse(c, "Invalid request body: "+err.Error())
		return
	}

	common.ValidationErrorResponse(c, fieldErrors)
}

// getValidationMessage возвращает человекочитаемое сообщение об ошибке.
func getValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "uuid":
		return "Invalid UUID format"
	case "min":
		return "Value is too short (minimum: " + fe.Param() + ")"
	case "max":
		return "Value is too long (maximum: " + fe.Param() + ")"
	case "len":
		return "Value must be exactly " + fe.Param() + " characters"
	case "oneof":
		return "Value must be one of: " + fe.Param()
	case "currency_code":
		return "Invalid currency code (must be 3 uppercase letters)"
	case "money_amount":
		return "Invalid amount format (use decimal like '100.50')"
	case "kyc_status":
		return "Invalid KYC status"
	case "wallet_status":
		return "Invalid wallet status"
	case "transaction_type":
		return "Invalid transaction type"
	default:
		return "Invalid value"
	}
}

// ============================================
// Request Parsing Helpers
// ============================================

// BindJSON биндит JSON тело запроса и возвращает ошибку если что-то не так.
// Возвращает true если успешно, false если была ошибка (ответ уже отправлен).
func BindJSON[T any](c *gin.Context, req *T) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		HandleValidationErrors(c, err)
		return false
	}
	return true
}

// BindQuery биндит query параметры.
func BindQuery[T any](c *gin.Context, req *T) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		HandleValidationErrors(c, err)
		return false
	}
	return true
}

// BindURI биндит URI параметры.
func BindURI[T any](c *gin.Context, req *T) bool {
	if err := c.ShouldBindUri(req); err != nil {
		HandleValidationErrors(c, err)
		return false
	}
	return true
}

// ============================================
// Pagination Helper
// ============================================

// PaginationParams - параметры пагинации из query string.
type PaginationParams struct {
	Page    int `form:"page" binding:"min=1"`
	PerPage int `form:"per_page" binding:"min=1,max=100"`
}

// DefaultPaginationParams возвращает параметры по умолчанию.
func DefaultPaginationParams() PaginationParams {
	return PaginationParams{
		Page:    1,
		PerPage: 20,
	}
}

// Offset вычисляет offset для SQL запроса.
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// ParsePagination парсит параметры пагинации из запроса.
func ParsePagination(c *gin.Context) PaginationParams {
	params := DefaultPaginationParams()

	if page := c.Query("page"); page != "" {
		if p := parseInt(page); p > 0 {
			params.Page = p
		}
	}

	if perPage := c.Query("per_page"); perPage != "" {
		if pp := parseInt(perPage); pp > 0 && pp <= 100 {
			params.PerPage = pp
		}
	}

	return params
}

// parseInt парсит строку в int.
func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// BuildMeta создаёт мета-информацию для пагинированного ответа.
func BuildMeta(params PaginationParams, total int) *common.APIMeta {
	totalPages := total / params.PerPage
	if total%params.PerPage > 0 {
		totalPages++
	}

	return &common.APIMeta{
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}
