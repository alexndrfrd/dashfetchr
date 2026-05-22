// Package ports defines the interfaces (driven ports) that the core domain
// uses to communicate with the outside world. Implementations live in
// internal/adapters.
//
// Core domain code MUST NOT import any adapter package directly. The only
// way to talk to an external system (carrier API, payment provider, etc.)
// is through one of these ports.
package ports

import (
	"context"
	"errors"
	"time"
)

// ----------------------------------------------------------------------------
// CarrierPort — interface that every carrier adapter must implement.
//
// Adding a new carrier (Wolt, FAN, DHL, etc.) is just a matter of:
//  1. Implementing this interface
//  2. Declaring its capabilities
//  3. Mapping its data structures via an Anti-Corruption Layer
//  4. Registering the adapter
//
// See docs/ADDING_A_CARRIER.md for the full playbook.
// ----------------------------------------------------------------------------

type CarrierPort interface {
	// Capabilities is what the routing engine uses to decide whether this
	// carrier is even eligible for a given shipment. Must be stable and
	// cheap to call (cached in memory).
	Capabilities() CarrierCapabilities

	// CreateShipment dispatches a new shipment with this carrier.
	// Must be idempotent on IdempotencyKey.
	CreateShipment(ctx context.Context, req ShipmentRequest) (*ShipmentResponse, error)

	// GetShipmentStatus returns the current status from the carrier (truth
	// source for reconciliation).
	GetShipmentStatus(ctx context.Context, externalID string) (*ShipmentStatus, error)

	// CancelShipment attempts to cancel; carrier may refuse if already picked up.
	CancelShipment(ctx context.Context, externalID string, reason string) error

	// GetEstimate returns a quote (price, ETA) without creating a shipment.
	// Used by the routing engine to compare candidates.
	GetEstimate(ctx context.Context, req EstimateRequest) (*Estimate, error)

	// ParseWebhook converts a raw carrier webhook into domain events.
	// MUST verify signature first; return ErrInvalidSignature if it fails.
	ParseWebhook(ctx context.Context, raw []byte, headers map[string]string) ([]DomainEvent, error)
}

// ----------------------------------------------------------------------------
// Capabilities
// ----------------------------------------------------------------------------

type CarrierID string

type CarrierRole string

const (
	RoleFirstMile CarrierRole = "first_mile"
	RoleMidMile   CarrierRole = "mid_mile"
	RoleLastMile  CarrierRole = "last_mile"
)

type DeliveryMode string

const (
	DeliveryInstant   DeliveryMode = "instant"
	DeliveryScheduled DeliveryMode = "scheduled"
)

type PickupMode string

const (
	PickupAddress PickupMode = "address"
	PickupLocker  PickupMode = "locker"
	PickupHub     PickupMode = "hub"
)

type ProofType string

const (
	ProofPhoto     ProofType = "photo"
	ProofGPS       ProofType = "gps"
	ProofSignature ProofType = "signature"
)

type AuthScheme string

const (
	AuthBearerToken AuthScheme = "bearer_token"
	AuthAPIKey      AuthScheme = "api_key"
	AuthOAuth2      AuthScheme = "oauth2"
	AuthHMAC        AuthScheme = "hmac"
)

type SLA struct {
	AvgPickupTime   time.Duration
	AvgDeliveryTime time.Duration
	MaxDelay        time.Duration
}

type RateLimit struct {
	Requests  int
	PerSecond float64
}

type GeoZone struct {
	// GeoJSON Polygon coordinates (lng, lat) — counterclockwise
	Name        string
	Coordinates [][]float64
}

type CarrierCapabilities struct {
	ID              CarrierID
	Name            string
	Roles           []CarrierRole
	MaxWeightKg     float64
	MaxDimensionsCm [3]float64
	Zones           []GeoZone
	DeliveryModes   []DeliveryMode
	PickupModes     []PickupMode
	POD             []ProofType
	SLA             SLA
	SupportsLocker  bool
	AuthScheme      AuthScheme
	APIRateLimit    RateLimit
	Quirks          []string
}

