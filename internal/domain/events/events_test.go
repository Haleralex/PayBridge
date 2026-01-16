package events

import (
	"testing"
	"time"

	"github.com/Haleralex/wallethub/internal/domain/valueobjects"
	"github.com/google/uuid"
)

// TestBaseEvent tests base event functionality
func TestBaseEvent(t *testing.T) {
	aggregateID := uuid.New()
	event := newBaseEvent("test.event", aggregateID)

	if event.EventID() == uuid.Nil {
		t.Error("EventID should not be nil")
	}

	if event.EventType() != "test.event" {
		t.Errorf("EventType = %q, want %q", event.EventType(), "test.event")
	}

	if event.AggregateID() != aggregateID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), aggregateID)
	}

	if event.OccurredAt().IsZero() {
		t.Error("OccurredAt should be set")
	}

	if time.Since(event.OccurredAt()) > 1*time.Second {
		t.Error("OccurredAt should be recent")
	}
}

// TestNewUserCreated tests UserCreated event creation
func TestNewUserCreated(t *testing.T) {
	userID := uuid.New()
	email := "test@example.com"
	fullName := "Test User"

	event := NewUserCreated(userID, email, fullName)

	if event.EventType() != EventTypeUserCreated {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeUserCreated)
	}

	if event.AggregateID() != userID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), userID)
	}

	if event.Email != email {
		t.Errorf("Email = %q, want %q", event.Email, email)
	}

	if event.FullName != fullName {
		t.Errorf("FullName = %q, want %q", event.FullName, fullName)
	}

	if event.EventID() == uuid.Nil {
		t.Error("EventID should not be nil")
	}

	if event.OccurredAt().IsZero() {
		t.Error("OccurredAt should be set")
	}
}

// TestNewUserKYCApproved tests UserKYCApproved event creation
func TestNewUserKYCApproved(t *testing.T) {
	userID := uuid.New()

	event := NewUserKYCApproved(userID)

	if event.EventType() != EventTypeUserKYCApproved {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeUserKYCApproved)
	}

	if event.AggregateID() != userID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), userID)
	}

	if event.UserID != userID {
		t.Errorf("UserID = %v, want %v", event.UserID, userID)
	}
}

// TestNewUserKYCRejected tests UserKYCRejected event creation
func TestNewUserKYCRejected(t *testing.T) {
	userID := uuid.New()
	reason := "Document expired"

	event := NewUserKYCRejected(userID, reason)

	if event.EventType() != EventTypeUserKYCRejected {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeUserKYCRejected)
	}

	if event.AggregateID() != userID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), userID)
	}

	if event.UserID != userID {
		t.Errorf("UserID = %v, want %v", event.UserID, userID)
	}

	if event.Reason != reason {
		t.Errorf("Reason = %q, want %q", event.Reason, reason)
	}
}

// TestNewWalletCreated tests WalletCreated event creation
func TestNewWalletCreated(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	currency := valueobjects.USD

	event := NewWalletCreated(walletID, userID, currency)

	if event.EventType() != EventTypeWalletCreated {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeWalletCreated)
	}

	if event.AggregateID() != walletID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), walletID)
	}

	if event.UserID != userID {
		t.Errorf("UserID = %v, want %v", event.UserID, userID)
	}

	if !event.Currency.Equals(currency) {
		t.Errorf("Currency = %v, want %v", event.Currency, currency)
	}
}

// TestNewWalletCredited tests WalletCredited event creation
func TestNewWalletCredited(t *testing.T) {
	walletID := uuid.New()
	transactionID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	balanceAfter, _ := valueobjects.NewMoneyFromInt(150, valueobjects.USD)

	event := NewWalletCredited(walletID, amount, transactionID, balanceAfter)

	if event.EventType() != EventTypeWalletCredited {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeWalletCredited)
	}

	if event.AggregateID() != walletID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), walletID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if !event.Amount.Equals(amount) {
		t.Errorf("Amount = %v, want %v", event.Amount, amount)
	}

	if event.TransactionID != transactionID {
		t.Errorf("TransactionID = %v, want %v", event.TransactionID, transactionID)
	}

	if !event.BalanceAfter.Equals(balanceAfter) {
		t.Errorf("BalanceAfter = %v, want %v", event.BalanceAfter, balanceAfter)
	}
}

