package webhook

import (
	"context"
	"fmt"
	"log/slog"

	"dashfetchr/internal/app/custodyapp"
	"dashfetchr/internal/carrier"
	"dashfetchr/internal/ports"
)

// Service processes inbound carrier webhooks into custody events.
type Service struct {
	deps Deps
}

type Deps struct {
	Carriers   *carrier.Registry
	Custody    *custodyapp.Service
	Deliveries ports.DeliveryRepository
	Logger     *slog.Logger
}

func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// Process ingests a raw webhook for the given carrier.
func (s *Service) Process(ctx context.Context, carrierID ports.CarrierID, raw []byte, headers map[string]string) error {
	c, err := s.deps.Carriers.Get(carrierID)
	if err != nil {
		return err
	}

	events, err := c.ParseWebhook(ctx, raw, headers)
	if err != nil {
		return fmt.Errorf("webhook: parse: %w", err)
	}

	for _, ev := range events {
		if err := s.handleDomainEvent(ctx, carrierID, ev); err != nil {
			s.deps.Logger.Error("webhook.event_failed", "type", ev.EventType(), "err", err)
		}
	}
	return nil
}

func (s *Service) handleDomainEvent(ctx context.Context, carrierID ports.CarrierID, ev ports.DomainEvent) error {
	// Skeleton: map carrier external ID → our delivery, then record custody.
	// Full implementation in M1 will query by carrier_external_id.
	_ = carrierID
	_ = ev

	switch e := ev.(type) {
	case ports.PackageDeliveredEvent:
		s.deps.Logger.Info("webhook.delivered", "external_id", e.ExternalID())
	case ports.DeliveryFailedEvent:
		s.deps.Logger.Info("webhook.failed", "external_id", e.ExternalID(), "reason", e.Reason)
	default:
		s.deps.Logger.Info("webhook.event", "type", ev.EventType())
	}
	return nil
}
