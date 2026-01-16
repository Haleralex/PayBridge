package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/application/usecases/user"
	"github.com/Haleralex/wallethub/internal/application/usecases/wallet"
	"github.com/Haleralex/wallethub/internal/infrastructure/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 1. Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("🚀 Starting PayBridge API Server...")

	// 2. Database Connection
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/paybridge?sslmode=disable")

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("Failed to ping database:", err)
	}
	logger.Info("✅ Database connected successfully")

	// 3. Repositories
	userRepo := postgres.NewUserRepository(pool)
	walletRepo := postgres.NewWalletRepository(pool)
	transactionRepo := postgres.NewTransactionRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)

	// Unit of Work - только принимает pool
	uow := postgres.NewUnitOfWork(pool)

	// Event Publisher - OutboxRepository реализует интерфейс
	var eventPublisher ports.EventPublisher = outboxRepo

	// 4. Use Cases
	createUserUC := user.NewCreateUserUseCase(userRepo, eventPublisher, uow)
	createWalletUC := wallet.NewCreateWalletUseCase(userRepo, walletRepo, eventPublisher, uow)
	creditWalletUC := wallet.NewCreditWalletUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	logger.Info("✅ Use cases initialized")

	// 5. Router Configuration
	routerConfig := &http.RouterConfig{
		Logger:             logger,
		Pool:               pool,
		Version:            "1.0.0",
		BuildTime:          time.Now().Format(time.RFC3339),
		Environment:        getEnv("ENVIRONMENT", "development"),
		AllowedOrigins:     []string{"*"},
		AuthTokenValidator: nil, // Will use default mock validator
	}

	router := http.NewRouterBuilder(routerConfig).
		WithUserUseCases(&http.UserUseCases{
			CreateUser: createUserUC,
		}).
		WithWalletUseCases(&http.WalletUseCases{
			CreateWallet: createWalletUC,
			CreditWallet: creditWalletUC,
		}).
		Build()

	logger.Info("✅ HTTP router configured")

	// 6. HTTP Server
	serverConfig := &http.ServerConfig{
		Host:            getEnv("HOST", "0.0.0.0"),
		Port:            getEnv("PORT", "8080"),
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Logger:          logger,
	}

	server := http.NewServer(serverConfig, router)

	// 7. Start Server
	logger.Info(fmt.Sprintf("🌍 Server starting on http://%s:%s", serverConfig.Host, serverConfig.Port))
	logger.Info("📚 API Documentation: http://localhost:8080/health")
	logger.Info("Press Ctrl+C to stop")

	if err := server.Run(); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("👋 Server stopped gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
