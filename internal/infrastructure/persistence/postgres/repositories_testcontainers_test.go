// Package postgres - интеграционные тесты для PostgreSQL repositories с testcontainers.
//
// Запуск тестов:
//
//	go test ./internal/infrastructure/persistence/postgres/...
//
// Требования:
//   - Docker Desktop запущен
//   - testcontainers-go установлен
package postgres

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domerrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// ============================================
// Test Helpers
// ============================================

// testContainer хранит контейнер и pool для тестов.
type testContainer struct {
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
}

// Shared container for all tests (performance optimization)
var sharedTestContainer *testContainer

// setupSharedTestDB создаёт или возвращает переиспользуемый PostgreSQL контейнер.
// Оптимизация: один контейнер для всех тестов вместо создания нового для каждого.
func setupSharedTestDB(t *testing.T) *testContainer {
	if sharedTestContainer != nil {
		// Очищаем данные между тестами
		cleanupTables(t, sharedTestContainer.pool)
		return sharedTestContainer
	}

	ctx := context.Background()

	// Путь к миграциям относительно текущего файла
	migrationsPath := filepath.Join("..", "migrations")

	// Создаём PostgreSQL контейнер
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts(
			filepath.Join(migrationsPath, "001_create_users_up.sql"),
			filepath.Join(migrationsPath, "002_create_wallets_up.sql"),
			filepath.Join(migrationsPath, "003_create_transactions_up.sql"),
			filepath.Join(migrationsPath, "004_create_outbox_up.sql"),
		),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	// Получаем connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Создаём connection pool
	poolConfig, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)

	// Проверяем подключение
	err = pool.Ping(ctx)
	require.NoError(t, err)

	sharedTestContainer = &testContainer{
		container: container,
		pool:      pool,
	}

	return sharedTestContainer
}

// setupTestDB создаёт временный PostgreSQL контейнер для тестов.
// Используется для тестов требующих полной изоляции.
func setupTestDB(t *testing.T) *testContainer {
	ctx := context.Background()

	// Путь к миграциям относительно текущего файла
	migrationsPath := filepath.Join("..", "migrations")

	// Создаём PostgreSQL контейнер
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts(
			filepath.Join(migrationsPath, "001_create_users_up.sql"),
			filepath.Join(migrationsPath, "002_create_wallets_up.sql"),
			filepath.Join(migrationsPath, "003_create_transactions_up.sql"),
			filepath.Join(migrationsPath, "004_create_outbox_up.sql"),
		),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	// Получаем connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Создаём connection pool
	poolConfig, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	// Проверяем подключение
	err = pool.Ping(ctx)
	require.NoError(t, err)

	return &testContainer{
		container: container,
		pool:      pool,
	}
}

// cleanupTables очищает все таблицы для следующего теста.
func cleanupTables(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	// Важно: очищаем в правильном порядке из-за foreign keys
	tables := []string{"outbox_events", "transactions", "wallets", "users"}
	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Logf("Warning: failed to cleanup %s: %v", table, err)
		}
	}
}

// ============================================
// UserRepository Tests
// ============================================

