package bolt

import (
	"time"

	"dashfetchr/internal/ports"
)

// buildCapabilities declares what Bolt Food can do. These values are
// read by the routing engine to decide whether Bolt is eligible for a
// given shipment.
//
// IMPORTANT: keep this accurate; misleading capabilities cause the
// engine to dispatch shipments Bolt then rejects.
func buildCapabilities(cfg Config) ports.CarrierCapabilities {
	return ports.CarrierCapabilities{
		ID:    "bolt_food",
		Name:  "Bolt Food (Bolt Delivery)",
		Roles: []ports.CarrierRole{ports.RoleLastMile},

		// Bolt riders are typically on motorcycle/scooter; medium capacity.
		MaxWeightKg:     15,
		MaxDimensionsCm: [3]float64{60, 40, 40},

		// Zones come from the service area configured at registration.
		// In v1 we hardcode Bucharest sectors; v2 will load polygons from DB.
		Zones: bucharestZones(),

		DeliveryModes: []ports.DeliveryMode{
			ports.DeliveryInstant,
			ports.DeliveryScheduled, // up to 24h ahead
		},
		PickupModes: []ports.PickupMode{
			ports.PickupAddress,
			// Bolt riders can pick up from lockers ONLY if the customer
			// or partner pre-opens the locker (no native locker API).
			// We model this as SupportsLocker = false; the LockerAccessPort
			// handles the pre-opening flow separately.
			ports.PickupHub,
		},
		POD: []ports.ProofType{
			ports.ProofPhoto,
			ports.ProofGPS,
		},
		SLA: ports.SLA{
			AvgPickupTime:   12 * time.Minute,
			AvgDeliveryTime: 25 * time.Minute,
			MaxDelay:        15 * time.Minute,
		},

		SupportsLocker: false,

		AuthScheme: ports.AuthBearerToken,
		APIRateLimit: ports.RateLimit{
			Requests:  60,
			PerSecond: 1,
		},
		Quirks: []string{
			"Bolt riders cannot natively open Sameday lockers; pickup requires pre-opened drop or partner human at pickup",
			"Webhook retries up to 5 times with exponential backoff; idempotency required on consumer side",
			"Service area metadata in capabilities is approximate; engine should still call GetEstimate to confirm",
		},
	}
}

// bucharestZones returns the bounding polygons we consider in pilot.
// In v2 these come from DB and are full GeoJSON.
func bucharestZones() []ports.GeoZone {
	return []ports.GeoZone{
		{
			Name: "bucharest_sector_1",
			Coordinates: [][]float64{
				// Approximate bounding box; replace with real polygon in prod.
				{26.05, 44.50},
				{26.15, 44.50},
				{26.15, 44.55},
				{26.05, 44.55},
				{26.05, 44.50},
			},
		},
	}
}
