//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/core/custody"
	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/infra/postgres"
)

func TestReposRoundTrip(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://dashfetchr:dashfetchr@localhost:5432/dashfetchr?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, url, 4)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	retailerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	awbRepo := postgres.NewAWBRepo(pool)
	delRepo := postgres.NewDeliveryRepo(pool)
	custodyRepo := postgres.NewCustodyRepo(pool)

	a, err := awb.New(retailerID, awb.Package{WeightKg: 1.2})
	if err != nil {
		t.Fatal(err)
	}
	if err := awbRepo.Save(ctx, a); err != nil {
		t.Fatal(err)
	}

	del, err := delivery.New(a.ID, 1, "bolt_food", "v1",
		delivery.Location{Type: delivery.LocationLocker, Lat: 44.45, Lng: 26.09, LockerID: "L1"},
		delivery.Location{Type: delivery.LocationAddress, Lat: 44.46, Lng: 26.10, Address: "Test"},
	)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(time.Hour)
	if err := del.Schedule(start, end); err != nil {
		t.Fatal(err)
	}
	del.PriceQuotedMinor = 1200
	if err := delRepo.Save(ctx, del); err != nil {
		t.Fatal(err)
	}

	got, err := delRepo.GetByID(ctx, del.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PriceQuotedMinor != 1200 {
		t.Fatalf("quoted price: got %d", got.PriceQuotedMinor)
	}

	extID := "bolt-order-" + del.ID.String()
	del.CarrierExternalID = extID
	if err := delRepo.Save(ctx, del); err != nil {
		t.Fatal(err)
	}
	byExt, err := delRepo.GetByCarrierExternalID(ctx, "bolt_food", extID)
	if err != nil {
		t.Fatal(err)
	}
	if byExt.ID != del.ID {
		t.Fatalf("external lookup: got %s", byExt.ID)
	}

	ledger := custody.NewLedger(custodyRepo)
	_, err = ledger.Record(ctx, custody.Event{
		DeliveryID: del.ID,
		Type:       custody.EventPickupRequestedByCustomer,
		OccurredAt: time.Now().UTC(),
		Actor:      custody.Actor{Type: "customer", ID: "c1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Verify(ctx, del.ID); err != nil {
		t.Fatal(err)
	}
}
