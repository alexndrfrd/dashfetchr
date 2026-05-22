// Package routing decides which carrier to use for a given shipment.
//
// The core principle: the engine NEVER references a specific carrier
// (Bolt, Glovo, Sameday, ...). It works exclusively against the
// CarrierPort interface and CarrierCapabilities. Adding a new carrier
// requires zero changes here.
package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dashfetchr/internal/ports"
)

// Engine selects the best carrier for a shipment using a pluggable
// Strategy (cheapest, fastest, weighted-score, ML, etc.).
type Engine struct {
	registry CarrierRegistry
	strategy Strategy
	clock    func() time.Time
}

// CarrierRegistry is the small subset of the global registry the engine
// depends on. Keeping it as an interface here lets tests inject fakes.
type CarrierRegistry interface {
	ListByRole(role ports.CarrierRole) []ports.CarrierPort
}

// NewEngine constructs a routing engine.
func NewEngine(reg CarrierRegistry, s Strategy) *Engine {
	return &Engine{registry: reg, strategy: s, clock: time.Now}
}

// Decide returns the chosen carrier together with an auditable
// RoutingDecision describing why. Callers should persist the decision
// before calling CreateShipment on the carrier (helps debug fallbacks).
func (e *Engine) Decide(ctx context.Context, req ports.ShipmentRequest, role ports.CarrierRole) (Decision, error) {
	candidates := e.eligible(req, role)
	if len(candidates) == 0 {
		return Decision{}, ErrNoEligibleCarrier
	}

	scored := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		est, err := c.GetEstimate(ctx, ports.EstimateRequest{
			Pickup:  req.Pickup,
			Drop:    req.Drop,
			Package: req.Package,
			When:    req.ScheduledStart,
		})
		if err != nil {
			scored = append(scored, Candidate{
				Carrier:   c,
				Available: false,
				Err:       err,
			})
			continue
		}
		scored = append(scored, Candidate{
			Carrier:   c,
			Estimate:  *est,
			Available: true,
		})
	}

	sel, err := e.strategy.Select(ctx, req, scored)
	if err != nil {
		return Decision{}, err
	}

	return Decision{
		Strategy:    e.strategy.Name(),
		Chosen:      sel.Carrier,
		Score:       sel.Score,
		Reasoning:   sel.Reasoning,
		Candidates:  scored,
		DecidedAt:   e.clock().UTC(),
	}, nil
}

// eligible filters the registered carriers by structural fit:
//   - Role match (last_mile vs mid_mile vs first_mile)
//   - Weight and dimensions within carrier limits
//   - Pickup type supported (e.g. locker access for our concierge flow)
//   - Zones cover the pickup and drop points (in v1: simple bounding box;
//     v2: GeoJSON polygon containment)
func (e *Engine) eligible(req ports.ShipmentRequest, role ports.CarrierRole) []ports.CarrierPort {
	out := make([]ports.CarrierPort, 0)
	for _, c := range e.registry.ListByRole(role) {
		caps := c.Capabilities()
		if !hasRole(caps.Roles, role) {
			continue
		}
		if req.Package.WeightKg > caps.MaxWeightKg {
			continue
		}
		if exceedsDims(req.Package, caps.MaxDimensionsCm) {
			continue
		}
		if !supportsPickup(caps, req.Pickup.Type) {
			continue
		}
		if req.Pickup.Type == ports.PickupLocker && !caps.SupportsLocker {
			continue
		}
		// Zone check would go here. For v1 we trust caller; for v2 implement
		// point-in-polygon against caps.Zones.
		out = append(out, c)
	}
	return out
}

func hasRole(roles []ports.CarrierRole, want ports.CarrierRole) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

func exceedsDims(p ports.Package, max [3]float64) bool {
	dims := [3]float64{p.LengthCm, p.WidthCm, p.HeightCm}
	// Sort both desc and compare; a package fits if for every dimension
	// the largest sorted matches.
	sortDesc(&dims)
	maxs := max
	sortDesc(&maxs)
	for i := range dims {
		if dims[i] > maxs[i] {
			return true
		}
	}
	return false
}

func sortDesc(a *[3]float64) {
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if a[j] > a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func supportsPickup(caps ports.CarrierCapabilities, t ports.PickupMode) bool {
	for _, m := range caps.PickupModes {
		if m == t {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// Decision is the auditable output of a routing call.
// ----------------------------------------------------------------------------

type Decision struct {
	Strategy   string
	Chosen     ports.CarrierPort
	Score      float64
	Reasoning  string
	Candidates []Candidate
	DecidedAt  time.Time
}

type Candidate struct {
	Carrier   ports.CarrierPort
	Estimate  ports.Estimate
	Available bool
	Err       error
}

// ----------------------------------------------------------------------------
// errors
// ----------------------------------------------------------------------------

var (
	ErrNoEligibleCarrier = errors.New("routing: no eligible carrier")
)

// CarrierID is a convenience accessor used by audit logging.
func (c Candidate) CarrierID() ports.CarrierID {
	if c.Carrier == nil {
		return ""
	}
	return c.Carrier.Capabilities().ID
}

// String makes Decision easier to log.
func (d Decision) String() string {
	if d.Chosen == nil {
		return fmt.Sprintf("strategy=%s no_chosen", d.Strategy)
	}
	return fmt.Sprintf("strategy=%s chosen=%s score=%.2f", d.Strategy, d.Chosen.Capabilities().ID, d.Score)
}
