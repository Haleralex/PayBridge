// Package container - Dependency Injection container for the application.
//
// Container управляет жизненным циклом всех зависимостей:
// - Создание (lazy initialization)
// - Доступ (getters)
// - Закрытие (cleanup)
//
// Pattern: Composition Root
// - Все зависимости собираются в одном месте
// - Легко тестировать
// - Легко заменять реализации
package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http"
	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/application/usecases/transaction"
	"github.com/Haleralex/wallethub/internal/application/usecases/user"
	"github.com/Haleralex/wallethub/internal/application/usecases/wallet"
	"github.com/Haleralex/wallethub/internal/config"
	"github.com/Haleralex/wallethub/internal/infrastructure/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================
// Container
// ============================================

// Container - DI контейнер приложения.
type Container struct {
	config *config.Config
	logger *slog.Logger

	// Infrastructure
	pool *pgxpool.Pool

	// Repositories
	userRepo        ports.UserRepository
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	outboxRepo      *postgres.OutboxRepository

	// Unit of Work
	uow ports.UnitOfWork

	// Event Publisher
	eventPublisher ports.EventPublisher

	// Use Cases
	createUserUC             *user.CreateUserUseCase
	approveKYCUC             *user.ApproveKYCUseCase
	createWalletUC           *wallet.CreateWalletUseCase
	creditWalletUC           *wallet.CreditWalletUseCase
	createTransactionUC      *transaction.CreateTransactionUseCase
	processTransactionUC     *transaction.ProcessTransactionUseCase
	cancelTransactionUC      *transaction.CancelTransactionUseCase
	transferBetweenWalletsUC *transaction.TransferBetweenWalletsUseCase

	// HTTP
	httpServer *http.Server
}

// New создаёт новый контейнер с заданной конфигурацией.
func New(cfg *config.Config) *Container {
	return &Container{
		config: cfg,
	}
}

// ============================================
// Initialization
// ============================================

