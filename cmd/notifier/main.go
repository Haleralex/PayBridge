// Notification service for PayBridge.
// Reads events from the outbox table, publishes to NATS JetStream,
// and sends Telegram notifications to users.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"

	natsadapter "github.com/Haleralex/wallethub/internal/adapters/nats"
	"github.com/Haleralex/wallethub/internal/config"
	"github.com/Haleralex/wallethub/internal/domain/events"
	"github.com/Haleralex/wallethub/internal/infrastructure/notification"
	"github.com/Haleralex/wallethub/internal/infrastructure/persistence/postgres"
	"github.com/Haleralex/wallethub/internal/infrastructure/poller"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	logger.Info("Starting PayBridge Notification Service")

	// Load config
	cfg, err := config.Load("./configs", "config")
	if err != nil {
		logger.Warn("Failed to load config file, using env vars", slog.String("error", err.Error()))
		cfg, err = config.LoadFromEnv()
		if err != nil {
			cfg = config.Development()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		logger.Error("Failed to parse database config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	poolConfig.MaxConns = 5
	poolConfig.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("Failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("Failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("Connected to PostgreSQL")

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATS.URL,
		nats.ReconnectWait(cfg.NATS.ReconnectWait),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", slog.String("error", err.Error()))
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("NATS reconnected")
		}),
	)
	if err != nil {
		logger.Error("Failed to connect to NATS", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer nc.Close()
	logger.Info("Connected to NATS", slog.String("url", cfg.NATS.URL))

	// Create NATS publisher
	publisher, err := natsadapter.NewPublisher(nc, cfg.NATS.StreamName, logger)
	if err != nil {
		logger.Error("Failed to create NATS publisher", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create repositories
	outboxRepo := postgres.NewOutboxRepository(pool)
	walletRepo := postgres.NewWalletRepository(pool)
	userRepo := postgres.NewUserRepository(pool)

	// Create Telegram sender
	botToken := cfg.Auth.TelegramBotToken
	if botToken == "" {
		logger.Error("Telegram bot token is required for notification service")
		os.Exit(1)
	}
	telegramSender := notification.NewTelegramSender(botToken, logger)

	// Create notification handler
	notifHandler := notification.NewHandler(telegramSender, walletRepo, userRepo, logger)

	// Create NATS subscriber
	subscriber, err := natsadapter.NewSubscriber(nc, logger)
	if err != nil {
		logger.Error("Failed to create NATS subscriber", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Register event handlers — only notify the recipient (wallet.credited)
	subscriber.Handle("paybridge.events."+events.EventTypeWalletCredited, notifHandler.HandleWalletCredited)

	// Start NATS subscriber
	if err := subscriber.Start(ctx); err != nil {
		logger.Error("Failed to start NATS subscriber", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer subscriber.Stop()

	// Start outbox poller
	outboxPoller := poller.New(outboxRepo, publisher, logger, poller.Config{
		PollInterval: cfg.Notifier.PollInterval,
		BatchSize:    cfg.Notifier.BatchSize,
		MaxRetries:   cfg.Notifier.MaxRetries,
	})
	go outboxPoller.Start(ctx)

	logger.Info("Notification service is running",
		slog.Duration("poll_interval", cfg.Notifier.PollInterval),
		slog.Int("batch_size", cfg.Notifier.BatchSize),
	)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down notification service", slog.String("signal", sig.String()))

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	outboxPoller.Stop()
	_ = subscriber.Stop()
	nc.Drain()

	<-shutdownCtx.Done()
	fmt.Println() // clean newline
	logger.Info("Notification service stopped")
}
