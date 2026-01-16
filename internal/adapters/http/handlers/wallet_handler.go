// Package handlers - Wallet HTTP handlers.
package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/adapters/http/common"
	"github.com/yourusername/wallethub/internal/adapters/http/middleware"
	"github.com/yourusername/wallethub/internal/application/dtos"
)

// ============================================
// Use Case Interfaces
// ============================================

// CreateWalletUseCase - интерфейс для создания кошелька.
type CreateWalletUseCase interface {
	Execute(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error)
}

// CreditWalletUseCase - интерфейс для пополнения кошелька.
type CreditWalletUseCase interface {
	Execute(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error)
}

// DebitWalletUseCase - интерфейс для списания с кошелька.
type DebitWalletUseCase interface {
	Execute(ctx context.Context, cmd dtos.DebitWalletCommand) (*dtos.WalletOperationDTO, error)
}

// TransferFundsUseCase - интерфейс для перевода между кошельками.
type TransferFundsUseCase interface {
	Execute(ctx context.Context, cmd dtos.TransferFundsCommand) (*dtos.TransferResultDTO, error)
}

// GetWalletUseCase - интерфейс для получения кошелька.
type GetWalletUseCase interface {
	Execute(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error)
}

// ListWalletsUseCase - интерфейс для получения списка кошельков.
type ListWalletsUseCase interface {
	Execute(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error)
}

// ============================================
// Wallet Handler
// ============================================

// WalletHandler обрабатывает HTTP запросы для кошельков.
type WalletHandler struct {
	createWallet  CreateWalletUseCase
	creditWallet  CreditWalletUseCase
	debitWallet   DebitWalletUseCase
	transferFunds TransferFundsUseCase
	getWallet     GetWalletUseCase
	listWallets   ListWalletsUseCase
}

// NewWalletHandler создаёт новый WalletHandler.
func NewWalletHandler(
	createWallet CreateWalletUseCase,
	creditWallet CreditWalletUseCase,
	debitWallet DebitWalletUseCase,
	transferFunds TransferFundsUseCase,
	getWallet GetWalletUseCase,
	listWallets ListWalletsUseCase,
) *WalletHandler {
	return &WalletHandler{
		createWallet:  createWallet,
		creditWallet:  creditWallet,
		debitWallet:   debitWallet,
		transferFunds: transferFunds,
		getWallet:     getWallet,
		listWallets:   listWallets,
	}
}

// ============================================
// Request DTOs
// ============================================

// CreateWalletRequest - запрос на создание кошелька.
//
// @Description Create wallet request body
type CreateWalletRequest struct {
	UserID       string `json:"user_id" binding:"required,uuid"`
	CurrencyCode string `json:"currency_code" binding:"required,len=3,currency_code"`
}

// CreditWalletRequest - запрос на пополнение кошелька.
//
// @Description Credit wallet request body
type CreditWalletRequest struct {
	Amount            string `json:"amount" binding:"required,money_amount"`
	IdempotencyKey    string `json:"idempotency_key" binding:"required,uuid"`
	Description       string `json:"description" binding:"required,min=1,max=500"`
	ExternalReference string `json:"external_reference,omitempty"`
}

// DebitWalletRequest - запрос на списание с кошелька.
//
// @Description Debit wallet request body
type DebitWalletRequest struct {
	Amount            string `json:"amount" binding:"required,money_amount"`
	IdempotencyKey    string `json:"idempotency_key" binding:"required,uuid"`
	Description       string `json:"description" binding:"required,min=1,max=500"`
	ExternalReference string `json:"external_reference,omitempty"`
}

// TransferFundsRequest - запрос на перевод между кошельками.
//
// @Description Transfer funds request body
type TransferFundsRequest struct {
	DestinationWalletID string `json:"destination_wallet_id" binding:"required,uuid"`
	Amount              string `json:"amount" binding:"required,money_amount"`
	IdempotencyKey      string `json:"idempotency_key" binding:"required,uuid"`
	Description         string `json:"description" binding:"required,min=1,max=500"`
}