// Initialize инициализирует все зависимости.
func (c *Container) Initialize(ctx context.Context) error {
	c.logger = c.initLogger()
	c.logger.Info("Initializing application container...")

	// 1. Database
	if err := c.initDatabase(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	c.logger.Info("Database connected")

	// 2. Repositories
	c.initRepositories()
	c.logger.Info("Repositories initialized")

	// 3. Use Cases
	c.initUseCases()
	c.logger.Info("Use cases initialized")

	// 4. HTTP Server
	c.initHTTPServer()
	c.logger.Info("HTTP server initialized")

	c.logger.Info("Container initialization complete")
	return nil
}

// initLogger инициализирует логгер.
func (c *Container) initLogger() *slog.Logger {
	var handler slog.Handler

	level := slog.LevelInfo
	switch c.config.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: c.config.App.Debug,
	}

	if c.config.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// initDatabase инициализирует подключение к БД.
func (c *Container) initDatabase(ctx context.Context) error {
	poolConfig, err := pgxpool.ParseConfig(c.config.Database.DSN())
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = c.config.Database.MaxConnections
	poolConfig.MinConns = c.config.Database.MinConnections
	poolConfig.MaxConnLifetime = c.config.Database.MaxConnLifetime
	poolConfig.MaxConnIdleTime = c.config.Database.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.pool = pool
	return nil
}

// initRepositories инициализирует репозитории.
func (c *Container) initRepositories() {
	c.userRepo = postgres.NewUserRepository(c.pool)
	c.walletRepo = postgres.NewWalletRepository(c.pool)
	c.transactionRepo = postgres.NewTransactionRepository(c.pool)
	c.outboxRepo = postgres.NewOutboxRepository(c.pool)

	// Unit of Work
	c.uow = postgres.NewUnitOfWork(c.pool)

	// Event Publisher (OutboxRepository реализует интерфейс)
	c.eventPublisher = c.outboxRepo
}

// initUseCases инициализирует use cases.
func (c *Container) initUseCases() {
	// User Use Cases
	c.createUserUC = user.NewCreateUserUseCase(c.userRepo, c.eventPublisher, c.uow)
	c.approveKYCUC = user.NewApproveKYCUseCase(c.userRepo, c.eventPublisher, c.uow)

	// Wallet Use Cases
	c.createWalletUC = wallet.NewCreateWalletUseCase(c.userRepo, c.walletRepo, c.eventPublisher, c.uow)
	c.creditWalletUC = wallet.NewCreditWalletUseCase(c.walletRepo, c.transactionRepo, c.eventPublisher, c.uow)

	// Transaction Use Cases
	c.createTransactionUC = transaction.NewCreateTransactionUseCase(
		c.walletRepo,
		c.transactionRepo,
		c.eventPublisher,
		c.uow,
	)
	c.processTransactionUC = transaction.NewProcessTransactionUseCase(
		c.walletRepo,
		c.transactionRepo,
		c.eventPublisher,
		c.uow,
	)
	c.cancelTransactionUC = transaction.NewCancelTransactionUseCase(
		c.walletRepo,
		c.transactionRepo,
		c.eventPublisher,
		c.uow,
	)
	c.transferBetweenWalletsUC = transaction.NewTransferBetweenWalletsUseCase(
		c.walletRepo,
		c.transactionRepo,
		c.eventPublisher,
		c.uow,
	)
}

// initHTTPServer инициализирует HTTP сервер.
func (c *Container) initHTTPServer() {
	// Token validator
	var tokenValidator func(token string) (*middleware.AuthClaims, error)
	if c.config.Auth.EnableMockAuth {
		tokenValidator = middleware.MockTokenValidator
	}
	// В production здесь будет реальный JWT validator

	// Router Config
	routerConfig := &http.RouterConfig{
		Logger:             c.logger,
		Pool:               c.pool,
		Version:            c.config.App.Version,
		BuildTime:          c.config.App.BuildTime,
		Environment:        c.config.App.Environment,
		AllowedOrigins:     c.config.CORS.AllowedOrigins,
		AuthTokenValidator: tokenValidator,
	}

	// Build Router
	router := http.NewRouterBuilder(routerConfig).
		WithUserUseCases(&http.UserUseCases{
			CreateUser: c.createUserUC,
			ApproveKYC: c.approveKYCUC,
		}).
		WithWalletUseCases(&http.WalletUseCases{
			CreateWallet: c.createWalletUC,
			CreditWallet: c.creditWalletUC,
			// TransferFunds not wired - interface mismatch, needs adapter
		}).
		Build()

	// Server Config
	serverConfig := &http.ServerConfig{
		Host:            c.config.Server.Host,
		Port:            fmt.Sprintf("%d", c.config.Server.Port),
		ReadTimeout:     c.config.Server.ReadTimeout,
		WriteTimeout:    c.config.Server.WriteTimeout,
		IdleTimeout:     c.config.Server.IdleTimeout,
		ShutdownTimeout: c.config.Server.ShutdownTimeout,
		Logger:          c.logger,
	}

	c.httpServer = http.NewServer(serverConfig, router)
}

// ============================================
// Getters
// ============================================

// Config возвращает конфигурацию.
func (c *Container) Config() *config.Config {
	return c.config
}

// Logger возвращает логгер.
func (c *Container) Logger() *slog.Logger {
	return c.logger
}

// Pool возвращает пул соединений к БД.
func (c *Container) Pool() *pgxpool.Pool {
	return c.pool
}

// HTTPServer возвращает HTTP сервер.
func (c *Container) HTTPServer() *http.Server {
	return c.httpServer
}

// ============================================
// Repository Getters
// ============================================

// UserRepository возвращает репозиторий пользователей.
func (c *Container) UserRepository() ports.UserRepository {
	return c.userRepo
}

// WalletRepository возвращает репозиторий кошельков.
func (c *Container) WalletRepository() ports.WalletRepository {
	return c.walletRepo
}

// TransactionRepository возвращает репозиторий транзакций.
func (c *Container) TransactionRepository() ports.TransactionRepository {
	return c.transactionRepo
}

// UnitOfWork возвращает Unit of Work.
func (c *Container) UnitOfWork() ports.UnitOfWork {
	return c.uow
}

// ============================================
// Use Case Getters
// ============================================

// CreateUserUseCase возвращает use case создания пользователя.
func (c *Container) CreateUserUseCase() *user.CreateUserUseCase {
	return c.createUserUC
}

// ApproveKYCUseCase возвращает use case подтверждения KYC.
func (c *Container) ApproveKYCUseCase() *user.ApproveKYCUseCase {
	return c.approveKYCUC
}

// CreateWalletUseCase возвращает use case создания кошелька.
func (c *Container) CreateWalletUseCase() *wallet.CreateWalletUseCase {
	return c.createWalletUC
}

// CreditWalletUseCase возвращает use case пополнения кошелька.
func (c *Container) CreditWalletUseCase() *wallet.CreditWalletUseCase {
	return c.creditWalletUC
}

// TransferBetweenWalletsUseCase возвращает use case перевода между кошельками.
func (c *Container) TransferBetweenWalletsUseCase() *transaction.TransferBetweenWalletsUseCase {
	return c.transferBetweenWalletsUC
}

// ============================================
// Shutdown
// ============================================

// Shutdown выполняет graceful shutdown всех компонентов.
func (c *Container) Shutdown(ctx context.Context) error {
	c.logger.Info("Shutting down container...")

	var errs []error

	// 1. HTTP Server
	if c.httpServer != nil {
		if err := c.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP server shutdown: %w", err))
		}
	}

	// 2. Database (даём время на завершение транзакций)
	if c.pool != nil {
		// Graceful close с таймаутом
		done := make(chan struct{})
		go func() {
			c.pool.Close()
			close(done)
		}()

		select {
		case <-done:
			c.logger.Info("Database connection closed")
		case <-ctx.Done():
			c.logger.Warn("Database close timeout")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	c.logger.Info("Container shutdown complete")
	return nil
}

// ============================================
// Run
// ============================================

// Run запускает приложение и ожидает сигнал завершения.
func (c *Container) Run() error {
	c.logger.Info("Starting PayBridge API Server",
		slog.String("version", c.config.App.Version),
		slog.String("environment", c.config.App.Environment),
		slog.String("address", c.config.Server.Address()),
	)

	return c.httpServer.Run()
}

// ============================================
// Builder Pattern (Alternative)
// ============================================

// ContainerBuilder - builder для создания контейнера с кастомными компонентами.
type ContainerBuilder struct {
	cfg            *config.Config
	logger         *slog.Logger
	pool           *pgxpool.Pool
	eventPublisher ports.EventPublisher
}

// NewBuilder создаёт новый builder.
func NewBuilder(cfg *config.Config) *ContainerBuilder {
	return &ContainerBuilder{
		cfg: cfg,
	}
}

// WithLogger устанавливает кастомный логгер.
func (b *ContainerBuilder) WithLogger(logger *slog.Logger) *ContainerBuilder {
	b.logger = logger
	return b
}

// WithPool устанавливает готовый пул соединений.
func (b *ContainerBuilder) WithPool(pool *pgxpool.Pool) *ContainerBuilder {
	b.pool = pool
	return b
}

// WithEventPublisher устанавливает кастомный event publisher.
func (b *ContainerBuilder) WithEventPublisher(ep ports.EventPublisher) *ContainerBuilder {
	b.eventPublisher = ep
	return b
}

// Build создаёт контейнер.
func (b *ContainerBuilder) Build(ctx context.Context) (*Container, error) {
	c := New(b.cfg)

	// Use provided or initialize
	if b.logger != nil {
		c.logger = b.logger
	} else {
		c.logger = c.initLogger()
	}

	if b.pool != nil {
		c.pool = b.pool
	} else {
		if err := c.initDatabase(ctx); err != nil {
			return nil, err
		}
	}

	c.initRepositories()

	if b.eventPublisher != nil {
		c.eventPublisher = b.eventPublisher
	}

	c.initUseCases()
	c.initHTTPServer()

	return c, nil
}

// ============================================
// Health Check
// ============================================

// HealthStatus - статус здоровья приложения.
type HealthStatus struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Uptime    time.Duration     `json:"uptime"`
	Checks    map[string]string `json:"checks"`
}

// Health возвращает статус здоровья приложения.
func (c *Container) Health(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:  "healthy",
		Version: c.config.App.Version,
		Checks:  make(map[string]string),
	}

	// Database check
	if err := c.pool.Ping(ctx); err != nil {
		status.Status = "unhealthy"
		status.Checks["database"] = "error: " + err.Error()
	} else {
		status.Checks["database"] = "ok"
	}

	return status
}
