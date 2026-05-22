// Package awb manages DashFetchr-internal Air Way Bill identifiers.
//
// We assign every parcel that enters our system a normalized internal AWB
// (e.g. "DF-2026-AB12CD34"). External carriers each have their own
// tracking numbers; we map them to ours via the AWB aggregate so the
// rest of the system can refer to a single stable identity.
package awb

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AWB is the aggregate root that ties together one logical parcel and
// all the carrier-side tracking numbers used to move it.
type AWB struct {
	ID            uuid.UUID
	InternalAWB   string
	RetailerID    uuid.UUID
	CustomerID    *uuid.UUID
	ExternalAWBs  []ExternalRef
	Package       Package
	State         State
	StateReason   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ExternalRef binds a carrier's own tracking number to our internal AWB.
type ExternalRef struct {
	Carrier string // "sameday", "bolt_food", ...
	AWB     string
	Role    Role
}

type Role string

const (
	RoleFirstMile Role = "first_mile"
	RoleMidMile   Role = "mid_mile"
	RoleLastMile  Role = "last_mile"
)

type Package struct {
	WeightKg           float64
	LengthCm           float64
	WidthCm            float64
	HeightCm           float64
	DeclaredValueMinor int64
	DeclaredValueCcy   string
	Description        string
	Fragile            bool
}

// State represents the high-level lifecycle of a parcel from DashFetchr'
// point of view. It is intentionally separate from the per-leg Delivery
// state machine.
type State string

const (
	StateCreated    State = "created"
	StateAtLocker   State = "at_locker"
	StateScheduled  State = "scheduled"
	StateDispatched State = "dispatched"
	StateInTransit  State = "in_transit"
	StateDelivered  State = "delivered"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
	StateReturned   State = "returned"
)

// New creates a new AWB with a freshly generated internal identifier.
func New(retailerID uuid.UUID, pkg Package) (*AWB, error) {
	if err := validatePackage(pkg); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	internal, err := generateInternalAWB(now)
	if err != nil {
		return nil, err
	}
	return &AWB{
		ID:          uuid.New(),
		InternalAWB: internal,
		RetailerID:  retailerID,
		Package:     pkg,
		State:       StateCreated,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// AttachExternal links a carrier-side tracking number to this AWB.
// Idempotent: attaching the same (carrier, awb) twice is a no-op.
func (a *AWB) AttachExternal(carrier, externalAWB string, role Role) {
	for _, ref := range a.ExternalAWBs {
		if ref.Carrier == carrier && ref.AWB == externalAWB {
			return
		}
	}
	a.ExternalAWBs = append(a.ExternalAWBs, ExternalRef{
		Carrier: carrier,
		AWB:     externalAWB,
		Role:    role,
	})
	a.UpdatedAt = time.Now().UTC()
}

// FindExternal returns the carrier-side tracking number for a given carrier.
func (a *AWB) FindExternal(carrier string) (ExternalRef, bool) {
	for _, ref := range a.ExternalAWBs {
		if ref.Carrier == carrier {
			return ref, true
		}
	}
	return ExternalRef{}, false
}

// Transition moves the AWB to a new state, enforcing the allowed graph.
func (a *AWB) Transition(to State, reason string) error {
	if !allowedTransitions[a.State][to] {
		return fmt.Errorf("awb: invalid transition %s -> %s", a.State, to)
	}
	a.State = to
	a.StateReason = reason
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// allowedTransitions is the AWB-level state machine. Per-leg deliveries
// have their own, finer-grained machine (see internal/core/delivery).
var allowedTransitions = map[State]map[State]bool{
	StateCreated: {
		StateAtLocker: true, StateDispatched: true, StateCancelled: true,
	},
	StateAtLocker: {
		StateScheduled: true, StateCancelled: true, StateReturned: true,
	},
	StateScheduled: {
		StateDispatched: true, StateCancelled: true, StateAtLocker: true,
	},
	StateDispatched: {
		StateInTransit: true, StateFailed: true, StateCancelled: true,
	},
	StateInTransit: {
		StateDelivered: true, StateFailed: true,
	},
	StateFailed: {
		StateReturned: true, StateDispatched: true, StateCancelled: true,
	},
	StateDelivered: {},
	StateCancelled: {},
	StateReturned:  {StateScheduled: true, StateCancelled: true},
}

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

var (
	ErrPackageTooHeavy = errors.New("awb: package weight exceeds limit (50kg)")
	ErrPackageZero     = errors.New("awb: package weight must be > 0")
	ErrPackageDimension = errors.New("awb: package dimension exceeds limit (100cm)")
)

func validatePackage(p Package) error {
	if p.WeightKg <= 0 {
		return ErrPackageZero
	}
	if p.WeightKg > 50 {
		return ErrPackageTooHeavy
	}
	for _, d := range []float64{p.LengthCm, p.WidthCm, p.HeightCm} {
		if d > 100 {
			return ErrPackageDimension
		}
	}
	return nil
}

func generateInternalAWB(t time.Time) (string, error) {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	enc := strings.TrimRight(base32.StdEncoding.EncodeToString(b[:]), "=")
	return fmt.Sprintf("DF-%d-%s", t.Year(), enc), nil
}
