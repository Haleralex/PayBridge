// Package handlers - Transaction HTTP handlers.
package handlers

import (
	"net/http"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/application/cqrs"
	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================
// Transaction Handler
// ============================================

// TransactionHandler обрабатывает HTTP запросы для транзакций.
// Все операции диспатчатся через CQRS Command/Query Bus.
type TransactionHandler struct {
	commandBus *cqrs.CommandBus
	queryBus   *cqrs.QueryBus
}

// NewTransactionHandler создаёт новый TransactionHandler.
func NewTransactionHandler(commandBus *cqrs.CommandBus, queryBus *cqrs.QueryBus) *TransactionHandler {
	return &TransactionHandler{
		commandBus: commandBus,
		queryBus:   queryBus,
	}
}

// ============================================
// Request DTOs
// ============================================

// TransactionIDParam - параметр ID транзакции из URL.
type TransactionIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// ListTransactionsParams - параметры фильтрации для списка транзакций.
type ListTransactionsParams struct {
	WalletID string `form:"wallet_id" binding:"omitempty,uuid"`
	UserID   string `form:"user_id" binding:"omitempty,uuid"`
	Type     string `form:"type" binding:"omitempty,oneof=DEPOSIT WITHDRAW PAYOUT TRANSFER FEE REFUND ADJUSTMENT"`
	Status   string `form:"status" binding:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED CANCELLED"`
}

// CancelTransactionRequest - запрос на отмену транзакции.
//
// @Description Cancel transaction request body
type CancelTransactionRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// ============================================
// HTTP Handlers
// ============================================

// GetTransaction возвращает транзакцию по ID.
//
// @Summary Get transaction by ID
// @Description Get transaction details by UUID
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID" format(uuid)
// @Success 200 {object} common.APIResponse{data=dtos.TransactionDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	var params TransactionIDParam
	if !BindURI(c, &params) {
		return
	}

	if _, err := uuid.Parse(params.ID); err != nil {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "id", Message: "Invalid UUID format", Code: "uuid"},
		})
		return
	}

	query := dtos.GetTransactionQuery{TransactionID: params.ID}

	result, err := cqrs.DispatchQuery[dtos.GetTransactionQuery, *dtos.TransactionDTO](h.queryBus, c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// ListTransactions возвращает список транзакций с фильтрацией.
//
// @Summary List transactions
// @Description Get paginated list of transactions with optional filters
// @Tags Transactions
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20) maximum(100)
// @Param wallet_id query string false "Filter by wallet ID" format(uuid)
// @Param user_id query string false "Filter by user ID" format(uuid)
// @Param type query string false "Filter by type" Enums(DEPOSIT, WITHDRAW, PAYOUT, TRANSFER, FEE, REFUND, ADJUSTMENT)
// @Param status query string false "Filter by status" Enums(PENDING, PROCESSING, COMPLETED, FAILED, CANCELLED)
// @Success 200 {object} common.APIResponse{data=dtos.TransactionListDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/transactions [get]
func (h *TransactionHandler) ListTransactions(c *gin.Context) {
	pagination := ParsePagination(c)

	var filters ListTransactionsParams
	if !BindQuery(c, &filters) {
		return
	}

	query := dtos.ListTransactionsQuery{
		Offset: pagination.Offset(),
		Limit:  pagination.PerPage,
	}

	if filters.WalletID != "" {
		query.WalletID = &filters.WalletID
	}
	if filters.UserID != "" {
		query.UserID = &filters.UserID
	}
	if filters.Type != "" {
		query.Type = &filters.Type
	}
	if filters.Status != "" {
		query.Status = &filters.Status
	}

	result, err := cqrs.DispatchQuery[dtos.ListTransactionsQuery, *dtos.TransactionListDTO](h.queryBus, c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	meta := BuildMeta(pagination, result.TotalCount)
	common.SuccessWithMeta(c, http.StatusOK, result, meta)
}

// GetTransactionByIdempotencyKey возвращает транзакцию по ключу идемпотентности.
//
// @Summary Get transaction by idempotency key
// @Description Get transaction details by idempotency key (useful for checking duplicates)
// @Tags Transactions
// @Accept json
// @Produce json
// @Param key path string true "Idempotency Key"
// @Success 200 {object} common.APIResponse{data=dtos.TransactionDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/transactions/by-key/{key} [get]
func (h *TransactionHandler) GetTransactionByIdempotencyKey(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "key", Message: "Idempotency key is required", Code: "required"},
		})
		return
	}

	query := dtos.GetTransactionByIdempotencyKeyQuery{
		IdempotencyKey: key,
	}

	result, err := cqrs.DispatchQuery[dtos.GetTransactionByIdempotencyKeyQuery, *dtos.TransactionDTO](h.queryBus, c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// RetryTransaction повторяет failed транзакцию.
//
// @Summary Retry a failed transaction
// @Description Retry a transaction that previously failed
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID" format(uuid)
// @Success 200 {object} common.APIResponse{data=dtos.TransactionDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 422 {object} common.APIResponse "Transaction is not in failed state"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/transactions/{id}/retry [post]
func (h *TransactionHandler) RetryTransaction(c *gin.Context) {
	var params TransactionIDParam
	if !BindURI(c, &params) {
		return
	}

	cmd := dtos.RetryTransactionCommand{TransactionID: params.ID}

	result, err := cqrs.DispatchCommand[dtos.RetryTransactionCommand, *dtos.TransactionDTO](h.commandBus, c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// CancelTransaction отменяет pending транзакцию.
//
// @Summary Cancel a pending transaction
// @Description Cancel a transaction that is in pending state
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID" format(uuid)
// @Param request body CancelTransactionRequest true "Cancel reason"
// @Success 200 {object} common.APIResponse{data=dtos.TransactionDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 422 {object} common.APIResponse "Transaction cannot be cancelled"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/transactions/{id}/cancel [post]
func (h *TransactionHandler) CancelTransaction(c *gin.Context) {
	var params TransactionIDParam
	if !BindURI(c, &params) {
		return
	}

	var req CancelTransactionRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.CancelTransactionCommand{
		TransactionID: params.ID,
		Reason:        req.Reason,
	}

	result, err := cqrs.DispatchCommand[dtos.CancelTransactionCommand, *dtos.TransactionDTO](h.commandBus, c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// GetWalletTransactions возвращает транзакции конкретного кошелька.
//
// @Summary Get wallet transactions
// @Description Get paginated list of transactions for a specific wallet
// @Tags Transactions
// @Accept json
// @Produce json
// @Param wallet_id path string true "Wallet ID" format(uuid)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20) maximum(100)
// @Param type query string false "Filter by type" Enums(DEPOSIT, WITHDRAW, PAYOUT, TRANSFER, FEE, REFUND, ADJUSTMENT)
// @Param status query string false "Filter by status" Enums(PENDING, PROCESSING, COMPLETED, FAILED, CANCELLED)
// @Success 200 {object} common.APIResponse{data=dtos.TransactionListDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/{id}/transactions [get]
func (h *TransactionHandler) GetWalletTransactions(c *gin.Context) {
	walletID := c.Param("id")
	if walletID == "" {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "wallet_id", Message: "Wallet ID is required", Code: "required"},
		})
		return
	}

	if _, err := uuid.Parse(walletID); err != nil {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "wallet_id", Message: "Invalid UUID format", Code: "uuid"},
		})
		return
	}

	pagination := ParsePagination(c)

	var filters ListTransactionsParams
	if !BindQuery(c, &filters) {
		return
	}

	query := dtos.ListTransactionsQuery{
		WalletID: &walletID,
		Offset:   pagination.Offset(),
		Limit:    pagination.PerPage,
	}

	if filters.Type != "" {
		query.Type = &filters.Type
	}
	if filters.Status != "" {
		query.Status = &filters.Status
	}

	result, err := cqrs.DispatchQuery[dtos.ListTransactionsQuery, *dtos.TransactionListDTO](h.queryBus, c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	meta := BuildMeta(pagination, result.TotalCount)
	common.SuccessWithMeta(c, http.StatusOK, result, meta)
}

// RegisterRoutes регистрирует маршруты для TransactionHandler.
func (h *TransactionHandler) RegisterRoutes(router *gin.RouterGroup) {
	transactions := router.Group("/transactions")
	{
		transactions.GET("", h.ListTransactions)
		transactions.GET("/:id", h.GetTransaction)
		transactions.GET("/by-key/:key", h.GetTransactionByIdempotencyKey)
		transactions.POST("/:id/retry", h.RetryTransaction)
		transactions.POST("/:id/cancel", h.CancelTransaction)
	}
}

// RegisterWalletTransactionsRoute регистрирует маршрут для транзакций кошелька.
func (h *TransactionHandler) RegisterWalletTransactionsRoute(walletRoutes *gin.RouterGroup) {
	walletRoutes.GET("/:id/transactions", h.GetWalletTransactions)
}
