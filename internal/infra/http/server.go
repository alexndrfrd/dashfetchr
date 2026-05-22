package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"dashfetchr/internal/app"
	"dashfetchr/internal/config"
	"dashfetchr/internal/infra/http/handlers"
	"dashfetchr/internal/infra/http/middleware"
)

// NewServer builds the API router with all v1 routes.
func NewServer(application *app.App, cfg config.HTTPConfig, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recover(log))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(log))
	r.Use(chimw.RealIP)
	r.Use(chimw.Timeout(60 * time.Second))

	health := handlers.Health{}
	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)

	bookings := handlers.Bookings{App: application}
	custodyH := handlers.Custody{App: application}
	webhooks := handlers.Webhooks{App: application}

	r.Route("/v1", func(r chi.Router) {
		r.Post("/bookings", bookings.Create)
		r.Get("/bookings/{deliveryID}", bookings.Get)
		r.Post("/bookings/{deliveryID}/dispatch", bookings.Dispatch)

		r.Post("/deliveries/{deliveryID}/custody", custodyH.Record)
	})

	r.Route("/webhooks", func(r chi.Router) {
		r.Post("/bolt", webhooks.Bolt)
		// r.Post("/glovo", webhooks.Glovo)   // v2
		// r.Post("/sameday", webhooks.Sameday) // v2
	})

	return r
}
