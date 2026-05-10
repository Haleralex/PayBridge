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
	"path/filepath"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/adapters/http/handlers"
	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/Haleralex/wallethub/internal/application/cqrs"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
	// TelegramBotToken - Telegram bot token for Mini App auth validation
	TelegramBotToken string
	// JWTSecret - secret for signing JWT tokens (used by Telegram auth handler)
	JWTSecret string
	// JWTIssuer - issuer claim for JWT tokens
	JWTIssuer string
	// RedisClient - optional Redis client for distributed rate limiting.
	// If nil, falls back to in-memory rate limiting.
	RedisClient *redis.Client
	// TokenBlacklist - optional token blacklist for logout support.
	TokenBlacklist ports.TokenBlacklist
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
// Router Builder
// ============================================

// TelegramAuthDeps - dependencies for Telegram auth handler.
type TelegramAuthDeps struct {
	UserRepo   ports.UserRepository
	WalletRepo ports.WalletRepository
}

// RouterBuilder - builder для создания роутера.
//
// Pattern: Builder
// - Позволяет пошагово настроить роутер
// - CQRS buses dispatch commands/queries through middleware pipeline
// - Проще тестировать
type RouterBuilder struct {
	config       *RouterConfig
	commandBus   *cqrs.CommandBus
	queryBus     *cqrs.QueryBus
	telegramAuth *TelegramAuthDeps
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

// WithCQRS добавляет CQRS Command Bus и Query Bus.
func (b *RouterBuilder) WithCQRS(commandBus *cqrs.CommandBus, queryBus *cqrs.QueryBus) *RouterBuilder {
	b.commandBus = commandBus
	b.queryBus = queryBus
	return b
}

// WithTelegramAuth добавляет Telegram auth dependencies.
func (b *RouterBuilder) WithTelegramAuth(deps *TelegramAuthDeps) *RouterBuilder {
	b.telegramAuth = deps
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

	// 2. OpenTelemetry tracing
	router.Use(otelgin.Middleware("paybridge-api"))

	// 3. Request ID
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

	// 5. Rate Limiting (global) — Redis if available, otherwise in-memory
	if b.config.RedisClient != nil {
		router.Use(middleware.RedisRateLimit(b.config.RedisClient, middleware.DefaultRateLimitConfig()))
	} else {
		router.Use(middleware.RateLimit(middleware.DefaultRateLimitConfig()))
	}

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
		if b.commandBus != nil {
			userHandler := handlers.NewUserHandler(b.commandBus, b.queryBus)
			publicGroup.POST("/users", userHandler.CreateUser)
		}

		// Telegram Mini App authentication (public)
		if b.telegramAuth != nil {
			tgHandler := handlers.NewTelegramAuthHandler(handlers.TelegramAuthConfig{
				UserRepo:    b.telegramAuth.UserRepo,
				WalletRepo:  b.telegramAuth.WalletRepo,
				BotToken:    b.config.TelegramBotToken,
				JWTSecret:   b.config.JWTSecret,
				JWTIssuer:   b.config.JWTIssuer,
				TokenExpiry: 15 * time.Minute,
				Blacklist:   b.config.TokenBlacklist,
			})
			publicGroup.POST("/auth/telegram", tgHandler.Authenticate)

			// Logout requires auth (token must be valid to be revoked)
			logoutGroup := v1.Group("")
			logoutGroup.Use(middleware.Auth(&middleware.AuthConfig{
				TokenValidator: b.config.AuthTokenValidator,
			}))
			logoutGroup.POST("/auth/logout", tgHandler.Logout)
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
		if b.commandBus != nil {
			userHandler := handlers.NewUserHandler(b.commandBus, b.queryBus)
			users := protectedGroup.Group("/users")
			{
				users.GET("", userHandler.ListUsers)
				users.GET("/:id", userHandler.GetUser)
				users.POST("/:id/kyc", userHandler.ApproveKYC)
				users.POST("/:id/kyc/start", userHandler.StartKYC)
			}
		}

		// Wallet routes
		if b.commandBus != nil {
			walletHandler := handlers.NewWalletHandler(b.commandBus, b.queryBus)
			wallets := protectedGroup.Group("/wallets")
			{
				wallets.POST("", walletHandler.CreateWallet)
				wallets.GET("", walletHandler.ListWallets)
				wallets.GET("/me", walletHandler.GetMyWallets)
				wallets.POST("/me", walletHandler.GetMyWallets) // POST duplicate for ngrok compatibility
				wallets.GET("/:id", walletHandler.GetWallet)

				// Financial operations with stricter rate limiting
				financialOps := wallets.Group("")
				financialOps.Use(middleware.TransactionRateLimit())
				{
					financialOps.POST("/:id/credit", walletHandler.CreditWallet)
					financialOps.POST("/:id/debit", walletHandler.DebitWallet)
					financialOps.POST("/:id/transfer", walletHandler.Transfer)
					financialOps.POST("/:id/exchange", walletHandler.ExchangeCurrency)
				}
			}
		}

		// Transaction routes
		if b.commandBus != nil {
			txHandler := handlers.NewTransactionHandler(b.commandBus, b.queryBus)
			transactions := protectedGroup.Group("/transactions")
			{
				transactions.GET("", txHandler.ListTransactions)
				transactions.GET("/:id", txHandler.GetTransaction)
				transactions.GET("/by-key/:key", txHandler.GetTransactionByIdempotencyKey)
				transactions.POST("/:id/retry", txHandler.RetryTransaction)
				transactions.POST("/:id/cancel", txHandler.CancelTransaction)
			}

			// Nested route: /wallets/:id/transactions
			protectedGroup.GET("/wallets/:id/transactions", txHandler.GetWalletTransactions)
			protectedGroup.POST("/wallets/:id/transactions", txHandler.GetWalletTransactions) // POST duplicate for ngrok compatibility
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
	}

	// ============================================
	// Static Files (Webapp / Telegram Mini App)
	// ============================================

	// Serve webapp with no-cache headers
	serveWebapp := func(c *gin.Context) {
		fp := c.Param("filepath")
		if fp == "/" || fp == "" {
			fp = "/index.html"
		}
		fullPath := filepath.Join("./webapp", fp)
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(fullPath)
	}
	router.GET("/app/*filepath", serveWebapp)
	router.GET("/m/*filepath", serveWebapp) // alternate path to bust WebView cache

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
		AuthTokenValidator: nil, // Должен быть установлен!
	}
	return NewRouter(config)
}
