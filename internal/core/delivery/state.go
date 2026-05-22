package delivery

// State is the per-leg lifecycle. Each delivery (one carrier, one leg)
// has its own machine; the AWB aggregate coordinates multiple legs.
type State string

const (
	StatePending          State = "pending"
	StateAssigned         State = "assigned"
	StateEnRoutePickup    State = "en_route_pickup"
	StatePickedUp         State = "picked_up"
	StateInTransit        State = "in_transit"
	StateEnRouteDrop      State = "en_route_drop"
	StateDelivered        State = "delivered"
	StateFailed           State = "failed"
	StateCancelled        State = "cancelled"
	StateReturnedToOrigin State = "returned_to_origin"
)

// allowedTransitions describes the state graph. Any transition not
// listed here is rejected at runtime.
//
//	pending → assigned → en_route_pickup → picked_up → in_transit →
//	  en_route_drop → delivered
//
// Failure branches:
//   - failed at any active stage → returned_to_origin or retry (assigned)
//   - cancelled allowed only pre-pickup
var allowedTransitions = map[State]map[State]bool{
	StatePending: {
		StateAssigned:  true,
		StateCancelled: true,
	},
	StateAssigned: {
		StateEnRoutePickup: true,
		StateCancelled:     true,
		StateFailed:        true,
	},
	StateEnRoutePickup: {
		StatePickedUp:  true,
		StateFailed:    true,
		StateCancelled: true,
	},
	StatePickedUp: {
		StateInTransit:        true,
		StateEnRouteDrop:      true,
		StateFailed:           true,
		StateReturnedToOrigin: true,
	},
	StateInTransit: {
		StateEnRouteDrop:      true,
		StateFailed:           true,
		StateReturnedToOrigin: true,
		StateDelivered:        true,
	},
	StateEnRouteDrop: {
		StateDelivered:        true,
		StateFailed:           true,
		StateReturnedToOrigin: true,
	},
	StateFailed: {
		StateReturnedToOrigin: true,
		StateAssigned:         true, // retry
	},
	StateDelivered:        {},
	StateCancelled:        {},
	StateReturnedToOrigin: {},
}

// IsTerminal returns true if no further transitions are allowed.
func (s State) IsTerminal() bool {
	return s == StateDelivered || s == StateCancelled || s == StateReturnedToOrigin
}
