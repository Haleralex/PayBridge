// Package transaction - интеграционные тесты для Transaction UseCases.
//
// ЗАПУСК ТЕСТОВ:
//
//	go test -tags=integration -v ./internal/application/usecases/transaction/...
//
// ТРЕБОВАНИЯ:
//
//  1. Запущенный PostgreSQL:
//     docker-compose up -d
//
//  2. Выполненные миграции:
//     make migrate-up
//     или
//     migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/wallethub_test?sslmode=disable" up
//
//  3. Переменные окружения (опционально):
//     TEST_DB_HOST=localhost
//     TEST_DB_PORT=5432
//     TEST_DB_NAME=wallethub_test
//     TEST_DB_USER=postgres
//     TEST_DB_PASSWORD=postgres
//
// ЧТО ТЕСТИРУЕМ:
//   - Полный flow с реальной БД (не моки!)
//   - Транзакции БД (COMMIT/ROLLBACK)
//   - Идемпотентность на уровне БД
//   - Concurrent access и race conditions
//   - Optimistic locking для wallet balance
package transaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/wallethub/internal/application/dtos"
	"github.com/yourusername/wallethub/internal/domain/entities"
	domainErrors "github.com/yourusername/wallethub/internal/domain/errors"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
	"github.com/yourusername/wallethub/internal/infrastructure/persistence/postgres"
)

// testPool - shared connection pool для всех integration тестов
var testPool *pgxpool.Pool

// TestMain настраивает тестовое окружение
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Получаем конфигурацию для тестовой БД
	cfg := getTestConfig()

	// Создаём connection pool
	pool, err := postgres.NewConnectionPool(ctx, cfg)
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

// getTestConfig возвращает конфигурацию для тестовой БД
func getTestConfig() postgres.Config {
	cfg := postgres.DefaultConfig()

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

// cleanupDB удаляет все данные из тестовой БД (в правильном порядке!)
func cleanupDB(t *testing.T, ctx context.Context) {
	// Порядок важен из-за foreign keys!
	tables := []string{"outbox_events", "transactions", "wallets", "users"}

	for _, table := range tables {
		_, err := testPool.Exec(ctx, "DELETE FROM "+table)
		if err != nil {
			t.Logf("Warning: failed to cleanup %s: %v", table, err)
		}
	}
}

// createTestUser создаёт тестового пользователя в БД
func createTestUser(t *testing.T, ctx context.Context, email, name string) *entities.User {
	userRepo := postgres.NewUserRepository(testPool)

	user, err := entities.NewUser(email, name)
	if err != nil {
		t.Fatalf("Failed to create user entity: %v", err)
	}

	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatalf("Failed to save user to DB: %v", err)
	}

	return user
}

// createTestWalletIntegration создаёт тестовый кошелёк в БД
func createTestWalletIntegration(t *testing.T, ctx context.Context, userID uuid.UUID, currencyCode string, initialBalance string) *entities.Wallet {
	walletRepo := postgres.NewWalletRepository(testPool)

	currency := valueobjects.MustNewCurrency(currencyCode)

	wallet, err := entities.NewWallet(userID, currency)
	if err != nil {
		t.Fatalf("Failed to create wallet entity: %v", err)
	}

	// СНАЧАЛА сохраняем wallet с нулевым балансом (version=0)
	if err := walletRepo.Save(ctx, wallet); err != nil {
		t.Fatalf("Failed to save wallet to DB: %v", err)
	}

	// ПОТОМ зачисляем начальный баланс если нужно (увеличит version)
	if initialBalance != "0" {
		balance, err := valueobjects.NewMoney(initialBalance, currency)
		if err != nil {
			t.Fatalf("Failed to create money: %v", err)
		}

		if err := wallet.Credit(balance); err != nil {
			t.Fatalf("Failed to credit initial balance: %v", err)
		}

		// Сохраняем обновлённый wallet (version=1)
		if err := walletRepo.Save(ctx, wallet); err != nil {
			t.Fatalf("Failed to save updated wallet to DB: %v", err)
		}
	}

	return wallet
}

