package ports

import (
	"context"

	"github.com/google/uuid"

	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/core/delivery"
)

// AWBRepository persists AWB aggregates.
type AWBRepository interface {
	Save(ctx context.Context, a *awb.AWB) error
	GetByID(ctx context.Context, id uuid.UUID) (*awb.AWB, error)
	GetByInternalAWB(ctx context.Context, internalAWB string) (*awb.AWB, error)
}

// DeliveryRepository persists Delivery aggregates.
type DeliveryRepository interface {
	Save(ctx context.Context, d *delivery.Delivery) error
	GetByID(ctx context.Context, id uuid.UUID) (*delivery.Delivery, error)
	ListByAWB(ctx context.Context, awbID uuid.UUID) ([]*delivery.Delivery, error)
}

// RoutingDecisionRepository stores audit records for carrier selection.
type RoutingDecisionRepository interface {
	Save(ctx context.Context, deliveryID uuid.UUID, record RoutingDecisionRecord) error
}

// RoutingDecisionRecord is the persisted form of a routing decision.
type RoutingDecisionRecord struct {
	Strategy   string
	ChosenID   string
	Score      float64
	Reasoning  string
	Candidates string // JSON blob in real impl
}
