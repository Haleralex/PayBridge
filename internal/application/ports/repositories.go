// Package ports определяет интерфейсы (порты) для внешних зависимостей.
// Эти интерфейсы реализуются в Infrastructure Layer.
//
// SOLID Principles:
// - DIP: Application зависит от абстракций, не от конкретных реализаций
// - ISP: Каждый интерфейс фокусируется на одной сущности
// - SRP: Repository отвечает только за persistence
//
// Pattern: Repository Pattern + Ports & Adapters (Hexagonal Architecture)
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/wallethub/internal/domain/entities"
	"github.com/yourusername/wallethub/internal/domain/valueobjects"
)

// UserRepository определяет контракт для хранения пользователей.
// Infrastructure Layer предоставит реализацию (PostgreSQL, MongoDB, in-memory для тестов).
//
// Why interface? (DIP)
// - Application Layer не знает о БД
// - Можно легко заменить PostgreSQL на MongoDB
// - Легко мокировать для тестов
type UserRepository interface {
	// Save сохраняет пользователя (create or update).
	// Использует Upsert логику на основе ID.
	Save(ctx context.Context, user *entities.User) error

	// FindByID загружает пользователя по ID.
	// Возвращает error если не найден (может быть ErrNotFound).
	FindByID(ctx context.Context, id uuid.UUID) (*entities.User, error)

	// FindByEmail загружает пользователя по email.
	// Email уникален в системе (UNIQUE constraint).
	FindByEmail(ctx context.Context, email string) (*entities.User, error)

	// ExistsByEmail проверяет существование без загрузки всей entity.
	// Оптимизация для проверки уникальности.
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// List возвращает список пользователей с пагинацией.
	// offset: пропустить N записей
	// limit: вернуть максимум N записей
	List(ctx context.Context, offset, limit int) ([]*entities.User, error)
}

// WalletRepository определяет контракт для хранения кошельков.
//
// Важно: Wallet - это Aggregate Root.
// Repository сохраняет весь Aggregate (включая Balance) атомарно.
type WalletRepository interface {
	// Save сохраняет кошелёк с проверкой версии (optimistic locking).
	// Если version не совпадает, возвращает ConcurrencyError.
	Save(ctx context.Context, wallet *entities.Wallet) error

	// FindByID загружает кошелёк по ID со всеми вложенными данными.
	FindByID(ctx context.Context, id uuid.UUID) (*entities.Wallet, error)

	// FindByUserAndCurrency находит кошелёк пользователя для конкретной валюты.
	// У пользователя может быть только один кошелёк на валюту.
	FindByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (*entities.Wallet, error)

	// FindByUserID возвращает все кошельки пользователя.
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Wallet, error)

	// ExistsByUserAndCurrency проверяет существование без загрузки.
	ExistsByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency valueobjects.Currency) (bool, error)

	// List возвращает кошельки с фильтрацией и пагинацией.
	List(ctx context.Context, filter WalletFilter, offset, limit int) ([]*entities.Wallet, error)
}

// WalletFilter определяет критерии фильтрации для кошельков.
type WalletFilter struct {
	UserID   *uuid.UUID                // Фильтр по пользователю
	Currency *valueobjects.Currency    // Фильтр по валюте
	Status   *entities.WalletStatus    // Фильтр по статусу
}

// TransactionRepository определяет контракт для хранения транзакций.
type TransactionRepository interface {
	// Save сохраняет транзакцию.
	Save(ctx context.Context, tx *entities.Transaction) error

	// FindByID загружает транзакцию по ID.
	FindByID(ctx context.Context, id uuid.UUID) (*entities.Transaction, error)

	// FindByIdempotencyKey находит транзакцию по ключу идемпотентности.
	// Критично для предотвращения дубликатов!
	FindByIdempotencyKey(ctx context.Context, key string) (*entities.Transaction, error)

	// FindByWalletID возвращает транзакции кошелька.
	FindByWalletID(ctx context.Context, walletID uuid.UUID, offset, limit int) ([]*entities.Transaction, error)

	// FindPendingByWallet возвращает транзакции в статусе PENDING для кошелька.
	// Используется для обработки очереди.
	FindPendingByWallet(ctx context.Context, walletID uuid.UUID) ([]*entities.Transaction, error)

	// FindFailedRetryable возвращает failed транзакции, которые можно повторить.
	// Для фоновой обработки retry logic.
	FindFailedRetryable(ctx context.Context, maxRetries int, limit int) ([]*entities.Transaction, error)

	// List возвращает транзакции с фильтрацией и пагинацией.
	List(ctx context.Context, filter TransactionFilter, offset, limit int) ([]*entities.Transaction, error)
}

// TransactionFilter определяет критерии фильтрации для транзакций.
type TransactionFilter struct {
	WalletID *uuid.UUID                     // Фильтр по кошельку
	UserID   *uuid.UUID                     // Фильтр по пользователю (join через wallet)
	Type     *entities.TransactionType      // Фильтр по типу
	Status   *entities.TransactionStatus    // Фильтр по статусу
}
