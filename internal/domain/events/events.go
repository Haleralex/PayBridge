// Package events defines domain events that represent significant business occurrences.
// Events are immutable facts about what happened in the past.
//
// SOLID Principles:
// - SRP: Each event type represents one business occurrence
// - OCP: New events can be added without modifying existing code
// - ISP: Event consumers only handle events they care about
//
// Pattern: Domain Events (Observer Pattern foundation)
// - Events are raised by entities when state changes
// - Handlers can react asynchronously
// - Enables loose coupling between domain modules
package events

import (
	"time"

	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// DomainEvent is the base interface for all domain events.
// All events must have an ID, timestamp, and type.
//
// Why interface? (ISP principle)
// - Consumers can work with any event type
// - Easy to add new event types
// - Type-safe event handling with type switches
type DomainEvent interface {
	EventID() uuid.UUID
	EventType() string
	OccurredAt() time.Time
	AggregateID() uuid.UUID // ID of the entity that raised this event
}

// BaseEvent provides common fields for all events.
// Embedded in specific event types to avoid duplication (DRY).
type BaseEvent struct {
	eventID     uuid.UUID
	eventType   string
	occurredAt  time.Time
	aggregateID uuid.UUID
}

func newBaseEvent(eventType string, aggregateID uuid.UUID) BaseEvent {
	return BaseEvent{
		eventID:     uuid.New(),
		eventType:   eventType,
		occurredAt:  time.Now(),
		aggregateID: aggregateID,
	}
}

func (e BaseEvent) EventID() uuid.UUID {
	return e.eventID
}

func (e BaseEvent) EventType() string {
	return e.eventType
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e BaseEvent) AggregateID() uuid.UUID {
	return e.aggregateID
}

// Event Types (constants for type checking)
const (
	EventTypeUserCreated          = "user.created"
	EventTypeUserKYCApproved      = "user.kyc.approved"
	EventTypeUserKYCRejected      = "user.kyc.rejected"
	EventTypeWalletCreated        = "wallet.created"
	EventTypeWalletCredited       = "wallet.credited"
	EventTypeWalletDebited        = "wallet.debited"
	EventTypeWalletSuspended      = "wallet.suspended"
	EventTypeTransactionCreated   = "transaction.created"
	EventTypeTransactionCompleted = "transaction.completed"
	EventTypeTransactionFailed    = "transaction.failed"
)

// ===== User Events =====

// UserCreated is raised when a new user is created.
type UserCreated struct {
	BaseEvent
	Email    string
	FullName string
}

func NewUserCreated(userID uuid.UUID, email, fullName string) *UserCreated {
	return &UserCreated{
		BaseEvent: newBaseEvent(EventTypeUserCreated, userID),
		Email:     email,
		FullName:  fullName,
	}
}

// UserKYCApproved is raised when a user's KYC is approved.
// This might trigger wallet creation or increased limits.
type UserKYCApproved struct {
	BaseEvent
	UserID uuid.UUID
}

func NewUserKYCApproved(userID uuid.UUID) *UserKYCApproved {
	return &UserKYCApproved{
		BaseEvent: newBaseEvent(EventTypeUserKYCApproved, userID),
		UserID:    userID,
	}
}

// UserKYCRejected is raised when KYC verification is rejected.
type UserKYCRejected struct {
	BaseEvent
	UserID uuid.UUID
	Reason string
}

func NewUserKYCRejected(userID uuid.UUID, reason string) *UserKYCRejected {
	return &UserKYCRejected{
		BaseEvent: newBaseEvent(EventTypeUserKYCRejected, userID),
		UserID:    userID,
		Reason:    reason,
	}
}

// ===== Wallet Events =====

// WalletCreated is raised when a new wallet is created.
type WalletCreated struct {
	BaseEvent
	UserID   uuid.UUID
	Currency valueobjects.Currency
}

func NewWalletCreated(walletID, userID uuid.UUID, currency valueobjects.Currency) *WalletCreated {
	return &WalletCreated{
		BaseEvent: newBaseEvent(EventTypeWalletCreated, walletID),
		UserID:    userID,
		Currency:  currency,
	}
}

// WalletCredited is raised when funds are added to a wallet.
// This event might trigger notifications, analytics, etc.
type WalletCredited struct {
	BaseEvent
	WalletID      uuid.UUID
	Amount        valueobjects.Money
	TransactionID uuid.UUID
	BalanceAfter  valueobjects.Money
}

func NewWalletCredited(
	walletID uuid.UUID,
	amount valueobjects.Money,
	transactionID uuid.UUID,
	balanceAfter valueobjects.Money,
) *WalletCredited {
	return &WalletCredited{
		BaseEvent:     newBaseEvent(EventTypeWalletCredited, walletID),
		WalletID:      walletID,
		Amount:        amount,
		TransactionID: transactionID,
		BalanceAfter:  balanceAfter,
	}
}

// WalletDebited is raised when funds are removed from a wallet.
type WalletDebited struct {
	BaseEvent
	WalletID      uuid.UUID
	Amount        valueobjects.Money
	TransactionID uuid.UUID
	BalanceAfter  valueobjects.Money
}

func NewWalletDebited(
	walletID uuid.UUID,
	amount valueobjects.Money,
	transactionID uuid.UUID,
	balanceAfter valueobjects.Money,
) *WalletDebited {
	return &WalletDebited{
		BaseEvent:     newBaseEvent(EventTypeWalletDebited, walletID),
		WalletID:      walletID,
		Amount:        amount,
		TransactionID: transactionID,
		BalanceAfter:  balanceAfter,
	}
}

// WalletSuspended is raised when a wallet is suspended.
// This might trigger alerts, stop pending transactions, etc.
type WalletSuspended struct {
	BaseEvent
	WalletID uuid.UUID
	Reason   string
}

func NewWalletSuspended(walletID uuid.UUID, reason string) *WalletSuspended {
	return &WalletSuspended{
		BaseEvent: newBaseEvent(EventTypeWalletSuspended, walletID),
		WalletID:  walletID,
		Reason:    reason,
	}
}

// ===== Transaction Events =====

// TransactionCreated is raised when a new transaction is created.
// Consumers might validate, apply risk checks, or start processing.
type TransactionCreated struct {
	BaseEvent
	TransactionID   uuid.UUID
	WalletID        uuid.UUID
	TransactionType string
	Amount          valueobjects.Money
	IdempotencyKey  string
}

func NewTransactionCreated(
	transactionID, walletID uuid.UUID,
	transactionType string,
	amount valueobjects.Money,
	idempotencyKey string,
) *TransactionCreated {
	return &TransactionCreated{
		BaseEvent:       newBaseEvent(EventTypeTransactionCreated, transactionID),
		TransactionID:   transactionID,
		WalletID:        walletID,
		TransactionType: transactionType,
		Amount:          amount,
		IdempotencyKey:  idempotencyKey,
	}
}

// TransactionCompleted is raised when a transaction completes successfully.
// This might trigger notifications, webhooks to merchants, analytics updates.
type TransactionCompleted struct {
	BaseEvent
	TransactionID   uuid.UUID
	WalletID        uuid.UUID
	TransactionType string
	Amount          valueobjects.Money
	CompletedAt     time.Time
}

func NewTransactionCompleted(
	transactionID, walletID uuid.UUID,
	transactionType string,
	amount valueobjects.Money,
) *TransactionCompleted {
	return &TransactionCompleted{
		BaseEvent:       newBaseEvent(EventTypeTransactionCompleted, transactionID),
		TransactionID:   transactionID,
		WalletID:        walletID,
		TransactionType: transactionType,
		Amount:          amount,
		CompletedAt:     time.Now(),
	}
}

// TransactionFailed is raised when a transaction fails.
// Consumers might retry, alert admins, or notify users.
type TransactionFailed struct {
	BaseEvent
	TransactionID   uuid.UUID
	WalletID        uuid.UUID
	TransactionType string
	Amount          valueobjects.Money
	FailureReason   string
	IsRetryable     bool
}

func NewTransactionFailed(
	transactionID, walletID uuid.UUID,
	transactionType string,
	amount valueobjects.Money,
	failureReason string,
	isRetryable bool,
) *TransactionFailed {
	return &TransactionFailed{
		BaseEvent:       newBaseEvent(EventTypeTransactionFailed, transactionID),
		TransactionID:   transactionID,
		WalletID:        walletID,
		TransactionType: transactionType,
		Amount:          amount,
		FailureReason:   failureReason,
		IsRetryable:     isRetryable,
	}
}

// EventStore is a simple in-memory store for events during a transaction.
// In Phase 6, we'll replace this with Kafka publishing.
//
// Pattern: Event Sourcing foundation
// - Collect events during entity operations
// - Publish them atomically with state changes
// - Enables eventual consistency and event-driven architecture
type EventStore struct {
	events []DomainEvent
}

// NewEventStore creates a new event store.
func NewEventStore() *EventStore {
	return &EventStore{
		events: make([]DomainEvent, 0),
	}
}

// Add appends an event to the store.
func (s *EventStore) Add(event DomainEvent) {
	s.events = append(s.events, event)
}

// GetAll returns all collected events.
func (s *EventStore) GetAll() []DomainEvent {
	return s.events
}

// Clear removes all events from the store.
func (s *EventStore) Clear() {
	s.events = make([]DomainEvent, 0)
}

// Count returns the number of events in the store.
func (s *EventStore) Count() int {
	return len(s.events)
}
