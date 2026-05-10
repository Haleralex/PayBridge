package grpc

import (
	"context"

	"github.com/Haleralex/wallethub/internal/application/ports"
)

// NoOpFraudDetector always approves transactions. Used as fallback when
// the fraud detection service is unavailable or disabled.
type NoOpFraudDetector struct{}

func NewNoOpFraudDetector() *NoOpFraudDetector {
	return &NoOpFraudDetector{}
}

func (d *NoOpFraudDetector) Check(_ context.Context, _ *ports.FraudCheckRequest) (*ports.FraudCheckResult, error) {
	return &ports.FraudCheckResult{
		Approved:  true,
		RiskScore: 0.0,
		Reason:    "",
	}, nil
}
