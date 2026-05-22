package bolt

import (
	"errors"
	"time"

	"dashfetchr/internal/ports"
)

// Mapper is the Anti-Corruption Layer: it is the ONLY component that
// knows both Bolt's vocabulary and the domain's vocabulary. If Bolt
// changes their API tomorrow, this is the only file we touch.
type Mapper struct {
	serviceArea string
}

func NewMapper(serviceArea string) *Mapper {
	return &Mapper{serviceArea: serviceArea}
}

// ----------------------------------------------------------------------------
// Domain -> Bolt
// ----------------------------------------------------------------------------

func (m *Mapper) ToBoltCreateOrder(r ports.ShipmentRequest) (boltCreateOrderRequest, error) {
	if r.Recipient.Phone == "" {
		return boltCreateOrderRequest{}, errors.New("bolt: recipient phone required")
	}
	pickup, err := m.toBoltLocation(r.Pickup)
	if err != nil {
		return boltCreateOrderRequest{}, err
	}
	drop, err := m.toBoltLocation(r.Drop)
	if err != nil {
		return boltCreateOrderRequest{}, err
	}

	pkg, err := m.toBoltPackage(r.Package)
	if err != nil {
		return boltCreateOrderRequest{}, err
	}

	return boltCreateOrderRequest{
		ExternalRef: r.IdempotencyKey,
		Pickup:      pickup,
		Drop:        drop,
		Package:     pkg,
		Recipient: boltRecipient{
			Name:  r.Recipient.Name,
			Phone: r.Recipient.Phone,
		},
		ServiceArea:      m.serviceArea,
		RequiresPhotoPOD: true, // always require photo POD; do not let Bolt skip
		ScheduledFor:     r.ScheduledStart,
	}, nil
}

func (m *Mapper) ToBoltEstimate(r ports.EstimateRequest) (boltEstimateRequest, error) {
	pickup, err := m.toBoltLocation(r.Pickup)
	if err != nil {
		return boltEstimateRequest{}, err
	}
	drop, err := m.toBoltLocation(r.Drop)
	if err != nil {
		return boltEstimateRequest{}, err
	}
	pkg, err := m.toBoltPackage(r.Package)
	if err != nil {
		return boltEstimateRequest{}, err
	}
	return boltEstimateRequest{
		Pickup:       pickup,
		Drop:         drop,
		Package:      pkg,
		ServiceArea:  m.serviceArea,
		ScheduledFor: r.When,
	}, nil
}

func (m *Mapper) toBoltLocation(l ports.Location) (boltLocation, error) {
	if l.Point.Lat == 0 && l.Point.Lng == 0 {
		return boltLocation{}, errors.New("bolt: location requires coordinates")
	}
	return boltLocation{
		Lat:       l.Point.Lat,
		Lng:       l.Point.Lng,
		Address:   l.Address.Street + ", " + l.Address.City,
		Floor:     l.Address.Floor,
		Apartment: l.Address.Apartment,
		Notes:     l.Address.Instructions,
	}, nil
}

func (m *Mapper) toBoltPackage(p ports.Package) (boltPackage, error) {
	if p.WeightKg <= 0 {
		return boltPackage{}, ports.ErrInvalidRequest
	}
	return boltPackage{
		WeightGrams: int(p.WeightKg * 1000),
		Dimensions: boltPackageDims{
			LengthCm: int(p.LengthCm),
			WidthCm:  int(p.WidthCm),
			HeightCm: int(p.HeightCm),
		},
		Description: p.Description,
		Value:       int(p.DeclaredValueMinor),
		Currency:    p.DeclaredValueCcy,
	}, nil
}

// ----------------------------------------------------------------------------
// Bolt -> Domain
// ----------------------------------------------------------------------------

func (m *Mapper) FromBoltOrder(o *boltOrder) *ports.ShipmentResponse {
	return &ports.ShipmentResponse{
		ExternalID: o.OrderID,
		State:      m.fromBoltStatus(o.Status),
		EstimatedCost: ports.Money{
			Currency: o.Price.Currency,
			Minor:    int64(o.Price.Cents),
		},
		EstimatedPickupAt: o.EstimatedPickup,
		EstimatedDropAt:   o.EstimatedDrop,
		TrackingURL:       o.TrackingURL,
	}
}

