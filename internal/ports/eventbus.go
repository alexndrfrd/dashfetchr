package ports

import "context"

// EventBus publishes domain events to async consumers (notifications,
// audit, retailer webhooks, analytics). Implementations: in-memory (local),
// SQS (v1 prod), Kafka (v2 scale).
type EventBus interface {
	Publish(ctx context.Context, events ...DomainEvent) error
}

// EventHandler processes a single domain event. Workers subscribe by type.
type EventHandler interface {
	Handle(ctx context.Context, e DomainEvent) error
}
