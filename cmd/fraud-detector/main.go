// Fraud Detection gRPC service for PayBridge.
// Evaluates financial transactions for fraud risk using rule-based analysis.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	grpcadapter "github.com/Haleralex/wallethub/internal/adapters/grpc"
	pb "github.com/Haleralex/wallethub/internal/adapters/grpc/pb"
	"github.com/Haleralex/wallethub/internal/config"
	"github.com/Haleralex/wallethub/internal/infrastructure/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	logger.Info("Starting PayBridge Fraud Detection Service")

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

	// Initialize tracing
	if cfg.Telemetry.Enabled {
		tp, err := telemetry.InitTracer(ctx, "paybridge-fraud-detector", cfg.Telemetry.OTLPEndpoint)
		if err != nil {
			logger.Warn("Failed to initialize tracing", slog.String("error", err.Error()))
		} else {
			defer func() {
				shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				_ = tp.Shutdown(shutdownCtx)
			}()
			logger.Info("Tracing initialized", slog.String("endpoint", cfg.Telemetry.OTLPEndpoint))
		}
	}

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

	// Create gRPC server with OTel instrumentation
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	// Register fraud detection service
	fraudServer := grpcadapter.NewFraudServer(pool, logger, grpcadapter.DefaultFraudRulesConfig())
	pb.RegisterFraudDetectionServiceServer(grpcServer, fraudServer)

	// Register health check
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("fraud.v1.FraudDetectionService", healthpb.HealthCheckResponse_SERVING)

	// Enable reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	// Start listening
	port := "50051"
	if p := os.Getenv("PAYBRIDGE_FRAUD_GRPC_PORT"); p != "" {
		port = p
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		logger.Error("Failed to listen", slog.String("port", port), slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Start server in goroutine
	go func() {
		logger.Info("Fraud Detection gRPC server started", slog.String("port", port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", slog.String("error", err.Error()))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down fraud detection service", slog.String("signal", sig.String()))

	// Graceful shutdown
	healthServer.SetServingStatus("fraud.v1.FraudDetectionService", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()

	logger.Info("Fraud detection service stopped")
}
