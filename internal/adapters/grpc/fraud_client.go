package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/Haleralex/wallethub/internal/adapters/grpc/pb"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FraudClient is a gRPC client for the fraud detection service.
type FraudClient struct {
	conn    *grpc.ClientConn
	client  pb.FraudDetectionServiceClient
	timeout time.Duration
	logger  *slog.Logger
}

// NewFraudClient creates a new gRPC fraud detection client.
func NewFraudClient(endpoint string, timeout time.Duration, logger *slog.Logger) (*FraudClient, error) {
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to fraud service at %s: %w", endpoint, err)
	}

	return &FraudClient{
		conn:    conn,
		client:  pb.NewFraudDetectionServiceClient(conn),
		timeout: timeout,
		logger:  logger,
	}, nil
}

// Check calls the fraud detection gRPC service to evaluate a transaction.
func (c *FraudClient) Check(ctx context.Context, req *ports.FraudCheckRequest) (*ports.FraudCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.CheckTransaction(ctx, &pb.CheckTransactionRequest{
		UserId:              req.UserID,
		SourceWalletId:      req.SourceWalletID,
		DestinationWalletId: req.DestinationWalletID,
		Amount:              req.Amount,
		Currency:            req.Currency,
		TransactionType:     req.TransactionType,
	})
	if err != nil {
		return nil, fmt.Errorf("fraud check gRPC call failed: %w", err)
	}

	return &ports.FraudCheckResult{
		Approved:  resp.Approved,
		RiskScore: resp.RiskScore,
		Reason:    resp.Reason,
	}, nil
}

// Close closes the gRPC connection.
func (c *FraudClient) Close() error {
	return c.conn.Close()
}
