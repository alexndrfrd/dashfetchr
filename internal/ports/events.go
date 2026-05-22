package ports

import "time"

// DomainEvent is the contract for all events flowing through the system.
// Carrier adapters parse webhooks into domain events; the core domain
// consumes these events without ever knowing they came from a specific
// carrier.
type DomainEvent interface {
	EventType() string
	OccurredAt() time.Time
	ExternalID() string
}

// ----------------------------------------------------------------------------
// Standard events emitted by carrier adapters (via ParseWebhook)
// ----------------------------------------------------------------------------

type BaseEvent struct {
	Type      string
	Time      time.Time
	ExtID     string
	CarrierID CarrierID
}

func (b BaseEvent) EventType() string     { return b.Type }
func (b BaseEvent) OccurredAt() time.Time { return b.Time }
func (b BaseEvent) ExternalID() string    { return b.ExtID }

type RiderAssignedEvent struct {
	BaseEvent
	Rider Rider
}

type RiderArrivedAtPickupEvent struct {
	BaseEvent
	Location *GeoPoint
	PhotoURL string
}

type PackagePickedUpEvent struct {
	BaseEvent
	Location *GeoPoint
	PhotoURL string
}

type RiderInTransitEvent struct {
	BaseEvent
	Location *GeoPoint
}

type RiderArrivedAtDestinationEvent struct {
	BaseEvent
	Location *GeoPoint
}

type PackageDeliveredEvent struct {
	BaseEvent
	Location  *GeoPoint
	PhotoURL  string
	Signature string
}

type DeliveryFailedEvent struct {
	BaseEvent
	Reason   string
	Location *GeoPoint
	PhotoURL string
}

type ShipmentCancelledEvent struct {
	BaseEvent
	Reason string
}

type PackageReturnedToLockerEvent struct {
	BaseEvent
	Location *GeoPoint
	PhotoURL string
	LockerID string
}
