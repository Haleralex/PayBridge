// Package transaction - helper functions for testing
//go:build integration || !integration

package transaction

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/application/dtos"
	"github.com/Haleralex/wallethub/internal/domain/events"
)

// ============================================
// Enhanced Mock Event Publisher
// ============================================

// EnhancedMockEventPublisher - улучшенный mock с группировкой по типам событий
type EnhancedMockEventPublisher struct {
	mu              sync.Mutex
	publishedEvents []events.DomainEvent
	eventsByType    map[string][]events.DomainEvent
}

func NewEnhancedMockEventPublisher() *EnhancedMockEventPublisher {
	return &EnhancedMockEventPublisher{
		publishedEvents: make([]events.DomainEvent, 0),
		eventsByType:    make(map[string][]events.DomainEvent),
	}
}

func (m *EnhancedMockEventPublisher) Publish(ctx context.Context, event events.DomainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.publishedEvents = append(m.publishedEvents, event)

	eventType := event.EventType()
	m.eventsByType[eventType] = append(m.eventsByType[eventType], event)

	return nil
}

func (m *EnhancedMockEventPublisher) PublishBatch(ctx context.Context, evts []events.DomainEvent) error {
	for _, event := range evts {
		if err := m.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// GetAllEvents возвращает все опубликованные события
func (m *EnhancedMockEventPublisher) GetAllEvents() []events.DomainEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]events.DomainEvent{}, m.publishedEvents...)
}

// GetEventsByType возвращает события определённого типа
func (m *EnhancedMockEventPublisher) GetEventsByType(eventType string) []events.DomainEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]events.DomainEvent{}, m.eventsByType[eventType]...)
}

// AssertEventPublished проверяет что событие определённого типа было опубликовано
func (m *EnhancedMockEventPublisher) AssertEventPublished(t *testing.T, eventType string) {
	t.Helper()
	events := m.GetEventsByType(eventType)
	if len(events) == 0 {
		t.Errorf("Expected event type '%s' to be published, but it wasn't", eventType)
	}
}

// AssertEventCount проверяет количество событий определённого типа
func (m *EnhancedMockEventPublisher) AssertEventCount(t *testing.T, eventType string, expectedCount int) {
	t.Helper()
	events := m.GetEventsByType(eventType)
	if len(events) != expectedCount {
		t.Errorf("Expected %d events of type '%s', got %d", expectedCount, eventType, len(events))
	}
}

// GetEventTypes возвращает список всех типов событий
func (m *EnhancedMockEventPublisher) GetEventTypes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	types := make([]string, 0, len(m.eventsByType))
	for eventType := range m.eventsByType {
		types = append(types, eventType)
	}
	return types
}

// Reset очищает все события
func (m *EnhancedMockEventPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishedEvents = make([]events.DomainEvent, 0)
	m.eventsByType = make(map[string][]events.DomainEvent)
}

// ============================================
// Retry Helper for Concurrent Operations
// ============================================

// RetryConfig конфигурация для retry механизма
type RetryConfig struct {
	MaxAttempts     int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiple float64
}

// DefaultRetryConfig возвращает дефолтную конфигурацию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     10, // Увеличено с 5 до 10 для high-concurrency тестов
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      1000 * time.Millisecond, // Увеличено с 500 до 1000 мс
		BackoffMultiple: 2.0,
	}
}

// ExecuteWithRetry выполняет команду с retry при concurrency errors
func ExecuteWithRetry(
	ctx context.Context,
	useCase interface{},
	cmd interface{},
	config RetryConfig,
) (interface{}, error) {
	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		var result interface{}
		var err error

		// Вызываем соответствующий use case
		switch uc := useCase.(type) {
		case *CreateTransactionUseCase:
			result, err = uc.Execute(ctx, cmd.(dtos.CreateTransactionCommand))
		case *TransferBetweenWalletsUseCase:
			result, err = uc.Execute(ctx, cmd.(dtos.TransferBetweenWalletsCommand))
		default:
			return nil, fmt.Errorf("unsupported use case type")
		}

		// Успех - возвращаем результат
		if err == nil {
			return result, nil
		}

		// Проверяем является ли это concurrency error
		if !isConcurrencyError(err) {
			// Не concurrency error - не retry
			return nil, err
		}

		lastErr = err

		// Последняя попытка - возвращаем ошибку
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Ждём перед следующей попыткой (exponential backoff)
		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * config.BackoffMultiple)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return nil, fmt.Errorf("max retry attempts (%d) reached: %w", config.MaxAttempts, lastErr)
}

// isConcurrencyError проверяет является ли ошибка concurrency error
func isConcurrencyError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "concurrency error") ||
		strings.Contains(errStr, "was modified by another transaction") ||
		strings.Contains(errStr, "optimistic locking")
}

// ============================================
// Test Assertion Helpers
// ============================================

// AssertNoError проверяет отсутствие ошибки
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError проверяет наличие ошибки
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertErrorContains проверяет что ошибка содержит подстроку
func AssertErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing '%s' but got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error to contain '%s', got: %v", substr, err)
	}
}

// AssertNotNil проверяет что значение не nil
func AssertNotNil(t *testing.T, value interface{}, msg string) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s: expected non-nil value", msg)
	}
}