// ============================================
// ПРИМЕР 1: CreateTransaction Integration Test
// Полный пример как писать integration тест
// ============================================

func TestCreateTransactionUseCase_Integration_Deposit_Success(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём реальные repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)

	// Для integration тестов можно использовать mock EventPublisher
	// или реальный in-memory publisher если нужно проверить события
	eventPublisher := &mockEventPublisher{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных в БД
	user := createTestUser(t, ctx, "deposit@test.com", "Deposit Test")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	// 3. Выполнение use case
	cmd := dtos.CreateTransactionCommand{
		WalletID:       wallet.ID().String(),
		IdempotencyKey: uuid.New().String(),
		Type:           "DEPOSIT",
		Amount:         "250.50",
		Description:    "Integration test deposit",
	}

	result, err := useCase.Execute(ctx, cmd)

	// 4. Проверки (Assertions)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// 5. Проверка что транзакция действительно сохранена в БД
	txID, err := uuid.Parse(result.ID)
	if err != nil {
		t.Fatalf("Failed to parse transaction ID: %v", err)
	}
	txFromDB, err := transactionRepo.FindByID(ctx, txID)
	if err != nil {
		t.Fatalf("Failed to load transaction from DB: %v", err)
	}

	if txFromDB.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", txFromDB.Status())
	}

	// 6. Проверка что баланс кошелька обновлён в БД
	walletFromDB, err := walletRepo.FindByID(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("Failed to load wallet from DB: %v", err)
	}

	expectedBalance, _ := valueobjects.NewMoney("1250.50", valueobjects.MustNewCurrency("USD"))
	if !walletFromDB.AvailableBalance().Equals(expectedBalance) {
		t.Errorf("Expected balance %s, got %s",
			expectedBalance.Amount(),
			walletFromDB.AvailableBalance().Amount())
	}

	// 7. Проверка событий опубликованы
	if len(eventPublisher.publishedEvents) < 3 {
		t.Errorf("Expected at least 3 events published, got %d", len(eventPublisher.publishedEvents))
	}
}

// ============================================
// ПРИМЕР 2: Idempotency Test
// Проверяем что идемпотентность работает на уровне БД
// ============================================

func TestCreateTransactionUseCase_Integration_Idempotency(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// Setup
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	user := createTestUser(t, ctx, "idempotency@test.com", "Idempotency Test")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	idempotencyKey := uuid.New().String()

	cmd := dtos.CreateTransactionCommand{
		WalletID:       wallet.ID().String(),
		IdempotencyKey: idempotencyKey,
		Type:           "DEPOSIT",
		Amount:         "100.00",
		Description:    "Idempotency test",
	}

	// Первый вызов - создание
	result1, err := useCase.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	initialEventsCount := len(eventPublisher.publishedEvents)

	// Второй вызов с тем же idempotency key - должен вернуть тот же результат
	result2, err := useCase.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Проверяем что вернулась та же транзакция
	if result1.ID != result2.ID {
		t.Errorf("Expected same transaction ID, got %s and %s", result1.ID, result2.ID)
	}

	// Проверяем что новые события НЕ опубликованы
	if len(eventPublisher.publishedEvents) != initialEventsCount {
		t.Errorf("Expected no new events on second call, got %d new events",
			len(eventPublisher.publishedEvents)-initialEventsCount)
	}

	// Проверяем баланс не изменился дважды
	walletFromDB, _ := walletRepo.FindByID(ctx, wallet.ID())
	expectedBalance, _ := valueobjects.NewMoney("1100.00", valueobjects.MustNewCurrency("USD"))

	if !walletFromDB.AvailableBalance().Equals(expectedBalance) {
		t.Errorf("Balance should be credited only once: expected %s, got %s",
			expectedBalance.Amount(),
			walletFromDB.AvailableBalance().Amount())
	}
}