func TestUserRepository_Integration_Save(t *testing.T) {
	tc := setupSharedTestDB(t)

	repo := NewUserRepository(tc.pool)
	ctx := context.Background()

	t.Run("SaveNewUser", func(t *testing.T) {
		user, err := entities.NewUser("test@example.com", "Test User")
		require.NoError(t, err)

		err = repo.Save(ctx, user)
		assert.NoError(t, err)

		// Verify saved
		loaded, err := repo.FindByID(ctx, user.ID())
		require.NoError(t, err)
		assert.Equal(t, user.Email(), loaded.Email())
		assert.Equal(t, user.FullName(), loaded.FullName())
		assert.Equal(t, "UNVERIFIED", string(loaded.KYCStatus()))
	})

	t.Run("UpdateExistingUser", func(t *testing.T) {
		user, _ := entities.NewUser("update@example.com", "Original Name")
		repo.Save(ctx, user)

		// Update KYC status: UNVERIFIED → PENDING → VERIFIED
		err := user.StartKYCVerification()
		require.NoError(t, err)
		err = user.ApproveKYC()
		require.NoError(t, err)

		err = repo.Save(ctx, user)
		assert.NoError(t, err)

		// Verify update
		loaded, _ := repo.FindByID(ctx, user.ID())
		assert.Equal(t, "VERIFIED", string(loaded.KYCStatus()))
	})

	t.Run("DuplicateEmail", func(t *testing.T) {
		user1, _ := entities.NewUser("duplicate@example.com", "User 1")
		repo.Save(ctx, user1)

		user2, _ := entities.NewUser("duplicate@example.com", "User 2")
		err := repo.Save(ctx, user2)

		assert.Error(t, err)
		assert.True(t, domerrors.IsBusinessRuleViolation(err))
	})
}

func TestUserRepository_Integration_FindByID(t *testing.T) {
	tc := setupSharedTestDB(t)

	repo := NewUserRepository(tc.pool)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user, _ := entities.NewUser("find@example.com", "Find User")
		repo.Save(ctx, user)

		found, err := repo.FindByID(ctx, user.ID())

		assert.NoError(t, err)
		assert.Equal(t, user.ID(), found.ID())
		assert.Equal(t, user.Email(), found.Email())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.FindByID(ctx, uuid.New())

		assert.Error(t, err)
		assert.True(t, domerrors.IsNotFound(err))
	})
}

func TestUserRepository_Integration_FindByEmail(t *testing.T) {
	tc := setupSharedTestDB(t)

	repo := NewUserRepository(tc.pool)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user, _ := entities.NewUser("email@example.com", "Email User")
		repo.Save(ctx, user)

		found, err := repo.FindByEmail(ctx, "email@example.com")

		assert.NoError(t, err)
		assert.Equal(t, user.ID(), found.ID())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.FindByEmail(ctx, "nonexistent@example.com")

		assert.Error(t, err)
		assert.True(t, domerrors.IsNotFound(err))
	})
}

