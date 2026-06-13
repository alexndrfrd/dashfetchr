// Package app is the composition root: it wires core domain logic with
// ports (repositories, carriers, event bus) into application services.
//
// Binaries (cmd/api, cmd/dispatcher, cmd/webhook-listener) depend only
// on this package + config, never on individual adapters directly.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

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
	"dashfetchr/internal/infra/postgres"
	"dashfetchr/internal/ports"
)

// App holds all application services and shared infrastructure.
type App struct {
	Config   config.Config
	Logger   *slog.Logger
	Carriers *carrier.Registry
	Bus      ports.EventBus
	Pool     *pgxpool.Pool // nil when using in-memory storage

	Retailers   ports.RetailerRepository
	RiderSecrets map[string]string // carrier_id → shared secret for rider auth

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
	var retailerRepo ports.RetailerRepository
	var pool *pgxpool.Pool

	if cfg.DB.UseMemory {
		awbRepo = memory.NewAWBRepo()
		delRepo = memory.NewDeliveryRepo()
		routingRepo = memory.NewRoutingRepo()
		custodyRepo = memory.NewCustodyRepo()
		retailerRepo = memory.NewRetailerRepo()
		logger.Info("storage: in-memory (set USE_MEMORY_STORE=false for Postgres)")
	} else {
		ctx := context.Background()
		pool, err = postgres.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns)
		if err != nil {
			return nil, err
		}
		awbRepo = postgres.NewAWBRepo(pool)
		delRepo = postgres.NewDeliveryRepo(pool)
		routingRepo = postgres.NewRoutingRepo(pool)
		custodyRepo = postgres.NewCustodyRepo(pool)
		retailerRepo = postgres.NewRetailerRepo(pool)
		logger.Info("storage: postgres", "url", redactDBURL(cfg.DB.URL))
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

	riderSecrets := map[string]string{
		"bolt_food": cfg.Carriers.Bolt.RiderSecret,
	}

	return &App{
		Config:       cfg,
		Logger:       logger,
		Carriers:     reg,
		Bus:          bus,
		Pool:         pool,
		Retailers:    retailerRepo,
		RiderSecrets: riderSecrets,
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

// PingDB checks Postgres connectivity when a pool is configured.
func (a *App) PingDB(ctx context.Context) error {
	if a.Pool == nil {
		return nil
	}
	return a.Pool.Ping(ctx)
}

func redactDBURL(url string) string {
	// Avoid logging passwords in startup logs.
	const needle = "://"
	i := 0
	if j := indexOf(url, needle); j >= 0 {
		i = j + len(needle)
	}
	if at := indexOf(url[i:], "@"); at > 0 {
		return url[:i] + "***@" + url[i+at+1:]
	}
	return url
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
