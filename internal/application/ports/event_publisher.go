// Package ports - EventPublisher для публикации domain events.
//
// SOLID Principles:
// - DIP: Application не знает о Kafka/RabbitMQ деталях
// - OCP: Можно заменить Kafka на другую систему без изменения use cases
// - ISP: Простой интерфейс с одним методом
//
// Pattern: Publisher/Subscriber (Observer на уровне инфраструктуры)
package ports

import (
	"context"

	"github.com/Haleralex/wallethub/internal/domain/events"
)

// EventPublisher определяет контракт для публикации domain events.
//
// Реализации могут быть:
// - Kafka (Phase 6 - production)
// - In-memory (тесты)
// - RabbitMQ (альтернатива)
// - Database Outbox + Poller (для гарантий доставки)
type EventPublisher interface {
	// Publish публикует одно событие.
	//
	// Behaviour:
	// - Асинхронная публикация (не блокирует)
	// - At-least-once delivery (может быть дубликаты)
	// - Consumers должны быть идемпотентными!
	//
	// Example:
	//   event := events.NewWalletCredited(walletID, amount, txID, balance)
	//   err := publisher.Publish(ctx, event)
	Publish(ctx context.Context, event events.DomainEvent) error

	// PublishBatch публикует несколько событий за один вызов.
	// Более эффективно для множественных событий.
	//
	// Важно: Если один event не удаётся опубликовать, вся batch должна провалиться
	// (атомарность на уровне batch).
	//
	// Example:
	//   events := []events.DomainEvent{
	//       events.NewWalletCredited(...),
	//       events.NewTransactionCompleted(...),
	//   }
	//   err := publisher.PublishBatch(ctx, events)
	PublishBatch(ctx context.Context, events []events.DomainEvent) error
}

// EventSubscriber определяет контракт для подписки на события (consumers).
// Будет использоваться в Phase 6 для обработчиков событий.
//
// Пока оставляем как placeholder для архитектуры.
type EventSubscriber interface {
	// Subscribe регистрирует обработчик для типа события.
	//
	// eventType: например, "wallet.credited"
	// handler: функция-обработчик
	//
	// Example:
	//   subscriber.Subscribe("wallet.credited", func(ctx context.Context, event events.DomainEvent) error {
	//       walletCredited := event.(*events.WalletCredited)
	//       // Отправить уведомление пользователю
	//       return notificationService.Send(walletCredited.WalletID, ...)
	//   })
	Subscribe(eventType string, handler EventHandler) error

	// Start начинает потребление событий (blocking call).
	// Обычно запускается в отдельной горутине.
	Start(ctx context.Context) error

	// Stop останавливает потребление.
	Stop(ctx context.Context) error
}

// EventHandler - функция-обработчик события.
type EventHandler func(ctx context.Context, event events.DomainEvent) error

// OutboxRepository - интерфейс для Transactional Outbox Pattern (Phase 6).
//
// Transactional Outbox решает проблему:
// "Как гарантировать, что event опубликуется, если БД-транзакция успешна?"
//
// Решение:
// 1. В той же БД-транзакции сохраняем event в таблицу outbox
// 2. Отдельный процесс (poller) читает outbox и публикует в Kafka
// 3. После успешной публикации помечает event как published
//
// Это гарантирует exactly-once semantics для событий!
type OutboxRepository interface {
	// Save сохраняет событие в outbox таблицу.
	// Должно выполняться в той же транзакции, что и бизнес-операция!
	Save(ctx context.Context, event events.DomainEvent) error

	// FindUnpublished возвращает события, которые ещё не опубликованы.
	// Используется poller'ом для публикации.
	FindUnpublished(ctx context.Context, limit int) ([]events.DomainEvent, error)

	// MarkPublished помечает событие как опубликованное.
	// После этого poller не будет пытаться публиковать его снова.
	MarkPublished(ctx context.Context, eventID string) error

	// MarkFailed помечает событие как failed после N неудачных попыток.
	MarkFailed(ctx context.Context, eventID string, reason string) error
}
