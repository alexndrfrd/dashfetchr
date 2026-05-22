package booking

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/core/pricing"
	"dashfetchr/internal/ports"
)

// Service handles customer bookings: locker → home on a chosen interval.
type Service struct {
	deps Deps
}

type Deps struct {
	AWBs       ports.AWBRepository
	Deliveries ports.DeliveryRepository
	Pricing    *pricing.Engine
	Logger     *slog.Logger
}

func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// CreateBookingInput is the PWA payload for scheduling a concierge delivery.
type CreateBookingInput struct {
	RetailerID       uuid.UUID
	ExternalAWB      string // Sameday/eMAG tracking the customer pasted
	ExternalCarrier  string // "sameday", "fan", etc.
	CustomerPhone    string
	CustomerName     string

	PickupLockerID   string
	PickupLat        float64
	PickupLng        float64
	PickupAddress    string

	DropLat          float64
	DropLng          float64
	DropAddress      string
	DropFloor        string
	DropApartment    string
	DropInstructions string

	WeightKg float64
	SlotStart time.Time
	SlotEnd   time.Time

	LastMileCarrier string // optional override; empty = routing engine decides later
}

// CreateBookingResult is returned to the PWA after a successful booking.
type CreateBookingResult struct {
	AWBID           uuid.UUID
	InternalAWB     string
	DeliveryID      uuid.UUID
	QuotedPriceMinor int64
	Currency        string
	State           string
}

// CreateBooking creates the AWB + last-mile delivery leg in pending state.
func (s *Service) CreateBooking(ctx context.Context, in CreateBookingInput) (CreateBookingResult, error) {
	if in.SlotEnd.Before(in.SlotStart) {
		return CreateBookingResult{}, fmt.Errorf("booking: invalid slot")
	}

	pkg := awb.Package{
		WeightKg: in.WeightKg,
	}
	if pkg.WeightKg <= 0 {
		pkg.WeightKg = 1
	}

	a, err := awb.New(in.RetailerID, pkg)
	if err != nil {
		return CreateBookingResult{}, err
	}
	if in.ExternalAWB != "" {
		a.AttachExternal(in.ExternalCarrier, in.ExternalAWB, awb.RoleMidMile)
	}
	if err := a.Transition(awb.StateAtLocker, "customer_requested_concierge"); err != nil {
		return CreateBookingResult{}, err
	}

	carrierID := in.LastMileCarrier
	if carrierID == "" {
		carrierID = "bolt_food"
	}

	del, err := delivery.New(a.ID, 1, carrierID, "v1",
		delivery.Location{
			Type:     delivery.LocationLocker,
			Lat:      in.PickupLat,
			Lng:      in.PickupLng,
			Address:  in.PickupAddress,
			LockerID: in.PickupLockerID,
		},
		delivery.Location{
			Type:         delivery.LocationAddress,
			Lat:          in.DropLat,
			Lng:          in.DropLng,
			Address:      in.DropAddress,
			Floor:        in.DropFloor,
			Apartment:    in.DropApartment,
			Instructions: in.DropInstructions,
		},
	)
	if err != nil {
		return CreateBookingResult{}, err
	}
	if err := del.Schedule(in.SlotStart, in.SlotEnd); err != nil {
		return CreateBookingResult{}, err
	}
	del.IdempotencyKey = del.ID.String()

	quote := s.deps.Pricing.Quote(pricing.Request{
		Pickup: pricing.Coord{Lat: in.PickupLat, Lng: in.PickupLng},
		Drop:   pricing.Coord{Lat: in.DropLat, Lng: in.DropLng},
		WeightKg: in.WeightKg,
		SlotStart: &in.SlotStart,
	})
	del.PriceQuotedMinor = quote.Total.Minor
	del.PriceCurrency = quote.Currency

	if err := s.deps.AWBs.Save(ctx, a); err != nil {
		return CreateBookingResult{}, err
	}
	if err := s.deps.Deliveries.Save(ctx, del); err != nil {
		return CreateBookingResult{}, err
	}

	s.deps.Logger.Info("booking.created",
		"awb", a.InternalAWB,
		"delivery_id", del.ID,
		"quoted_minor", del.PriceQuotedMinor,
	)

	return CreateBookingResult{
		AWBID:            a.ID,
		InternalAWB:      a.InternalAWB,
		DeliveryID:       del.ID,
		QuotedPriceMinor: del.PriceQuotedMinor,
		Currency:         del.PriceCurrency,
		State:            string(del.State),
	}, nil
}

// GetBooking returns AWB + deliveries for status polling.
func (s *Service) GetBooking(ctx context.Context, deliveryID uuid.UUID) (*delivery.Delivery, *awb.AWB, error) {
	del, err := s.deps.Deliveries.GetByID(ctx, deliveryID)
	if err != nil {
		return nil, nil, err
	}
	a, err := s.deps.AWBs.GetByID(ctx, del.AWBID)
	if err != nil {
		return nil, nil, err
	}
	return del, a, nil
}