func (m *Mapper) FromBoltOrderStatus(o *boltOrder) *ports.ShipmentStatus {
	var rider *ports.Rider
	if o.Rider != nil {
		rider = &ports.Rider{
			ID:      o.Rider.ID,
			Name:    o.Rider.Name,
			Phone:   o.Rider.Phone,
			Vehicle: o.Rider.Vehicle,
		}
	}
	var loc *ports.GeoPoint
	if o.LastLocation != nil {
		loc = &ports.GeoPoint{
			Lat: o.LastLocation.Lat,
			Lng: o.LastLocation.Lng,
		}
	}
	return &ports.ShipmentStatus{
		ExternalID:    o.OrderID,
		State:         m.fromBoltStatus(o.Status),
		Rider:         rider,
		LastLocation:  loc,
		LastUpdatedAt: o.UpdatedAt,
	}
}

func (m *Mapper) FromBoltEstimate(e *boltEstimateResponse) *ports.Estimate {
	return &ports.Estimate{
		Cost: ports.Money{
			Currency: e.Price.Currency,
			Minor:    int64(e.Price.Cents),
		},
		EstimatedETA:    time.Duration(e.EstimatedETASec) * time.Second,
		ConfidenceScore: e.Confidence,
	}
}

func (m *Mapper) fromBoltStatus(s string) ports.ShipmentState {
	switch s {
	case "pending":
		return ports.StatePending
	case "assigned":
		return ports.StateAssigned
	case "picking_up", "en_route_pickup":
		return ports.StateRiderEnRouteToPickup
	case "picked_up":
		return ports.StatePickedUp
	case "delivering", "in_transit":
		return ports.StateInTransit
	case "arrived_at_drop":
		return ports.StateRiderEnRouteToDrop
	case "delivered", "completed":
		return ports.StateDelivered
	case "failed":
		return ports.StateFailed
	case "cancelled":
		return ports.StateCancelled
	default:
		return ports.StateUnknown
	}
}

// ----------------------------------------------------------------------------
// Webhook -> Domain events
// ----------------------------------------------------------------------------

func (m *Mapper) ToDomainEvents(wh boltWebhookPayload) []ports.DomainEvent {
	base := ports.BaseEvent{
		Time:      wh.Timestamp,
		ExtID:     wh.OrderID,
		CarrierID: "bolt_food",
	}

	switch wh.Event {
	case "order.assigned":
		base.Type = "rider.assigned"
		var rider ports.Rider
		if wh.Rider != nil {
			rider = ports.Rider{
				ID:      wh.Rider.ID,
				Name:    wh.Rider.Name,
				Phone:   wh.Rider.Phone,
				Vehicle: wh.Rider.Vehicle,
			}
		}
		return []ports.DomainEvent{ports.RiderAssignedEvent{
			BaseEvent: base,
			Rider:     rider,
		}}

	case "order.arrived_at_pickup":
		base.Type = "rider.arrived_at_pickup"
		return []ports.DomainEvent{ports.RiderArrivedAtPickupEvent{
			BaseEvent: base,
			Location:  fromBoltGeo(wh.Location),
			PhotoURL:  wh.PhotoURL,
		}}

	case "order.picked_up":
		base.Type = "package.picked_up"
		return []ports.DomainEvent{ports.PackagePickedUpEvent{
			BaseEvent: base,
			Location:  fromBoltGeo(wh.Location),
			PhotoURL:  wh.PhotoURL,
		}}

	case "order.in_transit":
		base.Type = "rider.in_transit"
		return []ports.DomainEvent{ports.RiderInTransitEvent{
			BaseEvent: base,
			Location:  fromBoltGeo(wh.Location),
		}}

	case "order.arrived_at_drop":
		base.Type = "rider.arrived_at_destination"
		return []ports.DomainEvent{ports.RiderArrivedAtDestinationEvent{
			BaseEvent: base,
			Location:  fromBoltGeo(wh.Location),
		}}

	case "order.delivered":
		base.Type = "package.delivered"
		return []ports.DomainEvent{ports.PackageDeliveredEvent{
			BaseEvent: base,
			Location:  fromBoltGeo(wh.Location),
			PhotoURL:  wh.PhotoURL,
		}}

	case "order.failed":
		base.Type = "delivery.failed"
		return []ports.DomainEvent{ports.DeliveryFailedEvent{
			BaseEvent: base,
			Reason:    wh.Reason,
			Location:  fromBoltGeo(wh.Location),
			PhotoURL:  wh.PhotoURL,
		}}

	case "order.cancelled":
		base.Type = "shipment.cancelled"
		return []ports.DomainEvent{ports.ShipmentCancelledEvent{
			BaseEvent: base,
			Reason:    wh.Reason,
		}}
	}

	// Unknown event type: don't crash, just emit nothing. The webhook
	// listener logs the raw payload for later inspection.
	return nil
}

func fromBoltGeo(l *boltLocation) *ports.GeoPoint {
	if l == nil {
		return nil
	}
	return &ports.GeoPoint{Lat: l.Lat, Lng: l.Lng}
}
