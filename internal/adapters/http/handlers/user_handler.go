// Package handlers - User HTTP handlers.
package handlers

import (
	"net/http"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/Haleralex/wallethub/internal/application/cqrs"
	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================
// User Handler
// ============================================

// UserHandler обрабатывает HTTP запросы для пользователей.
// Все операции диспатчатся через CQRS Command/Query Bus.
type UserHandler struct {
	commandBus *cqrs.CommandBus
	queryBus   *cqrs.QueryBus
}

// NewUserHandler создаёт новый UserHandler.
func NewUserHandler(commandBus *cqrs.CommandBus, queryBus *cqrs.QueryBus) *UserHandler {
	return &UserHandler{
		commandBus: commandBus,
		queryBus:   queryBus,
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

	cmd := dtos.CreateUserCommand{
		Email:    req.Email,
		FullName: req.FullName,
	}

	result, err := cqrs.DispatchCommand[dtos.CreateUserCommand, *dtos.UserCreatedDTO](h.commandBus, c.Request.Context(), cmd)
	if err != nil {
		common.HandleDomainError(c, err)
		return
	}

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

	requestedID, err := uuid.Parse(params.ID)
	if err != nil {
		common.ValidationErrorResponse(c, []common.FieldError{
			{Field: "id", Message: "Invalid UUID format", Code: "uuid"},
		})
		return
	}

	// Self-only: users can only fetch their own profile.
	authUserID := middleware.GetAuthUserID(c)
	if authUserID == uuid.Nil {
		common.UnauthorizedResponse(c, "User not authenticated")
		return
	}
	if requestedID != authUserID {
		common.ForbiddenResponse(c, "You can only access your own profile")
		return
	}

	query := dtos.GetUserQuery{UserID: params.ID}

	result, err := cqrs.DispatchQuery[dtos.GetUserQuery, *dtos.UserDTO](h.queryBus, c.Request.Context(), query)
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
// - GET    /users/:id      - Get user by ID (self only)
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.POST("", h.CreateUser)
		users.GET("/:id", h.GetUser)
	}
}
