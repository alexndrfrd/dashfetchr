# Cum adaugi un carrier nou in DashFetchr

**Audienta**: Backend engineer care primeste sarcina "integreaza Wolt / DHL / FAN / etc."

**Premise**: Daca respecti acest playbook, nu modifici nicio linie in `internal/core/*`. Contract tests-ele iti garanteaza ca nu ai rupt nimic.

---

## TL;DR — 7 pasi

1. Creezi folder nou: `internal/adapters/carriers/<carrier_name>/`
2. Implementezi `ports.CarrierPort` (5 metode + `Capabilities()`)
3. Scrii Anti-Corruption Layer (`mapper.go`)
4. Inregistrezi adapter-ul in `wire.go`
5. Treci contract tests: `make test-contract CARRIER=<carrier_name>`
6. Adaugi feature flag in DB (`carrier_routing_config`) cu `rollout_percent=0`
7. Treci la 10% → 50% → 100% pe baza de metrici

Adaugarea unui carrier nou nu trebuie sa dureze mai mult de **2-3 zile** pentru un engineer cu acces la API docs.

---

## Pas 1: Folder structure

```bash
mkdir -p internal/adapters/carriers/wolt
cd internal/adapters/carriers/wolt
```

Fisiere standard:

```
wolt/
├── adapter.go          # Implementeaza ports.CarrierPort
├── client.go           # HTTP client peste API-ul Wolt (transport pur)
├── mapper.go           # Anti-Corruption Layer (Wolt ↔ Domain)
├── types.go            # Tipurile Wolt-specifice (din OpenAPI lor)
├── webhook.go          # Parsing + signature verification webhook
├── capabilities.go     # CarrierCapabilities pentru Wolt
└── adapter_test.go     # Wire-up pentru contract tests
```

---

## Pas 2: Implementare CarrierPort

```go
// adapter.go
package wolt

import (
    "context"
    "dashfetchr/internal/ports"
)

type Adapter struct {
    client *Client
    mapper *Mapper
    caps   ports.CarrierCapabilities
}

func New(cfg Config) *Adapter {
    return &Adapter{
        client: NewClient(cfg),
        mapper: NewMapper(),
        caps:   buildCapabilities(cfg),
    }
}

func (a *Adapter) Capabilities() ports.CarrierCapabilities {
    return a.caps
}

func (a *Adapter) CreateShipment(ctx context.Context, req ports.ShipmentRequest) (*ports.ShipmentResponse, error) {
    woltReq := a.mapper.ToWoltDeliveryRequest(req)
    woltResp, err := a.client.CreateDelivery(ctx, woltReq)
    if err != nil {
        return nil, mapError(err)
    }
    return a.mapper.FromWoltDeliveryResponse(woltResp), nil
}

func (a *Adapter) GetShipmentStatus(ctx context.Context, externalID string) (*ports.ShipmentStatus, error) {
    woltStatus, err := a.client.GetDelivery(ctx, externalID)
    if err != nil {
        return nil, mapError(err)
    }
    return a.mapper.FromWoltStatus(woltStatus), nil
}

func (a *Adapter) CancelShipment(ctx context.Context, externalID string, reason string) error {
    return a.client.CancelDelivery(ctx, externalID, reason)
}

func (a *Adapter) GetEstimate(ctx context.Context, req ports.EstimateRequest) (*ports.Estimate, error) {
    woltReq := a.mapper.ToWoltEstimateRequest(req)
    est, err := a.client.GetEstimate(ctx, woltReq)
    if err != nil {
        return nil, mapError(err)
    }
    return a.mapper.FromWoltEstimate(est), nil
}

func (a *Adapter) ParseWebhook(ctx context.Context, raw []byte, headers map[string]string) ([]ports.DomainEvent, error) {
    if err := verifySignature(raw, headers, a.client.WebhookSecret()); err != nil {
        return nil, ports.ErrInvalidSignature
    }
    var wh WoltWebhookPayload
    if err := json.Unmarshal(raw, &wh); err != nil {
        return nil, err
    }
    return a.mapper.ToDomainEvents(wh), nil
}
```

---

## Pas 3: Anti-Corruption Layer (mapper.go)

**Aceasta este partea critica**. Tot mapping-ul Wolt ↔ Domain sta aici. Daca Wolt schimba API-ul, modifici **DOAR** acest fisier.

