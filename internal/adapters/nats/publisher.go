// Package natsadapter provides NATS JetStream integration for event publishing.
package natsadapter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Publisher publishes events to NATS JetStream.
type Publisher struct {
	js     nats.JetStreamContext
	logger *slog.Logger
}

// EventMessage is the message format published to NATS.
type EventMessage struct {
	EventID     string          `json:"event_id"`
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	Payload     json.RawMessage `json:"payload"`
	OccurredAt  time.Time       `json:"occurred_at"`
}

// NewPublisher creates a new NATS publisher.
func NewPublisher(nc *nats.Conn, streamName string, logger *slog.Logger) (*Publisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	// Create stream if it doesn't exist
	_, err = js.StreamInfo(streamName)
	if err != nil {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{"paybridge.events.>"},
			Retention: nats.WorkQueuePolicy,
			MaxAge:    24 * time.Hour,
			Storage:   nats.FileStorage,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create stream: %w", err)
		}
		logger.Info("Created NATS JetStream stream", slog.String("name", streamName))
	}

	return &Publisher{js: js, logger: logger}, nil
}

// Publish publishes an event message to NATS.
func (p *Publisher) Publish(msg *EventMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Convert event type to NATS subject: "wallet.credited" → "paybridge.events.wallet.credited"
	subject := "paybridge.events." + strings.ReplaceAll(msg.EventType, ".", ".")

	_, err = p.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	p.logger.Debug("Published event to NATS",
		slog.String("event_id", msg.EventID),
		slog.String("subject", subject),
	)

	return nil
}
