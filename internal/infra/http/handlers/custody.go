package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"dashfetchr/internal/app"
	"dashfetchr/internal/app/custodyapp"
	"dashfetchr/internal/core/custody"
	"dashfetchr/internal/infra/http/response"
)

type Custody struct {
	App *app.App
}

type recordCustodyRequest struct {
	Type     string `json:"type"`
	Actor    struct {
		Type      string `json:"type"`
		CarrierID string `json:"carrier_id,omitempty"`
		ID        string `json:"id"`
		Name      string `json:"name"`
	} `json:"actor"`
	Location *struct {
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		AccuracyM float64 `json:"accuracy_m"`
	} `json:"location"`
	Photos []struct {
		S3URI  string `json:"s3_uri"`
		SHA256 string `json:"sha256"`
	} `json:"photos"`
	Reason string `json:"reason,omitempty"`
}

func (h Custody) Record(w http.ResponseWriter, r *http.Request) {
	deliveryID, err := uuid.Parse(chi.URLParam(r, "deliveryID"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_id", "deliveryID must be UUID")
		return
	}

	var body recordCustodyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	var loc *custody.GeoPoint
	if body.Location != nil {
		loc = &custody.GeoPoint{
			Lat: body.Location.Lat, Lng: body.Location.Lng, AccuracyM: body.Location.AccuracyM,
		}
	}
	photos := make([]custody.PhotoRef, 0, len(body.Photos))
	for _, p := range body.Photos {
		photos = append(photos, custody.PhotoRef{S3URI: p.S3URI, SHA256: p.SHA256, ContentType: "image/jpeg"})
	}

	ev, err := h.App.Custody.Record(r.Context(), custodyapp.RecordInput{
		DeliveryID: deliveryID,
		Type:       custody.EventType(body.Type),
		Actor: custody.Actor{
			Type: body.Actor.Type, CarrierID: body.Actor.CarrierID,
			ID: body.Actor.ID, Name: body.Actor.Name,
		},
		Location: loc,
		Photos:   photos,
		Reason:   body.Reason,
	})
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "custody_failed", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"event_id":    ev.EventID,
		"sequence":    ev.SequenceNum,
		"hash":        ev.Hash,
		"occurred_at": ev.OccurredAt.Format(time.RFC3339),
	})
}
