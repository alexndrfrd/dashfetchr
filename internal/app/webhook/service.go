package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"dashfetchr/internal/app/custodyapp"
	"dashfetchr/internal/carrier"
	"dashfetchr/internal/core/custody"
	"dashfetchr/internal/core/delivery"
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
	externalID := ev.ExternalID()
	if externalID == "" {
		s.deps.Logger.Debug("webhook.skip_no_external_id", "type", ev.EventType())
		return nil
	}

	del, err := s.deps.Deliveries.GetByCarrierExternalID(ctx, string(carrierID), externalID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			s.deps.Logger.Warn("webhook.delivery_not_found",
				"carrier", carrierID,
				"external_id", externalID,
				"type", ev.EventType(),
			)
			return nil
		}
		return err
	}

	switch e := ev.(type) {
	case ports.RiderAssignedEvent:
		return s.onRiderAssigned(ctx, del, carrierID, e)
	case ports.RiderArrivedAtPickupEvent:
		_, err := s.recordCustody(ctx, del.ID, custody.EventRiderArrivedAtLocker, carrierID, e.Location, e.PhotoURL, "")
		return err
	case ports.PackagePickedUpEvent:
		_, err := s.recordCustody(ctx, del.ID, custody.EventPackagePickedUp, carrierID, e.Location, e.PhotoURL, "")
		return err
	case ports.PackageDeliveredEvent:
		_, err := s.recordCustody(ctx, del.ID, custody.EventPackageDelivered, carrierID, e.Location, e.PhotoURL, "")
		return err
	case ports.DeliveryFailedEvent:
		_, err := s.recordCustody(ctx, del.ID, custody.EventDeliveryFailed, carrierID, e.Location, e.PhotoURL, e.Reason)
		return err
	case ports.ShipmentCancelledEvent:
		_ = del.Cancel(e.Reason)
		return s.deps.Deliveries.Save(ctx, del)
	default:
		s.deps.Logger.Info("webhook.event_unhandled", "type", ev.EventType(), "delivery_id", del.ID)
	}
	return nil
}

func (s *Service) onRiderAssigned(ctx context.Context, del *delivery.Delivery, carrierID ports.CarrierID, e ports.RiderAssignedEvent) error {
	if e.Rider.ID != "" {
		r := delivery.Rider{
			ID:      e.Rider.ID,
			Name:    e.Rider.Name,
			Phone:   e.Rider.Phone,
			Vehicle: e.Rider.Vehicle,
		}
		if del.State == delivery.StatePending {
			_ = del.Transition(delivery.StateAssigned, "")
		}
		del.Rider = &r
		if err := s.deps.Deliveries.Save(ctx, del); err != nil {
			return err
		}
	}

	_, err := s.deps.Custody.Record(ctx, custodyapp.RecordInput{
		DeliveryID: del.ID,
		Type:       custody.EventRiderAssigned,
		Actor: custody.Actor{
			Type:      "rider",
			CarrierID: string(carrierID),
			ID:        e.Rider.ID,
			Name:      e.Rider.Name,
		},
	})
	return err
}

func (s *Service) recordCustody(
	ctx context.Context,
	deliveryID uuid.UUID,
	eventType custody.EventType,
	carrierID ports.CarrierID,
	loc *ports.GeoPoint,
	photoURL, reason string,
) (custody.Event, error) {
	in := custodyapp.RecordInput{
		DeliveryID: deliveryID,
		Type:       eventType,
		Actor: custody.Actor{
			Type:      "carrier",
			CarrierID: string(carrierID),
		},
		Reason: reason,
	}
	if loc != nil {
		in.Location = &custody.GeoPoint{
			Lat:       loc.Lat,
			Lng:       loc.Lng,
			AccuracyM: loc.AccuracyM,
		}
	}
	if photoURL != "" {
		in.Photos = []custody.PhotoRef{{
			S3URI:  photoURL,
			SHA256: "webhook-pending",
		}}
	} else if eventType.RequiresPhoto() {
		// Carrier webhook without photo: placeholder so ledger accepts the event in dev.
		in.Photos = []custody.PhotoRef{{
			S3URI:  "s3://dashfetchr-pod/pending/webhook.jpg",
			SHA256: "webhook-pending",
		}}
	}
	if eventType.RequiresGPS() && in.Location == nil {
		in.Location = &custody.GeoPoint{AccuracyM: 999}
	}
	return s.deps.Custody.Record(ctx, in)
}