// WalletIDParam - параметр ID кошелька из URL.
type WalletIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// ListWalletsParams - параметры для списка кошельков.
type ListWalletsParams struct {
	UserID       string `form:"user_id" binding:"omitempty,uuid"`
	CurrencyCode string `form:"currency_code" binding:"omitempty,len=3"`
	Status       string `form:"status" binding:"omitempty,oneof=ACTIVE SUSPENDED LOCKED CLOSED"`
}

// ============================================
// HTTP Handlers
// ============================================

// CreateWallet создаёт новый кошелёк.
//
// @Summary Create a new wallet
// @Description Create a new wallet for a user with specified currency
// @Tags Wallets
// @Accept json
// @Produce json
// @Param request body CreateWalletRequest true "Wallet data"
// @Success 201 {object} common.APIResponse{data=dtos.WalletDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse "User not found"
// @Failure 409 {object} common.APIResponse "Wallet already exists"
// @Failure 422 {object} common.APIResponse "User not verified"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets [post]
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	var req CreateWalletRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.CreateWalletCommand{
		UserID:       req.UserID,
		CurrencyCode: req.CurrencyCode,
	}

	result, err := h.createWallet.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusCreated, result)
}

// GetWallet возвращает кошелёк по ID.
//
// @Summary Get wallet by ID
// @Description Get wallet details by UUID
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "Wallet ID" format(uuid)
// @Success 200 {object} common.APIResponse{data=dtos.WalletDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/{id} [get]
func (h *WalletHandler) GetWallet(c *gin.Context) {
	var params WalletIDParam
	if !BindURI(c, &params) {
		return
	}

	if _, err := uuid.Parse(params.ID); err != nil {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "id", Message: "Invalid UUID format", Code: "uuid"},
		})
		return
	}

	query := dtos.GetWalletQuery{WalletID: params.ID}

	if h.getWallet == nil {
		common.InternalErrorResponse(c, "GetWallet use case not implemented")
		return
	}

	result, err := h.getWallet.Execute(c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// ListWallets возвращает список кошельков с фильтрацией.
//
// @Summary List wallets
// @Description Get paginated list of wallets with optional filters
// @Tags Wallets
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20) maximum(100)
// @Param user_id query string false "Filter by user ID" format(uuid)
// @Param currency_code query string false "Filter by currency code"
// @Param status query string false "Filter by status" Enums(ACTIVE, SUSPENDED, LOCKED, CLOSED)
// @Success 200 {object} common.APIResponse{data=dtos.WalletListDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets [get]
func (h *WalletHandler) ListWallets(c *gin.Context) {
	pagination := ParsePagination(c)

	var filters ListWalletsParams
	if !BindQuery(c, &filters) {
		return
	}

	query := dtos.ListWalletsQuery{
		Offset: pagination.Offset(),
		Limit:  pagination.PerPage,
	}

	if filters.UserID != "" {
		query.UserID = &filters.UserID
	}
	if filters.CurrencyCode != "" {
		query.CurrencyCode = &filters.CurrencyCode
	}
	if filters.Status != "" {
		query.Status = &filters.Status
	}

	if h.listWallets == nil {
		common.InternalErrorResponse(c, "ListWallets use case not implemented")
		return
	}

	result, err := h.listWallets.Execute(c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	meta := BuildMeta(pagination, result.TotalCount)
	common.SuccessWithMeta(c, http.StatusOK, result, meta)
}

// CreditWallet пополняет кошелёк.
//
// @Summary Credit wallet (deposit)
// @Description Add funds to a wallet
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "Wallet ID" format(uuid)
// @Param request body CreditWalletRequest true "Credit data"
// @Success 200 {object} common.APIResponse{data=dtos.WalletOperationDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse "Wallet not found"
// @Failure 409 {object} common.APIResponse "Concurrency error"
// @Failure 422 {object} common.APIResponse "Wallet not active"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/{id}/credit [post]
func (h *WalletHandler) CreditWallet(c *gin.Context) {
	var params WalletIDParam
	if !BindURI(c, &params) {
		return
	}

	var req CreditWalletRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.CreditWalletCommand{
		WalletID:          params.ID,
		Amount:            req.Amount,
		IdempotencyKey:    req.IdempotencyKey,
		Description:       req.Description,
		ExternalReference: req.ExternalReference,
	}

	result, err := h.creditWallet.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// DebitWallet списывает средства с кошелька.
//
// @Summary Debit wallet (withdraw)
// @Description Remove funds from a wallet
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "Wallet ID" format(uuid)
// @Param request body DebitWalletRequest true "Debit data"
// @Success 200 {object} common.APIResponse{data=dtos.WalletOperationDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse "Wallet not found"
// @Failure 409 {object} common.APIResponse "Concurrency error"
// @Failure 422 {object} common.APIResponse "Insufficient balance"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/{id}/debit [post]
func (h *WalletHandler) DebitWallet(c *gin.Context) {
	var params WalletIDParam
	if !BindURI(c, &params) {
		return
	}

	var req DebitWalletRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.DebitWalletCommand{
		WalletID:          params.ID,
		Amount:            req.Amount,
		IdempotencyKey:    req.IdempotencyKey,
		Description:       req.Description,
		ExternalReference: req.ExternalReference,
	}

	if h.debitWallet == nil {
		common.InternalErrorResponse(c, "DebitWallet use case not implemented")
		return
	}

	result, err := h.debitWallet.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// Transfer переводит средства между кошельками.
//
// @Summary Transfer funds between wallets
// @Description Transfer funds from source wallet to destination wallet
// @Tags Wallets
// @Accept json
// @Produce json
// @Param id path string true "Source Wallet ID" format(uuid)
// @Param request body TransferFundsRequest true "Transfer data"
// @Success 200 {object} common.APIResponse{data=dtos.TransferResultDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse "Wallet not found"
// @Failure 409 {object} common.APIResponse "Concurrency error"
// @Failure 422 {object} common.APIResponse "Insufficient balance or currency mismatch"
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/{id}/transfer [post]
func (h *WalletHandler) Transfer(c *gin.Context) {
	var params WalletIDParam
	if !BindURI(c, &params) {
		return
	}

	var req TransferFundsRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.TransferFundsCommand{
		SourceWalletID:      params.ID,
		DestinationWalletID: req.DestinationWalletID,
		Amount:              req.Amount,
		IdempotencyKey:      req.IdempotencyKey,
		Description:         req.Description,
	}

	if h.transferFunds == nil {
		common.InternalErrorResponse(c, "TransferFunds use case not implemented")
		return
	}

	result, err := h.transferFunds.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// GetMyWallets возвращает кошельки авторизованного пользователя.
//
// @Summary Get my wallets
// @Description Get wallets of the authenticated user
// @Tags Wallets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} common.APIResponse{data=dtos.WalletListDTO}
// @Failure 401 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/wallets/me [get]
func (h *WalletHandler) GetMyWallets(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	if userID == uuid.Nil {
		common.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	userIDStr := userID.String()
	query := dtos.ListWalletsQuery{
		UserID: &userIDStr,
		Offset: 0,
		Limit:  100, // Максимум кошельков для одного пользователя
	}

	if h.listWallets == nil {
		common.InternalErrorResponse(c, "ListWallets use case not implemented")
		return
	}

	result, err := h.listWallets.Execute(c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// RegisterRoutes регистрирует маршруты для WalletHandler.
//
// Routes:
// - POST   /wallets              - Create wallet
// - GET    /wallets              - List wallets
// - GET    /wallets/me           - Get my wallets (authenticated)
// - GET    /wallets/:id          - Get wallet by ID
// - POST   /wallets/:id/credit   - Credit wallet
// - POST   /wallets/:id/debit    - Debit wallet
// - POST   /wallets/:id/transfer - Transfer funds
func (h *WalletHandler) RegisterRoutes(router *gin.RouterGroup) {
	wallets := router.Group("/wallets")
	{
		wallets.POST("", h.CreateWallet)
		wallets.GET("", h.ListWallets)
		wallets.GET("/me", h.GetMyWallets)
		wallets.GET("/:id", h.GetWallet)
		wallets.POST("/:id/credit", h.CreditWallet)
		wallets.POST("/:id/debit", h.DebitWallet)
		wallets.POST("/:id/transfer", h.Transfer)
	}
}
