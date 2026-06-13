package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"dashfetchr/internal/app"
	"dashfetchr/internal/app/booking"
	"dashfetchr/internal/auth"
	"dashfetchr/internal/infra/http/response"
)

type Bookings struct {
	App *app.App
}

type createBookingRequest struct {
	ExternalAWB      string  `json:"external_awb"`
	ExternalCarrier  string  `json:"external_carrier"`
	CustomerPhone    string  `json:"customer_phone"`
	CustomerName     string  `json:"customer_name"`
	PickupLockerID   string  `json:"pickup_locker_id"`
	PickupLat        float64 `json:"pickup_lat"`
	PickupLng        float64 `json:"pickup_lng"`
	PickupAddress    string  `json:"pickup_address"`
	DropLat          float64 `json:"drop_lat"`
	DropLng          float64 `json:"drop_lng"`
	DropAddress      string  `json:"drop_address"`
	DropFloor        string  `json:"drop_floor"`
	DropApartment    string  `json:"drop_apartment"`
	DropInstructions string  `json:"drop_instructions"`
	WeightKg         float64 `json:"weight_kg"`
	SlotStart        string  `json:"slot_start"` // RFC3339
	SlotEnd          string  `json:"slot_end"`
	LastMileCarrier  string  `json:"last_mile_carrier,omitempty"`
}

func (h Bookings) Create(w http.ResponseWriter, r *http.Request) {
	ret, ok := auth.RetailerFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthenticated", "missing retailer context")
		return
	}

	var body createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	slotStart, err := time.Parse(time.RFC3339, body.SlotStart)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_slot_start", "use RFC3339")
		return
	}
	slotEnd, err := time.Parse(time.RFC3339, body.SlotEnd)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_slot_end", "use RFC3339")
		return
	}

	out, err := h.App.Booking.CreateBooking(r.Context(), booking.CreateBookingInput{
		RetailerID:       ret.ID,
		ExternalAWB:      body.ExternalAWB,
		ExternalCarrier:  body.ExternalCarrier,
		CustomerPhone:    body.CustomerPhone,
		CustomerName:     body.CustomerName,
		PickupLockerID:   body.PickupLockerID,
		PickupLat:        body.PickupLat,
		PickupLng:        body.PickupLng,
		PickupAddress:    body.PickupAddress,
		DropLat:          body.DropLat,
		DropLng:          body.DropLng,
		DropAddress:      body.DropAddress,
		DropFloor:        body.DropFloor,
		DropApartment:    body.DropApartment,
		DropInstructions: body.DropInstructions,
		WeightKg:         body.WeightKg,
		SlotStart:        slotStart,
		SlotEnd:          slotEnd,
		LastMileCarrier:  body.LastMileCarrier,
	})
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "booking_failed", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, out)
}

func (h Bookings) Get(w http.ResponseWriter, r *http.Request) {
	ret, ok := auth.RetailerFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthenticated", "missing retailer context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "deliveryID"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_id", "deliveryID must be UUID")
		return
	}

	del, awb, err := h.App.Booking.GetBooking(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	// Ownership: don't leak other retailers' bookings; 404 rather than 403 to
	// avoid confirming the ID exists.
	if awb.RetailerID != ret.ID {
		response.Error(w, http.StatusNotFound, "not_found", "booking not found")
		return
	}

	events, _ := h.App.Custody.ListEvents(r.Context(), id)

	response.JSON(w, http.StatusOK, map[string]any{
		"delivery": del,
		"awb":      awb,
		"custody":  events,
	})
}

func (h Bookings) Dispatch(w http.ResponseWriter, r *http.Request) {
	ret, ok := auth.RetailerFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthenticated", "missing retailer context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "deliveryID"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_id", "deliveryID must be UUID")
		return
	}

	// Ownership check before dispatching someone else's delivery.
	_, awb, err := h.App.Booking.GetBooking(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if awb.RetailerID != ret.ID {
		response.Error(w, http.StatusNotFound, "not_found", "booking not found")
		return
	}

	if err := h.App.Dispatch.DispatchDelivery(r.Context(), id); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "dispatch_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]string{"status": "dispatched"})
}
