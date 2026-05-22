# Glovo adapter

Status: **not implemented** (v2 — Luna 5-6 conform roadmap).

See `internal/adapters/carriers/bolt/` for the canonical template. To implement:

1. Copy the structure from `bolt/`:
   - `adapter.go` — implements `ports.CarrierPort`
   - `client.go` — HTTP transport over Glovo's `Couriers as a Service` API
   - `mapper.go` — Anti-Corruption Layer (Glovo ↔ domain)
   - `types.go` — Glovo-specific JSON shapes
   - `webhook.go` — signature verification
   - `capabilities.go` — declares max weight, zones, etc.
   - `adapter_test.go` — runs `contract.RunCarrierContract(t, adapter)`

2. Follow [`docs/ADDING_A_CARRIER.md`](../../../../docs/ADDING_A_CARRIER.md).

3. All standard contract tests in `tests/contract/` must pass.
