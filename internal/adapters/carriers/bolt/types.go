package bolt

import "time"

// ----------------------------------------------------------------------------
// Bolt-specific request/response shapes.
//
// These types intentionally use Bolt's vocabulary ("delivery_order",
// "pickup_address", etc). They never leak outside this package.
// ----------------------------------------------------------------------------

type boltLocation struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Address   string  `json:"address"`
	Floor     string  `json:"floor,omitempty"`
	Apartment string  `json:"apartment,omitempty"`
	Notes     string  `json:"notes,omitempty"`
}

type boltPackageDims struct {
	LengthCm int `json:"length_cm"`
	WidthCm  int `json:"width_cm"`
	HeightCm int `json:"height_cm"`
}

type boltPackage struct {
	WeightGrams int             `json:"weight_g"`
	Dimensions  boltPackageDims `json:"dimensions"`
	Description string          `json:"description,omitempty"`
	Value       int             `json:"declared_value_cents,omitempty"`
	Currency    string          `json:"currency,omitempty"`
}

type boltRecipient struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type boltCreateOrderRequest struct {
	ExternalRef       string        `json:"external_ref"`
	Pickup            boltLocation  `json:"pickup"`
	Drop              boltLocation  `json:"drop"`
	Package           boltPackage   `json:"package"`
	Recipient         boltRecipient `json:"recipient"`
	ServiceArea       string        `json:"service_area"`
	RequiresPhotoPOD  bool          `json:"requires_photo_pod"`
	ScheduledFor      *time.Time    `json:"scheduled_for,omitempty"`
}

type boltMoney struct {
	Cents    int    `json:"cents"`
	Currency string `json:"currency"`
}

type boltOrder struct {
	OrderID           string      `json:"order_id"`
	Status            string      `json:"status"`
	Price             boltMoney   `json:"price"`
	EstimatedPickup   *time.Time  `json:"estimated_pickup_at,omitempty"`
	EstimatedDrop     *time.Time  `json:"estimated_drop_at,omitempty"`
	TrackingURL       string      `json:"tracking_url,omitempty"`
	Rider             *boltRider  `json:"rider,omitempty"`
	LastLocation      *boltLocation `json:"last_location,omitempty"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type boltRider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Vehicle string `json:"vehicle"`
}

type boltEstimateRequest struct {
	Pickup       boltLocation `json:"pickup"`
	Drop         boltLocation `json:"drop"`
	Package      boltPackage  `json:"package"`
	ServiceArea  string       `json:"service_area"`
	ScheduledFor *time.Time   `json:"scheduled_for,omitempty"`
}

type boltEstimateResponse struct {
	Price            boltMoney `json:"price"`
	EstimatedETASec  int       `json:"estimated_eta_seconds"`
	Confidence       float64   `json:"confidence"`
}

// ----------------------------------------------------------------------------
// Webhook payload
// ----------------------------------------------------------------------------

type boltWebhookPayload struct {
	Event     string         `json:"event"`
	OrderID   string         `json:"order_id"`
	Timestamp time.Time      `json:"timestamp"`
	Rider     *boltRider     `json:"rider,omitempty"`
	Location  *boltLocation  `json:"location,omitempty"`
	PhotoURL  string         `json:"photo_url,omitempty"`
	Reason    string         `json:"failure_reason,omitempty"`
}
