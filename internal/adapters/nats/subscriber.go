package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EventHandler processes an event message.
type EventHandler func(ctx context.Context, msg *EventMessage) error

// Subscriber subscribes to NATS JetStream events.
type Subscriber struct {
	js       nats.JetStreamContext
	logger   *slog.Logger
	handlers map[string]EventHandler
	subs     []*nats.Subscription
}

// NewSubscriber creates a new NATS subscriber.
func NewSubscriber(nc *nats.Conn, logger *slog.Logger) (*Subscriber, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Subscriber{
		js:       js,
		logger:   logger,
		handlers: make(map[string]EventHandler),
	}, nil
}

// Handle registers a handler for a specific event type subject.
func (s *Subscriber) Handle(subject string, handler EventHandler) {
	s.handlers[subject] = handler
}

// Start begins consuming messages from all registered subjects.
func (s *Subscriber) Start(ctx context.Context) error {
	for subject, handler := range s.handlers {
		h := handler // capture for closure
		subj := subject
		// Each subject needs a unique durable consumer name
		durableName := "notifier-" + strings.ReplaceAll(subject, ".", "-")

		sub, err := s.js.Subscribe(subject, func(msg *nats.Msg) {
			var eventMsg EventMessage
			if err := json.Unmarshal(msg.Data, &eventMsg); err != nil {
				s.logger.Error("Failed to unmarshal event", slog.String("error", err.Error()))
				_ = msg.Nak()
				return
			}

			// Extract trace context from NATS headers
			propagator := otel.GetTextMapPropagator()
			msgCtx := propagator.Extract(ctx, natsHeaderCarrier(msg.Header))

			tracer := otel.Tracer("paybridge.nats.subscriber")
			msgCtx, span := tracer.Start(msgCtx, "nats.process",
				trace.WithAttributes(
					attribute.String("messaging.system", "nats"),
					attribute.String("messaging.source", subj),
					attribute.String("messaging.event_id", eventMsg.EventID),
					attribute.String("messaging.event_type", eventMsg.EventType),
				),
			)

			if err := h(msgCtx, &eventMsg); err != nil {
				span.RecordError(err)
				s.logger.Error("Failed to handle event",
					slog.String("event_id", eventMsg.EventID),
					slog.String("event_type", eventMsg.EventType),
					slog.String("error", err.Error()),
				)
				span.End()
				_ = msg.Nak()
				return
			}

			span.End()
			_ = msg.Ack()
		}, nats.Durable(durableName), nats.ManualAck())

		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}

		s.subs = append(s.subs, sub)
		s.logger.Info("Subscribed to NATS subject", slog.String("subject", subject))
	}

	return nil
}

// Stop unsubscribes from all subjects.
func (s *Subscriber) Stop() error {
	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.logger.Error("Failed to unsubscribe", slog.String("error", err.Error()))
		}
	}
	return nil
}
