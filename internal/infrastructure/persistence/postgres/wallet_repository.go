// Package postgres - WalletRepository implementation with optimistic locking.
package postgres

import (
	"context"
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
var _ ports.WalletRepository = (*WalletRepository)(nil)

// WalletRepository реализует ports.WalletRepository.
//
// Особенности:
// - Optimistic Locking через balance_version
// - Money хранится как BIGINT (cents/satoshis)
// - Currency хранится как VARCHAR
type WalletRepository struct {
	pool *pgxpool.Pool
}

// NewWalletRepository создаёт новый WalletRepository.
func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

// getQuerier возвращает querier из context или pool.
func (r *WalletRepository) getQuerier(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// Save сохраняет кошелёк с проверкой версии (optimistic locking).
//
// Optimistic Locking:
// - При UPDATE проверяем, что balance_version не изменилась
// - Если изменилась - возвращаем ConcurrencyError
// - Клиент должен перечитать wallet и повторить операцию
func (r *WalletRepository) Save(ctx context.Context, wallet *entities.Wallet) error {
	q := r.getQuerier(ctx)

	// Для нового кошелька (version = 0) делаем INSERT
	if wallet.BalanceVersion() == 0 {
		return r.insert(ctx, q, wallet)
	}

	// Для существующего - UPDATE с проверкой версии
	return r.update(ctx, q, wallet)
}

// insert создаёт новый кошелёк.
func (r *WalletRepository) insert(ctx context.Context, q querier, wallet *entities.Wallet) error {
	query := `
		INSERT INTO wallets (
			id, user_id, currency, wallet_type, status,
			available_balance, pending_balance, balance_version,
			daily_limit, monthly_limit, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := q.Exec(ctx, query,
		wallet.ID(),
		wallet.UserID(),
		wallet.Currency().Code(),
		string(wallet.WalletType()),
		string(wallet.Status()),
		wallet.AvailableBalance().Cents(),
		wallet.PendingBalance().Cents(),
		wallet.BalanceVersion(),
		wallet.DailyLimit().Cents(),
		wallet.MonthlyLimit().Cents(),
		wallet.CreatedAt(),
		wallet.UpdatedAt(),
	)

	if err != nil {
		if isUniqueViolation(err, "wallets_user_currency_unique") {
			return domainErrors.NewBusinessRuleViolation(
				"WALLET_ALREADY_EXISTS",
				fmt.Sprintf("wallet for currency %s already exists", wallet.Currency().Code()),
				map[string]interface{}{
					"user_id":  wallet.UserID().String(),
					"currency": wallet.Currency().Code(),
				},
			)
		}
		if isForeignKeyViolation(err) {
			return domainErrors.NewDomainError("USER_NOT_FOUND", "user not found", err)
		}
		return fmt.Errorf("failed to insert wallet: %w", err)
	}

	return nil
}

// update обновляет кошелёк с optimistic locking.
func (r *WalletRepository) update(ctx context.Context, q querier, wallet *entities.Wallet) error {
	// Запрос с проверкой версии
	// balance_version = $8 - 1 означает "предыдущая версия до нашего изменения"
	query := `
		UPDATE wallets SET
			status = $2,
			available_balance = $3,
			pending_balance = $4,
			balance_version = $5,
			daily_limit = $6,
			monthly_limit = $7,
			updated_at = $8
		WHERE id = $1 AND balance_version = $9
	`

	// Текущая версия в domain entity уже увеличена после операции
	// Поэтому ожидаемая версия в БД = текущая - 1
	expectedVersion := wallet.BalanceVersion() - 1

	result, err := q.Exec(ctx, query,
		wallet.ID(),
		string(wallet.Status()),
		wallet.AvailableBalance().Cents(),
		wallet.PendingBalance().Cents(),
		wallet.BalanceVersion(),
		wallet.DailyLimit().Cents(),
		wallet.MonthlyLimit().Cents(),
		wallet.UpdatedAt(),
		expectedVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	// Проверяем, была ли обновлена запись
	if result.RowsAffected() == 0 {
		return domainErrors.NewConcurrencyError(
			"Wallet",
			wallet.ID().String(),
			fmt.Sprintf("wallet was modified by another transaction (expected version: %d)", expectedVersion),
		)
	}

	return nil
}

// FindByID загружает кошелёк по ID.
func (r *WalletRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, user_id, currency, wallet_type, status,
			   available_balance, pending_balance, balance_version,
			   daily_limit, monthly_limit, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`

	return r.scanWallet(q.QueryRow(ctx, query, id))
}

// FindByUserAndCurrency находит кошелёк пользователя для конкретной валюты.
func (r *WalletRepository) FindByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, user_id, currency, wallet_type, status,
			   available_balance, pending_balance, balance_version,
			   daily_limit, monthly_limit, created_at, updated_at
		FROM wallets
		WHERE user_id = $1 AND currency = $2
	`

	return r.scanWallet(q.QueryRow(ctx, query, userID, currency.Code()))
}

// FindByUserID возвращает все кошельки пользователя.
func (r *WalletRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Wallet, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, user_id, currency, wallet_type, status,
			   available_balance, pending_balance, balance_version,
			   daily_limit, monthly_limit, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		ORDER BY created_at ASC
	`

	rows, err := q.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find wallets by user: %w", err)
	}
	defer rows.Close()

	return r.scanWallets(rows)
}