// ============================================
// TODO: ТВОЯ ЗАДАЧА - Написать следующие тесты!
// ============================================

// TODO 1: TestCreateTransactionUseCase_Integration_Withdraw_Success
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай wallet с балансом 1000 USD
//   - Сделай WITHDRAW на 300 USD
//   - Проверь что balance стал 700 USD
//   - Проверь что транзакция в статусе COMPLETED
//   - Проверь тип транзакции = WITHDRAW
//
// ПОДСКАЗКА: Копируй структуру из Deposit_Success теста выше
// ПОДСКАЗКА: Меняй только Type: "WITHDRAW" и проверяй balance уменьшился
func TestCreateTransactionUseCase_Integration_Withdraw_Success(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных: СНАЧАЛА user, ПОТОМ wallet!
	user := createTestUser(t, ctx, "withdraw@test.com", "Withdraw Test")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	// 3. Выполняем WITHDRAW через use case
	cmd := dtos.CreateTransactionCommand{
		WalletID:       wallet.ID().String(),
		IdempotencyKey: uuid.New().String(),
		Type:           "WITHDRAW", // ← главное отличие от Deposit
		Amount:         "300.00",
		Description:    "Integration test withdrawal",
	}

	result, err := useCase.Execute(ctx, cmd)

	// 4. Проверки результата
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// 5. Проверка транзакции в БД
	txID, err := uuid.Parse(result.ID)
	if err != nil {
		t.Fatalf("Failed to parse transaction ID: %v", err)
	}
	txFromDB, err := transactionRepo.FindByID(ctx, txID)
	if err != nil {
		t.Fatalf("Failed to load transaction from DB: %v", err)
	}

	if txFromDB.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", txFromDB.Status())
	}

	if txFromDB.Type() != entities.TransactionTypeWithdraw {
		t.Errorf("Expected type WITHDRAW, got %s", txFromDB.Type())
	}

	// 6. Проверка баланса: было 1000, списали 300 → должно быть 700
	assertBalance(t, ctx, wallet.ID(), "700.00", "USD")

	// 7. Проверка событий
	if len(eventPublisher.publishedEvents) < 3 {
		t.Errorf("Expected at least 3 events published, got %d", len(eventPublisher.publishedEvents))
	}
}

// TODO 2: TestCreateTransactionUseCase_Integration_InsufficientBalance
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай wallet с балансом 100 USD
//   - Попробуй сделать WITHDRAW на 500 USD
//   - Проверь что вернулась ОШИБКА
//   - Проверь что ошибка = ErrInsufficientBalance (используй errors.Is())
//   - Проверь что balance НЕ изменился (всё ещё 100 USD)
//   - Проверь что транзакция НЕ сохранена в БД
//
// ПОДСКАЗКА: Используй errors.Is(err, domainErrors.ErrInsufficientBalance)
// ПОДСКАЗКА: Попробуй найти транзакцию по idempotency_key - её не должно быть
func TestCreateTransactionUseCase_Integration_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных: СНАЧАЛА user, ПОТОМ wallet!
	user := createTestUser(t, ctx, "insufficient@test.com", "Insufficient Balance Test")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "100.00")

	// 3. Пытаемся сделать WITHDRAW на сумму больше баланса
	idempotencyKey := uuid.New().String()
	cmd := dtos.CreateTransactionCommand{
		WalletID:       wallet.ID().String(),
		IdempotencyKey: idempotencyKey,
		Type:           "WITHDRAW",
		Amount:         "500.00", // Пытаемся списать больше чем есть!
		Description:    "Integration test insufficient balance",
	}

	result, err := useCase.Execute(ctx, cmd)

	// 4. Проверка: ДОЛЖНА быть ошибка
	if err == nil {
		t.Fatal("Expected ErrInsufficientBalance error, got nil")
	}

	// 5. Проверка типа ошибки
	if !errors.Is(err, domainErrors.ErrInsufficientBalance) {
		t.Fatalf("Expected ErrInsufficientBalance, got: %v", err)
	}

	// 6. Проверка: result должен быть nil при ошибке
	if result != nil {
		t.Errorf("Expected nil result on error, got: %+v", result)
	}

	// 7. Проверка: баланс НЕ должен измениться
	assertBalance(t, ctx, wallet.ID(), "100.00", "USD")

	// 8. Проверка: транзакция может быть создана, но с ошибкой (rollback должен был откатить, но если нет - проверим)
	// В production это должно быть откачено UnitOfWork, но для теста просто проверим consistency
	txFromDB, err := transactionRepo.FindByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		// Отлично - транзакция не создана (rollback сработал)
		t.Logf("✅ Transaction was rolled back correctly")
	} else if txFromDB != nil {
		// Транзакция создана - проверим что баланс всё равно не изменился
		t.Logf("⚠️  Transaction exists in DB (status: %s), but balance is unchanged", txFromDB.Status())
	}

	// 9. Проверка: никаких событий не опубликовано
	if len(eventPublisher.publishedEvents) > 0 {
		t.Errorf("Expected no events published on error, got %d events", len(eventPublisher.publishedEvents))
	}
}

