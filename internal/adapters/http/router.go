// Package http - Router configuration for REST API.
//
// Router собирает все handlers и middleware в единую точку входа.
//
// Pattern: Composition Root
// - Все зависимости собираются здесь
// - Handlers получают только нужные им use cases
// - Middleware применяется к соответствующим группам routes
package http

import (
	"log/slog"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/adapters/http/handlers"
	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ============================================
// Router Configuration
// ============================================

// RouterConfig - конфигурация роутера.
type RouterConfig struct {
	// Logger для middleware
	Logger *slog.Logger
	// Database pool для health checks
	Pool *pgxpool.Pool
	// Version приложения
	Version string
	// BuildTime время сборки
	BuildTime string
	// Environment (development, staging, production)
	Environment string
	// AllowedOrigins для CORS (production)
	AllowedOrigins []string
	// AuthTokenValidator - функция валидации токена
	AuthTokenValidator func(token string) (*middleware.AuthClaims, error)
}

// DefaultRouterConfig - конфигурация по умолчанию для development.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		Logger:             slog.Default(),
		Version:            "dev",
		BuildTime:          "unknown",
		Environment:        "development",
		AllowedOrigins:     []string{"*"},
		AuthTokenValidator: middleware.MockTokenValidator,
	}
}

// ============================================
// Use Case Providers
// ============================================

// UserUseCases - provider для user use cases.
type UserUseCases struct {
	CreateUser handlers.CreateUserUseCase
	ApproveKYC handlers.ApproveKYCUseCase
	GetUser    handlers.GetUserUseCase
	ListUsers  handlers.ListUsersUseCase
}

// WalletUseCases - provider для wallet use cases.
type WalletUseCases struct {
	CreateWallet  handlers.CreateWalletUseCase
	CreditWallet  handlers.CreditWalletUseCase
	DebitWallet   handlers.DebitWalletUseCase
	TransferFunds handlers.TransferFundsUseCase
	GetWallet     handlers.GetWalletUseCase
	ListWallets   handlers.ListWalletsUseCase
}

// TransactionUseCases - provider для transaction use cases.
type TransactionUseCases struct {
	GetTransaction    handlers.GetTransactionUseCase
	ListTransactions  handlers.ListTransactionsUseCase
	RetryTransaction  handlers.RetryTransactionUseCase
	CancelTransaction handlers.CancelTransactionUseCase
}

// ============================================
// Router Builder
// ============================================

// RouterBuilder - builder для создания роутера.
//
// Pattern: Builder
// - Позволяет пошагово настроить роутер
// - Проще тестировать
// - Можно переиспользовать части конфигурации
type RouterBuilder struct {
	config       *RouterConfig
	users        *UserUseCases
	wallets      *WalletUseCases
	transactions *TransactionUseCases
}

// NewRouterBuilder создаёт новый builder.
func NewRouterBuilder(config *RouterConfig) *RouterBuilder {
	if config == nil {
		config = DefaultRouterConfig()
	}
	return &RouterBuilder{
		config: config,
	}
}

// WithUserUseCases добавляет user use cases.
func (b *RouterBuilder) WithUserUseCases(useCases *UserUseCases) *RouterBuilder {
	b.users = useCases
	return b
}

// WithWalletUseCases добавляет wallet use cases.
func (b *RouterBuilder) WithWalletUseCases(useCases *WalletUseCases) *RouterBuilder {
	b.wallets = useCases
	return b
}

// WithTransactionUseCases добавляет transaction use cases.
func (b *RouterBuilder) WithTransactionUseCases(useCases *TransactionUseCases) *RouterBuilder {
	b.transactions = useCases
	return b
}