// ExistsByUserAndCurrency проверяет существование кошелька.
func (r *WalletRepository) ExistsByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error) {
	q := r.getQuerier(ctx)

	query := `SELECT EXISTS(SELECT 1 FROM wallets WHERE user_id = $1 AND currency = $2)`

	var exists bool
	err := q.QueryRow(ctx, query, userID, currency.Code()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check wallet existence: %w", err)
	}

	return exists, nil
}

// List возвращает кошельки с фильтрацией и пагинацией.
func (r *WalletRepository) List(ctx context.Context, filter ports.WalletFilter, offset, limit int) ([]*entities.Wallet, error) {
	q := r.getQuerier(ctx)

	// Строим динамический запрос с фильтрами
	query := `
		SELECT id, user_id, currency, wallet_type, status,
			   available_balance, pending_balance, balance_version,
			   daily_limit, monthly_limit, created_at, updated_at
		FROM wallets
		WHERE 1=1
	`

	args := []interface{}{}
	argNum := 1

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argNum)
		args = append(args, *filter.UserID)
		argNum++
	}

	if filter.Currency != nil {
		query += fmt.Sprintf(" AND currency = $%d", argNum)
		args = append(args, filter.Currency.Code())
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(*filter.Status))
		argNum++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC OFFSET $%d LIMIT $%d", argNum, argNum+1)
	args = append(args, offset, limit)

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}
	defer rows.Close()

	return r.scanWallets(rows)
}

// scanWallet сканирует одну строку в Wallet entity.
func (r *WalletRepository) scanWallet(row pgx.Row) (*entities.Wallet, error) {
	var (
		id, userID                                 uuid.UUID
		currencyCode, walletTypeStr, statusStr     string
		availableBalance, pendingBalance           int64
		balanceVersion                             int64
		dailyLimitCents, monthlyLimitCents         int64
		createdAt, updatedAt                       time.Time
	)

	err := row.Scan(
		&id,
		&userID,
		&currencyCode,
		&walletTypeStr,
		&statusStr,
		&availableBalance,
		&pendingBalance,
		&balanceVersion,
		&dailyLimitCents,
		&monthlyLimitCents,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErrors.ErrEntityNotFound
		}
		return nil, fmt.Errorf("failed to scan wallet: %w", err)
	}

	// Reconstruct value objects
	currency, err := valueobjects.NewCurrency(currencyCode)
	if err != nil {
		return nil, fmt.Errorf("invalid currency in database: %w", err)
	}

	// Конвертируем cents обратно в Money
	available, err := valueobjects.NewMoneyFromCents(availableBalance, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to convert available balance: %w", err)
	}

	pending, err := valueobjects.NewMoneyFromCents(pendingBalance, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pending balance: %w", err)
	}

	dailyLimit, err := valueobjects.NewMoneyFromCents(dailyLimitCents, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to convert daily limit: %w", err)
	}

	monthlyLimit, err := valueobjects.NewMoneyFromCents(monthlyLimitCents, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to convert monthly limit: %w", err)
	}

	// Reconstruct domain entity
	wallet := entities.ReconstructWallet(
		id,
		userID,
		currency,
		entities.WalletType(walletTypeStr),
		entities.WalletStatus(statusStr),
		available,
		pending,
		balanceVersion,
		dailyLimit,
		monthlyLimit,
		createdAt,
		updatedAt,
	)

	return wallet, nil
}

// scanWallets сканирует несколько строк в список Wallet entities.
func (r *WalletRepository) scanWallets(rows pgx.Rows) ([]*entities.Wallet, error) {
	var wallets []*entities.Wallet

	for rows.Next() {
		var (
			id, userID                                 uuid.UUID
			currencyCode, walletTypeStr, statusStr     string
			availableBalance, pendingBalance           int64
			balanceVersion                             int64
			dailyLimitCents, monthlyLimitCents         int64
			createdAt, updatedAt                       time.Time
		)

		err := rows.Scan(
			&id,
			&userID,
			&currencyCode,
			&walletTypeStr,
			&statusStr,
			&availableBalance,
			&pendingBalance,
			&balanceVersion,
			&dailyLimitCents,
			&monthlyLimitCents,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan wallet row: %w", err)
		}

		currency, err := valueobjects.NewCurrency(currencyCode)
		if err != nil {
			return nil, fmt.Errorf("invalid currency in database: %w", err)
		}

		available, _ := valueobjects.NewMoneyFromCents(availableBalance, currency)
		pending, _ := valueobjects.NewMoneyFromCents(pendingBalance, currency)
		dailyLimit, _ := valueobjects.NewMoneyFromCents(dailyLimitCents, currency)
		monthlyLimit, _ := valueobjects.NewMoneyFromCents(monthlyLimitCents, currency)

		wallet := entities.ReconstructWallet(
			id,
			userID,
			currency,
			entities.WalletType(walletTypeStr),
			entities.WalletStatus(statusStr),
			available,
			pending,
			balanceVersion,
			dailyLimit,
			monthlyLimit,
			createdAt,
			updatedAt,
		)

		wallets = append(wallets, wallet)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating wallet rows: %w", err)
	}

	return wallets, nil
}
