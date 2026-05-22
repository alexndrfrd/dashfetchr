# DashFetchr

**Concierge layer peste reteaua de easybox-uri din Bucuresti.** Aducem coletul de la locker la usa clientului, pe intervalul ales de el.

---

## Status: pre-development (scaffold + docs)

**DashFetchr** — concierge delivery from locker to your door, on your schedule.

Acest repository contine documentatia de produs, arhitectura tehnica si scaffold-ul de cod, **inainte** de inceperea dezvoltarii efective in productie. Trebuie aprobat de product owner + tech lead inainte de prima linie de cod productie.

## Citeste in ordinea asta

1. **[docs/PRODUCT.md](docs/PRODUCT.md)** - documentul principal, pentru product owner si stakeholders. Vision, scope, user flows, KPI.
2. **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - arhitectura tehnica detaliata, design patterns, bounded contexts, DB schema.
3. **[docs/DECISIONS.md](docs/DECISIONS.md)** - lista deciziilor deschise care trebuie aprobate inainte de kickoff. **Acesta este documentul de sign-off.**
4. **[docs/ADDING_A_CARRIER.md](docs/ADDING_A_CARRIER.md)** - playbook pentru cum se adauga un curier nou (e.g. Wolt) fara breaking changes in core.
5. **[docs/SKELETON.md](docs/SKELETON.md)** - cum rulezi API-ul local, endpoints, exemple curl.

## Structura repository

```
dashfetchr/
├── docs/                     # Documentatie produs + tehnica + SKELETON.md
├── migrations/               # Postgres schema migrations
├── cmd/                      # api | dispatcher | webhook-listener
├── internal/
│   ├── config/               # Env config
│   ├── ports/                # Interfete (carrier, repo, event bus, …)
│   ├── core/                 # Domain pur (awb, delivery, custody, routing, pricing)
│   ├── app/                  # Application services (booking, dispatch, …)
│   ├── carrier/              # Registry + wire
│   ├── adapters/carriers/    # Bolt template + README pentru restul
│   └── infra/
│       ├── memory/           # Repos in-memory (dev)
│       ├── events/           # Event bus in-memory
│       ├── http/             # chi + handlers + middleware
│       └── postgres/         # M1 placeholder
└── tests/contract/
```

## Stack

- **Backend**: Go 1.22 (modulith), Node.js BFF (optional, v2+)
- **DB**: PostgreSQL 16 (cu JSONB + partial indexes)
- **Storage**: AWS S3 (cu Object Lock pentru poze POD)
- **Queue**: AWS SQS (Kafka la scale)
- **Cache**: Redis
- **Frontend**: Next.js 14 (PWA pentru client, dashboard admin, portal retailer)
- **Notificari**: Twilio + WhatsApp Business API
- **Plati**: Stripe + Netopia
- **Infra**: AWS ECS Fargate, RDS, S3, CloudFront, Route53
- **CI/CD**: GitHub Actions

## Workflow de dezvoltare (dupa kickoff)

```bash
# Local
docker compose up -d              # Postgres, Redis, LocalStack S3
make migrate                       # Apply migrations
make run-api                       # Start API
make run-dispatcher                # Start dispatcher
make test                          # Unit + contract tests
make test-contract CARRIER=bolt    # Test specific adapter
```

## Pasii urmatori

Vezi [docs/DECISIONS.md](docs/DECISIONS.md). Are lista de decizii care trebuie sa primeasca OK inainte sa incepem.