```go
// mapper.go
package wolt

import (
    "dashfetchr/internal/ports"
    "dashfetchr/internal/core/awb"
)

type Mapper struct{}

func NewMapper() *Mapper { return &Mapper{} }

// Domain → Wolt
func (m *Mapper) ToWoltDeliveryRequest(r ports.ShipmentRequest) WoltDeliveryRequest {
    return WoltDeliveryRequest{
        Pickup: WoltLocation{
            Lat:     r.Pickup.Lat,
            Lon:     r.Pickup.Lng,
            Address: r.Pickup.Address,
        },
        Drop: WoltLocation{
            Lat:     r.Drop.Lat,
            Lon:     r.Drop.Lng,
            Address: r.Drop.Address,
        },
        Package: WoltPackage{
            WeightG:    int(r.Package.WeightKg * 1000),
            DimensionsCM: WoltDims{
                Length: r.Package.LengthCm,
                Width:  r.Package.WidthCm,
                Height: r.Package.HeightCm,
            },
        },
        ScheduledFor: r.ScheduledStart, // poate fi nil pentru instant
        ExternalRef:  r.IdempotencyKey,
        Recipient: WoltRecipient{
            Name:  r.Recipient.Name,
            Phone: r.Recipient.Phone,
        },
        PODRequired: true,  // forteaza poza la livrare
    }
}

// Wolt → Domain
func (m *Mapper) FromWoltDeliveryResponse(r WoltDeliveryResponse) *ports.ShipmentResponse {
    return &ports.ShipmentResponse{
        ExternalID:  r.DeliveryID,
        Status:      m.fromWoltStatus(r.Status),
        EstimatedCost: ports.Money{
            Currency: r.Price.Currency,
            Minor:    r.Price.Cents,
        },
        EstimatedPickupAt:   r.EstimatedPickup,
        EstimatedDropAt:     r.EstimatedDrop,
        TrackingURL:         r.TrackingURL,
    }
}

func (m *Mapper) fromWoltStatus(s string) ports.ShipmentState {
    switch s {
    case "pending":        return ports.StatePending
    case "assigned":       return ports.StateAssigned
    case "picking_up":     return ports.StateRiderEnRouteToPickup
    case "picked_up":      return ports.StatePickedUp
    case "delivering":     return ports.StateInTransit
    case "delivered":      return ports.StateDelivered
    case "cancelled":      return ports.StateCancelled
    case "failed":         return ports.StateFailed
    default:               return ports.StateUnknown
    }
}

// Webhook → Domain Events
func (m *Mapper) ToDomainEvents(wh WoltWebhookPayload) []ports.DomainEvent {
    events := []ports.DomainEvent{}
    switch wh.Event {
    case "delivery.assigned":
        events = append(events, ports.RiderAssignedEvent{
            ExternalID: wh.DeliveryID,
            RiderID:    wh.RiderID,
            CarrierID:  "wolt",
            OccurredAt: wh.Timestamp,
        })
    case "delivery.picked_up":
        events = append(events, ports.PackagePickedUpEvent{
            ExternalID: wh.DeliveryID,
            Location:   m.fromWoltLocation(wh.Location),
            PhotoURL:   wh.PhotoURL,
            OccurredAt: wh.Timestamp,
        })
    case "delivery.completed":
        events = append(events, ports.PackageDeliveredEvent{
            ExternalID:  wh.DeliveryID,
            Location:    m.fromWoltLocation(wh.Location),
            PhotoURL:    wh.PhotoURL,
            Signature:   wh.Signature,
            OccurredAt:  wh.Timestamp,
        })
    case "delivery.failed":
        events = append(events, ports.DeliveryFailedEvent{
            ExternalID: wh.DeliveryID,
            Reason:     wh.FailureReason,
            Location:   m.fromWoltLocation(wh.Location),
            PhotoURL:   wh.PhotoURL,
            OccurredAt: wh.Timestamp,
        })
    }
    return events
}
```

---

## Pas 4: Capabilities

Asta e ce **vede routing engine-ul** despre carrier-ul tau. Atentie la corectitudine.

```go
// capabilities.go
package wolt

import (
    "dashfetchr/internal/ports"
    "dashfetchr/internal/core/awb"
)

func buildCapabilities(cfg Config) ports.CarrierCapabilities {
    return ports.CarrierCapabilities{
        ID:    "wolt",
        Name:  "Wolt Drive",
        Roles: []ports.CarrierRole{ports.RoleLastMile},
        MaxWeightKg: 15,
        MaxDimensionsCm: [3]float64{60, 40, 40},
        Zones: woltZones(cfg.Environment), // []GeoZone definite din coverage Wolt
        DeliveryModes: []ports.DeliveryMode{
            ports.DeliveryInstant,
            ports.DeliveryScheduled,
        },
        PickupModes: []ports.PickupMode{
            ports.PickupAddress,
            // ATENTIE: Wolt nu suporta pickup din locker fara prezenta fizica
            // → SupportsLocker: false
        },
        POD: []ports.ProofType{
            ports.ProofPhoto,
            ports.ProofGPS,
            ports.ProofSignature, // optional Wolt
        },
        SLA: ports.SLA{
            AvgPickupTime:   10 * time.Minute,
            AvgDeliveryTime: 25 * time.Minute,
            MaxDelay:        15 * time.Minute,
        },
        SupportsLocker: false,
        AuthScheme:     ports.AuthBearerToken,
        APIRateLimit:   ports.RateLimit{Requests: 100, PerSecond: 1},
        Quirks: []string{
            "Wolt requires explicit pickup address; cannot serve locker pickups directly",
            "Webhook timeouts at 5s",
        },
    }
}
```

---

## Pas 5: Inregistrare

