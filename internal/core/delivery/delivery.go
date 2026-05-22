// Package delivery models the concrete movement of a parcel by one
// carrier on one leg of its journey.
//
// One AWB can have multiple Deliveries:
//   - Leg 1: Sameday warehouse -> easybox (mid-mile)
//   - Leg 2: easybox -> customer door (last-mile, our concierge service)
//
// Each Delivery has its own state machine; cross-leg coordination
// happens at the AWB aggregate level.
package delivery

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Delivery is the leg-level aggregate.
type Delivery struct {
	ID                  uuid.UUID
	AWBID               uuid.UUID
	LegNumber           int
	CarrierID           string
	CarrierVersion      string
	CarrierExternalID   string

	Pickup              Location
	Drop                Location
	ScheduledWindowStart *time.Time
	ScheduledWindowEnd   *time.Time

	State       State
	StateReason string

	Rider               *Rider
	PriceQuotedMinor    int64
	PriceChargedMinor   int64
	PriceCurrency       string

	EstimatedPickupAt   *time.Time
	EstimatedDropAt     *time.Time
	ActualPickupAt      *time.Time
	ActualDropAt        *time.Time

	IdempotencyKey string
	Metadata       map[string]any

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Location is a leg endpoint. The type distinguishes pickups/drops at
// addresses, lockers, or hubs.
type Location struct {
	Type     LocationType
	Lat      float64
	Lng      float64
	Address  string
	LockerID string // when Type == LocationLocker
	HubID    string // when Type == LocationHub
	Floor    string
	Apartment string
	Instructions string
}

type LocationType string

const (
	LocationAddress LocationType = "address"
	LocationLocker  LocationType = "locker"
	LocationHub     LocationType = "hub"
)

type Rider struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string
}

// New creates a Delivery in the Pending state.
func New(awbID uuid.UUID, leg int, carrierID, version string, pickup, drop Location) (*Delivery, error) {
	if leg < 1 {
		return nil, errors.New("delivery: leg must be >= 1")
	}
	if carrierID == "" || version == "" {
		return nil, errors.New("delivery: carrier ID and version required")
	}
	now := time.Now().UTC()
	return &Delivery{
		ID:             uuid.New(),
		AWBID:          awbID,
		LegNumber:      leg,
		CarrierID:      carrierID,
		CarrierVersion: version,
		Pickup:         pickup,
		Drop:           drop,
		State:          StatePending,
		PriceCurrency:  "RON",
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Schedule sets the customer-chosen delivery window.
func (d *Delivery) Schedule(start, end time.Time) error {
	if !start.Before(end) {
		return errors.New("delivery: schedule end must be after start")
	}
	if start.Before(time.Now().UTC()) {
		return errors.New("delivery: schedule cannot be in the past")
	}
	d.ScheduledWindowStart = &start
	d.ScheduledWindowEnd = &end
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// AssignRider records that a carrier has assigned a specific rider.
func (d *Delivery) AssignRider(r Rider) error {
	if err := d.Transition(StateAssigned, ""); err != nil {
		return err
	}
	d.Rider = &r
	return nil
}

// MarkPickedUp records that the rider has the package in hand.
// Must be preceded by photo+GPS evidence in the custody ledger.
func (d *Delivery) MarkPickedUp(at time.Time) error {
	if err := d.Transition(StatePickedUp, ""); err != nil {
		return err
	}
	d.ActualPickupAt = &at
	return nil
}

// MarkDelivered records successful delivery to the recipient.
func (d *Delivery) MarkDelivered(at time.Time) error {
	if err := d.Transition(StateDelivered, ""); err != nil {
		return err
	}
	d.ActualDropAt = &at
	return nil
}

// MarkFailed records a failed delivery attempt with a reason code.
func (d *Delivery) MarkFailed(reason string) error {
	return d.Transition(StateFailed, reason)
}

// Cancel attempts to cancel; only allowed before the rider has picked up.
func (d *Delivery) Cancel(reason string) error {
	if d.State == StatePickedUp || d.State == StateInTransit ||
		d.State == StateEnRouteDrop || d.State == StateDelivered {
		return fmt.Errorf("delivery: cannot cancel from state %s", d.State)
	}
	return d.Transition(StateCancelled, reason)
}

// Transition is the public state-machine driver. It is also called
// from the high-level methods above.
func (d *Delivery) Transition(to State, reason string) error {
	if !d.canTransitionTo(to) {
		return fmt.Errorf("delivery: invalid transition %s -> %s", d.State, to)
	}
	d.State = to
	d.StateReason = reason
	d.UpdatedAt = time.Now().UTC()
	return nil
}

func (d *Delivery) canTransitionTo(to State) bool {
	allowed, ok := allowedTransitions[d.State]
	if !ok {
		return false
	}
	return allowed[to]
}
