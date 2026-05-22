# Schelet arhitectural — ghid rapid

Repository-ul are acum **straturi complete** (nu doar domain + un adapter):

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
    memory/             → repos in-memory (dev fără Postgres)
    events/             → event bus in-memory
    http/               → chi router + handlers + middleware
    postgres/           → placeholder M1
tests/contract/         → contract tests per carrier
migrations/             → schema Postgres
```

## Rulează local (fără Postgres)

```bash
cd /Users/xndr/Projects/riders
go run ./cmd/api
```

Implicit `USE_MEMORY_STORE=true` — datele dispar la restart.

## API disponibil (schelet funcțional)

| Method | Path | Rol |
|--------|------|-----|
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness |
| POST | `/v1/bookings` | Client programează locker → casă |
| GET | `/v1/bookings/{deliveryID}` | Status + lanț custody |
| POST | `/v1/bookings/{deliveryID}/dispatch` | Trimite la carrier (Bolt API real dacă ai key) |
| POST | `/v1/deliveries/{deliveryID}/custody` | Rider: poză + GPS + timestamp |
| POST | `/webhooks/bolt` | Webhook Bolt (pe port 8080 sau listener 8081) |

### Exemplu booking

```bash
RETAILER_ID="00000000-0000-0000-0000-000000000001"

curl -s -X POST http://localhost:8080/v1/bookings \
  -H 'Content-Type: application/json' \
  -d "{
    \"retailer_id\": \"$RETAILER_ID\",
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

Răspunsul conține `delivery_id` — folosește-l pentru GET, dispatch, custody.

### Exemplu custody (poză obligatorie la pickup/delivery)

```bash
DELIVERY_ID="<din răspunsul de mai sus>"

curl -s -X POST "http://localhost:8080/v1/deliveries/$DELIVERY_ID/custody" \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "package.picked_up",
    "actor": {"type": "rider", "carrier_id": "bolt_food", "id": "r1", "name": "Andrei"},
    "location": {"lat": 44.4546, "lng": 26.0987, "accuracy_m": 8},
    "photos": [{"s3_uri": "s3://pod/demo.jpg", "sha256": "abc123"}]
  }'
```

## Flux în cod

1. **Booking** (`app/booking`) → creează AWB + Delivery `pending`
2. **Dispatch** (`app/dispatch`) → routing engine → `CarrierPort.CreateShipment`
3. **Webhook** (`app/webhook`) → `ParseWebhook` → custody (M1: legătură delivery ID)
4. **Custody** (`app/custodyapp`) → `Ledger.Record` (hash chain) + update delivery state

## Ce lipsește (M1, după OK produs)

- Repos Postgres (`infra/postgres`)
- Plată Stripe/Netopia
- Notificări WhatsApp/SMS
- Upload poze S3 presigned
- Auth (OTP client, API key retailer)
- Frontend PWA + admin dashboard

## Enforce arhitectură

```bash
go build ./...
go test ./...
# go-arch-lint check   # după install
```

`internal/core` nu importă `internal/adapters` — verificat în `.go-arch-lint.yml`.
