// Package poller provides an outbox poller that reads unpublished events and publishes to NATS.
package poller

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	natsadapter "github.com/Haleralex/wallethub/internal/adapters/nats"
	"github.com/Haleralex/wallethub/internal/application/ports"
)

// OutboxPoller reads unpublished events from the outbox and publishes them to NATS.
type OutboxPoller struct {
	outboxRepo   ports.OutboxRepository
	publisher    *natsadapter.Publisher
	logger       *slog.Logger
	pollInterval time.Duration
	batchSize    int
	maxRetries   int
	stopCh       chan struct{}
}

// Config holds outbox poller configuration.
type Config struct {
	PollInterval time.Duration
	BatchSize    int
	MaxRetries   int
}

// New creates a new OutboxPoller.
func New(
	outboxRepo ports.OutboxRepository,
	publisher *natsadapter.Publisher,
	logger *slog.Logger,
	cfg Config,
) *OutboxPoller {
	return &OutboxPoller{
		outboxRepo:   outboxRepo,
		publisher:    publisher,
		logger:       logger,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		maxRetries:   cfg.MaxRetries,
		stopCh:       make(chan struct{}),
	}
}

// Start begins polling the outbox for unpublished events.
func (p *OutboxPoller) Start(ctx context.Context) {
	p.logger.Info("Outbox poller started",
		slog.Duration("interval", p.pollInterval),
		slog.Int("batch_size", p.batchSize),
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Outbox poller stopped (context cancelled)")
			return
		case <-p.stopCh:
			p.logger.Info("Outbox poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// Stop signals the poller to stop.
func (p *OutboxPoller) Stop() {
	close(p.stopCh)
}

func (p *OutboxPoller) poll(ctx context.Context) {
	events, err := p.outboxRepo.FindUnpublished(ctx, p.batchSize)
	if err != nil {
		p.logger.Error("Failed to find unpublished events", slog.String("error", err.Error()))
		return
	}

	if len(events) == 0 {
		return
	}

	p.logger.Debug("Found unpublished events", slog.Int("count", len(events)))

	for _, event := range events {
		// Use raw payload from outbox if available (genericEvent stores it),
		// otherwise fall back to JSON marshaling.
		var payload []byte
		type payloader interface {
			Payload() []byte
		}
		if pl, ok := event.(payloader); ok {
			payload = pl.Payload()
		} else {
			var err error
			payload, err = json.Marshal(event)
			if err != nil {
				p.logger.Error("Failed to marshal event",
					slog.String("event_id", event.EventID().String()),
					slog.String("error", err.Error()),
				)
				_ = p.outboxRepo.MarkFailed(ctx, event.EventID().String(), err.Error())
				continue
			}
		}

		msg := &natsadapter.EventMessage{
			EventID:     event.EventID().String(),
			EventType:   event.EventType(),
			AggregateID: event.AggregateID().String(),
			Payload:     payload,
			OccurredAt:  event.OccurredAt(),
		}

		if err := p.publisher.Publish(msg); err != nil {
			p.logger.Error("Failed to publish event to NATS",
				slog.String("event_id", event.EventID().String()),
				slog.String("error", err.Error()),
			)
			_ = p.outboxRepo.MarkFailed(ctx, event.EventID().String(), err.Error())
			continue
		}

		if err := p.outboxRepo.MarkPublished(ctx, event.EventID().String()); err != nil {
			p.logger.Error("Failed to mark event as published",
				slog.String("event_id", event.EventID().String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		p.logger.Debug("Event published and marked",
			slog.String("event_id", event.EventID().String()),
			slog.String("type", event.EventType()),
		)
	}
}
