# Schelet arhitectural — ghid rapid

Repository-ul are straturi complete (domain + adapters + Postgres):

```
cmd/                    → binaries (api, dispatcher, webhook-listener)
internal/
  config/               → env config
  ports/                → interfețe (carrier, repo, event bus, payment, …)
  core/                 → domain pur (awb, delivery, custody, routing, pricing)
  app/                  → application services (booking, dispatch, custody, webhook)
  carrier/              → registry + wire carriers
  adapters/carriers/    → Bolt template (+ README pentru restul)
  infra/
    memory/             → repos in-memory (dev rapid fără Docker)
    postgres/           → repos Postgres (recomandat)
    events/             → event bus in-memory
    http/               → chi router + handlers + middleware
tests/contract/         → contract tests per carrier
migrations/             → schema Postgres + seed dev retailer
scripts/demo.sh         → flux E2E curl
```

## Rulează local (Postgres — recomandat)

```bash
cp .env.example .env
make docker-up
make migrate
USE_MEMORY_STORE=false go run ./cmd/api
```

Sau fără Postgres (date pierdute la restart):

```bash
USE_MEMORY_STORE=true go run ./cmd/api
```

## Demo E2E (curl)

Cu API pornit și migrările aplicate:

```bash
chmod +x scripts/demo.sh
./scripts/demo.sh
# sau: make demo
```

## API disponibil

| Method | Path | Auth | Rol |
|--------|------|------|-----|
| GET | `/healthz` | — | Liveness |
| GET | `/readyz` | — | Readiness (ping Postgres când nu e memory) |
| POST | `/v1/bookings` | API key retailer | Client programează locker → casă |
| GET | `/v1/bookings/{deliveryID}` | API key retailer | Status + lanț custody (doar bookings proprii) |
| POST | `/v1/bookings/{deliveryID}/dispatch` | API key retailer | Trimite la carrier (Bolt API real dacă ai key) |
| POST | `/v1/deliveries/{deliveryID}/custody` | — (rider, TODO) | Rider: poză + GPS + timestamp |
| POST | `/webhooks/bolt` | semnătură Bolt | Webhook Bolt |

## Autentificare retailer

Endpoint-urile `/v1/bookings*` cer un API key per retailer:

```
Authorization: Bearer <api_key>
```

Cheia e stocată hash-uită (SHA-256) în `retailers.api_key_hash`; `retailer_id` nu
se mai trimite în body — vine din cheie. Un booking poate fi citit/dispatch-uit
doar de retailerul care l-a creat (altfel `404`).

Cheie dev (seed local — migrația 0003 / store in-memory): `df_dev_pilot_2026`
Retailer dev (seed): `00000000-0000-0000-0000-000000000001`

### Exemplu booking

```bash
API_KEY="df_dev_pilot_2026"

curl -s -X POST http://localhost:8080/v1/bookings \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_KEY" \
  -d "{
    \"external_awb\": \"SD123456\",
    \"external_carrier\": \"sameday\",
    \"customer_phone\": \"+40712345678\",
    \"pickup_locker_id\": \"L-FLO-01\",
    \"pickup_lat\": 44.4546,
    \"pickup_lng\": 26.0987,
    \"pickup_address\": \"Easybox Floreasca\",
    \"drop_lat\": 44.4632,
    \"drop_lng\": 26.1062,
    \"drop_address\": \"Str. Aviatorilor 10\",
    \"weight_kg\": 1.2,
    \"slot_start\": \"2026-05-20T19:00:00+03:00\",
    \"slot_end\": \"2026-05-20T20:00:00+03:00\"
  }"
```

### Dispatcher (pending → carrier)

```bash
USE_MEMORY_STORE=false go run ./cmd/dispatcher
```

Poll la fiecare 30s (`DISPATCH_POLL_INTERVAL`), max 20 comenzi (`DISPATCH_BATCH_SIZE`).

### Webhook Bolt (după dispatch)

```bash
curl -s -X POST http://localhost:8080/webhooks/bolt \
  -H 'Content-Type: application/json' \
  -H 'X-Bolt-Signature: <dacă e configurat>' \
  -d '{
    "event": "order.delivered",
    "order_id": "<carrier_external_id din dispatch>",
    "timestamp": "2026-05-20T19:30:00Z",
    "location": {"lat": 44.4632, "lng": 26.1062},
    "photo_url": "https://example.com/pod.jpg"
  }'
```

## Flux în cod

1. **Booking** (`app/booking`) → AWB + Delivery `pending`
2. **Dispatch** (`app/dispatch`) → routing → `CreateShipment` → `assigned` + `carrier_external_id`
3. **Webhook** (`app/webhook`) → lookup by `carrier_external_id` → custody + state
4. **Custody** (`app/custodyapp`) → `Ledger.Record` (hash chain) + delivery state

## Ce lipsește (după sign-off produs)

- Plată Stripe/Netopia
- Notificări WhatsApp/SMS
- Upload poze S3 presigned
- Auth (OTP client, API key retailer)
- Frontend PWA + admin dashboard

## Teste & CI

```bash
go test ./...
make test-integration   # necesită Postgres + migrate
```

GitHub Actions: `.github/workflows/ci.yml` — unit + migrate + integration + build.

## Enforce arhitectură

```bash
go build ./...
go test ./...
# go-arch-lint check
```

`internal/core` nu importă `internal/adapters`.
