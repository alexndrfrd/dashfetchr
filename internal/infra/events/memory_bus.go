package events

import (
	"context"
	"log/slog"
	"sync"

	"dashfetchr/internal/ports"
)

// MemoryBus is a synchronous in-process event bus for local dev.
// Handlers run inline; production should use SQS with retries.
type MemoryBus struct {
	mu       sync.RWMutex
	handlers []ports.EventHandler
	logger   *slog.Logger
}

func NewMemoryBus(logger *slog.Logger) *MemoryBus {
	return &MemoryBus{logger: logger}
}

func (b *MemoryBus) Subscribe(h ports.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

func (b *MemoryBus) Publish(ctx context.Context, events ...ports.DomainEvent) error {
	b.mu.RLock()
	handlers := append([]ports.EventHandler(nil), b.handlers...)
	b.mu.RUnlock()

	for _, e := range events {
		b.logger.Info("event.published",
			"type", e.EventType(),
			"external_id", e.ExternalID(),
		)
		for _, h := range handlers {
			if err := h.Handle(ctx, e); err != nil {
				b.logger.Error("event.handler_failed",
					"type", e.EventType(),
					"err", err,
				)
			}
		}
	}
	return nil
}

// AuditHandler logs every event (skeleton consumer).
type AuditHandler struct {
	Logger *slog.Logger
}

func (h AuditHandler) Handle(ctx context.Context, e ports.DomainEvent) error {
	_ = ctx
	h.Logger.Info("audit.event",
		"type", e.EventType(),
		"external_id", e.ExternalID(),
		"at", e.OccurredAt(),
	)
	return nil
}