func TestUserRepository_Integration_ExistsByEmail(t *testing.T) {
	tc := setupSharedTestDB(t)

	repo := NewUserRepository(tc.pool)
	ctx := context.Background()

	t.Run("Exists", func(t *testing.T) {
		user, _ := entities.NewUser("exists@example.com", "Exists User")
		repo.Save(ctx, user)

		exists, err := repo.ExistsByEmail(ctx, "exists@example.com")

		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("DoesNotExist", func(t *testing.T) {
		exists, err := repo.ExistsByEmail(ctx, "notexists@example.com")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// ============================================
// WalletRepository Tests
// ============================================

func TestWalletRepository_Integration_Save(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	ctx := context.Background()

	// Create user first
	user, _ := entities.NewUser("wallet@example.com", "Wallet User")
	userRepo.Save(ctx, user)

	t.Run("SaveNewWallet", func(t *testing.T) {
		currency, _ := valueobjects.NewCurrency("USD")
		wallet, err := entities.NewWallet(user.ID(), currency)
		require.NoError(t, err)

		err = walletRepo.Save(ctx, wallet)
		assert.NoError(t, err)

		// Verify
		loaded, err := walletRepo.FindByID(ctx, wallet.ID())
		require.NoError(t, err)
		assert.Equal(t, wallet.ID(), loaded.ID())
		assert.Equal(t, user.ID(), loaded.UserID())
		assert.Equal(t, "USD", loaded.Currency().Code())
	})

	t.Run("UpdateWalletBalance", func(t *testing.T) {
		currency, _ := valueobjects.NewCurrency("EUR")
		wallet, _ := entities.NewWallet(user.ID(), currency)
		walletRepo.Save(ctx, wallet)

		// Credit wallet
		amount, _ := valueobjects.NewMoney("100.50", currency)
		wallet.Credit(amount)

		err := walletRepo.Save(ctx, wallet)
		assert.NoError(t, err)

		// Verify balance
		loaded, _ := walletRepo.FindByID(ctx, wallet.ID())
		assert.Equal(t, "100.50 EUR", loaded.AvailableBalance().String())
	})

	t.Run("OptimisticLockingConflict", func(t *testing.T) {
		currency, _ := valueobjects.NewCurrency("BTC")
		wallet, _ := entities.NewWallet(user.ID(), currency)
		walletRepo.Save(ctx, wallet)

		// Load wallet twice
		wallet1, _ := walletRepo.FindByID(ctx, wallet.ID())
		wallet2, _ := walletRepo.FindByID(ctx, wallet.ID())

		// Update wallet1
		amount1, _ := valueobjects.NewMoney("1.0", currency)
		wallet1.Credit(amount1)
		walletRepo.Save(ctx, wallet1)

		// Try to update wallet2 (stale version)
		amount2, _ := valueobjects.NewMoney("2.0", currency)
		wallet2.Credit(amount2)
		err := walletRepo.Save(ctx, wallet2)

		assert.Error(t, err)
		assert.True(t, domerrors.IsConcurrencyError(err))
	})
}

func TestWalletRepository_Integration_FindByUserAndCurrency(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	ctx := context.Background()

	user, _ := entities.NewUser("multi@example.com", "Multi Wallet User")
	userRepo.Save(ctx, user)

	t.Run("Success", func(t *testing.T) {
		usd, _ := valueobjects.NewCurrency("USD")
		wallet, _ := entities.NewWallet(user.ID(), usd)
		walletRepo.Save(ctx, wallet)

		found, err := walletRepo.FindByUserAndCurrency(ctx, user.ID(), usd)

		assert.NoError(t, err)
		assert.Equal(t, wallet.ID(), found.ID())
	})

	t.Run("NotFound", func(t *testing.T) {
		eur, _ := valueobjects.NewCurrency("EUR")

		_, err := walletRepo.FindByUserAndCurrency(ctx, user.ID(), eur)

		assert.Error(t, err)
		assert.True(t, domerrors.IsNotFound(err))
	})
}

func TestWalletRepository_Integration_FindByUserID(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	ctx := context.Background()

	user, _ := entities.NewUser("list@example.com", "List User")
	userRepo.Save(ctx, user)

	// Create multiple wallets
	currencies := []string{"USD", "EUR", "BTC"}
	for _, code := range currencies {
		currency, _ := valueobjects.NewCurrency(code)
		wallet, _ := entities.NewWallet(user.ID(), currency)
		walletRepo.Save(ctx, wallet)
	}

	wallets, err := walletRepo.FindByUserID(ctx, user.ID())

	assert.NoError(t, err)
	assert.Len(t, wallets, 3)
}

// ============================================
// TransactionRepository Tests
// ============================================

func TestTransactionRepository_Integration_Save(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	txRepo := NewTransactionRepository(tc.pool)
	ctx := context.Background()

	// Setup: user + wallet
	user, _ := entities.NewUser("tx@example.com", "TX User")
	userRepo.Save(ctx, user)

	currency, _ := valueobjects.NewCurrency("USD")
	wallet, _ := entities.NewWallet(user.ID(), currency)
	walletRepo.Save(ctx, wallet)

	t.Run("SaveNewTransaction", func(t *testing.T) {
		amount, _ := valueobjects.NewMoney("50.00", currency)
		tx, err := entities.NewTransaction(
			wallet.ID(),
			uuid.New().String(),
			entities.TransactionTypeDeposit,
			amount,
			"Test deposit",
		)
		require.NoError(t, err)

		err = txRepo.Save(ctx, tx)
		assert.NoError(t, err)

		// Verify
		loaded, err := txRepo.FindByID(ctx, tx.ID())
		require.NoError(t, err)
		assert.Equal(t, tx.ID(), loaded.ID())
		assert.Equal(t, "PENDING", string(loaded.Status()))
	})

	t.Run("UpdateTransactionStatus", func(t *testing.T) {
		amount, _ := valueobjects.NewMoney("100.00", currency)
		tx, _ := entities.NewTransaction(
			wallet.ID(),
			uuid.New().String(),
			entities.TransactionTypeDeposit,
			amount,
			"Complete test",
		)
		txRepo.Save(ctx, tx)

		// Complete transaction (start processing first)
		tx.StartProcessing()
		tx.MarkCompleted()
		err := txRepo.Save(ctx, tx)
		assert.NoError(t, err)

		// Verify status
		loaded, _ := txRepo.FindByID(ctx, tx.ID())
		assert.Equal(t, "COMPLETED", string(loaded.Status()))
		assert.NotNil(t, loaded.CompletedAt())
	})
}

func TestTransactionRepository_Integration_FindByIdempotencyKey(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	txRepo := NewTransactionRepository(tc.pool)
	ctx := context.Background()

	// Setup
	user, _ := entities.NewUser("idem@example.com", "Idem User")
	userRepo.Save(ctx, user)

	currency, _ := valueobjects.NewCurrency("USD")
	wallet, _ := entities.NewWallet(user.ID(), currency)
	walletRepo.Save(ctx, wallet)

	idempotencyKey := uuid.New().String()
	amount, _ := valueobjects.NewMoney("25.00", currency)
	tx, _ := entities.NewTransaction(wallet.ID(), idempotencyKey, entities.TransactionTypeDeposit, amount, "Idempotent")
	txRepo.Save(ctx, tx)

	t.Run("Success", func(t *testing.T) {
		found, err := txRepo.FindByIdempotencyKey(ctx, idempotencyKey)

		assert.NoError(t, err)
		assert.Equal(t, tx.ID(), found.ID())
	})

	t.Run("NotFound", func(t *testing.T) {
		found, err := txRepo.FindByIdempotencyKey(ctx, uuid.New().String())

		assert.NoError(t, err) // Repository returns nil, nil when not found
		assert.Nil(t, found)
	})
}

func TestTransactionRepository_Integration_ListByWalletID(t *testing.T) {
	tc := setupSharedTestDB(t)

	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	txRepo := NewTransactionRepository(tc.pool)
	ctx := context.Background()

	// Setup
	user, _ := entities.NewUser("txlist@example.com", "TX List User")
	userRepo.Save(ctx, user)

	currency, _ := valueobjects.NewCurrency("USD")
	wallet, _ := entities.NewWallet(user.ID(), currency)
	walletRepo.Save(ctx, wallet)

	// Create multiple transactions
	for i := 0; i < 5; i++ {
		amount, _ := valueobjects.NewMoney(fmt.Sprintf("%d.00", i+1), currency)
		tx, _ := entities.NewTransaction(
			wallet.ID(),
			uuid.New().String(),
			entities.TransactionTypeDeposit,
			amount,
			fmt.Sprintf("TX %d", i+1),
		)
		txRepo.Save(ctx, tx)
	}

	txs, err := txRepo.FindByWalletID(ctx, wallet.ID(), 0, 10)

	assert.NoError(t, err)
	assert.Len(t, txs, 5)
}

// ============================================
// UnitOfWork Tests
// ============================================

func TestUnitOfWork_Integration_Commit(t *testing.T) {
	tc := setupSharedTestDB(t)

	uow := NewUnitOfWork(tc.pool)
	userRepo := NewUserRepository(tc.pool)
	ctx := context.Background()

	t.Run("CommitSuccess", func(t *testing.T) {
		err := uow.Execute(ctx, func(ctx context.Context) error {
			user, _ := entities.NewUser("commit@example.com", "Commit User")
			return userRepo.Save(ctx, user)
		})

		assert.NoError(t, err)

		// Verify committed
		_, err = userRepo.FindByEmail(ctx, "commit@example.com")
		assert.NoError(t, err)
	})

	t.Run("RollbackOnError", func(t *testing.T) {
		err := uow.Execute(ctx, func(ctx context.Context) error {
			user, _ := entities.NewUser("rollback@example.com", "Rollback User")
			userRepo.Save(ctx, user)

			return fmt.Errorf("intentional error")
		})

		assert.Error(t, err)

		// Verify rolled back
		_, err = userRepo.FindByEmail(ctx, "rollback@example.com")
		assert.Error(t, err)
		assert.True(t, domerrors.IsNotFound(err))
	})
}

func TestUnitOfWork_Integration_AtomicTransfer(t *testing.T) {
	tc := setupSharedTestDB(t)

	uow := NewUnitOfWork(tc.pool)
	userRepo := NewUserRepository(tc.pool)
	walletRepo := NewWalletRepository(tc.pool)
	ctx := context.Background()

	// Setup users
	user1, _ := entities.NewUser("transfer1@example.com", "User 1")
	user2, _ := entities.NewUser("transfer2@example.com", "User 2")
	require.NoError(t, userRepo.Save(ctx, user1))
	require.NoError(t, userRepo.Save(ctx, user2))

	currency, _ := valueobjects.NewCurrency("USD")
	wallet1, _ := entities.NewWallet(user1.ID(), currency)
	wallet2, _ := entities.NewWallet(user2.ID(), currency)

	// Save empty wallets first
	require.NoError(t, walletRepo.Save(ctx, wallet1))
	require.NoError(t, walletRepo.Save(ctx, wallet2))

	// Credit wallet1 with initial balance in a transaction
	initialAmount, _ := valueobjects.NewMoney("1000.00", currency)
	err := uow.Execute(ctx, func(txCtx context.Context) error {
		w1, err := walletRepo.FindByID(txCtx, wallet1.ID())
		if err != nil {
			return err
		}
		if err := w1.Credit(initialAmount); err != nil {
			return err
		}
		return walletRepo.Save(txCtx, w1)
	})
	require.NoError(t, err, "Initial credit should succeed")

	transferAmount, _ := valueobjects.NewMoney("100.00", currency)

	// Execute atomic transfer
	err = uow.Execute(ctx, func(txCtx context.Context) error {
		// Load wallets in transaction context
		w1, err := walletRepo.FindByID(txCtx, wallet1.ID())
		if err != nil {
			return fmt.Errorf("failed to load wallet1: %w", err)
		}

		w2, err := walletRepo.FindByID(txCtx, wallet2.ID())
		if err != nil {
			return fmt.Errorf("failed to load wallet2: %w", err)
		}

		// Debit from wallet1
		if err := w1.Debit(transferAmount); err != nil {
			return fmt.Errorf("failed to debit wallet1: %w", err)
		}

		// Credit to wallet2
		if err := w2.Credit(transferAmount); err != nil {
			return fmt.Errorf("failed to credit wallet2: %w", err)
		}

		// Save both in transaction
		if err := walletRepo.Save(txCtx, w1); err != nil {
			return fmt.Errorf("failed to save wallet1: %w", err)
		}
		if err := walletRepo.Save(txCtx, w2); err != nil {
			return fmt.Errorf("failed to save wallet2: %w", err)
		}

		return nil
	})

	require.NoError(t, err, "Transaction should succeed")

	// Verify balances after transaction
	w1, err := walletRepo.FindByID(ctx, wallet1.ID())
	require.NoError(t, err)
	w2, err := walletRepo.FindByID(ctx, wallet2.ID())
	require.NoError(t, err)

	assert.Equal(t, "900.00 USD", w1.AvailableBalance().String(), "Wallet1 should have 900 USD")
	assert.Equal(t, "100.00 USD", w2.AvailableBalance().String(), "Wallet2 should have 100 USD")
}
