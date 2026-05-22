package dispatch

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"dashfetchr/internal/carrier"
	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/core/routing"
	"dashfetchr/internal/ports"
)

// Service assigns a delivery to a carrier via the routing engine.
type Service struct {
	deps Deps
}

type Deps struct {
	Deliveries ports.DeliveryRepository
	AWBs       ports.AWBRepository
	Routing    *routing.Engine
	RoutingLog ports.RoutingDecisionRepository
	Carriers   *carrier.Registry
	Bus        ports.EventBus
	Logger     *slog.Logger
}

func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// DispatchDelivery runs routing, creates the carrier shipment, updates state.
func (s *Service) DispatchDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	del, err := s.deps.Deliveries.GetByID(ctx, deliveryID)
	if err != nil {
		return err
	}
	if del.State != delivery.StatePending {
		return fmt.Errorf("dispatch: delivery %s not pending (state=%s)", deliveryID, del.State)
	}

	a, err := s.deps.AWBs.GetByID(ctx, del.AWBID)
	if err != nil {
		return err
	}

	req := buildShipmentRequest(a, del)
	decision, err := s.deps.Routing.Decide(ctx, req, ports.RoleLastMile)
	if err != nil {
		return fmt.Errorf("dispatch: routing: %w", err)
	}

	_ = s.deps.RoutingLog.Save(ctx, deliveryID, ports.RoutingDecisionRecord{
		Strategy:  decision.Strategy,
		ChosenID:  string(decision.Chosen.Capabilities().ID),
		Score:     decision.Score,
		Reasoning: decision.Reasoning,
	})

	resp, err := decision.Chosen.CreateShipment(ctx, req)
	if err != nil {
		return fmt.Errorf("dispatch: carrier create: %w", err)
	}

	del.CarrierID = string(decision.Chosen.Capabilities().ID)
	del.CarrierExternalID = resp.ExternalID
	del.EstimatedPickupAt = resp.EstimatedPickupAt
	del.EstimatedDropAt = resp.EstimatedDropAt
	if err := del.Transition(delivery.StateAssigned, ""); err != nil {
		return err
	}
	if err := s.deps.Deliveries.Save(ctx, del); err != nil {
		return err
	}

	_ = s.deps.Bus.Publish(ctx, ports.RiderAssignedEvent{
		BaseEvent: ports.BaseEvent{
			Type:      "rider.assigned",
			Time:      decision.DecidedAt,
			ExtID:     resp.ExternalID,
			CarrierID: decision.Chosen.Capabilities().ID,
		},
	})

	s.deps.Logger.Info("dispatch.completed",
		"delivery_id", deliveryID,
		"carrier", del.CarrierID,
		"external_id", del.CarrierExternalID,
	)
	return nil
}

func buildShipmentRequest(a *awb.AWB, del *delivery.Delivery) ports.ShipmentRequest {
	return ports.ShipmentRequest{
		InternalAWB:    a.InternalAWB,
		IdempotencyKey: del.IdempotencyKey,
		Pickup: ports.Location{
			Type: toPickupMode(del.Pickup.Type),
			Point: ports.GeoPoint{Lat: del.Pickup.Lat, Lng: del.Pickup.Lng},
			Address: ports.Address{Street: del.Pickup.Address},
			LockerID: del.Pickup.LockerID,
		},
		Drop: ports.Location{
			Type: ports.PickupAddress,
			Point: ports.GeoPoint{Lat: del.Drop.Lat, Lng: del.Drop.Lng},
			Address: ports.Address{
				Street:       del.Drop.Address,
				Floor:        del.Drop.Floor,
				Apartment:    del.Drop.Apartment,
				Instructions: del.Drop.Instructions,
			},
		},
		Package: ports.Package{
			WeightKg: a.Package.WeightKg,
		},
		ScheduledStart: del.ScheduledWindowStart,
		ScheduledEnd:   del.ScheduledWindowEnd,
		RequirePhotoPOD: true,
	}
}

func toPickupMode(t delivery.LocationType) ports.PickupMode {
	switch t {
	case delivery.LocationLocker:
		return ports.PickupLocker
	case delivery.LocationHub:
		return ports.PickupHub
	default:
		return ports.PickupAddress
	}
}