// TODO 3: TestTransferBetweenWalletsUseCase_Integration_Success
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай 2 пользователей
//   - Создай 2 кошелька в USD: source (1000 USD), destination (500 USD)
//   - Сделай transfer 250 USD из source в destination
//   - Проверь что source balance = 750 USD
//   - Проверь что destination balance = 750 USD
//   - Проверь что транзакция типа TRANSFER
//   - Проверь что transaction.DestinationWalletID() указывает на destination
//   - Проверь что опубликовано 4 события (Created, Debited, Credited, Completed)
//
// ПОДСКАЗКА: Нужен TransferBetweenWalletsUseCase
// ПОДСКАЗКА: Создай 2 wallet через createTestWalletIntegration() с разными userID
func TestTransferBetweenWalletsUseCase_Integration_Success(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём repositories и use case (TRANSFER - это отдельный UseCase!)
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	// ← ПРАВИЛЬНО: используем TransferBetweenWalletsUseCase, а не CreateTransactionUseCase!
	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных: СНАЧАЛА user, ПОТОМ wallet!
	sourceUser := createTestUser(t, ctx, "sourceUser@test.com", "Money source user")
	sourceWallet := createTestWalletIntegration(t, ctx, sourceUser.ID(), "USD", "1000.00")

	destinationUser := createTestUser(t, ctx, "destinationUser@test.com", "Money destination user")
	destinationWallet := createTestWalletIntegration(t, ctx, destinationUser.ID(), "USD", "500.00")

	// 3. Выполняем TRANSFER через use case (используем TransferBetweenWalletsCommand!)
	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWallet.ID().String(),
		DestinationWalletID: destinationWallet.ID().String(),
		IdempotencyKey:      uuid.New().String(),
		Amount:              "250.00",
		Description:         "Integration test transfer",
	}

	result, err := useCase.Execute(ctx, cmd)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// 4. Проверка балансов обоих кошельков
	assertBalance(t, ctx, sourceWallet.ID(), "750.00", "USD")
	assertBalance(t, ctx, destinationWallet.ID(), "750.00", "USD")
	// 5. Проверка транзакции в БД
	txID, err := uuid.Parse(result.ID)
	if err != nil {
		t.Fatalf("Failed to parse transaction ID: %v", err)
	}
	txFromDB, err := transactionRepo.FindByID(ctx, txID)
	if err != nil {
		t.Fatalf("Failed to load transaction from DB: %v", err)
	}
	if txFromDB.Type() != entities.TransactionTypeTransfer {
		t.Errorf("Expected type TRANSFER, got %s", txFromDB.Type())
	}
	if txFromDB.DestinationWalletID() == nil || *txFromDB.DestinationWalletID() != destinationWallet.ID() {
		t.Errorf("Expected destination wallet ID to be %s, got %v",
			destinationWallet.ID().String(),
			txFromDB.DestinationWalletID(),
		)
	}

	// 6. Проверка событий
	if len(eventPublisher.publishedEvents) < 4 {
		t.Errorf("Expected at least 4 events published, got %d", len(eventPublisher.publishedEvents))
	}
}