// ----------------------------------------------------------------------------
// Shipment types (domain language, not carrier-specific)
// ----------------------------------------------------------------------------

type Money struct {
	Currency string // ISO 4217
	Minor    int64  // amount in minor units (bani for RON, cents for EUR)
}

type GeoPoint struct {
	Lat       float64
	Lng       float64
	AccuracyM float64
}

type Address struct {
	Street      string
	City        string
	PostalCode  string
	Country     string
	Floor       string
	Apartment   string
	Instructions string
}

type Location struct {
	Type     PickupMode // address | locker | hub
	Point    GeoPoint
	Address  Address
	LockerID string // when Type == PickupLocker
	HubID    string // when Type == PickupHub
}

type Package struct {
	WeightKg            float64
	LengthCm            float64
	WidthCm             float64
	HeightCm            float64
	DeclaredValueMinor  int64
	DeclaredValueCcy    string
	Description         string
	Fragile             bool
}

type Recipient struct {
	Name  string
	Phone string
	Email string
}

type ShipmentRequest struct {
	InternalAWB     string
	IdempotencyKey  string
	Pickup          Location
	Drop            Location
	Package         Package
	Recipient       Recipient
	ScheduledStart  *time.Time
	ScheduledEnd    *time.Time
	Notes           string
	RequirePhotoPOD bool
}

type ShipmentResponse struct {
	ExternalID        string
	State             ShipmentState
	EstimatedCost     Money
	EstimatedPickupAt *time.Time
	EstimatedDropAt   *time.Time
	TrackingURL       string
}

type ShipmentState string

const (
	StateUnknown                ShipmentState = "unknown"
	StatePending                ShipmentState = "pending"
	StateAssigned               ShipmentState = "assigned"
	StateRiderEnRouteToPickup   ShipmentState = "en_route_pickup"
	StatePickedUp               ShipmentState = "picked_up"
	StateInTransit              ShipmentState = "in_transit"
	StateRiderEnRouteToDrop     ShipmentState = "en_route_drop"
	StateDelivered              ShipmentState = "delivered"
	StateFailed                 ShipmentState = "failed"
	StateCancelled              ShipmentState = "cancelled"
	StateReturnedToOrigin       ShipmentState = "returned_to_origin"
)

type ShipmentStatus struct {
	ExternalID    string
	State         ShipmentState
	Rider         *Rider
	LastLocation  *GeoPoint
	LastUpdatedAt time.Time
}

type Rider struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string
}

type EstimateRequest struct {
	Pickup   Location
	Drop     Location
	Package  Package
	When     *time.Time // nil for instant
}

type Estimate struct {
	Cost            Money
	EstimatedETA    time.Duration
	WindowStartAt   *time.Time
	WindowEndAt     *time.Time
	ConfidenceScore float64 // 0..1
}

// ----------------------------------------------------------------------------
// Standard errors carriers should map their failures to.
//
// Routing engine uses error type to decide fallback behavior:
//   - ErrCarrierUnavailable, ErrCapacityExceeded → try next candidate
//   - ErrOversizedPackage, ErrOutOfZone → don't retry, return to caller
//   - ErrRateLimited → backoff, retry same carrier
// ----------------------------------------------------------------------------

var (
	ErrCarrierUnavailable  = errors.New("carrier: unavailable")
	ErrInvalidRequest      = errors.New("carrier: invalid request")
	ErrCapacityExceeded    = errors.New("carrier: capacity exceeded")
	ErrOutOfZone           = errors.New("carrier: out of zone")
	ErrOversizedPackage    = errors.New("carrier: oversized package")
	ErrInvalidSignature    = errors.New("carrier: invalid webhook signature")
	ErrIdempotencyMismatch = errors.New("carrier: idempotency key mismatch")
	ErrRateLimited         = errors.New("carrier: rate limited")
	ErrAuthFailed          = errors.New("carrier: authentication failed")
	ErrShipmentNotFound    = errors.New("carrier: shipment not found")
	ErrAlreadyCancelled    = errors.New("carrier: shipment already cancelled")
)
