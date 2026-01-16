// Package postgres - UnitOfWork implementation для PostgreSQL.
//
// Unit of Work Pattern:
// - Управляет границами транзакций
// - Обеспечивает атомарность операций
// - Автоматический ROLLBACK при ошибках
// - Automatic COMMIT при успехе
//
// Usage:
//
//	err := uow.Execute(ctx, func(txCtx context.Context) error {
//	    // Все операции с репозиториями используют txCtx
//	    user, _ := userRepo.FindByID(txCtx, userID)
//	    wallet := entities.NewWallet(user.ID(), currency)
//	    walletRepo.Save(txCtx, wallet)
//	    return nil // COMMIT
//	    // return err // ROLLBACK
//	})
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Haleralex/wallethub/internal/application/ports"
)

// Compile-time check
var _ ports.UnitOfWork = (*UnitOfWork)(nil)
var _ ports.UnitOfWorkFactory = (*UnitOfWorkFactory)(nil)

// UnitOfWork реализует ports.UnitOfWork с PostgreSQL транзакциями.
//
// Thread-safe: использует connection pool.
// Transaction isolation: по умолчанию READ COMMITTED.
type UnitOfWork struct {
	pool *pgxpool.Pool
	opts pgx.TxOptions
}

// NewUnitOfWork создаёт новый UnitOfWork.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{
		pool: pool,
		opts: pgx.TxOptions{
			IsoLevel: pgx.ReadCommitted, // Default isolation level
		},
	}
}

// NewUnitOfWorkWithIsolation создаёт UnitOfWork с указанным уровнем изоляции.
//
// Уровни изоляции:
// - pgx.ReadCommitted (default): стандартный уровень, подходит для большинства случаев
// - pgx.RepeatableRead: гарантирует консистентность чтения в рамках транзакции
// - pgx.Serializable: полная изоляция, самая строгая (может вызвать retry при конфликтах)
func NewUnitOfWorkWithIsolation(pool *pgxpool.Pool, isolation pgx.TxIsoLevel) *UnitOfWork {
	return &UnitOfWork{
		pool: pool,
		opts: pgx.TxOptions{
			IsoLevel: isolation,
		},
	}
}

// Execute выполняет функцию внутри транзакции.
//
// Поведение:
// - Начинает транзакцию
// - Внедряет транзакцию в context
// - Выполняет fn с новым context
// - Если fn возвращает nil: COMMIT
// - Если fn возвращает error: ROLLBACK
// - Если panic: ROLLBACK + re-panic
//
// ВАЖНО: Все repositories внутри fn должны использовать переданный txCtx!
func (u *UnitOfWork) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Проверяем, есть ли уже транзакция в context (nested transaction)
	if hasTx(ctx) {
		// Уже внутри транзакции - просто выполняем функцию
		// (PostgreSQL не поддерживает true nested transactions, только savepoints)
		return fn(ctx)
	}

	// Начинаем новую транзакцию
	tx, err := u.pool.BeginTx(ctx, u.opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer для гарантированного cleanup
	defer func() {
		if r := recover(); r != nil {
			// Panic - откатываем и re-panic
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	// Внедряем транзакцию в context
	txCtx := injectTx(ctx, tx)

	// Выполняем бизнес-логику
	if err := fn(txCtx); err != nil {
		// Ошибка - откатываем
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	// Успех - коммитим
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ExecuteWithResult выполняет функцию и возвращает результат.
//
// Аналогичен Execute, но позволяет вернуть значение из транзакции.
// Полезно когда нужно вернуть созданную entity.
func (u *UnitOfWork) ExecuteWithResult(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	var result interface{}

	err := u.Execute(ctx, func(txCtx context.Context) error {
		var fnErr error
		result, fnErr = fn(txCtx)
		return fnErr
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// ExecuteWithRetry выполняет транзакцию с автоматическим retry при конфликтах.
//
// Полезно для optimistic locking и serialization failures.
// maxRetries: максимальное количество попыток (0 = без retry)
func (u *UnitOfWork) ExecuteWithRetry(ctx context.Context, maxRetries int, fn func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := u.Execute(ctx, fn)
		if err == nil {
			return nil // Успех
		}

		// Проверяем, можно ли retry
		if !isRetryableError(err) {
			return err // Не retryable - возвращаем сразу
		}

		lastErr = err
		// Можно добавить exponential backoff здесь
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// UnitOfWorkFactory создаёт новые UnitOfWork.
// Полезно когда нужны разные настройки транзакций.
type UnitOfWorkFactory struct {
	pool *pgxpool.Pool
}

// NewUnitOfWorkFactory создаёт фабрику UnitOfWork.
func NewUnitOfWorkFactory(pool *pgxpool.Pool) *UnitOfWorkFactory {
	return &UnitOfWorkFactory{pool: pool}
}

// New создаёт новый UnitOfWork с настройками по умолчанию.
func (f *UnitOfWorkFactory) New() ports.UnitOfWork {
	return NewUnitOfWork(f.pool)
}

// NewWithIsolation создаёт UnitOfWork с указанным уровнем изоляции.
func (f *UnitOfWorkFactory) NewWithIsolation(isolation pgx.TxIsoLevel) *UnitOfWork {
	return NewUnitOfWorkWithIsolation(f.pool, isolation)
}

// NewSerializable создаёт UnitOfWork с SERIALIZABLE изоляцией.
// Используйте для критических финансовых операций.
func (f *UnitOfWorkFactory) NewSerializable() *UnitOfWork {
	return NewUnitOfWorkWithIsolation(f.pool, pgx.Serializable)
}