// TestNewWalletDebited tests WalletDebited event creation
func TestNewWalletDebited(t *testing.T) {
	walletID := uuid.New()
	transactionID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(50, valueobjects.USD)
	balanceAfter, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	event := NewWalletDebited(walletID, amount, transactionID, balanceAfter)

	if event.EventType() != EventTypeWalletDebited {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeWalletDebited)
	}

	if event.AggregateID() != walletID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), walletID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if !event.Amount.Equals(amount) {
		t.Errorf("Amount = %v, want %v", event.Amount, amount)
	}

	if event.TransactionID != transactionID {
		t.Errorf("TransactionID = %v, want %v", event.TransactionID, transactionID)
	}

	if !event.BalanceAfter.Equals(balanceAfter) {
		t.Errorf("BalanceAfter = %v, want %v", event.BalanceAfter, balanceAfter)
	}
}

// TestNewWalletSuspended tests WalletSuspended event creation
func TestNewWalletSuspended(t *testing.T) {
	walletID := uuid.New()
	reason := "Suspicious activity detected"

	event := NewWalletSuspended(walletID, reason)

	if event.EventType() != EventTypeWalletSuspended {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeWalletSuspended)
	}

	if event.AggregateID() != walletID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), walletID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if event.Reason != reason {
		t.Errorf("Reason = %q, want %q", event.Reason, reason)
	}
}

// TestNewTransactionCreated tests TransactionCreated event creation
func TestNewTransactionCreated(t *testing.T) {
	transactionID := uuid.New()
	walletID := uuid.New()
	txType := "DEPOSIT"
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	idempotencyKey := "key-123"

	event := NewTransactionCreated(transactionID, walletID, txType, amount, idempotencyKey)

	if event.EventType() != EventTypeTransactionCreated {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeTransactionCreated)
	}

	if event.AggregateID() != transactionID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), transactionID)
	}

	if event.TransactionID != transactionID {
		t.Errorf("TransactionID = %v, want %v", event.TransactionID, transactionID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if event.TransactionType != txType {
		t.Errorf("TransactionType = %q, want %q", event.TransactionType, txType)
	}

	if !event.Amount.Equals(amount) {
		t.Errorf("Amount = %v, want %v", event.Amount, amount)
	}

	if event.IdempotencyKey != idempotencyKey {
		t.Errorf("IdempotencyKey = %q, want %q", event.IdempotencyKey, idempotencyKey)
	}
}

// TestNewTransactionCompleted tests TransactionCompleted event creation
func TestNewTransactionCompleted(t *testing.T) {
	transactionID := uuid.New()
	walletID := uuid.New()
	txType := "DEPOSIT"
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	event := NewTransactionCompleted(transactionID, walletID, txType, amount)

	if event.EventType() != EventTypeTransactionCompleted {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeTransactionCompleted)
	}

	if event.AggregateID() != transactionID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), transactionID)
	}

	if event.TransactionID != transactionID {
		t.Errorf("TransactionID = %v, want %v", event.TransactionID, transactionID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if event.TransactionType != txType {
		t.Errorf("TransactionType = %q, want %q", event.TransactionType, txType)
	}

	if !event.Amount.Equals(amount) {
		t.Errorf("Amount = %v, want %v", event.Amount, amount)
	}

	if event.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}

// TestNewTransactionFailed tests TransactionFailed event creation
func TestNewTransactionFailed(t *testing.T) {
	transactionID := uuid.New()
	walletID := uuid.New()
	txType := "DEPOSIT"
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)
	failureReason := "Network timeout"
	isRetryable := true

	event := NewTransactionFailed(transactionID, walletID, txType, amount, failureReason, isRetryable)

	if event.EventType() != EventTypeTransactionFailed {
		t.Errorf("EventType = %q, want %q", event.EventType(), EventTypeTransactionFailed)
	}

	if event.AggregateID() != transactionID {
		t.Errorf("AggregateID = %v, want %v", event.AggregateID(), transactionID)
	}

	if event.TransactionID != transactionID {
		t.Errorf("TransactionID = %v, want %v", event.TransactionID, transactionID)
	}

	if event.WalletID != walletID {
		t.Errorf("WalletID = %v, want %v", event.WalletID, walletID)
	}

	if event.TransactionType != txType {
		t.Errorf("TransactionType = %q, want %q", event.TransactionType, txType)
	}

	if !event.Amount.Equals(amount) {
		t.Errorf("Amount = %v, want %v", event.Amount, amount)
	}

	if event.FailureReason != failureReason {
		t.Errorf("FailureReason = %q, want %q", event.FailureReason, failureReason)
	}

	if event.IsRetryable != isRetryable {
		t.Errorf("IsRetryable = %v, want %v", event.IsRetryable, isRetryable)
	}
}

