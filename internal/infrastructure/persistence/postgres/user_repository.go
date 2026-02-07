// Package postgres - UserRepository implementation.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	domainErrors "github.com/Haleralex/wallethub/internal/domain/errors"
)

// Compile-time check: UserRepository implements ports.UserRepository
var _ ports.UserRepository = (*UserRepository)(nil)

// UserRepository реализует ports.UserRepository с использованием PostgreSQL.
//
// Thread-safe: использует connection pool.
// Transaction-aware: автоматически использует транзакцию из context если есть.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository создаёт новый UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// querier - абстракция для выполнения запросов.
// Позволяет использовать как pool, так и transaction.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// getQuerier возвращает querier из context (transaction) или pool.
func (r *UserRepository) getQuerier(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// Save сохраняет пользователя (INSERT или UPDATE).
// Использует UPSERT для идемпотентности.
func (r *UserRepository) Save(ctx context.Context, user *entities.User) error {
	q := r.getQuerier(ctx)

	query := `
		INSERT INTO users (id, email, full_name, kyc_status, telegram_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			full_name = EXCLUDED.full_name,
			kyc_status = EXCLUDED.kyc_status,
			telegram_id = EXCLUDED.telegram_id,
			updated_at = EXCLUDED.updated_at
	`

	_, err := q.Exec(ctx, query,
		user.ID(),
		user.Email(),
		user.FullName(),
		string(user.KYCStatus()),
		user.TelegramID(),
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if err != nil {
		// Проверяем на duplicate email (UNIQUE constraint violation)
		if isUniqueViolation(err, "users_email_unique") {
			return domainErrors.NewBusinessRuleViolation(
				"EMAIL_ALREADY_EXISTS",
				fmt.Sprintf("user with email %s already exists", user.Email()),
				map[string]interface{}{"email": user.Email()},
			)
		}
		return fmt.Errorf("failed to save user: %w", err)
	}

	return nil
}

// scanUser сканирует строку в domain entity User.
func scanUser(scanner interface{ Scan(dest ...any) error }) (*entities.User, error) {
	var (
		userID               uuid.UUID
		email                string
		fullName             string
		kycStatus            string
		telegramID           *int64
		createdAt, updatedAt time.Time
	)

	err := scanner.Scan(
		&userID,
		&email,
		&fullName,
		&kycStatus,
		&telegramID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return entities.ReconstructUser(
		userID, email, fullName,
		entities.KYCStatus(kycStatus),
		telegramID,
		createdAt, updatedAt,
	), nil
}

const userColumns = `id, email, full_name, kyc_status, telegram_id, created_at, updated_at`

// FindByID загружает пользователя по ID.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	q := r.getQuerier(ctx)

	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`

	user, err := scanUser(q.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErrors.ErrEntityNotFound
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return user, nil
}

// FindByEmail загружает пользователя по email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	q := r.getQuerier(ctx)

	query := `SELECT ` + userColumns + ` FROM users WHERE email = $1`

	user, err := scanUser(q.QueryRow(ctx, query, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErrors.ErrEntityNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return user, nil
}

// ExistsByEmail проверяет существование пользователя по email.
// Оптимизированный запрос без загрузки всех полей.
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	q := r.getQuerier(ctx)

	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`

	var exists bool
	err := q.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}

	return exists, nil
}

// FindByTelegramID загружает пользователя по Telegram ID.
func (r *UserRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*entities.User, error) {
	q := r.getQuerier(ctx)

	query := `SELECT ` + userColumns + ` FROM users WHERE telegram_id = $1`

	user, err := scanUser(q.QueryRow(ctx, query, telegramID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErrors.ErrEntityNotFound
		}
		return nil, fmt.Errorf("failed to find user by telegram_id: %w", err)
	}

	return user, nil
}

// List возвращает список пользователей с пагинацией.
func (r *UserRepository) List(ctx context.Context, offset, limit int) ([]*entities.User, error) {
	q := r.getQuerier(ctx)

	query := `SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC OFFSET $1 LIMIT $2`

	rows, err := q.Query(ctx, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*entities.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}