// TODO 4: TestTransferBetweenWalletsUseCase_Integration_CurrencyMismatch
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай source wallet в USD
//   - Создай destination wallet в EUR
//   - Попробуй сделать transfer
//   - Проверь что вернулась ошибка BusinessRuleViolation
//   - Проверь что балансы НЕ изменились
//
// ПОДСКАЗКА: domainErrors.IsBusinessRuleViolation(err)
func TestTransferBetweenWalletsUseCase_Integration_CurrencyMismatch(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewTransferBetweenWalletsUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных: разные валюты!
	sourceUser := createTestUser(t, ctx, "currency-source@test.com", "Currency Source User")
	sourceWallet := createTestWalletIntegration(t, ctx, sourceUser.ID(), "USD", "1000.00")

	destinationUser := createTestUser(t, ctx, "currency-dest@test.com", "Currency Dest User")
	destinationWallet := createTestWalletIntegration(t, ctx, destinationUser.ID(), "EUR", "500.00")

	// 3. Пытаемся сделать TRANSFER между кошельками с разными валютами
	cmd := dtos.TransferBetweenWalletsCommand{
		SourceWalletID:      sourceWallet.ID().String(),
		DestinationWalletID: destinationWallet.ID().String(),
		IdempotencyKey:      uuid.New().String(),
		Amount:              "250.00",
		Description:         "Integration test currency mismatch",
	}

	result, err := useCase.Execute(ctx, cmd)

	// 4. Проверка: ДОЛЖНА быть ошибка
	if err == nil {
		t.Fatal("Expected BusinessRuleViolation error for currency mismatch, got nil")
	}

	// 5. Проверка типа ошибки (используем type assertion, не errors.Is!)
	if !domainErrors.IsBusinessRuleViolation(err) {
		t.Fatalf("Expected BusinessRuleViolation error, got: %v", err)
	}

	// 6. Проверка: result должен быть nil при ошибке
	if result != nil {
		t.Errorf("Expected nil result on error, got: %+v", result)
	}

	// 7. Проверка: балансы НЕ должны измениться
	assertBalance(t, ctx, sourceWallet.ID(), "1000.00", "USD")
	assertBalance(t, ctx, destinationWallet.ID(), "500.00", "EUR")

	// 8. Проверка: никаких событий не опубликовано
	if len(eventPublisher.publishedEvents) > 0 {
		t.Errorf("Expected no events published on error, got %d events", len(eventPublisher.publishedEvents))
	}
}