// TestEventTypeConstants tests event type constants
func TestEventTypeConstants(t *testing.T) {
	constants := map[string]string{
		"EventTypeUserCreated":          EventTypeUserCreated,
		"EventTypeUserKYCApproved":      EventTypeUserKYCApproved,
		"EventTypeUserKYCRejected":      EventTypeUserKYCRejected,
		"EventTypeWalletCreated":        EventTypeWalletCreated,
		"EventTypeWalletCredited":       EventTypeWalletCredited,
		"EventTypeWalletDebited":        EventTypeWalletDebited,
		"EventTypeWalletSuspended":      EventTypeWalletSuspended,
		"EventTypeTransactionCreated":   EventTypeTransactionCreated,
		"EventTypeTransactionCompleted": EventTypeTransactionCompleted,
		"EventTypeTransactionFailed":    EventTypeTransactionFailed,
	}

	for name, value := range constants {
		if value == "" {
			t.Errorf("%s should not be empty", name)
		}
	}
}

// TestNewEventStore tests EventStore creation
func TestNewEventStore(t *testing.T) {
	store := NewEventStore()

	if store == nil {
		t.Fatal("NewEventStore should not return nil")
	}

	if store.Count() != 0 {
		t.Errorf("New store Count = %d, want 0", store.Count())
	}

	if len(store.GetAll()) != 0 {
		t.Errorf("New store should have empty events")
	}
}

// TestEventStore_Add tests adding events to store
func TestEventStore_Add(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()

	event1 := NewUserCreated(userID, "test@example.com", "Test User")
	event2 := NewUserKYCApproved(userID)

	store.Add(event1)

	if store.Count() != 1 {
		t.Errorf("Count after 1 add = %d, want 1", store.Count())
	}

	store.Add(event2)

	if store.Count() != 2 {
		t.Errorf("Count after 2 adds = %d, want 2", store.Count())
	}
}

// TestEventStore_GetAll tests retrieving all events
func TestEventStore_GetAll(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()

	event1 := NewUserCreated(userID, "test@example.com", "Test User")
	event2 := NewUserKYCApproved(userID)

	store.Add(event1)
	store.Add(event2)

	events := store.GetAll()

	if len(events) != 2 {
		t.Fatalf("GetAll() returned %d events, want 2", len(events))
	}

	// Check event types
	if events[0].EventType() != EventTypeUserCreated {
		t.Errorf("First event type = %q, want %q", events[0].EventType(), EventTypeUserCreated)
	}

	if events[1].EventType() != EventTypeUserKYCApproved {
		t.Errorf("Second event type = %q, want %q", events[1].EventType(), EventTypeUserKYCApproved)
	}
}

// TestEventStore_Clear tests clearing events
func TestEventStore_Clear(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()

	store.Add(NewUserCreated(userID, "test@example.com", "Test User"))
	store.Add(NewUserKYCApproved(userID))

	if store.Count() != 2 {
		t.Fatalf("Setup failed: Count = %d, want 2", store.Count())
	}

	store.Clear()

	if store.Count() != 0 {
		t.Errorf("Count after Clear() = %d, want 0", store.Count())
	}

	if len(store.GetAll()) != 0 {
		t.Error("GetAll() after Clear() should return empty slice")
	}
}

