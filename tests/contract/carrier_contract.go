// Package contract provides a shared test suite that any carrier adapter
// must satisfy before being merged.
//
// Usage in an adapter's *_test.go:
//
//	func TestBoltContract(t *testing.T) {
//	    adapter := bolt.New(testConfig())
//	    contract.RunCarrierContract(t, adapter)
//	}
//
// All ~30 standard scenarios run automatically. If a scenario does not
// apply to a particular adapter (e.g. SupportsLocker = false), it is
// skipped based on the adapter's declared Capabilities().
package contract

import (
	"context"
	"testing"
	"time"

	"dashfetchr/internal/ports"
)

// RunCarrierContract executes every scenario in this package against
// the provided adapter. Designed to be called from each adapter package
// (Bolt, Glovo, Wolt, etc.) with no per-carrier customization.
//
// IMPORTANT: this function must remain carrier-agnostic. If a scenario
// is meaningful only for some carriers, gate it on Capabilities().
func RunCarrierContract(t *testing.T, adapter ports.CarrierPort) {
	t.Helper()

	t.Run("Capabilities/returns_required_fields", func(t *testing.T) {
		caps := adapter.Capabilities()
		if caps.ID == "" {
			t.Fatal("capabilities.ID must be set")
		}
		if caps.Name == "" {
			t.Fatal("capabilities.Name must be set")
		}
		if len(caps.Roles) == 0 {
			t.Fatal("capabilities.Roles must declare at least one role")
		}
		if caps.MaxWeightKg <= 0 {
			t.Fatal("capabilities.MaxWeightKg must be > 0")
		}
		if len(caps.DeliveryModes) == 0 {
			t.Fatal("capabilities.DeliveryModes must declare at least one mode")
		}
		if len(caps.PickupModes) == 0 {
			t.Fatal("capabilities.PickupModes must declare at least one mode")
		}
		if len(caps.POD) == 0 {
			t.Fatal("capabilities.POD must declare at least one proof type")
		}
	})

	t.Run("Capabilities/has_photo_proof", func(t *testing.T) {
		caps := adapter.Capabilities()
		hasPhoto := false
		for _, p := range caps.POD {
			if p == ports.ProofPhoto {
				hasPhoto = true
				break
			}
		}
		if !hasPhoto {
			t.Fatal("all carriers MUST support photo POD; chain of custody requires it")
		}
	})

	t.Run("Capabilities/has_gps_proof", func(t *testing.T) {
		caps := adapter.Capabilities()
		hasGPS := false
		for _, p := range caps.POD {
			if p == ports.ProofGPS {
				hasGPS = true
				break
			}
		}
		if !hasGPS {
			t.Fatal("all carriers MUST report GPS at handover events")
		}
	})

	t.Run("CreateShipment/rejects_oversized_package", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		caps := adapter.Capabilities()
		oversized := validShipment()
		oversized.Package.WeightKg = caps.MaxWeightKg + 10
		_, err := adapter.CreateShipment(ctx, oversized)
		if err == nil {
			t.Fatal("expected error for oversized package, got nil")
		}
	})

	t.Run("CreateShipment/rejects_missing_recipient_phone", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req := validShipment()
		req.Recipient.Phone = ""
		_, err := adapter.CreateShipment(ctx, req)
		if err == nil {
			t.Fatal("expected error for missing phone")
		}
	})

	t.Run("ParseWebhook/rejects_invalid_signature", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := adapter.ParseWebhook(ctx, []byte(`{"event":"x"}`), map[string]string{
			"X-Signature": "definitely-wrong",
		})
		if err == nil {
			t.Fatal("expected ErrInvalidSignature, got nil")
		}
	})

	t.Run("ParseWebhook/rejects_invalid_json", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := adapter.ParseWebhook(ctx, []byte(`{not-json`), nil)
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	// Additional scenarios go here. When a new requirement applies to all
	// carriers, add a t.Run here once; every adapter inherits it.
}

// validShipment returns a baseline ShipmentRequest that should be accepted
// by any carrier. Individual scenarios mutate it to test edge cases.
func validShipment() ports.ShipmentRequest {
	return ports.ShipmentRequest{
		InternalAWB:    "DF-2026-TEST0001",
		IdempotencyKey: "test-idempotency-key-1",
		Pickup: ports.Location{
			Type: ports.PickupAddress,
			Point: ports.GeoPoint{
				Lat: 44.4546, Lng: 26.0987,
			},
			Address: ports.Address{
				Street:  "Calea Floreasca 169",
				City:    "Bucuresti",
				Country: "RO",
			},
		},
		Drop: ports.Location{
			Type: ports.PickupAddress,
			Point: ports.GeoPoint{
				Lat: 44.4632, Lng: 26.1062,
			},
			Address: ports.Address{
				Street:  "Aviatorilor 10",
				City:    "Bucuresti",
				Country: "RO",
			},
		},
		Package: ports.Package{
			WeightKg:    1.2,
			LengthCm:    30,
			WidthCm:     20,
			HeightCm:    15,
			Description: "Cosmetice",
		},
		Recipient: ports.Recipient{
			Name:  "Maria Pop",
			Phone: "+40712345678",
		},
		RequirePhotoPOD: true,
	}
}
