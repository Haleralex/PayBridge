package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	domerrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================
// Mock Use Cases
// ============================================

type mockCreateWalletUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error)
}

func (m *mockCreateWalletUseCase) Execute(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

type mockCreditWalletUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error)
}

func (m *mockCreditWalletUseCase) Execute(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

type mockDebitWalletUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.DebitWalletCommand) (*dtos.WalletOperationDTO, error)
}

func (m *mockDebitWalletUseCase) Execute(ctx context.Context, cmd dtos.DebitWalletCommand) (*dtos.WalletOperationDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

type mockTransferFundsUseCase struct {
	ExecuteFn func(ctx context.Context, cmd dtos.TransferFundsCommand) (*dtos.TransferResultDTO, error)
}

func (m *mockTransferFundsUseCase) Execute(ctx context.Context, cmd dtos.TransferFundsCommand) (*dtos.TransferResultDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, cmd)
	}
	return nil, nil
}

type mockGetWalletUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error)
}

func (m *mockGetWalletUseCase) Execute(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, nil
}

type mockListWalletsUseCase struct {
	ExecuteFn func(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error)
}

func (m *mockListWalletsUseCase) Execute(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, query)
	}
	return nil, nil
}

// ============================================
// Helper Functions
// ============================================

func setupWalletTestRouter(handler *WalletHandler) *gin.Engine {
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}

// ============================================
// Test Cases
// ============================================

