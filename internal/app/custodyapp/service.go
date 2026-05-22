package custodyapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"dashfetchr/internal/core/custody"
	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/ports"
)

// Service records custody events (photo + GPS + timestamp) and drives
// delivery state transitions when appropriate.
type Service struct {
	deps Deps
}

type Deps struct {
	Ledger     *custody.Ledger
	Deliveries ports.DeliveryRepository
	Bus        ports.EventBus
	Logger     *slog.Logger
}

func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// RecordInput is the rider PWA payload for a custody checkpoint.
type RecordInput struct {
	DeliveryID uuid.UUID
	Type       custody.EventType
	Actor      custody.Actor
	Location   *custody.GeoPoint
	Photos     []custody.PhotoRef
	Reason     string
}

// Record persists a custody event and updates delivery state when the
// event type implies a lifecycle change.
func (s *Service) Record(ctx context.Context, in RecordInput) (custody.Event, error) {
	del, err := s.deps.Deliveries.GetByID(ctx, in.DeliveryID)
	if err != nil {
		return custody.Event{}, err
	}

	e := custody.Event{
		DeliveryID: in.DeliveryID,
		Type:       in.Type,
		OccurredAt: time.Now().UTC(),
		Actor:      in.Actor,
		Location:   in.Location,
		Photos:     in.Photos,
		Reason:     in.Reason,
	}

	recorded, err := s.deps.Ledger.Record(ctx, e)
	if err != nil {
		return custody.Event{}, err
	}

	if err := s.applyDeliveryTransition(del, in.Type); err != nil {
		return recorded, err
	}
	if err := s.deps.Deliveries.Save(ctx, del); err != nil {
		return recorded, err
	}

	s.deps.Logger.Info("custody.recorded",
		"delivery_id", in.DeliveryID,
		"type", in.Type,
		"hash", recorded.Hash[:12],
	)
	return recorded, nil
}

// ListEvents returns the full chain for a delivery.
func (s *Service) ListEvents(ctx context.Context, deliveryID uuid.UUID) ([]custody.Event, error) {
	return s.deps.Ledger.List(ctx, deliveryID)
}

// VerifyChain checks hash-chain integrity for a delivery.
func (s *Service) VerifyChain(ctx context.Context, deliveryID uuid.UUID) error {
	return s.deps.Ledger.Verify(ctx, deliveryID)
}

func (s *Service) applyDeliveryTransition(del *delivery.Delivery, t custody.EventType) error {
	switch t {
	case custody.EventPackagePickedUp:
		return del.MarkPickedUp(time.Now().UTC())
	case custody.EventPackageDelivered:
		return del.MarkDelivered(time.Now().UTC())
	case custody.EventDeliveryFailed:
		return del.MarkFailed("custody_event")
	default:
		return nil
	}
}
