//go:build integration

// Package postgres - интеграционные тесты для PostgreSQL repositories.
//
// Запуск тестов:
//   go test -tags=integration ./internal/infrastructure/persistence/postgres/...
//
// Требования:
//   - Запущенный PostgreSQL (docker-compose up -d)
//   - Выполненные миграции
//
// Переменные окружения:
//   - TEST_DB_HOST (default: localhost)
//   - TEST_DB_PORT (default: 5432)
//   - TEST_DB_NAME (default: wallethub_test)
//   - TEST_DB_USER (default: postgres)
//   - TEST_DB_PASSWORD (default: postgres)
package postgres

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// testPool - shared connection pool для всех тестов
var testPool *pgxpool.Pool

// TestMain настраивает тестовое окружение.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Получаем конфигурацию из переменных окружения
	cfg := getTestConfig()

	// Создаём connection pool
	pool, err := NewConnectionPool(ctx, cfg)
	if err != nil {
		panic("Failed to connect to test database: " + err.Error())
	}
	testPool = pool

	// Запускаем тесты
	code := m.Run()

	// Cleanup
	pool.Close()

	os.Exit(code)
}

// getTestConfig возвращает конфигурацию для тестовой БД.
func getTestConfig() Config {
	cfg := DefaultConfig()

	if host := os.Getenv("TEST_DB_HOST"); host != "" {
		cfg.Host = host
	}
	if port := os.Getenv("TEST_DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if name := os.Getenv("TEST_DB_NAME"); name != "" {
		cfg.Database = name
	} else {
		cfg.Database = "wallethub_test"
	}
	if user := os.Getenv("TEST_DB_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("TEST_DB_PASSWORD"); password != "" {
		cfg.Password = password
	}

	return cfg
}

// cleanupUsers удаляет всех пользователей из тестовой БД.
func cleanupUsers(t *testing.T, ctx context.Context) {
	_, err := testPool.Exec(ctx, "DELETE FROM wallets")
	if err != nil {
		t.Logf("Warning: failed to cleanup wallets: %v", err)
	}
	_, err = testPool.Exec(ctx, "DELETE FROM users")
	if err != nil {
		t.Fatalf("Failed to cleanup users: %v", err)
	}
}

// ============================================
// UserRepository Integration Tests
// ============================================

func TestUserRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	repo := NewUserRepository(testPool)

	// Create user
	user, err := entities.NewUser("integration@test.com", "Integration Test")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Save
	err = repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Verify by loading
	loaded, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("Failed to load user: %v", err)
	}

	if loaded.Email() != user.Email() {
		t.Errorf("Expected email %s, got %s", user.Email(), loaded.Email())
	}
	if loaded.FullName() != user.FullName() {
		t.Errorf("Expected name %s, got %s", user.FullName(), loaded.FullName())
	}
}

func TestUserRepository_Save_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	repo := NewUserRepository(testPool)

	// Create and save first user
	user1, _ := entities.NewUser("duplicate@test.com", "User 1")
	if err := repo.Save(ctx, user1); err != nil {
		t.Fatalf("Failed to save first user: %v", err)
	}

	// Try to save second user with same email
	user2, _ := entities.NewUser("duplicate@test.com", "User 2")
	err := repo.Save(ctx, user2)

	// Should fail with business rule violation
	if err == nil {
		t.Fatal("Expected error for duplicate email")
	}

	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Errorf("Expected BusinessRuleViolation, got %T: %v", err, err)
	}
}

func TestUserRepository_FindByEmail(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	repo := NewUserRepository(testPool)

	user, _ := entities.NewUser("findbyemail@test.com", "Find By Email")
	if err := repo.Save(ctx, user); err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Find by email
	found, err := repo.FindByEmail(ctx, "findbyemail@test.com")
	if err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	if found.ID() != user.ID() {
		t.Errorf("Expected ID %s, got %s", user.ID(), found.ID())
	}
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	_, err := repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("Expected error for non-existent user")
	}

	if !domainErrors.IsNotFound(err) {
		t.Errorf("Expected ErrEntityNotFound, got %v", err)
	}
}

func TestUserRepository_ExistsByEmail(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	repo := NewUserRepository(testPool)

	// Should not exist initially
	exists, err := repo.ExistsByEmail(ctx, "exists@test.com")
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if exists {
		t.Error("Expected false for non-existent email")
	}

	// Create user
	user, _ := entities.NewUser("exists@test.com", "Exists Test")
	if err := repo.Save(ctx, user); err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Should exist now
	exists, err = repo.ExistsByEmail(ctx, "exists@test.com")
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Expected true for existing email")
	}
}

func TestUserRepository_List(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	repo := NewUserRepository(testPool)

	// Create 5 users
	for i := 0; i < 5; i++ {
		user, _ := entities.NewUser(
			"list"+strconv.Itoa(i)+"@test.com",
			"User "+strconv.Itoa(i),
		)
		if err := repo.Save(ctx, user); err != nil {
			t.Fatalf("Failed to save user %d: %v", i, err)
		}
	}

	// List with pagination
	users, err := repo.List(ctx, 0, 3)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}

	// Second page
	users, err = repo.List(ctx, 3, 3)
	if err != nil {
		t.Fatalf("Failed to list users page 2: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users on page 2, got %d", len(users))
	}
}

// ============================================
// UnitOfWork Integration Tests
// ============================================

