package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultRouterConfig(t *testing.T) {
	cfg := DefaultRouterConfig()

	assert.NotNil(t, cfg.Logger)
	assert.Equal(t, "dev", cfg.Version)
	assert.Equal(t, "unknown", cfg.BuildTime)
	assert.Equal(t, "development", cfg.Environment)
	assert.Contains(t, cfg.AllowedOrigins, "*")
	assert.NotNil(t, cfg.AuthTokenValidator)
}

func TestNewRouterBuilder(t *testing.T) {
	cfg := DefaultRouterConfig()
	builder := NewRouterBuilder(cfg)

	require.NotNil(t, builder)
	assert.Equal(t, cfg, builder.config)
}

func TestNewRouterBuilder_NilConfig(t *testing.T) {
	builder := NewRouterBuilder(nil)

	require.NotNil(t, builder)
	assert.NotNil(t, builder.config)
	assert.Equal(t, "development", builder.config.Environment)
}

func TestRouterBuilder_WithUserUseCases(t *testing.T) {
	cfg := DefaultRouterConfig()
	userUC := &UserUseCases{}

	builder := NewRouterBuilder(cfg).WithUserUseCases(userUC)

	assert.Equal(t, userUC, builder.users)
}

func TestRouterBuilder_WithWalletUseCases(t *testing.T) {
	cfg := DefaultRouterConfig()
	walletUC := &WalletUseCases{}

	builder := NewRouterBuilder(cfg).WithWalletUseCases(walletUC)

	assert.Equal(t, walletUC, builder.wallets)
}

func TestRouterBuilder_WithTransactionUseCases(t *testing.T) {
	cfg := DefaultRouterConfig()
	txUC := &TransactionUseCases{}

	builder := NewRouterBuilder(cfg).WithTransactionUseCases(txUC)

	assert.Equal(t, txUC, builder.transactions)
}

func TestRouterBuilder_Chain(t *testing.T) {
	cfg := DefaultRouterConfig()
	userUC := &UserUseCases{}
	walletUC := &WalletUseCases{}
	txUC := &TransactionUseCases{}

	builder := NewRouterBuilder(cfg).
		WithUserUseCases(userUC).
		WithWalletUseCases(walletUC).
		WithTransactionUseCases(txUC)

	assert.Equal(t, userUC, builder.users)
	assert.Equal(t, walletUC, builder.wallets)
	assert.Equal(t, txUC, builder.transactions)
}

func TestRouterBuilder_Build_Development(t *testing.T) {
	cfg := &RouterConfig{
		Logger:             slog.New(slog.NewTextHandler(os.Stdout, nil)),
		Version:            "1.0.0",
		BuildTime:          "2024-01-01",
		Environment:        "development",
		AllowedOrigins:     []string{"*"},
		AuthTokenValidator: middleware.MockTokenValidator,
	}

	router := NewRouterBuilder(cfg).Build()

	require.NotNil(t, router)
}

func TestRouterBuilder_Build_Production(t *testing.T) {
	cfg := &RouterConfig{
		Logger:             slog.New(slog.NewTextHandler(os.Stdout, nil)),
		Version:            "1.0.0",
		BuildTime:          "2024-01-01",
		Environment:        "production",
		AllowedOrigins:     []string{"https://example.com"},
		AuthTokenValidator: middleware.MockTokenValidator,
	}

	router := NewRouterBuilder(cfg).Build()

	require.NotNil(t, router)
}

func TestRouterBuilder_Build_HealthEndpoints(t *testing.T) {
	cfg := DefaultRouterConfig()
	router := NewRouterBuilder(cfg).Build()

	endpoints := []string{"/health", "/live", "/ready"}
	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRouterBuilder_Build_MetricsEndpoint(t *testing.T) {
	cfg := DefaultRouterConfig()
	router := NewRouterBuilder(cfg).Build()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "go_") // Prometheus Go metrics
}