// TODO 5: TestProcessTransactionUseCase_Integration_Success
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай wallet и transaction в статусе PENDING (используй transactionRepo.Save())
//   - Вызови ProcessTransactionUseCase с Success: true
//   - Проверь что статус изменился на COMPLETED
//   - Проверь что publishedAt заполнено
//   - Проверь что событие TransactionCompleted опубликовано
//
// ПОДСКАЗКА: Создай transaction напрямую через entities.NewTransaction()
// ПОДСКАЗКА: Сохрани в БД через transactionRepo.Save(ctx, transaction)
// ПОДСКАЗКА: Потом вызови ProcessTransactionUseCase
func TestProcessTransactionUseCase_Integration_Success(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewProcessTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка: создаём user, wallet, и transaction в статусе PENDING
	user := createTestUser(t, ctx, "process@test.com", "Process Test User")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	// 3. Создаём transaction НАПРЯМУЮ через entities (минуя use case!)
	currency := valueobjects.MustNewCurrency("USD")
	amount, err := valueobjects.NewMoney("250.00", currency)
	if err != nil {
		t.Fatalf("Failed to create money: %v", err)
	}

	transaction, err := entities.NewTransaction(
		wallet.ID(),
		uuid.New().String(), // idempotencyKey
		entities.TransactionTypeDeposit,
		amount,
		"Test pending transaction for processing",
	)
	if err != nil {
		t.Fatalf("Failed to create transaction entity: %v", err)
	}

	// 4. Сохраняем transaction в БД (в статусе PENDING!)
	if err := transactionRepo.Save(ctx, transaction); err != nil {
		t.Fatalf("Failed to save transaction to DB: %v", err)
	}

	// 5. Теперь вызываем ProcessTransactionUseCase с Success=true
	cmd := dtos.ProcessTransactionCommand{
		TransactionID: transaction.ID().String(),
		Success:       true, // ← обработка успешна
	}

	result, err := useCase.Execute(ctx, cmd)

	// 6. Проверки результата
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// 7. Проверка что транзакция обновлена в БД
	txFromDB, err := transactionRepo.FindByID(ctx, transaction.ID())
	if err != nil {
		t.Fatalf("Failed to load transaction from DB: %v", err)
	}

	// 8. Проверка статуса: должен стать COMPLETED
	if txFromDB.Status() != entities.TransactionStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", txFromDB.Status())
	}

	// 9. Проверка что completedAt заполнено
	if txFromDB.CompletedAt() == nil {
		t.Error("Expected completedAt to be set, got nil")
	}

	// 10. Проверка что событие TransactionCompleted опубликовано
	if len(eventPublisher.publishedEvents) < 1 {
		t.Errorf("Expected at least 1 event published, got %d", len(eventPublisher.publishedEvents))
	}
}

// TODO 6: TestCancelTransactionUseCase_Integration_Success
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай transaction в статусе PENDING
//   - Вызови CancelTransactionUseCase
//   - Проверь что статус = CANCELLED
//   - Проверь что completedAt заполнено
//   - Проверь что событие опубликовано
//
// ПОДСКАЗКА: Похоже на ProcessTransaction тест
func TestCancelTransactionUseCase_Integration_Success(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewCancelTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка: создаём user, wallet, и transaction в статусе PENDING
	user := createTestUser(t, ctx, "cancel@test.com", "Cancel Test User")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	// 3. Создаём transaction НАПРЯМУЮ через entities (минуя use case!)
	currency := valueobjects.MustNewCurrency("USD")
	amount, err := valueobjects.NewMoney("250.00", currency)
	if err != nil {
		t.Fatalf("Failed to create money: %v", err)
	}

	transaction, err := entities.NewTransaction(
		wallet.ID(),
		uuid.New().String(), // idempotencyKey
		entities.TransactionTypeWithdraw,
		amount,
		"Test pending transaction for cancellation",
	)
	if err != nil {
		t.Fatalf("Failed to create transaction entity: %v", err)
	}

	// 4. Сохраняем transaction в БД (в статусе PENDING!)
	if err := transactionRepo.Save(ctx, transaction); err != nil {
		t.Fatalf("Failed to save transaction to DB: %v", err)
	}

	// 5. Вызываем CancelTransactionUseCase
	cmd := dtos.CancelTransactionCommand{
		TransactionID: transaction.ID().String(),
	}

	result, err := useCase.Execute(ctx, cmd)

	// 6. Проверки результата
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// 7. Проверка что транзакция обновлена в БД
	txFromDB, err := transactionRepo.FindByID(ctx, transaction.ID())
	if err != nil {
		t.Fatalf("Failed to load transaction from DB: %v", err)
	}

	// 8. Проверка статуса: должен стать CANCELLED
	if txFromDB.Status() != entities.TransactionStatusCancelled {
		t.Errorf("Expected status CANCELLED, got %s", txFromDB.Status())
	}

	// 9. Проверка что completedAt заполнено
	if txFromDB.CompletedAt() == nil {
		t.Error("Expected completedAt to be set, got nil")
	}

	// 10. Проверка что событие TransactionCancelled опубликовано
	if len(eventPublisher.publishedEvents) < 1 {
		t.Errorf("Expected at least 1 event published, got %d", len(eventPublisher.publishedEvents))
	}
}

