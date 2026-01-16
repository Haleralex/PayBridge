package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/wallethub/internal/application/dtos"
	domerrors "github.com/yourusername/wallethub/internal/domain/errors"
)

// ============================================
// Mock Use Cases
// ============================================

type mockGetTransactionUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.GetTransactionQuery) (*dtos.TransactionDTO, error)
}

func (m *mockGetTransactionUseCase) Execute(ctx context.Context, query dtos.GetTransactionQuery) (*dtos.TransactionDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, nil
}

type mockListTransactionsUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error)
}

func (m *mockListTransactionsUseCase) Execute(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, nil
}

type mockRetryTransactionUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.RetryTransactionCommand) (*dtos.TransactionDTO, error)
}

func (m *mockRetryTransactionUseCase) Execute(ctx context.Context, cmd dtos.RetryTransactionCommand) (*dtos.TransactionDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

type mockCancelTransactionUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.CancelTransactionCommand) (*dtos.TransactionDTO, error)
}

func (m *mockCancelTransactionUseCase) Execute(ctx context.Context, cmd dtos.CancelTransactionCommand) (*dtos.TransactionDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

// ============================================
// Helper Functions
// ============================================

func setupTransactionTestRouter(handler *TransactionHandler) *gin.Engine {
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}

// ============================================
// Test Cases
// ============================================

func TestNewTransactionHandler(t *testing.T) {
	handler := NewTransactionHandler(nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestTransactionHandler_GetTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		txID := uuid.New().String()

		mockUseCase := &mockGetTransactionUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetTransactionQuery) (*dtos.TransactionDTO, error) {
				now := time.Now()
				return &dtos.TransactionDTO{
					ID:           txID,
					WalletID:     uuid.New().String(),
					Type:         "DEPOSIT",
					Status:       "COMPLETED",
					Amount:       "100.00",
					CurrencyCode: "USD",
					Description:  "Test deposit",
					CreatedAt:    now,
					CompletedAt:  &now,
				}, nil
			},
		}

		handler := NewTransactionHandler(mockUseCase, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response["success"].(bool))
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		handler := NewTransactionHandler(&mockGetTransactionUseCase{}, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/not-a-uuid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("TransactionNotFound", func(t *testing.T) {
		mockUseCase := &mockGetTransactionUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetTransactionQuery) (*dtos.TransactionDTO, error) {
				return nil, domerrors.NewDomainError("TRANSACTION_NOT_FOUND", "transaction not found", domerrors.ErrEntityNotFound)
			},
		}

		handler := NewTransactionHandler(mockUseCase, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_ListTransactions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockUseCase := &mockListTransactionsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
				return &dtos.TransactionListDTO{
					Transactions: []dtos.TransactionDTO{
						{ID: uuid.New().String(), Type: "DEPOSIT", Status: "COMPLETED", Amount: "100.00"},
						{ID: uuid.New().String(), Type: "WITHDRAW", Status: "COMPLETED", Amount: "50.00"},
					},
					TotalCount: 2,
					Offset:     0,
					Limit:      20,
				}, nil
			},
		}

		handler := NewTransactionHandler(nil, mockUseCase, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["meta"])
	})

	t.Run("WithFilters", func(t *testing.T) {
		mockUseCase := &mockListTransactionsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
				assert.NotNil(t, query.WalletID)
				assert.NotNil(t, query.Type)
				assert.NotNil(t, query.Status)
				return &dtos.TransactionListDTO{Transactions: []dtos.TransactionDTO{}, TotalCount: 0}, nil
			},
		}

		handler := NewTransactionHandler(nil, mockUseCase, nil, nil)
		router := setupTransactionTestRouter(handler)

		walletID := uuid.New().String()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?wallet_id="+walletID+"&type=DEPOSIT&status=COMPLETED", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_RetryTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		txID := uuid.New().String()

		mockUseCase := &mockRetryTransactionUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.RetryTransactionCommand) (*dtos.TransactionDTO, error) {
				return &dtos.TransactionDTO{
					ID:     txID,
					Status: "PROCESSING",
					Type:   "DEPOSIT",
					Amount: "100.00",
				}, nil
			},
		}

		handler := NewTransactionHandler(nil, nil, mockUseCase, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+txID+"/retry", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, &mockRetryTransactionUseCase{}, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/not-a-uuid/retry", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("NotInFailedState", func(t *testing.T) {
		mockUseCase := &mockRetryTransactionUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.RetryTransactionCommand) (*dtos.TransactionDTO, error) {
				return nil, domerrors.NewBusinessRuleViolation("INVALID_TRANSACTION_STATE", "transaction is not in failed state", nil)
			},
		}

		handler := NewTransactionHandler(nil, nil, mockUseCase, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+uuid.New().String()+"/retry", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+uuid.New().String()+"/retry", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_CancelTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		txID := uuid.New().String()

		mockUseCase := &mockCancelTransactionUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CancelTransactionCommand) (*dtos.TransactionDTO, error) {
				return &dtos.TransactionDTO{
					ID:     txID,
					Status: "CANCELLED",
					Type:   "DEPOSIT",
					Amount: "100.00",
				}, nil
			},
		}

		handler := NewTransactionHandler(nil, nil, nil, mockUseCase)
		router := setupTransactionTestRouter(handler)

		body, _ := json.Marshal(CancelTransactionRequest{
			Reason: "User requested cancellation",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+txID+"/cancel", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MissingReason", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, &mockCancelTransactionUseCase{})
		router := setupTransactionTestRouter(handler)

		body, _ := json.Marshal(map[string]interface{}{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+uuid.New().String()+"/cancel", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CannotBeCancelled", func(t *testing.T) {
		mockUseCase := &mockCancelTransactionUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CancelTransactionCommand) (*dtos.TransactionDTO, error) {
				return nil, domerrors.NewBusinessRuleViolation("INVALID_TRANSACTION_STATE", "transaction cannot be cancelled", nil)
			},
		}

		handler := NewTransactionHandler(nil, nil, nil, mockUseCase)
		router := setupTransactionTestRouter(handler)

		body, _ := json.Marshal(CancelTransactionRequest{Reason: "Test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+uuid.New().String()+"/cancel", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		body, _ := json.Marshal(CancelTransactionRequest{Reason: "Test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+uuid.New().String()+"/cancel", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_GetWalletTransactions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		walletID := uuid.New().String()

		mockUseCase := &mockListTransactionsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
				assert.NotNil(t, query.WalletID)
				assert.Equal(t, walletID, *query.WalletID)
				return &dtos.TransactionListDTO{
					Transactions: []dtos.TransactionDTO{{ID: uuid.New().String(), Type: "DEPOSIT"}},
					TotalCount:   1,
				}, nil
			},
		}

		handler := NewTransactionHandler(nil, mockUseCase, nil, nil)
		router := gin.New()

		walletGroup := router.Group("/api/v1/wallets")
		handler.RegisterWalletTransactionsRoute(walletGroup)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+walletID+"/transactions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidWalletID", func(t *testing.T) {
		handler := NewTransactionHandler(nil, &mockListTransactionsUseCase{}, nil, nil)
		router := gin.New()

		walletGroup := router.Group("/api/v1/wallets")
		handler.RegisterWalletTransactionsRoute(walletGroup)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/not-a-uuid/transactions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WithFilters", func(t *testing.T) {
		walletID := uuid.New().String()

		mockUseCase := &mockListTransactionsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListTransactionsQuery) (*dtos.TransactionListDTO, error) {
				assert.NotNil(t, query.Type)
				assert.NotNil(t, query.Status)
				return &dtos.TransactionListDTO{Transactions: []dtos.TransactionDTO{}, TotalCount: 0}, nil
			},
		}

		handler := NewTransactionHandler(nil, mockUseCase, nil, nil)
		router := gin.New()

		walletGroup := router.Group("/api/v1/wallets")
		handler.RegisterWalletTransactionsRoute(walletGroup)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+walletID+"/transactions?type=DEPOSIT&status=COMPLETED", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := gin.New()

		walletGroup := router.Group("/api/v1/wallets")
		handler.RegisterWalletTransactionsRoute(walletGroup)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+uuid.New().String()+"/transactions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_GetTransactionByIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("NotImplemented", func(t *testing.T) {
		handler := NewTransactionHandler(nil, nil, nil, nil)
		router := setupTransactionTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/by-key/some-key-123", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestTransactionHandler_RegisterRoutes(t *testing.T) {
	handler := NewTransactionHandler(nil, nil, nil, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	routes := router.Routes()
	expectedRoutes := []string{
		"GET /api/v1/transactions",
		"GET /api/v1/transactions/:id",
		"GET /api/v1/transactions/by-key/:key",
		"POST /api/v1/transactions/:id/retry",
		"POST /api/v1/transactions/:id/cancel",
	}

	assert.GreaterOrEqual(t, len(routes), len(expectedRoutes))

	for _, expected := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Method+" "+route.Path == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s not found", expected)
	}
}