func TestRouterBuilder_Build_404Handler(t *testing.T) {
	cfg := DefaultRouterConfig()
	router := NewRouterBuilder(cfg).Build()

	req := httptest.NewRequest("GET", "/nonexistent/path", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Endpoint not found")
}

func TestNewRouter(t *testing.T) {
	cfg := DefaultRouterConfig()
	router := NewRouter(cfg)

	require.NotNil(t, router)
}

func TestNewRouter_NilConfig(t *testing.T) {
	router := NewRouter(nil)

	require.NotNil(t, router)
}

func TestNewDevelopmentRouter(t *testing.T) {
	router := NewDevelopmentRouter()

	require.NotNil(t, router)
}

func TestNewProductionRouter(t *testing.T) {
	router := NewProductionRouter(nil, "1.0.0", []string{"https://example.com"})

	require.NotNil(t, router)
}

func TestRouter_CORS_Development(t *testing.T) {
	cfg := DefaultRouterConfig()
	cfg.Environment = "development"
	router := NewRouterBuilder(cfg).Build()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// OPTIONS request should return 204 or 200
	assert.True(t, w.Code == http.StatusNoContent || w.Code == http.StatusOK)
}

func TestRouter_CORS_Production(t *testing.T) {
	cfg := &RouterConfig{
		Logger:             slog.Default(),
		Version:            "1.0.0",
		Environment:        "production",
		AllowedOrigins:     []string{"https://example.com"},
		AuthTokenValidator: middleware.MockTokenValidator,
	}
	router := NewRouterBuilder(cfg).Build()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should allow the specific origin
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "https://example.com")
}

func TestRouter_RequestID(t *testing.T) {
	cfg := DefaultRouterConfig()
	router := NewRouterBuilder(cfg).Build()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should have X-Request-ID header
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRouter_WithUserUseCasesOnly(t *testing.T) {
	cfg := DefaultRouterConfig()

	router := NewRouterBuilder(cfg).
		WithUserUseCases(&UserUseCases{}).
		Build()

	require.NotNil(t, router)
}

func TestRouter_WithWalletUseCasesOnly(t *testing.T) {
	cfg := DefaultRouterConfig()

	router := NewRouterBuilder(cfg).
		WithWalletUseCases(&WalletUseCases{}).
		Build()

	require.NotNil(t, router)
}

func TestRouter_WithTransactionUseCasesOnly(t *testing.T) {
	cfg := DefaultRouterConfig()

	router := NewRouterBuilder(cfg).
		WithTransactionUseCases(&TransactionUseCases{}).
		Build()

	require.NotNil(t, router)
}

func TestUserUseCases_Structure(t *testing.T) {
	uc := &UserUseCases{}

	assert.Nil(t, uc.CreateUser)
	assert.Nil(t, uc.ApproveKYC)
	assert.Nil(t, uc.GetUser)
	assert.Nil(t, uc.ListUsers)
}

func TestWalletUseCases_Structure(t *testing.T) {
	uc := &WalletUseCases{}

	assert.Nil(t, uc.CreateWallet)
	assert.Nil(t, uc.CreditWallet)
	assert.Nil(t, uc.DebitWallet)
	assert.Nil(t, uc.TransferFunds)
	assert.Nil(t, uc.GetWallet)
	assert.Nil(t, uc.ListWallets)
}

func TestTransactionUseCases_Structure(t *testing.T) {
	uc := &TransactionUseCases{}

	assert.Nil(t, uc.GetTransaction)
	assert.Nil(t, uc.ListTransactions)
	assert.Nil(t, uc.RetryTransaction)
	assert.Nil(t, uc.CancelTransaction)
}

func TestRouterConfig_AllFields(t *testing.T) {
	logger := slog.Default()
	validator := middleware.MockTokenValidator

	cfg := &RouterConfig{
		Logger:             logger,
		Pool:               nil,
		Version:            "1.0.0",
		BuildTime:          "2024-01-01",
		Environment:        "staging",
		AllowedOrigins:     []string{"https://staging.example.com"},
		AuthTokenValidator: validator,
	}

	assert.Equal(t, logger, cfg.Logger)
	assert.Nil(t, cfg.Pool)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "2024-01-01", cfg.BuildTime)
	assert.Equal(t, "staging", cfg.Environment)
	assert.Contains(t, cfg.AllowedOrigins, "https://staging.example.com")
	assert.NotNil(t, cfg.AuthTokenValidator)
}