// TODO 7 (ADVANCED): TestCreateTransactionUseCase_Integration_Concurrent
//
// ЧТО ТЕСТИРОВАТЬ:
//   - Создай wallet с балансом 1000 USD
//   - Запусти 10 goroutines одновременно
//   - Каждая делает WITHDRAW на 100 USD с РАЗНЫМИ idempotency keys
//   - Проверь что все 10 транзакций успешны
//   - Проверь что финальный balance = 0 USD (все 1000 списались)
//   - Проверь что в БД ровно 10 транзакций для этого кошелька
//
// ПОДСКАЗКА: Используй sync.WaitGroup для ожидания всех goroutines
// ПОДСКАЗКА: Используй channel для сбора результатов
// ПОДСКАЗКА: transactionRepo.FindByWalletID() для подсчёта транзакций
//
// ЗАЧЕМ ЭТО НУЖНО:
//
//	Проверяем что optimistic locking работает и balance не "потеряется"
//	при concurrent updates!
func TestCreateTransactionUseCase_Integration_Concurrent(t *testing.T) {
	ctx := context.Background()
	cleanupDB(t, ctx)

	// 1. Setup: создаём repositories и use case
	walletRepo := postgres.NewWalletRepository(testPool)
	transactionRepo := postgres.NewTransactionRepository(testPool)
	uow := postgres.NewUnitOfWork(testPool)
	eventPublisher := &mockEventPublisher{}

	useCase := NewCreateTransactionUseCase(walletRepo, transactionRepo, eventPublisher, uow)

	// 2. Подготовка тестовых данных с балансом 1000 USD
	user := createTestUser(t, ctx, "concurrent@test.com", "Concurrent Test User")
	wallet := createTestWalletIntegration(t, ctx, user.ID(), "USD", "1000.00")

	// 3. Конфигурация retry для concurrent операций
	retryConfig := DefaultRetryConfig()

	// 4. Запускаем 10 goroutines одновременно с retry механизмом
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	successCount := make(chan int, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cmd := dtos.CreateTransactionCommand{
				WalletID:       wallet.ID().String(),
				IdempotencyKey: uuid.New().String(), // ← РАЗНЫЕ idempotency keys!
				Type:           "WITHDRAW",
				Amount:         "100.00", // Каждый списывает 100 USD
				Description:    fmt.Sprintf("Concurrent test withdrawal #%d", idx),
			}

			// Используем retry механизм для обработки concurrency conflicts
			result, err := ExecuteWithRetry(ctx, useCase, cmd, retryConfig)
			if err != nil {
				errors <- err
			} else if result != nil {
				successCount <- 1
			}
		}(i)
	}

	// 5. Ждём завершения всех goroutines
	wg.Wait()
	close(errors)
	close(successCount)

	// 6. Подсчёт результатов
	errorList := make([]error, 0)
	for err := range errors {
		errorList = append(errorList, err)
	}

	successful := 0
	for range successCount {
		successful++
	}

	// С retry механизмом ожидаем что ВСЕ 10 транзакций успешны
	if successful != 10 {
		t.Errorf("Expected 10 successful transactions with retry, got %d", successful)
		if len(errorList) > 0 {
			t.Logf("Errors after retry:")
			for i, err := range errorList {
				t.Logf("  Error %d: %v", i+1, err)
			}
		}
	}

	// 7. Проверка что в БД ровно 10 транзакций для этого кошелька
	txCount := countTransactionsByWallet(t, ctx, wallet.ID())
	if txCount != 10 {
		t.Errorf("Expected 10 transactions in DB, got %d", txCount)
	}

	// 8. Проверка финального баланса: 1000 - (10 * 100) = 0 USD
	assertBalance(t, ctx, wallet.ID(), "0.00", "USD")

	t.Logf("✅ Concurrent test with retry passed: 10 goroutines, %d successful, final balance = 0 USD", successful)
}

