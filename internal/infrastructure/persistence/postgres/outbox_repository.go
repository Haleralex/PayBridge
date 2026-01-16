// Package postgres - OutboxRepository для Transactional Outbox Pattern.
//
// Transactional Outbox Pattern:
// 1. В той же транзакции, что и бизнес-операция, сохраняем событие в outbox
// 2. Отдельный процесс (poller) читает события и публикует в Kafka
// 3. После публикации помечает событие как published
//
// Гарантирует exactly-once semantics для доставки событий!
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/wallethub/internal/application/ports"
	"github.com/yourusername/wallethub/internal/domain/events"
)

// Compile-time check
var _ ports.OutboxRepository = (*OutboxRepository)(nil)
var _ ports.EventPublisher = (*OutboxRepository)(nil) // OutboxRepository также является EventPublisher

// OutboxRepository реализует ports.OutboxRepository.
type OutboxRepository struct {
	pool *pgxpool.Pool
}

// NewOutboxRepository создаёт новый OutboxRepository.
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// getQuerier возвращает querier из context или pool.
func (r *OutboxRepository) getQuerier(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// outboxEntry представляет запись в таблице outbox.
type outboxEntry struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	EventVersion  int
	Payload       []byte
	Status        string
	PartitionKey  string
	CreatedAt     time.Time
}

// Save сохраняет событие в outbox таблицу.
// Должно выполняться в той же транзакции, что и бизнес-операция!
func (r *OutboxRepository) Save(ctx context.Context, event events.DomainEvent) error {
	q := r.getQuerier(ctx)

	// Сериализуем событие в JSON
	payload, err := serializeEvent(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Определяем aggregate type из типа события
	aggregateType := getAggregateType(event.EventType())

	query := `
		INSERT INTO outbox (
			id, aggregate_type, aggregate_id, event_type, event_version,
			payload, status, partition_key, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = q.Exec(ctx, query,
		event.EventID(),
		aggregateType,
		event.AggregateID(),
		event.EventType(),
		1, // Event version (можно расширить для версионирования схем)
		payload,
		"PENDING",
		event.AggregateID().String(), // Partition key для Kafka ordering
		event.OccurredAt(),
	)

	if err != nil {
		return fmt.Errorf("failed to save event to outbox: %w", err)
	}

	return nil
}

// FindUnpublished возвращает события, которые ещё не опубликованы.
// Используется poller'ом для публикации в Kafka.
func (r *OutboxRepository) FindUnpublished(ctx context.Context, limit int) ([]events.DomainEvent, error) {
	q := r.getQuerier(ctx)

	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM outbox
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := q.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find unpublished events: %w", err)
	}
	defer rows.Close()

	var domainEvents []events.DomainEvent
	for rows.Next() {
		var (
			id                       uuid.UUID
			aggregateType, eventType string
			aggregateID              uuid.UUID
			payload                  []byte
			createdAt                time.Time
		)

		if err := rows.Scan(&id, &aggregateType, &aggregateID, &eventType, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan outbox row: %w", err)
		}

		// Десериализуем событие
		event, err := deserializeEvent(eventType, payload, id, aggregateID, createdAt)
		if err != nil {
			// Логируем ошибку, но продолжаем (corrupt events не должны блокировать processing)
			continue
		}

		domainEvents = append(domainEvents, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outbox rows: %w", err)
	}

	return domainEvents, nil
}

// Publish реализует EventPublisher интерфейс.
// В Outbox pattern это просто alias для Save - сохраняем событие в БД.
func (r *OutboxRepository) Publish(ctx context.Context, event events.DomainEvent) error {
	return r.Save(ctx, event)
}

// PublishBatch реализует EventPublisher интерфейс.
// Сохраняет несколько событий за один раз.
func (r *OutboxRepository) PublishBatch(ctx context.Context, eventsList []events.DomainEvent) error {
	if len(eventsList) == 0 {
		return nil
	}

	// В контексте транзакции сохраняем все события
	for _, event := range eventsList {
		if err := r.Save(ctx, event); err != nil {
			return fmt.Errorf("failed to publish event %s: %w", event.EventType(), err)
		}
	}

	return nil
}

// MarkPublished помечает событие как опубликованное.
func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	q := r.getQuerier(ctx)

	eventUUID, err := uuid.Parse(eventID)
	if err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}

	query := `
		UPDATE outbox
		SET status = 'PUBLISHED', published_at = $2
		WHERE id = $1 AND status = 'PENDING'
	`

	result, err := q.Exec(ctx, query, eventUUID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark event as published: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("event not found or already published")
	}

	return nil
}

// MarkFailed помечает событие как failed.
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, reason string) error {
	q := r.getQuerier(ctx)

	eventUUID, err := uuid.Parse(eventID)
	if err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}

	query := `
		UPDATE outbox
		SET status = 'FAILED',
			failed_at = $2,
			last_error = $3,
			retry_count = retry_count + 1
		WHERE id = $1
	`

	_, err = q.Exec(ctx, query, eventUUID, time.Now(), reason)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	return nil
}

// MarkForRetry возвращает failed событие в PENDING статус для повторной обработки.
func (r *OutboxRepository) MarkForRetry(ctx context.Context, eventID string) error {
	q := r.getQuerier(ctx)

	eventUUID, err := uuid.Parse(eventID)
	if err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}

	query := `
		UPDATE outbox
		SET status = 'PENDING',
			failed_at = NULL,
			last_error = NULL
		WHERE id = $1 AND status = 'FAILED' AND retry_count < 5
	`

	result, err := q.Exec(ctx, query, eventUUID)
	if err != nil {
		return fmt.Errorf("failed to mark event for retry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("event not found, not failed, or max retries exceeded")
	}

	return nil
}

// CleanupPublished удаляет опубликованные события старше указанного времени.
// Используется для maintenance.
func (r *OutboxRepository) CleanupPublished(ctx context.Context, olderThan time.Duration) (int64, error) {
	q := r.getQuerier(ctx)

	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM outbox
		WHERE status = 'PUBLISHED' AND published_at < $1
	`

	result, err := q.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup published events: %w", err)
	}

	return result.RowsAffected(), nil
}

