// Package natsadapter provides NATS JetStream integration for event publishing.
package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

// natsHeaderCarrier adapts nats.Header for OpenTelemetry propagation.
type natsHeaderCarrier nats.Header

func (c natsHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}

func (c natsHeaderCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// Publish publishes an event message to NATS with trace context propagation.
func (p *Publisher) Publish(ctx context.Context, msg *EventMessage) error {
	tracer := otel.Tracer("paybridge.nats.publisher")
	ctx, span := tracer.Start(ctx, "nats.publish",
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", msg.EventType),
			attribute.String("messaging.event_id", msg.EventID),
		),
	)
	defer span.End()

	data, err := json.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Convert event type to NATS subject: "wallet.credited" → "paybridge.events.wallet.credited"
	subject := "paybridge.events." + strings.ReplaceAll(msg.EventType, ".", ".")

	// Inject trace context into NATS headers
	natsMsg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(natsMsg.Header))

	_, err = p.js.PublishMsg(natsMsg)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	p.logger.Debug("Published event to NATS",
		slog.String("event_id", msg.EventID),
		slog.String("subject", subject),
	)

	return nil
}
