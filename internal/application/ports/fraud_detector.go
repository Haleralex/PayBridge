package ports

import "context"

// FraudCheckRequest contains transaction details for fraud analysis.
type FraudCheckRequest struct {
	UserID              string
	SourceWalletID      string
	DestinationWalletID string
	Amount              string
	Currency            string
	TransactionType     string // TRANSFER, EXCHANGE
}

// FraudCheckResult contains the fraud detection decision.
type FraudCheckResult struct {
	Approved  bool
	RiskScore float64
	Reason    string
}

// FraudDetector provides fraud detection for financial transactions.
type FraudDetector interface {
	Check(ctx context.Context, req *FraudCheckRequest) (*FraudCheckResult, error)
}
