// Package postgres - TransactionRepository implementation with idempotency support.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/wallethub/internal/application/ports"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// Compile-time check
var _ ports.TransactionRepository = (*TransactionRepository)(nil)

// TransactionRepository реализует ports.TransactionRepository.
//
// Ключевые особенности:
// - Idempotency через unique idempotency_key
// - Metadata хранится как JSONB
// - Amount хранится как BIGINT (cents/satoshis)
type TransactionRepository struct {
	pool *pgxpool.Pool
}

// NewTransactionRepository создаёт новый TransactionRepository.
func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

// getQuerier возвращает querier из context или pool.
func (r *TransactionRepository) getQuerier(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// Save сохраняет транзакцию.
// Для новых транзакций - INSERT, для существующих - UPDATE.
func (r *TransactionRepository) Save(ctx context.Context, tx *entities.Transaction) error {
	q := r.getQuerier(ctx)

	// Сериализуем metadata в JSON
	metadataJSON, err := json.Marshal(tx.Metadata())
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO transactions (
			id, wallet_id, idempotency_key, transaction_type, status,
			amount, currency, destination_wallet_id, external_reference,
			description, metadata, failure_reason, retry_count,
			created_at, updated_at, processed_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			external_reference = EXCLUDED.external_reference,
			description = EXCLUDED.description,
			metadata = EXCLUDED.metadata,
			failure_reason = EXCLUDED.failure_reason,
			retry_count = EXCLUDED.retry_count,
			updated_at = EXCLUDED.updated_at,
			processed_at = EXCLUDED.processed_at,
			completed_at = EXCLUDED.completed_at
	`

	_, err = q.Exec(ctx, query,
		tx.ID(),
		tx.WalletID(),
		tx.IdempotencyKey(),
		string(tx.Type()),
		string(tx.Status()),
		tx.Amount().Cents(),
		tx.Amount().Currency().Code(),
		tx.DestinationWalletID(),
		tx.ExternalReference(),
		tx.Description(),
		metadataJSON,
		tx.FailureReason(),
		tx.RetryCount(),
		tx.CreatedAt(),
		tx.UpdatedAt(),
		tx.ProcessedAt(),
		tx.CompletedAt(),
	)

	if err != nil {
		// Проверяем на duplicate idempotency key
		if isUniqueViolation(err, "transactions_idempotency_key_unique") {
			return domainErrors.ErrDuplicateTransaction
		}
		if isForeignKeyViolation(err) {
			return domainErrors.NewDomainError("WALLET_NOT_FOUND", "wallet not found", err)
		}
		return fmt.Errorf("failed to save transaction: %w", err)
	}

	return nil
}

// FindByID загружает транзакцию по ID.
func (r *TransactionRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, wallet_id, idempotency_key, transaction_type, status,
			   amount, currency, destination_wallet_id, external_reference,
			   description, metadata, failure_reason, retry_count,
			   created_at, updated_at, processed_at, completed_at
		FROM transactions
		WHERE id = $1
	`

	return r.scanTransaction(q.QueryRow(ctx, query, id))
}

// FindByIdempotencyKey находит транзакцию по ключу идемпотентности.
// Критично для предотвращения дубликатов!
func (r *TransactionRepository) FindByIdempotencyKey(ctx context.Context, key string) (*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, wallet_id, idempotency_key, transaction_type, status,
			   amount, currency, destination_wallet_id, external_reference,
			   description, metadata, failure_reason, retry_count,
			   created_at, updated_at, processed_at, completed_at
		FROM transactions
		WHERE idempotency_key = $1
	`

	tx, err := r.scanTransaction(q.QueryRow(ctx, query, key))
	if err != nil {
		if domainErrors.IsNotFound(err) {
			return nil, nil // Not found is expected - return nil without error
		}
		return nil, err
	}

	return tx, nil
}

// FindByWalletID возвращает транзакции кошелька с пагинацией.
func (r *TransactionRepository) FindByWalletID(ctx context.Context, walletID uuid.UUID, offset, limit int) ([]*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, wallet_id, idempotency_key, transaction_type, status,
			   amount, currency, destination_wallet_id, external_reference,
			   description, metadata, failure_reason, retry_count,
			   created_at, updated_at, processed_at, completed_at
		FROM transactions
		WHERE wallet_id = $1
		ORDER BY created_at DESC
		OFFSET $2 LIMIT $3
	`

	rows, err := q.Query(ctx, query, walletID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions by wallet: %w", err)
	}
	defer rows.Close()

	return r.scanTransactions(rows)
}

// FindPendingByWallet возвращает pending транзакции кошелька.
func (r *TransactionRepository) FindPendingByWallet(ctx context.Context, walletID uuid.UUID) ([]*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, wallet_id, idempotency_key, transaction_type, status,
			   amount, currency, destination_wallet_id, external_reference,
			   description, metadata, failure_reason, retry_count,
			   created_at, updated_at, processed_at, completed_at
		FROM transactions
		WHERE wallet_id = $1 AND status = 'PENDING'
		ORDER BY created_at ASC
	`

	rows, err := q.Query(ctx, query, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to find pending transactions: %w", err)
	}
	defer rows.Close()

	return r.scanTransactions(rows)
}

// FindFailedRetryable возвращает failed транзакции, которые можно повторить.
func (r *TransactionRepository) FindFailedRetryable(ctx context.Context, maxRetries int, limit int) ([]*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, wallet_id, idempotency_key, transaction_type, status,
			   amount, currency, destination_wallet_id, external_reference,
			   description, metadata, failure_reason, retry_count,
			   created_at, updated_at, processed_at, completed_at
		FROM transactions
		WHERE status = 'FAILED' AND retry_count < $1
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := q.Query(ctx, query, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find retryable transactions: %w", err)
	}
	defer rows.Close()

	return r.scanTransactions(rows)
}

// List возвращает транзакции с фильтрацией и пагинацией.
func (r *TransactionRepository) List(ctx context.Context, filter ports.TransactionFilter, offset, limit int) ([]*entities.Transaction, error) {
	q := r.getQuerier(ctx)

	// Строим динамический запрос
	query := `
		SELECT t.id, t.wallet_id, t.idempotency_key, t.transaction_type, t.status,
			   t.amount, t.currency, t.destination_wallet_id, t.external_reference,
			   t.description, t.metadata, t.failure_reason, t.retry_count,
			   t.created_at, t.updated_at, t.processed_at, t.completed_at
		FROM transactions t
	`

	// Для фильтра по user_id нужен JOIN
	if filter.UserID != nil {
		query += " JOIN wallets w ON t.wallet_id = w.id"
	}

	query += " WHERE 1=1"

	args := []interface{}{}
	argNum := 1

	if filter.WalletID != nil {
		query += fmt.Sprintf(" AND t.wallet_id = $%d", argNum)
		args = append(args, *filter.WalletID)
		argNum++
	}

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND w.user_id = $%d", argNum)
		args = append(args, *filter.UserID)
		argNum++
	}

	if filter.Type != nil {
		query += fmt.Sprintf(" AND t.transaction_type = $%d", argNum)
		args = append(args, string(*filter.Type))
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND t.status = $%d", argNum)
		args = append(args, string(*filter.Status))
		argNum++
	}

	query += fmt.Sprintf(" ORDER BY t.created_at DESC OFFSET $%d LIMIT $%d", argNum, argNum+1)
	args = append(args, offset, limit)

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer rows.Close()

	return r.scanTransactions(rows)
}

// scanTransaction сканирует одну строку в Transaction entity.
func (r *TransactionRepository) scanTransaction(row pgx.Row) (*entities.Transaction, error) {
	var (
		id, walletID                             uuid.UUID
		idempotencyKey, txTypeStr, statusStr     string
		amountCents                              int64
		currencyCode                             string
		destinationWalletID                      *uuid.UUID
		externalReference, description           *string
		metadataJSON                             []byte
		failureReason                            *string
		retryCount                               int
		createdAt, updatedAt                     time.Time
		processedAt, completedAt                 *time.Time
	)

	err := row.Scan(
		&id,
		&walletID,
		&idempotencyKey,
		&txTypeStr,
		&statusStr,
		&amountCents,
		&currencyCode,
		&destinationWalletID,
		&externalReference,
		&description,
		&metadataJSON,
		&failureReason,
		&retryCount,
		&createdAt,
		&updatedAt,
		&processedAt,
		&completedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErrors.ErrEntityNotFound
		}
		return nil, fmt.Errorf("failed to scan transaction: %w", err)
	}

	// Reconstruct value objects
	currency, err := valueobjects.NewCurrency(currencyCode)
	if err != nil {
		return nil, fmt.Errorf("invalid currency in database: %w", err)
	}

	amount, err := valueobjects.NewMoneyFromCents(amountCents, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount: %w", err)
	}

	// Handle nullable strings
	extRef := ""
	if externalReference != nil {
		extRef = *externalReference
	}

	desc := ""
	if description != nil {
		desc = *description
	}

	failReason := ""
	if failureReason != nil {
		failReason = *failureReason
	}

	// Reconstruct domain entity
	tx, err := entities.ReconstructTransaction(
		id,
		walletID,
		idempotencyKey,
		entities.TransactionType(txTypeStr),
		entities.TransactionStatus(statusStr),
		amount,
		destinationWalletID,
		extRef,
		desc,
		metadataJSON,
		failReason,
		retryCount,
		createdAt,
		updatedAt,
		processedAt,
		completedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct transaction: %w", err)
	}

	return tx, nil
}

// scanTransactions сканирует несколько строк.
func (r *TransactionRepository) scanTransactions(rows pgx.Rows) ([]*entities.Transaction, error) {
	var transactions []*entities.Transaction

	for rows.Next() {
		var (
			id, walletID                             uuid.UUID
			idempotencyKey, txTypeStr, statusStr     string
			amountCents                              int64
			currencyCode                             string
			destinationWalletID                      *uuid.UUID
			externalReference, description           *string
			metadataJSON                             []byte
			failureReason                            *string
			retryCount                               int
			createdAt, updatedAt                     time.Time
			processedAt, completedAt                 *time.Time
		)

		err := rows.Scan(
			&id,
			&walletID,
			&idempotencyKey,
			&txTypeStr,
			&statusStr,
			&amountCents,
			&currencyCode,
			&destinationWalletID,
			&externalReference,
			&description,
			&metadataJSON,
			&failureReason,
			&retryCount,
			&createdAt,
			&updatedAt,
			&processedAt,
			&completedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}

		currency, _ := valueobjects.NewCurrency(currencyCode)
		amount, _ := valueobjects.NewMoneyFromCents(amountCents, currency)

		extRef := ""
		if externalReference != nil {
			extRef = *externalReference
		}

		desc := ""
		if description != nil {
			desc = *description
		}

		failReason := ""
		if failureReason != nil {
			failReason = *failureReason
		}

		tx, err := entities.ReconstructTransaction(
			id,
			walletID,
			idempotencyKey,
			entities.TransactionType(txTypeStr),
			entities.TransactionStatus(statusStr),
			amount,
			destinationWalletID,
			extRef,
			desc,
			metadataJSON,
			failReason,
			retryCount,
			createdAt,
			updatedAt,
			processedAt,
			completedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct transaction: %w", err)
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transaction rows: %w", err)
	}

	return transactions, nil
}