// Build создаёт сконфигурированный Gin Engine.
func (b *RouterBuilder) Build() *gin.Engine {
	// Настраиваем режим Gin
	if b.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Создаём router без default middleware
	router := gin.New()

	// Настраиваем кастомные валидаторы
	handlers.SetupValidator()

	// ============================================
	// Global Middleware
	// ============================================

	// 1. Recovery - должен быть первым
	router.Use(middleware.Recovery(&middleware.RecoveryConfig{
		Logger:           b.config.Logger,
		EnableStackTrace: b.config.Environment != "production",
	}))

	// 2. Request ID
	router.Use(middleware.RequestID())

	// 3. CORS
	if b.config.Environment == "production" {
		router.Use(middleware.CORS(middleware.ProductionCORSConfig(b.config.AllowedOrigins)))
	} else {
		router.Use(middleware.CORS(middleware.DefaultCORSConfig()))
	}

	// 4. Logging
	router.Use(middleware.Logging(&middleware.LoggingConfig{
		Logger:    b.config.Logger,
		SkipPaths: []string{"/health", "/live", "/ready", "/metrics"},
	}))

	// 5. Rate Limiting (global)
	router.Use(middleware.RateLimit(middleware.DefaultRateLimitConfig()))

	// 6. Metrics (Prometheus)
	router.Use(middleware.Metrics())

	// ============================================
	// Metrics Endpoint (no auth)
	// ============================================

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ============================================
	// Health Check Routes (no auth)
	// ============================================

	healthHandler := handlers.NewHealthHandler(
		b.config.Pool,
		b.config.Version,
		b.config.BuildTime,
	)
	healthHandler.RegisterRoutes(router)

	// ============================================
	// API v1 Routes
	// ============================================

	v1 := router.Group("/api/v1")

	// Public routes (no auth required)
	publicGroup := v1.Group("")
	{
		// User registration (public)
		if b.users != nil {
			userHandler := handlers.NewUserHandler(
				b.users.CreateUser,
				b.users.ApproveKYC,
				b.users.GetUser,
				b.users.ListUsers,
			)
			publicGroup.POST("/users", userHandler.CreateUser)
		}
	}

	// Protected routes (auth required)
	protectedGroup := v1.Group("")
	protectedGroup.Use(middleware.Auth(&middleware.AuthConfig{
		TokenValidator: b.config.AuthTokenValidator,
		SkipPaths:      []string{}, // Auth обязательна
	}))
	{
		// User routes
		if b.users != nil {
			userHandler := handlers.NewUserHandler(
				b.users.CreateUser,
				b.users.ApproveKYC,
				b.users.GetUser,
				b.users.ListUsers,
			)
			users := protectedGroup.Group("/users")
			{
				users.GET("", userHandler.ListUsers)
				users.GET("/:id", userHandler.GetUser)
				users.POST("/:id/kyc", userHandler.ApproveKYC)
				users.POST("/:id/kyc/start", userHandler.StartKYC)
			}
		}

		// Wallet routes
		if b.wallets != nil {
			walletHandler := handlers.NewWalletHandler(
				b.wallets.CreateWallet,
				b.wallets.CreditWallet,
				b.wallets.DebitWallet,
				b.wallets.TransferFunds,
				b.wallets.GetWallet,
				b.wallets.ListWallets,
			)
			wallets := protectedGroup.Group("/wallets")
			{
				wallets.POST("", walletHandler.CreateWallet)
				wallets.GET("", walletHandler.ListWallets)
				wallets.GET("/me", walletHandler.GetMyWallets)
				wallets.GET("/:id", walletHandler.GetWallet)

				// Financial operations with stricter rate limiting
				financialOps := wallets.Group("")
				financialOps.Use(middleware.TransactionRateLimit())
				{
					financialOps.POST("/:id/credit", walletHandler.CreditWallet)
					financialOps.POST("/:id/debit", walletHandler.DebitWallet)
					financialOps.POST("/:id/transfer", walletHandler.Transfer)
				}
			}
		}

		// Transaction routes
		if b.transactions != nil {
			txHandler := handlers.NewTransactionHandler(
				b.transactions.GetTransaction,
				b.transactions.ListTransactions,
				b.transactions.RetryTransaction,
				b.transactions.CancelTransaction,
			)
			transactions := protectedGroup.Group("/transactions")
			{
				transactions.GET("", txHandler.ListTransactions)
				transactions.GET("/:id", txHandler.GetTransaction)
				transactions.GET("/by-key/:key", txHandler.GetTransactionByIdempotencyKey)
				transactions.POST("/:id/retry", txHandler.RetryTransaction)
				transactions.POST("/:id/cancel", txHandler.CancelTransaction)
			}

			// Nested route: /wallets/:wallet_id/transactions
			if b.wallets != nil {
				protectedGroup.GET("/wallets/:wallet_id/transactions", txHandler.GetWalletTransactions)
			}
		}
	}

	// ============================================
	// Admin Routes (admin role required)
	// ============================================

	adminGroup := v1.Group("/admin")
	adminGroup.Use(middleware.Auth(&middleware.AuthConfig{
		TokenValidator: b.config.AuthTokenValidator,
	}))
	adminGroup.Use(middleware.RequireRole("admin"))
	{
		// Admin-only endpoints можно добавить здесь
		// Например: просмотр всех транзакций, изменение лимитов и т.д.
	}

	// ============================================
	// 404 Handler
	// ============================================

	router.NoRoute(func(c *gin.Context) {
		common.Error(c, 404, &common.APIError{
			Code:    common.ErrCodeNotFound,
			Message: "Endpoint not found",
			Details: map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			},
		})
	})

	return router
}

// ============================================
// Quick Setup Functions
// ============================================

// NewRouter создаёт роутер с базовой конфигурацией (для простых случаев).
func NewRouter(config *RouterConfig) *gin.Engine {
	return NewRouterBuilder(config).Build()
}

// NewDevelopmentRouter создаёт роутер для development окружения.
func NewDevelopmentRouter() *gin.Engine {
	config := DefaultRouterConfig()
	config.Environment = "development"
	return NewRouter(config)
}

// NewProductionRouter создаёт роутер для production окружения.
func NewProductionRouter(pool *pgxpool.Pool, version string, allowedOrigins []string) *gin.Engine {
	config := &RouterConfig{
		Logger:         slog.Default(),
		Pool:           pool,
		Version:        version,
		Environment:    "production",
		AllowedOrigins: allowedOrigins,
		// В production нужен реальный token validator
		AuthTokenValidator: nil, // Должен быть установлен!
	}
	return NewRouter(config)
}
