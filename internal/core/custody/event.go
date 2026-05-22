// Package custody implements the chain-of-custody ledger.
//
// Every state change for a parcel is recorded as an immutable, append-only,
// hash-chained event. Each event includes (where applicable):
//   - server timestamp (NTP-synced, source of truth)
//   - carrier timestamp (for reconciliation)
//   - actor (rider/system/customer)
//   - GPS location with accuracy
//   - one or more photos (S3 URI + sha256)
//   - prev_hash + hash (Merkle-style chain, tamper-evident)
//
// The ledger is verifiable: walking the events for a delivery and
// recomputing hashes must yield the stored hash sequence. Any divergence
// indicates tampering.
package custody

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// EventType enumerates the lifecycle events we track. The set must be
// kept in sync with the documentation in docs/PRODUCT.md §7.
type EventType string

const (
	EventPackageDroppedAtLocker     EventType = "package.dropped_at_locker"
	EventPickupRequestedByCustomer  EventType = "pickup.requested_by_customer"
	EventRiderAssigned              EventType = "rider.assigned"
	EventRiderArrivedAtLocker       EventType = "rider.arrived_at_locker"
	EventPackagePickedUp            EventType = "package.picked_up"
	EventRiderInTransit             EventType = "rider.in_transit"
	EventRiderArrivedAtDestination  EventType = "rider.arrived_at_destination"
	EventPackageDelivered           EventType = "package.delivered"
	EventDeliveryFailed             EventType = "delivery.failed"
	EventPackageReturnedToLocker    EventType = "package.returned_to_locker"
)

// Event is the immutable record.
type Event struct {
	EventID      uuid.UUID
	DeliveryID   uuid.UUID
	SequenceNum  int64

	Type         EventType
	OccurredAt   time.Time  // server time (truth)
	CarrierTime  *time.Time // optional, as reported by carrier

	Actor        Actor
	Location     *GeoPoint
	Photos       []PhotoRef
	Signature    *SignatureRef
	Reason       string
	Metadata     json.RawMessage

	PrevHash string
	Hash     string
}

type Actor struct {
	Type      string // "rider" | "system" | "customer" | "carrier" | "admin"
	CarrierID string // when Type == "rider" or "carrier"
	ID        string
	Name      string
}

type GeoPoint struct {
	Lat       float64
	Lng       float64
	AccuracyM float64
	Altitude  float64
	Heading   float64
}

type PhotoRef struct {
	S3URI       string
	SHA256      string
	ContentType string
	Width       int
	Height      int
	ExifGPS     *GeoPoint
}

type SignatureRef struct {
	S3URI  string
	SHA256 string
}

// ----------------------------------------------------------------------------
// Required photo policy (used at validation time)
// ----------------------------------------------------------------------------

// RequiresPhoto returns true for events that MUST carry at least one PhotoRef.
// Recording such an event without a photo fails validation.
func (t EventType) RequiresPhoto() bool {
	switch t {
	case EventRiderArrivedAtLocker,
		EventPackagePickedUp,
		EventPackageDelivered,
		EventDeliveryFailed,
		EventPackageReturnedToLocker:
		return true
	}
	return false
}

// RequiresGPS returns true for events that must carry a GeoPoint.
func (t EventType) RequiresGPS() bool {
	switch t {
	case EventRiderArrivedAtLocker,
		EventPackagePickedUp,
		EventRiderInTransit,
		EventRiderArrivedAtDestination,
		EventPackageDelivered,
		EventDeliveryFailed,
		EventPackageReturnedToLocker:
		return true
	}
	return false
}

var (
	ErrPhotoRequired = errors.New("custody: event type requires photo")
	ErrGPSRequired   = errors.New("custody: event type requires GPS location")
	ErrInvalidEvent  = errors.New("custody: invalid event")
)

// Validate verifies the event meets the minimum requirements for its type.
func (e *Event) Validate() error {
	if e.Type == "" {
		return ErrInvalidEvent
	}
	if e.DeliveryID == uuid.Nil {
		return ErrInvalidEvent
	}
	if e.OccurredAt.IsZero() {
		return ErrInvalidEvent
	}
	if e.Type.RequiresPhoto() && len(e.Photos) == 0 {
		return ErrPhotoRequired
	}
	if e.Type.RequiresGPS() && e.Location == nil {
		return ErrGPSRequired
	}
	return nil
}
