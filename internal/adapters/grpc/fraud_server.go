package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	pb "github.com/Haleralex/wallethub/internal/adapters/grpc/pb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FraudServer implements the FraudDetectionService gRPC server.
type FraudServer struct {
	pb.UnimplementedFraudDetectionServiceServer
	pool   *pgxpool.Pool
	logger *slog.Logger
	config FraudRulesConfig
}

// FraudRulesConfig holds configurable thresholds for fraud rules.
type FraudRulesConfig struct {
	HighAmountThreshold    float64       // Amount above which transaction is high risk
	MaxTransactionsPerHour int           // Max transactions per hour before suspicious
	NewAccountAge          time.Duration // Account younger than this is considered new
	NewAccountMaxAmount    float64       // Max amount for new accounts
}

// DefaultFraudRulesConfig returns sensible defaults.
func DefaultFraudRulesConfig() FraudRulesConfig {
	return FraudRulesConfig{
		HighAmountThreshold:    10000.0,
		MaxTransactionsPerHour: 10,
		NewAccountAge:          24 * time.Hour,
		NewAccountMaxAmount:    1000.0,
	}
}

// NewFraudServer creates a new fraud detection gRPC server.
func NewFraudServer(pool *pgxpool.Pool, logger *slog.Logger, config FraudRulesConfig) *FraudServer {
	return &FraudServer{
		pool:   pool,
		logger: logger,
		config: config,
	}
}

// CheckTransaction evaluates a transaction for fraud risk.
func (s *FraudServer) CheckTransaction(ctx context.Context, req *pb.CheckTransactionRequest) (*pb.CheckTransactionResponse, error) {
	s.logger.Info("Fraud check requested",
		slog.String("user_id", req.UserId),
		slog.String("amount", req.Amount),
		slog.String("currency", req.Currency),
		slog.String("type", req.TransactionType),
	)

	// Parse amount
	amount, ok := new(big.Float).SetString(req.Amount)
	if !ok {
		return &pb.CheckTransactionResponse{
			Approved:  false,
			RiskScore: 1.0,
			Reason:    "invalid amount format",
		}, nil
	}
	amountFloat, _ := amount.Float64()

	// Rule 1: High amount threshold
	if amountFloat > s.config.HighAmountThreshold {
		s.logger.Warn("High amount detected",
			slog.String("user_id", req.UserId),
			slog.Float64("amount", amountFloat),
			slog.Float64("threshold", s.config.HighAmountThreshold),
		)
		return &pb.CheckTransactionResponse{
			Approved:  false,
			RiskScore: 0.9,
			Reason:    fmt.Sprintf("amount %.2f exceeds threshold %.2f", amountFloat, s.config.HighAmountThreshold),
		}, nil
	}

	// Rule 2: Transaction frequency (too many transactions in last hour)
	txCount, err := s.countRecentTransactions(ctx, req.UserId)
	if err != nil {
		s.logger.Error("Failed to count recent transactions", slog.String("error", err.Error()))
		// Don't block on internal errors — approve with warning
		return &pb.CheckTransactionResponse{
			Approved:  true,
			RiskScore: 0.5,
			Reason:    "fraud check partially failed, approved with caution",
		}, nil
	}

	if txCount >= s.config.MaxTransactionsPerHour {
		s.logger.Warn("High transaction frequency",
			slog.String("user_id", req.UserId),
			slog.Int("count", txCount),
			slog.Int("max", s.config.MaxTransactionsPerHour),
		)
		return &pb.CheckTransactionResponse{
			Approved:  false,
			RiskScore: 0.8,
			Reason:    fmt.Sprintf("too many transactions: %d in last hour (max %d)", txCount, s.config.MaxTransactionsPerHour),
		}, nil
	}

	// Rule 3: New account + large amount
	accountAge, err := s.getAccountAge(ctx, req.UserId)
	if err != nil {
		s.logger.Error("Failed to get account age", slog.String("error", err.Error()))
	} else if accountAge < s.config.NewAccountAge && amountFloat > s.config.NewAccountMaxAmount {
		s.logger.Warn("New account with large amount",
			slog.String("user_id", req.UserId),
			slog.Duration("account_age", accountAge),
			slog.Float64("amount", amountFloat),
		)
		return &pb.CheckTransactionResponse{
			Approved:  false,
			RiskScore: 0.85,
			Reason:    fmt.Sprintf("new account (%.0f hours old) with amount %.2f exceeds limit %.2f", accountAge.Hours(), amountFloat, s.config.NewAccountMaxAmount),
		}, nil
	}

	// Calculate risk score based on factors
	riskScore := 0.0
	if amountFloat > s.config.HighAmountThreshold*0.5 {
		riskScore += 0.2
	}
	if txCount > s.config.MaxTransactionsPerHour/2 {
		riskScore += 0.15
	}
	if accountAge < s.config.NewAccountAge*2 {
		riskScore += 0.1
	}

	s.logger.Info("Fraud check passed",
		slog.String("user_id", req.UserId),
		slog.Float64("risk_score", riskScore),
	)

	return &pb.CheckTransactionResponse{
		Approved:  true,
		RiskScore: riskScore,
		Reason:    "",
	}, nil
}

// countRecentTransactions counts transactions by user in the last hour.
func (s *FraudServer) countRecentTransactions(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transactions t
		JOIN wallets w ON t.wallet_id = w.id
		WHERE w.user_id = $1
		AND t.created_at > NOW() - INTERVAL '1 hour'
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}
	return count, nil
}

// getAccountAge returns how long ago the user was created.
func (s *FraudServer) getAccountAge(ctx context.Context, userID string) (time.Duration, error) {
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT created_at FROM users WHERE id = $1
	`, userID).Scan(&createdAt)
	if err != nil {
		return 0, fmt.Errorf("failed to get user created_at: %w", err)
	}
	return time.Since(createdAt), nil
}
