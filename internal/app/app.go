// Package app is the composition root: it wires core domain logic with
// ports (repositories, carriers, event bus) into application services.
//
// Binaries (cmd/api, cmd/dispatcher, cmd/webhook-listener) depend only
// on this package + config, never on individual adapters directly.
package app

import (
	"fmt"
	"log/slog"

	"dashfetchr/internal/app/booking"
	"dashfetchr/internal/app/custodyapp"
	"dashfetchr/internal/app/dispatch"
	"dashfetchr/internal/app/webhook"
	"dashfetchr/internal/carrier"
	"dashfetchr/internal/config"
	"dashfetchr/internal/core/custody"
	"dashfetchr/internal/core/pricing"
	"dashfetchr/internal/core/routing"
	"dashfetchr/internal/infra/events"
	"dashfetchr/internal/infra/memory"
	"dashfetchr/internal/ports"
)

// App holds all application services and shared infrastructure.
type App struct {
	Config   config.Config
	Logger   *slog.Logger
	Carriers *carrier.Registry
	Bus      ports.EventBus

	Booking  *booking.Service
	Dispatch *dispatch.Service
	Custody  *custodyapp.Service
	Webhook  *webhook.Service
}

// New builds the application graph. useMemory=true skips Postgres and uses
// in-memory stores so `go run ./cmd/api` works without docker.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	reg, err := carrier.Wire(cfg.Carriers)
	if err != nil {
		return nil, fmt.Errorf("wire carriers: %w", err)
	}

	bus := events.NewMemoryBus(logger)
	bus.Subscribe(events.AuditHandler{Logger: logger})

	var awbRepo ports.AWBRepository
	var delRepo ports.DeliveryRepository
	var routingRepo ports.RoutingDecisionRepository
	var custodyRepo custody.Repository

	if cfg.DB.UseMemory {
		awbRepo = memory.NewAWBRepo()
		delRepo = memory.NewDeliveryRepo()
		routingRepo = memory.NewRoutingRepo()
		custodyRepo = memory.NewCustodyRepo()
		logger.Info("storage: in-memory (set USE_MEMORY_STORE=false for Postgres)")
	} else {
		return nil, fmt.Errorf("postgres repositories not implemented yet; set USE_MEMORY_STORE=true")
	}

	ledger := custody.NewLedger(custodyRepo)
	pricingEngine := pricing.NewEngine(pricing.Default())
	router := routing.NewEngine(reg, routing.CheapestStrategy{})

	custodySvc := custodyapp.New(custodyapp.Deps{
		Ledger:     ledger,
		Deliveries: delRepo,
		Bus:        bus,
		Logger:     logger,
	})

	return &App{
		Config:   cfg,
		Logger:   logger,
		Carriers: reg,
		Bus:      bus,
		Booking: booking.New(booking.Deps{
			AWBs:       awbRepo,
			Deliveries: delRepo,
			Pricing:    pricingEngine,
			Logger:     logger,
		}),
		Dispatch: dispatch.New(dispatch.Deps{
			Deliveries: delRepo,
			AWBs:       awbRepo,
			Routing:    router,
			RoutingLog: routingRepo,
			Carriers:   reg,
			Bus:        bus,
			Logger:     logger,
		}),
		Custody: custodySvc,
		Webhook: webhook.New(webhook.Deps{
			Carriers:   reg,
			Custody:    custodySvc,
			Deliveries: delRepo,
			Logger:     logger,
		}),
	}, nil
}
