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

type MockApproveKYCUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error)
}

func (m *MockApproveKYCUseCase) Execute(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, errors.New("not implemented")
}

// ============================================
// Test Setup
// ============================================

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

// ============================================
// Test NewUserHandler
// ============================================

func TestNewUserHandler(t *testing.T) {
	createUser := &MockCreateUserUseCase{}
	approveKYC := &MockApproveKYCUseCase{}
	getUser := &MockGetUserUseCase{}
	listUsers := &MockListUsersUseCase{}

	handler := NewUserHandler(createUser, approveKYC, getUser, listUsers)

	assert.NotNil(t, handler)
	assert.Equal(t, createUser, handler.createUser)
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

		handler := NewUserHandler(mockUseCase, nil, nil, nil)
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
		handler := NewUserHandler(&MockCreateUserUseCase{}, nil, nil, nil)
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
		handler := NewUserHandler(&MockCreateUserUseCase{}, nil, nil, nil)
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

		handler := NewUserHandler(mockUseCase, nil, nil, nil)
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

		handler := NewUserHandler(nil, nil, mockUseCase, nil)
		router := setupUserTestRouter(handler)
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		handler := NewUserHandler(nil, nil, &MockGetUserUseCase{}, nil)
		router := setupUserTestRouter(handler)
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockGetUserUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetUserQuery) (*dtos.UserDTO, error) {
				return nil, domainerrors.NewDomainError("USER_NOT_FOUND", "User not found", domainerrors.ErrEntityNotFound)
			},
		}

		handler := NewUserHandler(nil, nil, mockUseCase, nil)
		router := setupUserTestRouter(handler)
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("UseCaseNil", func(t *testing.T) {
		handler := NewUserHandler(nil, nil, nil, nil)
		router := setupUserTestRouter(handler)
		router.GET("/users/:id", handler.GetUser)

		req := httptest.NewRequest(http.MethodGet, "/users/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// ============================================
// Test ListUsers Handler
// ============================================

func TestUserHandler_ListUsers(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockUseCase := &MockListUsersUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error) {
				return &dtos.UserListDTO{
					Users: []dtos.UserDTO{
						{ID: uuid.New().String(), Email: "user1@test.com"},
						{ID: uuid.New().String(), Email: "user2@test.com"},
					},
					TotalCount: 2,
				}, nil
			},
		}

		handler := NewUserHandler(nil, nil, nil, mockUseCase)
		router := setupUserTestRouter(handler)
		router.GET("/users", handler.ListUsers)

		req := httptest.NewRequest(http.MethodGet, "/users?page=1&per_page=20", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["meta"])
	})

	t.Run("UseCaseNil", func(t *testing.T) {
		handler := NewUserHandler(nil, nil, nil, nil)
		router := setupUserTestRouter(handler)
		router.GET("/users", handler.ListUsers)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("CustomPagination", func(t *testing.T) {
		mockUseCase := &MockListUsersUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListUsersQuery) (*dtos.UserListDTO, error) {
				assert.Equal(t, 20, query.Offset)
				assert.Equal(t, 10, query.Limit)
				return &dtos.UserListDTO{Users: []dtos.UserDTO{}, TotalCount: 0}, nil
			},
		}

		handler := NewUserHandler(nil, nil, nil, mockUseCase)
		router := setupUserTestRouter(handler)
		router.GET("/users", handler.ListUsers)

		req := httptest.NewRequest(http.MethodGet, "/users?page=3&per_page=10", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// ============================================
// Test ApproveKYC Handler
// ============================================

func TestUserHandler_ApproveKYC(t *testing.T) {
	t.Run("Success_Approve", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockApproveKYCUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error) {
				assert.True(t, cmd.Verified)
				return &dtos.UserDTO{ID: userID, KYCStatus: "VERIFIED"}, nil
			},
		}

		handler := NewUserHandler(nil, mockUseCase, nil, nil)
		router := setupUserTestRouter(handler)
		router.POST("/users/:id/kyc", handler.ApproveKYC)

		reqBody := ApproveKYCRequest{Approved: true}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/kyc", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		handler := NewUserHandler(nil, &MockApproveKYCUseCase{}, nil, nil)
		router := setupUserTestRouter(handler)
		router.POST("/users/:id/kyc", handler.ApproveKYC)

		reqBody := ApproveKYCRequest{Approved: true}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users/invalid/kyc", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("BusinessRuleViolation", func(t *testing.T) {
		userID := uuid.New().String()
		mockUseCase := &MockApproveKYCUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.ApproveKYCCommand) (*dtos.UserDTO, error) {
				return nil, domainerrors.NewBusinessRuleViolation("KYC_DONE", "Already verified", nil)
			},
		}

		handler := NewUserHandler(nil, mockUseCase, nil, nil)
		router := setupUserTestRouter(handler)
		router.POST("/users/:id/kyc", handler.ApproveKYC)

		reqBody := ApproveKYCRequest{Approved: true}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/kyc", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("UseCaseNil", func(t *testing.T) {
		handler := NewUserHandler(nil, nil, nil, nil)
		router := setupUserTestRouter(handler)
		router.POST("/users/:id/kyc", handler.ApproveKYC)

		reqBody := ApproveKYCRequest{Approved: true}
		bodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/users/"+uuid.New().String()+"/kyc", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
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

	handler := NewUserHandler(
		&MockCreateUserUseCase{},
		&MockApproveKYCUseCase{},
		&MockGetUserUseCase{},
		&MockListUsersUseCase{},
	)

	handler.RegisterRoutes(apiGroup)

	routes := router.Routes()
	require.GreaterOrEqual(t, len(routes), 5)
}
