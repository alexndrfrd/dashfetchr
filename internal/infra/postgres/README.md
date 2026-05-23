# Postgres adapters

Implements repository interfaces with `pgx` + `pgxpool` against `migrations/`.

| Interface | Implementation |
|-----------|----------------|
| `ports.AWBRepository` | `awb_repo.go` |
| `ports.DeliveryRepository` | `delivery_repo.go` |
| `custody.Repository` | `custody_repo.go` |
| `ports.RoutingDecisionRepository` | `routing_repo.go` |

## Local setup

```bash
make docker-up
make migrate          # requires golang-migrate CLI, or use docker per Makefile comment
USE_MEMORY_STORE=false make run-api
```

Integration tests:

```bash
make migrate
make test-integration
```

Wired in `internal/app/app.go` when `USE_MEMORY_STORE=false`.
