// Package handlers - User HTTP handlers.
package handlers

import (
	"context"
	"net/http"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================
// Use Case Interfaces
// ============================================

// CreateUserUseCase - интерфейс для создания пользователя.
type CreateUserUseCase interface {
	Execute(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error)
}

// ApproveKYCUseCase - интерфейс для одобрения KYC.
type ApproveKYCUseCase interface {
	Execute(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error)
}

// GetUserUseCase - интерфейс для получения пользователя (query).
type GetUserUseCase interface {
	Execute(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error)
}

// ListUsersUseCase - интерфейс для получения списка пользователей.
type ListUsersUseCase interface {
	Execute(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error)
}

// StartKYCUseCase - интерфейс для запуска KYC верификации.
type StartKYCUseCase interface {
	Execute(ctx context.Context, cmd dtos.StartKYCVerificationCommand) (*dtos.UserDTO, error)
}

// ============================================
// User Handler
// ============================================

// UserHandler обрабатывает HTTP запросы для пользователей.
//
// Pattern: Adapter (Hexagonal Architecture)
// - Преобразует HTTP запросы в Use Case вызовы
// - Преобразует результаты в HTTP ответы
type UserHandler struct {
	createUser CreateUserUseCase
	approveKYC ApproveKYCUseCase
	getUser    GetUserUseCase
	listUsers  ListUsersUseCase
	startKYC   StartKYCUseCase
}

// NewUserHandler создаёт новый UserHandler.
//
// Dependency Injection:
// - Все зависимости передаются через конструктор
// - Handler не создаёт зависимости сам
func NewUserHandler(
	createUser CreateUserUseCase,
	approveKYC ApproveKYCUseCase,
	getUser GetUserUseCase,
	listUsers ListUsersUseCase,
	startKYC StartKYCUseCase,
) *UserHandler {
	return &UserHandler{
		createUser: createUser,
		approveKYC: approveKYC,
		getUser:    getUser,
		listUsers:  listUsers,
		startKYC:   startKYC,
	}
}

// ============================================
// Request DTOs (HTTP layer)
// ============================================

// CreateUserRequest - запрос на создание пользователя.
//
// @Description Create user request body
type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required,min=2,max=100"`
}

// ApproveKYCRequest - запрос на одобрение/отклонение KYC.
//
// @Description Approve KYC request body
type ApproveKYCRequest struct {
	Approved bool   `json:"approved" binding:"required"`
	Reason   string `json:"reason,omitempty"`
}

// UserIDParam - параметр ID пользователя из URL.
type UserIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// ============================================
// HTTP Handlers
// ============================================

// CreateUser создаёт нового пользователя.
//
// @Summary Create a new user
// @Description Create a new user with email and full name
// @Tags Users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User data"
// @Success 201 {object} common.APIResponse{data=dtos.UserCreatedDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 409 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if !BindJSON(c, &req) {
		return
	}

	// Преобразуем HTTP request в Application Command
	cmd := dtos.CreateUserCommand{
		Email:    req.Email,
		FullName: req.FullName,
	}

	// Вызываем Use Case
	result, err := h.createUser.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	// Возвращаем успешный ответ
	common.Success(c, http.StatusCreated, result)
}

// GetUser возвращает пользователя по ID.
//
// @Summary Get user by ID
// @Description Get user details by UUID
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID" format(uuid)
// @Success 200 {object} common.APIResponse{data=dtos.UserDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	var params UserIDParam
	if !BindURI(c, &params) {
		return
	}

	// Проверяем, что это валидный UUID
	if _, err := uuid.Parse(params.ID); err != nil {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "id", Message: "Invalid UUID format", Code: "uuid"},
		})
		return
	}

	// Вызываем Use Case
	query := dtos.GetUserQuery{UserID: params.ID}

	if h.getUser == nil {
		// Если use case не реализован - временная заглушка
		common.InternalErrorResponse(c, "GetUser use case not implemented")
		return
	}

	result, err := h.getUser.Execute(c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// ListUsers возвращает список пользователей с пагинацией.
//
// @Summary List users
// @Description Get paginated list of users
// @Tags Users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20) maximum(100)
// @Success 200 {object} common.APIResponse{data=dtos.UserListDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	pagination := ParsePagination(c)

	query := dtos.ListUsersQuery{
		Offset: pagination.Offset(),
		Limit:  pagination.PerPage,
	}

	if h.listUsers == nil {
		common.InternalErrorResponse(c, "ListUsers use case not implemented")
		return
	}

	result, err := h.listUsers.Execute(c.Request.Context(), query)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	// Добавляем мета-информацию о пагинации
	meta := BuildMeta(pagination, result.TotalCount)
	common.SuccessWithMeta(c, http.StatusOK, result, meta)
}

// ApproveKYC одобряет или отклоняет KYC верификацию.
//
// @Summary Approve or reject KYC verification
// @Description Approve or reject user's KYC verification
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID" format(uuid)
// @Param request body ApproveKYCRequest true "KYC decision"
// @Success 200 {object} common.APIResponse{data=dtos.UserDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 422 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/users/{id}/kyc [post]
func (h *UserHandler) ApproveKYC(c *gin.Context) {
	var params UserIDParam
	if !BindURI(c, &params) {
		return
	}

	var req ApproveKYCRequest
	if !BindJSON(c, &req) {
		return
	}

	cmd := dtos.ApproveKYCCommand{
		UserID:   params.ID,
		Verified: req.Approved,
		Reason:   req.Reason,
	}

	if h.approveKYC == nil {
		common.InternalErrorResponse(c, "ApproveKYC use case not implemented")
		return
	}

	result, err := h.approveKYC.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// StartKYC начинает процесс KYC верификации.
//
// @Summary Start KYC verification
// @Description Start the KYC verification process for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID" format(uuid)
// @Success 200 {object} common.APIResponse{data=dtos.UserDTO}
// @Failure 400 {object} common.APIResponse
// @Failure 404 {object} common.APIResponse
// @Failure 422 {object} common.APIResponse
// @Failure 500 {object} common.APIResponse
// @Router /api/v1/users/{id}/kyc/start [post]
func (h *UserHandler) StartKYC(c *gin.Context) {
	var params UserIDParam
	if !BindURI(c, &params) {
		return
	}

	if h.startKYC == nil {
		common.InternalErrorResponse(c, "StartKYC use case not implemented")
		return
	}

	cmd := dtos.StartKYCVerificationCommand{
		UserID: params.ID,
	}

	result, err := h.startKYC.Execute(c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

	common.Success(c, http.StatusOK, result)
}

// RegisterRoutes регистрирует маршруты для UserHandler.
//
// Routes:
// - POST   /users          - Create user
// - GET    /users          - List users
// - GET    /users/:id      - Get user by ID
// - POST   /users/:id/kyc  - Approve/Reject KYC
// - POST   /users/:id/kyc/start - Start KYC process
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.POST("", h.CreateUser)
		users.GET("", h.ListUsers)
		users.GET("/:id", h.GetUser)
		users.POST("/:id/kyc", h.ApproveKYC)
		users.POST("/:id/kyc/start", h.StartKYC)
	}
}