func TestNewWalletHandler(t *testing.T) {
	handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestWalletHandler_CreateWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New().String()
		walletID := uuid.New().String()

		mockUseCase := &mockCreateWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error) {
				return &dtos.WalletDTO{
					ID:               walletID,
					UserID:           userID,
					CurrencyCode:     "USD",
					AvailableBalance: "0.00",
					Status:           "ACTIVE",
					CreatedAt:        time.Now(),
				}, nil
			},
		}

		handler := NewWalletHandler(mockUseCase, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreateWalletRequest{
			UserID:       userID,
			CurrencyCode: "USD",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response["success"].(bool))
		assert.NotNil(t, response["data"])
	})

	t.Run("InvalidUserID", func(t *testing.T) {
		handler := NewWalletHandler(&mockCreateWalletUseCase{}, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreateWalletRequest{
			UserID:       "invalid-uuid",
			CurrencyCode: "USD",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidCurrency", func(t *testing.T) {
		handler := NewWalletHandler(&mockCreateWalletUseCase{}, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreateWalletRequest{
			UserID:       uuid.New().String(),
			CurrencyCode: "usd", // lowercase invalid
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		mockUseCase := &mockCreateWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreateWalletCommand) (*dtos.WalletDTO, error) {
				return nil, domerrors.NewDomainError("USER_NOT_FOUND", "user not found", domerrors.ErrEntityNotFound)
			},
		}

		handler := NewWalletHandler(mockUseCase, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreateWalletRequest{
			UserID:       uuid.New().String(),
			CurrencyCode: "USD",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestWalletHandler_GetWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		walletID := uuid.New().String()

		mockUseCase := &mockGetWalletUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error) {
				return &dtos.WalletDTO{
					ID:               walletID,
					UserID:           uuid.New().String(),
					CurrencyCode:     "USD",
					AvailableBalance: "100.50",
					Status:           "ACTIVE",
				}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, nil, nil, mockUseCase, nil)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+walletID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, &mockGetWalletUseCase{}, nil)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/not-a-uuid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WalletNotFound", func(t *testing.T) {
		mockUseCase := &mockGetWalletUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.GetWalletQuery) (*dtos.WalletDTO, error) {
				return nil, domerrors.NewDomainError("WALLET_NOT_FOUND", "wallet not found", domerrors.ErrEntityNotFound)
			},
		}

		handler := NewWalletHandler(nil, nil, nil, nil, mockUseCase, nil)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestWalletHandler_ListWallets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mockUseCase := &mockListWalletsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error) {
				return &dtos.WalletListDTO{
					Wallets: []dtos.WalletDTO{
						{ID: uuid.New().String(), CurrencyCode: "USD", AvailableBalance: "100.00"},
						{ID: uuid.New().String(), CurrencyCode: "EUR", AvailableBalance: "50.00"},
					},
					TotalCount: 2,
					Offset:     0,
					Limit:      20,
				}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, nil, nil, nil, mockUseCase)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["meta"])
	})

	t.Run("WithFilters", func(t *testing.T) {
		mockUseCase := &mockListWalletsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error) {
				assert.NotNil(t, query.UserID)
				assert.NotNil(t, query.CurrencyCode)
				return &dtos.WalletListDTO{Wallets: []dtos.WalletDTO{}, TotalCount: 0}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, nil, nil, nil, mockUseCase)
		router := setupWalletTestRouter(handler)

		userID := uuid.New().String()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets?user_id="+userID+"&currency_code=USD", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestWalletHandler_CreditWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		walletID := uuid.New().String()

		mockUseCase := &mockCreditWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error) {
				return &dtos.WalletOperationDTO{
					Wallet: dtos.WalletDTO{
						ID:               walletID,
						AvailableBalance: "150.00",
					},
					TransactionID: uuid.New().String(),
					Message:       "Wallet credited successfully",
				}, nil
			},
		}

		handler := NewWalletHandler(nil, mockUseCase, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreditWalletRequest{
			Amount:         "50.00",
			IdempotencyKey: uuid.New().String(),
			Description:    "Test deposit",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+walletID+"/credit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidAmount", func(t *testing.T) {
		handler := NewWalletHandler(nil, &mockCreditWalletUseCase{}, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(map[string]interface{}{
			"amount":          "-50.00", // Negative amount
			"idempotency_key": uuid.New().String(),
			"description":     "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/credit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WalletNotActive", func(t *testing.T) {
		mockUseCase := &mockCreditWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.CreditWalletCommand) (*dtos.WalletOperationDTO, error) {
				return nil, domerrors.NewBusinessRuleViolation("WALLET_NOT_ACTIVE", "wallet is not active", nil)
			},
		}

		handler := NewWalletHandler(nil, mockUseCase, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(CreditWalletRequest{
			Amount:         "50.00",
			IdempotencyKey: uuid.New().String(),
			Description:    "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/credit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestWalletHandler_DebitWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		walletID := uuid.New().String()

		mockUseCase := &mockDebitWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.DebitWalletCommand) (*dtos.WalletOperationDTO, error) {
				return &dtos.WalletOperationDTO{
					Wallet: dtos.WalletDTO{
						ID:               walletID,
						AvailableBalance: "50.00",
					},
					TransactionID: uuid.New().String(),
					Message:       "Wallet debited successfully",
				}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, mockUseCase, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(DebitWalletRequest{
			Amount:         "50.00",
			IdempotencyKey: uuid.New().String(),
			Description:    "Test withdrawal",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+walletID+"/debit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InsufficientBalance", func(t *testing.T) {
		mockUseCase := &mockDebitWalletUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.DebitWalletCommand) (*dtos.WalletOperationDTO, error) {
				return nil, domerrors.NewBusinessRuleViolation("INSUFFICIENT_BALANCE", "insufficient balance", nil)
			},
		}

		handler := NewWalletHandler(nil, nil, mockUseCase, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(DebitWalletRequest{
			Amount:         "1000.00",
			IdempotencyKey: uuid.New().String(),
			Description:    "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/debit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(DebitWalletRequest{
			Amount:         "50.00",
			IdempotencyKey: uuid.New().String(),
			Description:    "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/debit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestWalletHandler_Transfer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		sourceID := uuid.New().String()
		destID := uuid.New().String()

		mockUseCase := &mockTransferFundsUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.TransferFundsCommand) (*dtos.TransferResultDTO, error) {
				return &dtos.TransferResultDTO{
					SourceWallet:      dtos.WalletDTO{ID: sourceID, AvailableBalance: "50.00"},
					DestinationWallet: dtos.WalletDTO{ID: destID, AvailableBalance: "150.00"},
				}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, nil, mockUseCase, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(TransferFundsRequest{
			DestinationWalletID: destID,
			Amount:              "100.00",
			IdempotencyKey:      uuid.New().String(),
			Description:         "Test transfer",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+sourceID+"/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("CurrencyMismatch", func(t *testing.T) {
		mockUseCase := &mockTransferFundsUseCase{
			ExecuteFn: func(ctx context.Context, cmd dtos.TransferFundsCommand) (*dtos.TransferResultDTO, error) {
				return nil, domerrors.NewBusinessRuleViolation("CURRENCY_MISMATCH", "wallets have different currencies", nil)
			},
		}

		handler := NewWalletHandler(nil, nil, nil, mockUseCase, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(TransferFundsRequest{
			DestinationWalletID: uuid.New().String(),
			Amount:              "100.00",
			IdempotencyKey:      uuid.New().String(),
			Description:         "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
		router := setupWalletTestRouter(handler)

		body, _ := json.Marshal(TransferFundsRequest{
			DestinationWalletID: uuid.New().String(),
			Amount:              "100.00",
			IdempotencyKey:      uuid.New().String(),
			Description:         "Test",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+uuid.New().String()+"/transfer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestWalletHandler_GetMyWallets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()

		mockUseCase := &mockListWalletsUseCase{
			ExecuteFn: func(ctx context.Context, query dtos.ListWalletsQuery) (*dtos.WalletListDTO, error) {
				assert.NotNil(t, query.UserID)
				assert.Equal(t, userID.String(), *query.UserID)
				return &dtos.WalletListDTO{
					Wallets:    []dtos.WalletDTO{{ID: uuid.New().String(), CurrencyCode: "USD"}},
					TotalCount: 1,
				}, nil
			},
		}

		handler := NewWalletHandler(nil, nil, nil, nil, nil, mockUseCase)
		router := gin.New()

		// Manually set auth middleware
		router.Use(func(c *gin.Context) {
			c.Set("auth_user_id", userID.String())
			c.Next()
		})

		handler.RegisterRoutes(router.Group("/api/v1"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		handler := NewWalletHandler(nil, nil, nil, nil, nil, &mockListWalletsUseCase{})
		router := setupWalletTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("NilUseCase", func(t *testing.T) {
		userID := uuid.New()

		handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
		router := gin.New()

		router.Use(func(c *gin.Context) {
			c.Set("auth_user_id", userID.String())
			c.Next()
		})

		handler.RegisterRoutes(router.Group("/api/v1"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestWalletHandler_RegisterRoutes(t *testing.T) {
	handler := NewWalletHandler(nil, nil, nil, nil, nil, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	routes := router.Routes()
	expectedRoutes := []string{
		"POST /api/v1/wallets",
		"GET /api/v1/wallets",
		"GET /api/v1/wallets/me",
		"GET /api/v1/wallets/:id",
		"POST /api/v1/wallets/:id/credit",
		"POST /api/v1/wallets/:id/debit",
		"POST /api/v1/wallets/:id/transfer",
	}

	assert.Len(t, routes, len(expectedRoutes))

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
