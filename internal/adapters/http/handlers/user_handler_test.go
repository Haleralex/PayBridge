package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/Haleralex/wallethub/internal/application/cqrs"
	"github.com/Haleralex/wallethub/internal/application/dtos"
	domainerrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Mock Use Cases
// ============================================

type MockCreateUserUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error)
}

func (m *MockCreateUserUseCase) Execute(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, errors.New("not implemented")
}

type MockGetUserUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error)
}

func (m *MockGetUserUseCase) Execute(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, errors.New("not implemented")
}

type MockListUsersUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error)
}

func (m *MockListUsersUseCase) Execute(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, errors.New("not implemented")
}

// ============================================
// Helper Functions
// ============================================

// buildUserBuses creates CommandBus and QueryBus with mock use cases registered.
func buildUserBuses(
	createUser *MockCreateUserUseCase,
	getUser *MockGetUserUseCase,
	listUsers *MockListUsersUseCase,
) (*cqrs.CommandBus, *cqrs.QueryBus) {
	cmdBus := cqrs.NewCommandBus()
	qBus := cqrs.NewQueryBus()

	if createUser != nil {
		cqrs.RegisterCommandHandler[dtos.CreateUserCommand, *dtos.UserCreatedDTO](cmdBus, createUser)
	}
	if getUser != nil {
		cqrs.RegisterQueryHandler[dtos.GetUserQuery, *dtos.UserDTO](qBus, getUser)
	}
	if listUsers != nil {
		cqrs.RegisterQueryHandler[dtos.ListUsersQuery, *dtos.UserListDTO](qBus, listUsers)
	}

	return cmdBus, qBus
}

func setupUserTestRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add request ID middleware (needed for response helpers)
	router.Use(func(c *gin.Context) {
		c.Set("X-Request-ID", "test-request-123")
		c.Next()
	})

	return router
}

// withAuth attaches a mock authenticated user to the request context.
// Used to satisfy self-only checks in GetUser without spinning up real JWT auth.
func withAuth(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.AuthUserIDKey, userID)
		c.Next()
	}
}

// ============================================
// Test NewUserHandler
// ============================================

func TestNewUserHandler(t *testing.T) {
	cmdBus, qBus := buildUserBuses(
		&MockCreateUserUseCase{},
		&MockGetUserUseCase{},
		&MockListUsersUseCase{},
	)

	handler := NewUserHandler(cmdBus, qBus)

	assert.NotNil(t, handler)
	assert.Equal(t, cmdBus, handler.commandBus)
}

// ============================================
// Test CreateUser Handler
// ============================================

func TestUserHandler_CreateUser(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockCreateUserUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error) {
				return &dtos.UserCreatedDTO{
					User: dtos.UserDTO{
						ID:        userID,
						Email:     "john@example.com",
						FullName:  "John Doe",
						KYCStatus: "UNVERIFIED",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}, nil
			},
		}

		cmdBus, qBus := buildUserBuses(mockUseCase, nil, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.POST("/users", handler.CreateUser)

		reqBody := CreateUserRequest{Email: "john@example.com", FullName: "John Doe"}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("ValidationError_MissingEmail", func(t *testing.T) {
		cmdBus, qBus := buildUserBuses(&MockCreateUserUseCase{}, nil, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.POST("/users", handler.CreateUser)

		reqBody := CreateUserRequest{FullName: "John Doe"}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ValidationError_InvalidEmail", func(t *testing.T) {
		cmdBus, qBus := buildUserBuses(&MockCreateUserUseCase{}, nil, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.POST("/users", handler.CreateUser)

		reqBody := CreateUserRequest{Email: "not-an-email", FullName: "John Doe"}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DomainError", func(t *testing.T) {
		mockUseCase := &MockCreateUserUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreateUserCommand) (*dtos.UserCreatedDTO, error) {
				return nil, domainerrors.NewDomainError("USER_EXISTS", "User already exists", nil)
			},
		}

		cmdBus, qBus := buildUserBuses(mockUseCase, nil, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.POST("/users", handler.CreateUser)

		reqBody := CreateUserRequest{Email: "john@example.com", FullName: "John Doe"}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================
// Test GetUser Handler
// ============================================

func TestUserHandler_GetUser(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockGetUserUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error) {
				return &dtos.UserDTO{
					ID:        userID,
					Email:     "john@example.com",
					FullName:  "John Doe",
					KYCStatus: "VERIFIED",
				}, nil
			},
		}

		cmdBus, qBus := buildUserBuses(nil, mockUseCase, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.Use(withAuth(userID))
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		cmdBus, qBus := buildUserBuses(nil, &MockGetUserUseCase{}, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.Use(withAuth(uuid.New().String()))
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ForbiddenWhenAccessingOtherUser", func(t *testing.T) {
		cmdBus, qBus := buildUserBuses(nil, &MockGetUserUseCase{}, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.Use(withAuth(uuid.New().String())) // authenticated as someone else
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockGetUserUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error) {
				return nil, domainerrors.NewDomainError("USER_NOT_FOUND", "User not found", domainerrors.ErrEntityNotFound)
			},
		}

		cmdBus, qBus := buildUserBuses(nil, mockUseCase, nil)
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.Use(withAuth(userID))
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("NoHandlerRegistered", func(t *testing.T) {
		userID := uuid.New().String()
		cmdBus := cqrs.NewCommandBus()
		qBus := cqrs.NewQueryBus()
		handler := NewUserHandler(cmdBus, qBus)
		router := setupUserTestRouter(handler)
		router.Use(withAuth(userID))
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// ============================================
// Test RegisterRoutes
// ============================================

func TestUserHandler_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api/v1")

	cmdBus, qBus := buildUserBuses(
		&MockCreateUserUseCase{},
		&MockGetUserUseCase{},
		nil,
	)

	handler := NewUserHandler(cmdBus, qBus)

	handler.RegisterRoutes(apiGroup)

	routes := router.Routes()
	require.GreaterOrEqual(t, len(routes), 2)
}
