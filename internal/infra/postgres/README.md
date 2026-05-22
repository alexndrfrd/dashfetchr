# Postgres adapters (M1)

Implement repository interfaces here:

| Interface | File (planned) |
|-----------|----------------|
| `ports.AWBRepository` | `awb_repo.go` |
| `ports.DeliveryRepository` | `delivery_repo.go` |
| `custody.Repository` | `custody_repo.go` |
| `ports.RoutingDecisionRepository` | `routing_repo.go` |

Use `pgx` + `sqlc` against `migrations/0001_initial.up.sql`.

Wire in `internal/app/app.go` when `USE_MEMORY_STORE=false`:

```go
pool, err := postgres.NewPool(cfg.DB.URL)
awbRepo = postgres.NewAWBRepo(pool)
// ...
```

Until then, local dev uses `internal/infra/memory`.