// TestEventStore_Count tests event counting
func TestEventStore_Count(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()

	tests := []struct {
		name     string
		action   func()
		expected int
	}{
		{"Initial count", func() {}, 0},
		{"After 1 add", func() { store.Add(NewUserCreated(userID, "test@example.com", "Test")) }, 1},
		{"After 2 adds", func() { store.Add(NewUserKYCApproved(userID)) }, 2},
		{"After 3 adds", func() { store.Add(NewUserKYCRejected(userID, "test")) }, 3},
		{"After clear", func() { store.Clear() }, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.action()
			if store.Count() != tt.expected {
				t.Errorf("Count = %d, want %d", store.Count(), tt.expected)
			}
		})
	}
}

// TestEventStore_MultipleEventTypes tests storing different event types
func TestEventStore_MultipleEventTypes(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()
	walletID := uuid.New()
	transactionID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	// Add different event types
	store.Add(NewUserCreated(userID, "test@example.com", "Test User"))
	store.Add(NewWalletCreated(walletID, userID, valueobjects.USD))
	store.Add(NewTransactionCreated(transactionID, walletID, "DEPOSIT", amount, "key-123"))

	events := store.GetAll()

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Verify each event can be type-asserted
	_, isUserCreated := events[0].(*UserCreated)
	_, isWalletCreated := events[1].(*WalletCreated)
	_, isTransactionCreated := events[2].(*TransactionCreated)

	if !isUserCreated {
		t.Error("First event should be UserCreated")
	}
	if !isWalletCreated {
		t.Error("Second event should be WalletCreated")
	}
	if !isTransactionCreated {
		t.Error("Third event should be TransactionCreated")
	}
}

// TestEventInterface_Compliance tests that all event types implement DomainEvent
func TestEventInterface_Compliance(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	transactionID := uuid.New()
	amount, _ := valueobjects.NewMoneyFromInt(100, valueobjects.USD)

	events := []DomainEvent{
		NewUserCreated(userID, "test@example.com", "Test User"),
		NewUserKYCApproved(userID),
		NewUserKYCRejected(userID, "reason"),
		NewWalletCreated(walletID, userID, valueobjects.USD),
		NewWalletCredited(walletID, amount, transactionID, amount),
		NewWalletDebited(walletID, amount, transactionID, amount),
		NewWalletSuspended(walletID, "reason"),
		NewTransactionCreated(transactionID, walletID, "DEPOSIT", amount, "key"),
		NewTransactionCompleted(transactionID, walletID, "DEPOSIT", amount),
		NewTransactionFailed(transactionID, walletID, "DEPOSIT", amount, "reason", true),
	}

	for i, event := range events {
		if event.EventID() == uuid.Nil {
			t.Errorf("Event %d: EventID should not be nil", i)
		}
		if event.EventType() == "" {
			t.Errorf("Event %d: EventType should not be empty", i)
		}
		if event.AggregateID() == uuid.Nil {
			t.Errorf("Event %d: AggregateID should not be nil", i)
		}
		if event.OccurredAt().IsZero() {
			t.Errorf("Event %d: OccurredAt should be set", i)
		}
	}
}

// TestEventStore_AddAfterClear tests that events can be added after clearing
func TestEventStore_AddAfterClear(t *testing.T) {
	store := NewEventStore()
	userID := uuid.New()

	store.Add(NewUserCreated(userID, "test1@example.com", "User 1"))
	store.Clear()
	store.Add(NewUserCreated(userID, "test2@example.com", "User 2"))

	if store.Count() != 1 {
		t.Errorf("Count after clear and add = %d, want 1", store.Count())
	}

	events := store.GetAll()
	if userCreated, ok := events[0].(*UserCreated); ok {
		if userCreated.Email != "test2@example.com" {
			t.Errorf("Event Email = %q, want test2@example.com", userCreated.Email)
		}
	} else {
		t.Error("Event should be UserCreated type")
	}
}
