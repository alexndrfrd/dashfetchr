// Package pricing computes the price DashFetchr charges the customer
// for a delivery. The output is what the customer sees in the PWA before
// confirming the booking.
//
// Price = base + distance * km_rate + size_modifier + slot_premium − discounts
//
// Carrier cost is a separate concern (driven by the carrier estimate).
// Pricing here is what the customer pays us.
package pricing

import (
	"math"
	"time"

	"dashfetchr/internal/ports"
)

// Quote describes the resulting price breakdown.
type Quote struct {
	BaseFare         ports.Money
	DistanceFare     ports.Money
	SizeModifier     ports.Money
	SlotPremium      ports.Money
	Discounts        []Discount
	Subtotal         ports.Money
	Total            ports.Money
	DistanceMeters   float64
	Currency         string
}

type Discount struct {
	Code   string
	Amount ports.Money
}

// Config tunes the pricing engine. Loaded from DB so it can be changed
// without redeploy.
type Config struct {
	Currency       string
	BaseFareMinor  int64   // e.g. 800 = 8.00 RON
	PerKmMinor     int64   // e.g. 200 = 2.00 RON / km
	OversizeFlatMinor int64
	OversizeWeightKg float64
	SlotPremiumMinor int64 // premium for evening slots
	EveningStart   int     // hour of day, e.g. 18
	EveningEnd     int     // hour of day, e.g. 22
}

// Default returns a sensible default config for Romania pilot (RON).
func Default() Config {
	return Config{
		Currency:          "RON",
		BaseFareMinor:     800,
		PerKmMinor:        200,
		OversizeFlatMinor: 300,
		OversizeWeightKg:  5.0,
		SlotPremiumMinor:  200,
		EveningStart:      18,
		EveningEnd:        22,
	}
}

// Engine computes price quotes.
type Engine struct {
	cfg Config
}

func NewEngine(cfg Config) *Engine { return &Engine{cfg: cfg} }

// Inputs to the quote.
type Request struct {
	Pickup       Coord
	Drop         Coord
	WeightKg     float64
	SlotStart    *time.Time
	DiscountCode string
}

type Coord struct {
	Lat float64
	Lng float64
}

// Quote returns the breakdown for the given inputs.
func (e *Engine) Quote(r Request) Quote {
	dist := haversineMeters(r.Pickup, r.Drop)

	base := ports.Money{Currency: e.cfg.Currency, Minor: e.cfg.BaseFareMinor}
	distance := ports.Money{
		Currency: e.cfg.Currency,
		Minor:    int64(math.Round(dist / 1000.0 * float64(e.cfg.PerKmMinor))),
	}

	size := ports.Money{Currency: e.cfg.Currency}
	if r.WeightKg > e.cfg.OversizeWeightKg {
		size.Minor = e.cfg.OversizeFlatMinor
	}

	slot := ports.Money{Currency: e.cfg.Currency}
	if r.SlotStart != nil && e.isEveningSlot(*r.SlotStart) {
		slot.Minor = e.cfg.SlotPremiumMinor
	}

	subtotal := base.Minor + distance.Minor + size.Minor + slot.Minor
	total := subtotal

	discounts := []Discount{}
	if r.DiscountCode != "" {
		if d, ok := e.lookupDiscount(r.DiscountCode, subtotal); ok {
			discounts = append(discounts, d)
			total -= d.Amount.Minor
			if total < 0 {
				total = 0
			}
		}
	}

	return Quote{
		BaseFare:       base,
		DistanceFare:   distance,
		SizeModifier:   size,
		SlotPremium:    slot,
		Discounts:      discounts,
		Subtotal:       ports.Money{Currency: e.cfg.Currency, Minor: subtotal},
		Total:          ports.Money{Currency: e.cfg.Currency, Minor: total},
		DistanceMeters: dist,
		Currency:       e.cfg.Currency,
	}
}

func (e *Engine) isEveningSlot(t time.Time) bool {
	h := t.Hour()
	return h >= e.cfg.EveningStart && h < e.cfg.EveningEnd
}

// lookupDiscount is a placeholder. Real implementation would consult a
// repository and check expiration, usage, retailer scoping, etc.
func (e *Engine) lookupDiscount(code string, subtotal int64) (Discount, bool) {
	switch code {
	case "WELCOME":
		return Discount{
			Code:   "WELCOME",
			Amount: ports.Money{Currency: e.cfg.Currency, Minor: 500},
		}, true
	case "FIRST_FREE":
		return Discount{
			Code:   "FIRST_FREE",
			Amount: ports.Money{Currency: e.cfg.Currency, Minor: subtotal},
		}, true
	}
	return Discount{}, false
}

// haversineMeters returns the great-circle distance between two coords.
func haversineMeters(a, b Coord) float64 {
	const R = 6371000.0 // earth radius in meters
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return R * c
}