func TestUnitOfWork_Execute_Commit(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	uow := NewUnitOfWork(testPool)
	userRepo := NewUserRepository(testPool)

	var savedUserID uuid.UUID

	err := uow.Execute(ctx, func(txCtx context.Context) error {
		user, err := entities.NewUser("uow@test.com", "UoW Test")
		if err != nil {
			return err
		}
		savedUserID = user.ID()

		return userRepo.Save(txCtx, user)
	})

	if err != nil {
		t.Fatalf("UoW execution failed: %v", err)
	}

	// Verify user was committed
	_, err = userRepo.FindByID(ctx, savedUserID)
	if err != nil {
		t.Errorf("User should exist after commit: %v", err)
	}
}

func TestUnitOfWork_Execute_Rollback(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	uow := NewUnitOfWork(testPool)
	userRepo := NewUserRepository(testPool)

	var savedUserID uuid.UUID

	err := uow.Execute(ctx, func(txCtx context.Context) error {
		user, err := entities.NewUser("rollback@test.com", "Rollback Test")
		if err != nil {
			return err
		}
		savedUserID = user.ID()

		if err := userRepo.Save(txCtx, user); err != nil {
			return err
		}

		// Return error to trigger rollback
		return domainErrors.NewBusinessRuleViolation("TEST_ERROR", "intentional error", nil)
	})

	if err == nil {
		t.Fatal("Expected error from UoW")
	}

	// Verify user was NOT committed
	_, err = userRepo.FindByID(ctx, savedUserID)
	if err == nil {
		t.Error("User should NOT exist after rollback")
	}
}

// ============================================
// WalletRepository Integration Tests
// ============================================

func TestWalletRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	userRepo := NewUserRepository(testPool)
	walletRepo := NewWalletRepository(testPool)

	// Create user first
	user, _ := entities.NewUser("wallet@test.com", "Wallet Test")
	user.StartKYCVerification()
	user.ApproveKYC()
	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Create wallet
	wallet, err := entities.NewWallet(user.ID(), valueobjects.USD)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Save wallet
	if err := walletRepo.Save(ctx, wallet); err != nil {
		t.Fatalf("Failed to save wallet: %v", err)
	}

	// Load and verify
	loaded, err := walletRepo.FindByID(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("Failed to load wallet: %v", err)
	}

	if loaded.UserID() != user.ID() {
		t.Errorf("Expected user ID %s, got %s", user.ID(), loaded.UserID())
	}

	if !loaded.Currency().Equals(valueobjects.USD) {
		t.Errorf("Expected currency USD, got %s", loaded.Currency().Code())
	}
}

func TestWalletRepository_OptimisticLocking(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	userRepo := NewUserRepository(testPool)
	walletRepo := NewWalletRepository(testPool)

	// Setup: Create user and wallet
	user, _ := entities.NewUser("locking@test.com", "Locking Test")
	user.StartKYCVerification()
	user.ApproveKYC()
	userRepo.Save(ctx, user)

	wallet, _ := entities.NewWallet(user.ID(), valueobjects.USD)
	walletRepo.Save(ctx, wallet)

	// Load wallet twice (simulating concurrent access)
	wallet1, _ := walletRepo.FindByID(ctx, wallet.ID())
	wallet2, _ := walletRepo.FindByID(ctx, wallet.ID())

	// Modify and save first wallet
	amount, _ := valueobjects.NewMoney("100", valueobjects.USD)
	wallet1.Credit(amount)
	if err := walletRepo.Save(ctx, wallet1); err != nil {
		t.Fatalf("First save should succeed: %v", err)
	}

	// Try to save second wallet (should fail due to version mismatch)
	wallet2.Credit(amount)
	err := walletRepo.Save(ctx, wallet2)
	if err == nil {
		t.Fatal("Second save should fail due to optimistic locking")
	}

	if !domainErrors.IsConcurrencyError(err) {
		t.Errorf("Expected ConcurrencyError, got %T: %v", err, err)
	}
}

func TestWalletRepository_FindByUserAndCurrency(t *testing.T) {
	ctx := context.Background()
	cleanupUsers(t, ctx)

	userRepo := NewUserRepository(testPool)
	walletRepo := NewWalletRepository(testPool)

	// Setup
	user, _ := entities.NewUser("findwallet@test.com", "Find Wallet")
	user.StartKYCVerification()
	user.ApproveKYC()
	userRepo.Save(ctx, user)

	wallet, _ := entities.NewWallet(user.ID(), valueobjects.EUR)
	walletRepo.Save(ctx, wallet)

	// Find
	found, err := walletRepo.FindByUserAndCurrency(ctx, user.ID(), valueobjects.EUR)
	if err != nil {
		t.Fatalf("Failed to find wallet: %v", err)
	}

	if found.ID() != wallet.ID() {
		t.Errorf("Expected wallet ID %s, got %s", wallet.ID(), found.ID())
	}
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkUserRepository_Save(b *testing.B) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user, _ := entities.NewUser(
			"bench"+strconv.Itoa(i)+time.Now().Format("150405.000000000")+"@test.com",
			"Benchmark User",
		)
		repo.Save(ctx, user)
	}
}

func BenchmarkUserRepository_FindByID(b *testing.B) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	// Create user
	user, _ := entities.NewUser("benchfind@test.com", "Benchmark Find")
	repo.Save(ctx, user)
	userID := user.ID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.FindByID(ctx, userID)
	}
}
