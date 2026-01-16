// Package postgres - вспомогательные функции для работы с PostgreSQL.
package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// txKey - ключ для хранения транзакции в context.
type txKey struct{}

// injectTx добавляет транзакцию в context.
// Используется UnitOfWork для передачи транзакции в repositories.
func injectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// extractTx извлекает транзакцию из context.
// Возвращает nil если транзакции нет.
func extractTx(ctx context.Context) pgx.Tx {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return nil
	}
	return tx
}

// hasTx проверяет наличие транзакции в context.
func hasTx(ctx context.Context) bool {
	return extractTx(ctx) != nil
}

// PostgreSQL error codes (из спецификации)
const (
	// Constraint violations
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
	pgNotNullViolation    = "23502"

	// Serialization failures (for optimistic locking)
	pgSerializationFailure = "40001"
	pgDeadlockDetected     = "40P01"
)

// isPgError проверяет, является ли ошибка PostgreSQL ошибкой с определённым кодом.
func isPgError(err error, code string) bool {
	if err == nil {
		return false
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		return false
	}

	return pgErr.Code == code
}

// isUniqueViolation проверяет, является ли ошибка нарушением UNIQUE constraint.
// constraintName - опциональное имя constraint для проверки.
func isUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		return false
	}

	if pgErr.Code != pgUniqueViolation {
		return false
	}

	// Если указано имя constraint, проверяем его
	if constraintName != "" {
		return strings.Contains(pgErr.ConstraintName, constraintName)
	}

	return true
}

// isForeignKeyViolation проверяет нарушение foreign key constraint.
func isForeignKeyViolation(err error) bool {
	return isPgError(err, pgForeignKeyViolation)
}

// isSerializationFailure проверяет ошибку сериализации (для retry).
func isSerializationFailure(err error) bool {
	return isPgError(err, pgSerializationFailure) || isPgError(err, pgDeadlockDetected)
}

// isNotNullViolation проверяет нарушение NOT NULL constraint.
func isNotNullViolation(err error) bool {
	return isPgError(err, pgNotNullViolation)
}

// isCheckViolation проверяет нарушение CHECK constraint.
func isCheckViolation(err error) bool {
	return isPgError(err, pgCheckViolation)
}

// isRetryableError проверяет, можно ли повторить операцию.
// Retryable: deadlock, serialization failure, connection errors.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Serialization failures можно retry
	if isSerializationFailure(err) {
		return true
	}

	// Connection errors часто можно retry
	pgErr, ok := err.(*pgconn.PgError)
	if ok {
		// Class 08 - Connection Exception
		return strings.HasPrefix(pgErr.Code, "08")
	}

	return false
}