```go
// internal/carrier/wire.go
package carrier

import (
    "dashfetchr/internal/adapters/carriers/bolt"
    "dashfetchr/internal/adapters/carriers/glovo"
    "dashfetchr/internal/adapters/carriers/sameday"
    "dashfetchr/internal/adapters/carriers/wolt"  // ⬅ AICI
)

func Wire(cfg Config) *Registry {
    r := NewRegistry()
    r.Register("bolt_food", "v1", bolt.New(cfg.Bolt))
    r.Register("glovo", "v1", glovo.New(cfg.Glovo))
    r.Register("sameday", "v1", sameday.New(cfg.Sameday))
    r.Register("wolt", "v1", wolt.New(cfg.Wolt))  // ⬅ O LINIE
    return r
}
```

---

## Pas 6: Contract tests

Acelasi suite ruleaza pentru toti carriers. Tu doar wire-up:

```go
// adapter_test.go
package wolt_test

import (
    "testing"
    "dashfetchr/internal/adapters/carriers/wolt"
    "dashfetchr/tests/contract"
)

func TestWoltCarrierContract(t *testing.T) {
    cfg := wolt.Config{
        BaseURL:  "https://sandbox.wolt.com",
        APIKey:   testAPIKey(),
        Sandbox:  true,
    }
    adapter := wolt.New(cfg)
    contract.RunCarrierContract(t, adapter)
}
```

Run:

```bash
make test-contract CARRIER=wolt
```

**Toate cele ~30 de scenarii trebuie sa treaca**. Daca treci si vrei sa fii sigur:

```bash
make test-contract-all
```

Aceasta ruleaza contract tests pentru toate carriers, inclusiv mock-uri pentru ce nu are sandbox.

---

## Pas 7: Rollout gradual

```sql
-- Initial: 0% (silent register, prod-ready dar fara traffic)
INSERT INTO carrier_routing_config (carrier_id, version, rollout_percent, enabled)
VALUES ('wolt', 'v1', 0, true);

-- Dupa o saptamana de monitoring:
UPDATE carrier_routing_config SET rollout_percent = 10 WHERE carrier_id = 'wolt';

-- Dupa inca o saptamana, daca metrics OK:
UPDATE carrier_routing_config SET rollout_percent = 50 WHERE carrier_id = 'wolt';

-- Final:
UPDATE carrier_routing_config SET rollout_percent = 100 WHERE carrier_id = 'wolt';
```

Daca apar erori la 10%, dai instant rollback la 0% fara sa atingi cod.

---

## Anti-patterns (sa nu faci)

❌ **NU** importa `internal/adapters/carriers/wolt` din `internal/core/*`. CI te va opri.

❌ **NU** adauga campuri Wolt-specifice in domain models (`Delivery`, `CustodyEvent`). Daca ai nevoie, foloseste `Metadata json.RawMessage`.

❌ **NU** lipi if/else pe `if carrier == "wolt"` in core. Daca routing-ul are nevoie de tratament special, expune o capability noua si fa decision-ul pe ea.

❌ **NU** sari peste contract tests. *Toate* trebuie sa treaca. Daca un test nu se aplica carrier-ului tau (ex. Wolt nu suporta locker pickup → testul de locker pickup nu se aplica), updatezi `Capabilities()` corespunzator si testul devine skip-automat.

❌ **NU** loga payload-uri raw Wolt fara redactare PII (telefon, adresa).

❌ **NU** foloseste timestamp-ul Wolt drept sursa de adevar. Folosesti `time.Now()` pe server cand procesezi webhook-ul, si pui timestamp-ul Wolt in `CarrierTime` separat.

---

## Checklist pre-merge

Cand iti deschizi PR-ul cu integrarea Wolt:

- [ ] `Capabilities()` returneaza valori corecte, cu zone GeoJSON valide
- [ ] Toate cele 5 metode `CarrierPort` sunt implementate
- [ ] Contract tests pass: `make test-contract CARRIER=wolt`
- [ ] Webhook signature verification implementat
- [ ] Mapper acopera **toate** statusurile cunoscute (vezi types.go cu OpenAPI lor)
- [ ] Erorile carrier sunt mapate la `ports.Err*` (pentru ca routing engine-ul sa stie de fallback)
- [ ] Rate limit respectat (nu trimitem mai mult decat permite Wolt)
- [ ] Idempotency: acelasi `IdempotencyKey` returneaza acelasi `ExternalID`
- [ ] Logs nu contin PII raw
- [ ] Documentatie API (link la docs Wolt oficial in README-ul adapter-ului)
- [ ] Feature flag DB seed in migrations
- [ ] Monitor dashboards updated (success_rate, latency per carrier)

---

## Anexa: erori standard

Mapeaza erorile carrier la setul standard:

```go
ports.ErrCarrierUnavailable     // 5xx, timeout
ports.ErrInvalidRequest          // 4xx with our request invalid
ports.ErrCapacityExceeded        // carrier zice "no riders available"
ports.ErrOutOfZone               // adresa in afara coverage
ports.ErrOversizedPackage        // peste limita lor
ports.ErrInvalidSignature        // webhook signature failed
ports.ErrIdempotencyMismatch     // same key, different payload
ports.ErrRateLimited             // 429
ports.ErrAuthFailed              // 401, 403
```

Routing engine-ul aliniaza fallback-ul in functie de tipul erorii.
