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
	grpcadapter "github.com/Haleralex/wallethub/internal/adapters/grpc"
	"github.com/Haleralex/wallethub/internal/application/cqrs"
	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/application/usecases/transaction"
	"github.com/Haleralex/wallethub/internal/application/usecases/user"
	"github.com/Haleralex/wallethub/internal/application/usecases/wallet"
	"github.com/Haleralex/wallethub/internal/config"
	"github.com/Haleralex/wallethub/internal/infrastructure/cache"
	"github.com/Haleralex/wallethub/internal/infrastructure/exchange"
	"github.com/Haleralex/wallethub/internal/infrastructure/persistence/postgres"
	"github.com/Haleralex/wallethub/internal/infrastructure/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ============================================
// Container
// ============================================

// Container - DI контейнер приложения.
type Container struct {
	config *config.Config
	logger *slog.Logger

	// Infrastructure
	pool           *pgxpool.Pool
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	redisClient    *redis.Client

	// Cache / Distributed primitives
	tokenBlacklist  ports.TokenBlacklist
	distributedLock ports.DistributedLock

	// Repositories
	userRepo        ports.UserRepository
	walletRepo      ports.WalletRepository
	transactionRepo ports.TransactionRepository
	outboxRepo      *postgres.OutboxRepository

	// Unit of Work
	uow ports.UnitOfWork

	// Event Publisher
	eventPublisher ports.EventPublisher

	// Fraud Detector
	fraudDetector ports.FraudDetector

	// CQRS Buses
	commandBus *cqrs.CommandBus
	queryBus   *cqrs.QueryBus

	// Use Cases
	createUserUC             *user.CreateUserUseCase
	getUserUC                *user.GetUserUseCase
	createWalletUC           *wallet.CreateWalletUseCase
	creditWalletUC           *wallet.CreditWalletUseCase
	debitWalletUC            *wallet.DebitWalletUseCase
	getWalletUC              *wallet.GetWalletUseCase
	listWalletsUC            *wallet.ListWalletsUseCase
	createTransactionUC      *transaction.CreateTransactionUseCase
	processTransactionUC     *transaction.ProcessTransactionUseCase
	cancelTransactionUC      *transaction.CancelTransactionUseCase
	transferBetweenWalletsUC *transaction.TransferBetweenWalletsUseCase
	exchangeCurrencyUC      *transaction.ExchangeCurrencyUseCase
	getByIdempotencyKeyUC   *transaction.GetTransactionByIdempotencyKeyUseCase
	getTransactionUC        *transaction.GetTransactionUseCase
	listTransactionsUC      *transaction.ListTransactionsUseCase
	retryTransactionUC      *transaction.RetryTransactionUseCase

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

	// 0. Telemetry (tracing)
	if err := c.initTracing(ctx); err != nil {
		c.logger.Warn("Failed to initialize tracing, continuing without it", slog.String("error", err.Error()))
	}

	// 0b. Telemetry (metrics push for Fly.io — locally Alloy scrapes /metrics instead)
	if err := c.initMetrics(ctx); err != nil {
		c.logger.Warn("Failed to initialize metrics provider, continuing without it",
			slog.String("error", err.Error()))
	}

	// 1. Database
	if err := c.initDatabase(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	c.logger.Info("Database connected")

	// 1b. Redis (optional — warn and continue if unavailable)
	if err := c.initRedis(); err != nil {
		c.logger.Warn("Redis unavailable, running without distributed cache",
			slog.String("error", err.Error()))
	}

	// 2. Repositories
	c.initRepositories()
	c.logger.Info("Repositories initialized")

	// 3. Fraud Detector
	c.initFraudDetector()

	// 4. Use Cases
	c.initUseCases()
	c.logger.Info("Use cases initialized")

	// 5. CQRS Buses
	c.initCQRS()
	c.logger.Info("CQRS buses initialized")

	// 6. HTTP Server
	c.initHTTPServer()
	c.logger.Info("HTTP server initialized")

	c.logger.Info("Container initialization complete")
	return nil
}

// initTracing инициализирует OpenTelemetry трейсинг.
func (c *Container) initTracing(ctx context.Context) error {
	if !c.config.Telemetry.Enabled {
		c.logger.Info("Telemetry disabled, skipping tracing init")
		return nil
	}

	tp, err := telemetry.InitTracer(ctx, c.config.App.Name, c.config.Telemetry.OTLPEndpoint)
	if err != nil {
		return err
	}

	c.tracerProvider = tp
	c.logger.Info("Tracing initialized",
		slog.String("endpoint", c.config.Telemetry.OTLPEndpoint),
	)
	return nil
}

// initMetrics initializes OpenTelemetry MeterProvider for OTLP push to Grafana Cloud.
// On Fly.io (no Alloy sidecar), this pushes paybridge_* Prometheus metrics every 30s.
// Locally, Alloy scrapes /metrics — this provider is still initialized but harmless.
func (c *Container) initMetrics(ctx context.Context) error {
	if !c.config.Telemetry.Enabled {
		return nil
	}

	mp, err := telemetry.InitMeterProvider(ctx, c.config.App.Name, c.config.Telemetry.OTLPEndpoint)
	if err != nil {
		return err
	}

	c.meterProvider = mp
	c.logger.Info("Metrics provider initialized",
		slog.String("endpoint", c.config.Telemetry.OTLPEndpoint),
	)
	return nil
}

// initFraudDetector инициализирует детектор фрода.
// В development/test используем NoOp (всегда разрешает).
// В production — gRPC клиент к fraud-detector сервису.
func (c *Container) initFraudDetector() {
	if !c.config.Fraud.Enabled {
		c.logger.Info("Fraud detection disabled, using no-op detector")
		c.fraudDetector = grpcadapter.NewNoOpFraudDetector()
		return
	}

	client, err := grpcadapter.NewFraudClient(
		c.config.Fraud.GRPCEndpoint,
		c.config.Fraud.Timeout,
		c.logger,
	)
	if err != nil {
		c.logger.Warn("Failed to connect to fraud detector, using no-op fallback",
			slog.String("endpoint", c.config.Fraud.GRPCEndpoint),
			slog.String("error", err.Error()),
		)
		c.fraudDetector = grpcadapter.NewNoOpFraudDetector()
		return
	}

	c.fraudDetector = client
	c.logger.Info("Fraud detector initialized",
		slog.String("endpoint", c.config.Fraud.GRPCEndpoint),
	)
}

// initRedis инициализирует Redis клиент и зависимые компоненты.
// Не возвращает fatal-ошибку — приложение продолжит работу без Redis.
func (c *Container) initRedis() error {
	rdb, err := cache.NewRedisClient(c.config.Redis)
	if err != nil {
		return err
	}
	c.redisClient = rdb
	c.tokenBlacklist = cache.NewRedisTokenBlacklist(rdb)
	c.distributedLock = cache.NewRedisDistributedLock(rdb)
	c.logger.Info("Redis connected",
		slog.String("addr", fmt.Sprintf("%s:%d", c.config.Redis.Host, c.config.Redis.Port)),
	)
	return nil
}

// initCQRS инициализирует Command Bus и Query Bus с middleware pipeline.
func (c *Container) initCQRS() {
	// Command Bus — middleware: Recovery → Tracing → Logging
	c.commandBus = cqrs.NewCommandBus(
		cqrs.RecoveryMiddleware(c.logger),
		cqrs.TracingMiddleware(),
		cqrs.LoggingMiddleware(c.logger),
	)

	// Query Bus — middleware: Recovery → Tracing → Logging
	c.queryBus = cqrs.NewQueryBus(
		cqrs.RecoveryMiddleware(c.logger),
		cqrs.TracingMiddleware(),
		cqrs.LoggingMiddleware(c.logger),
	)

	// Register Command Handlers
	cqrs.RegisterCommandHandler[dtos.CreateUserCommand, *dtos.UserCreatedDTO](c.commandBus, c.createUserUC)
	cqrs.RegisterCommandHandler[dtos.CreateWalletCommand, *dtos.WalletDTO](c.commandBus, c.createWalletUC)
	cqrs.RegisterCommandHandler[dtos.CreditWalletCommand, *dtos.WalletOperationDTO](c.commandBus, c.creditWalletUC)
	cqrs.RegisterCommandHandler[dtos.DebitWalletCommand, *dtos.WalletOperationDTO](c.commandBus, c.debitWalletUC)
	cqrs.RegisterCommandHandler[dtos.TransferFundsCommand, *dtos.TransferResultDTO](c.commandBus, c.transferBetweenWalletsUC)
	cqrs.RegisterCommandHandler[dtos.ExchangeCurrencyCommand, *dtos.ExchangeResultDTO](c.commandBus, c.exchangeCurrencyUC)
	cqrs.RegisterCommandHandler[dtos.RetryTransactionCommand, *dtos.TransactionDTO](c.commandBus, c.retryTransactionUC)
	cqrs.RegisterCommandHandler[dtos.CancelTransactionCommand, *dtos.TransactionDTO](c.commandBus, c.cancelTransactionUC)

	// Register Query Handlers
	cqrs.RegisterQueryHandler[dtos.GetUserQuery, *dtos.UserDTO](c.queryBus, c.getUserUC)
	cqrs.RegisterQueryHandler[dtos.GetWalletQuery, *dtos.WalletDTO](c.queryBus, c.getWalletUC)
	cqrs.RegisterQueryHandler[dtos.ListWalletsQuery, *dtos.WalletListDTO](c.queryBus, c.listWalletsUC)
	cqrs.RegisterQueryHandler[dtos.GetTransactionQuery, *dtos.TransactionDTO](c.queryBus, c.getTransactionUC)
	cqrs.RegisterQueryHandler[dtos.ListTransactionsQuery, *dtos.TransactionListDTO](c.queryBus, c.listTransactionsUC)
	cqrs.RegisterQueryHandler[dtos.GetTransactionByIdempotencyKeyQuery, *dtos.TransactionDTO](c.queryBus, c.getByIdempotencyKeyUC)
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
	c.getUserUC = user.NewGetUserUseCase(c.userRepo)

	// Wallet Use Cases
	c.createWalletUC = wallet.NewCreateWalletUseCase(c.userRepo, c.walletRepo, c.eventPublisher, c.uow)
	c.creditWalletUC = wallet.NewCreditWalletUseCase(c.walletRepo, c.transactionRepo, c.eventPublisher, c.uow)
	c.debitWalletUC = wallet.NewDebitWalletUseCase(c.walletRepo, c.transactionRepo, c.eventPublisher, c.uow)
	c.getWalletUC = wallet.NewGetWalletUseCase(c.walletRepo)
	c.listWalletsUC = wallet.NewListWalletsUseCase(c.walletRepo)

	// Transaction Use Cases
	c.createTransactionUC = transaction.NewCreateTransactionUseCase(
		c.walletRepo,
		c.transactionRepo,
		c.eventPublisher,
		c.uow,
		c.distributedLock, // nil if Redis unavailable
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
		c.fraudDetector,
	)

	// Exchange Currency
	exchangeProvider := exchange.NewProvider(
		c.config.Exchange.APIKey,
		c.config.Exchange.APIURL,
		c.config.Exchange.CacheTTL,
	)
	c.exchangeCurrencyUC = transaction.NewExchangeCurrencyUseCase(
		c.walletRepo,
		c.transactionRepo,
		exchangeProvider,
		c.eventPublisher,
		c.uow,
		c.config.Exchange.SpreadPercent,
		c.fraudDetector,
	)
	c.getByIdempotencyKeyUC = transaction.NewGetTransactionByIdempotencyKeyUseCase(c.transactionRepo)
	c.getTransactionUC = transaction.NewGetTransactionUseCase(c.transactionRepo)
	c.listTransactionsUC = transaction.NewListTransactionsUseCase(c.transactionRepo)
	c.retryTransactionUC = transaction.NewRetryTransactionUseCase(c.walletRepo, c.transactionRepo, c.eventPublisher, c.uow)
}

// initHTTPServer инициализирует HTTP сервер.
func (c *Container) initHTTPServer() {
	// Token validator - всегда используем настоящий JWT validator
	// Telegram Auth и все endpoint'ы требуют валидный JWT токен
	tokenValidator := middleware.NewJWTTokenValidator(
		c.config.Auth.JWTSecret,
		c.config.Auth.JWTIssuer,
		c.tokenBlacklist, // nil if Redis unavailable
	)

	if c.config.Auth.EnableMockAuth {
		c.logger.Warn("Mock auth enabled — development mode, real JWT validation still active")
	} else {
		c.logger.Info("Using real JWT authentication")
	}

	// Router Config
	routerConfig := &http.RouterConfig{
		Logger:             c.logger,
		Pool:               c.pool,
		Version:            c.config.App.Version,
		BuildTime:          c.config.App.BuildTime,
		Environment:        c.config.App.Environment,
		AllowedOrigins:     c.config.CORS.AllowedOrigins,
		AuthTokenValidator: tokenValidator,
		TelegramBotToken:   c.config.Auth.TelegramBotToken,
		JWTSecret:          c.config.Auth.JWTSecret,
		JWTIssuer:          c.config.Auth.JWTIssuer,
		RedisClient:        c.redisClient,        // nil if Redis unavailable
		TokenBlacklist:     c.tokenBlacklist,     // nil if Redis unavailable
	}

	// Build Router (CQRS buses dispatch commands/queries through middleware pipeline)
	router := http.NewRouterBuilder(routerConfig).
		WithCQRS(c.commandBus, c.queryBus).
		WithTelegramAuth(&http.TelegramAuthDeps{
			UserRepo:   c.userRepo,
			WalletRepo: c.walletRepo,
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

// CreateWalletUseCase возвращает use case создания кошелька.
func (c *Container) CreateWalletUseCase() *wallet.CreateWalletUseCase {
	return c.createWalletUC
}

// CreditWalletUseCase возвращает use case пополнения кошелька.
func (c *Container) CreditWalletUseCase() *wallet.CreditWalletUseCase {
	return c.creditWalletUC
}

// DebitWalletUseCase возвращает use case списания с кошелька.
func (c *Container) DebitWalletUseCase() *wallet.DebitWalletUseCase {
	return c.debitWalletUC
}

// GetWalletUseCase возвращает use case получения кошелька.
func (c *Container) GetWalletUseCase() *wallet.GetWalletUseCase {
	return c.getWalletUC
}

// ListWalletsUseCase возвращает use case списка кошельков.
func (c *Container) ListWalletsUseCase() *wallet.ListWalletsUseCase {
	return c.listWalletsUC
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

	// 2. Tracer Provider
	if c.tracerProvider != nil {
		if err := c.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
	}

	// 2b. Meter Provider
	if c.meterProvider != nil {
		if err := c.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	// 2c. Redis
	if c.redisClient != nil {
		if err := c.redisClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("redis shutdown: %w", err))
		}
	}

	// 3. Database (даём время на завершение транзакций)
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