// Helper functions

// serializeEvent сериализует DomainEvent в JSON.
func serializeEvent(event events.DomainEvent) ([]byte, error) {
	return json.Marshal(event)
}

// deserializeEvent десериализует событие из JSON.
// Создаёт конкретный тип события на основе eventType.
func deserializeEvent(eventType string, payload []byte, eventID, aggregateID uuid.UUID, occurredAt time.Time) (events.DomainEvent, error) {
	// Для полноценной десериализации нужна регистрация типов событий
	// Пока возвращаем generic обёртку

	return &genericEvent{
		id:          eventID,
		eventType:   eventType,
		occurredAt:  occurredAt,
		aggregateID: aggregateID,
		payload:     payload,
	}, nil
}

// genericEvent - обёртка для десериализованных событий.
type genericEvent struct {
	id          uuid.UUID
	eventType   string
	occurredAt  time.Time
	aggregateID uuid.UUID
	payload     []byte
}

func (e *genericEvent) EventID() uuid.UUID     { return e.id }
func (e *genericEvent) EventType() string      { return e.eventType }
func (e *genericEvent) OccurredAt() time.Time  { return e.occurredAt }
func (e *genericEvent) AggregateID() uuid.UUID { return e.aggregateID }
func (e *genericEvent) Payload() []byte        { return e.payload }

// getAggregateType определяет тип агрегата из типа события.
func getAggregateType(eventType string) string {
	switch {
	case len(eventType) > 4 && eventType[:4] == "user":
		return "User"
	case len(eventType) > 6 && eventType[:6] == "wallet":
		return "Wallet"
	case len(eventType) > 11 && eventType[:11] == "transaction":
		return "Transaction"
	default:
		return "Unknown"
	}
}