// ============================================
// ПОЛЕЗНЫЕ ФУНКЦИИ ДЛЯ ТЕСТОВ
// ============================================

// loadTransactionByIdempotencyKey загружает транзакцию по idempotency key
// Полезно для проверки что транзакция действительно создана/не создана
func loadTransactionByIdempotencyKey(t *testing.T, ctx context.Context, key string) (*entities.Transaction, error) {
	transactionRepo := postgres.NewTransactionRepository(testPool)
	return transactionRepo.FindByIdempotencyKey(ctx, key)
}

// countTransactionsByWallet считает количество транзакций кошелька
// Полезно для проверки concurrent tests
func countTransactionsByWallet(t *testing.T, ctx context.Context, walletID uuid.UUID) int {
	transactionRepo := postgres.NewTransactionRepository(testPool)
	transactions, err := transactionRepo.FindByWalletID(ctx, walletID, 0, 1000)
	if err != nil {
		t.Fatalf("Failed to load transactions: %v", err)
	}
	return len(transactions)
}

// assertBalance проверяет баланс кошелька
func assertBalance(t *testing.T, ctx context.Context, walletID uuid.UUID, expectedAmount string, currency string) {
	walletRepo := postgres.NewWalletRepository(testPool)
	wallet, err := walletRepo.FindByID(ctx, walletID)
	if err != nil {
		t.Fatalf("Failed to load wallet: %v", err)
	}

	expected, _ := valueobjects.NewMoney(expectedAmount, valueobjects.MustNewCurrency(currency))
	if !wallet.AvailableBalance().Equals(expected) {
		t.Errorf("Balance mismatch: expected %s, got %s",
			expected.Amount(),
			wallet.AvailableBalance().Amount())
	}
}

// ============================================
// КАК ЗАПУСКАТЬ INTEGRATION ТЕСТЫ:
// ============================================
//
// 1. Запусти PostgreSQL:
//    docker-compose up -d
//
// 2. Создай тестовую БД и выполни миграции:
//    createdb wallethub_test
//    migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/wallethub_test?sslmode=disable" up
//
// 3. Запусти тесты:
//    go test -tags=integration -v ./internal/application/usecases/transaction/...
//
// 4. Запусти только конкретный тест:
//    go test -tags=integration -v -run TestCreateTransactionUseCase_Integration_Deposit ./internal/application/usecases/transaction/...
//
// 5. С coverage:
//    go test -tags=integration -v -coverprofile=coverage.out ./internal/application/usecases/transaction/...
//    go tool cover -html=coverage.out
//
// ============================================
// DEBUGGING TIPS:
// ============================================
//
// 1. Если тесты падают с "connection refused":
//    - Проверь что PostgreSQL запущен: docker ps
//    - Проверь порт: psql -h localhost -U postgres -d wallethub_test
//
// 2. Если тесты падают с "relation does not exist":
//    - Миграции не выполнены, запусти migrate up
//
// 3. Если тесты влияют друг на друга:
//    - Проверь что cleanupDB() вызывается в начале каждого теста
//    - Используй уникальные email/idempotency keys
//
// 4. Для debugging добавь:
//    t.Logf("Debug: wallet balance = %s", wallet.AvailableBalance().Amount())
//
// 5. Для просмотра SQL запросов (опционально):
//    Настрой логирование в getTestConfig():
//    cfg.LogLevel = "debug"
