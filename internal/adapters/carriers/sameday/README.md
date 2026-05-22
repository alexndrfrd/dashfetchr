# Sameday adapter

Status: **not implemented** (v1 webhook listener, v2 full integration).

Role: `first_mile` / `mid_mile` — Sameday delivers parcels to easyboxes,
which is the trigger event for our concierge service.

Key responsibilities:
- Receive Sameday tracking webhooks (`package.dropped_at_locker`)
- Map their AWB to ours
- v2: programmatic locker access via B2B partnership API

See `internal/adapters/carriers/bolt/` for the canonical template
and [`docs/ADDING_A_CARRIER.md`](../../../../docs/ADDING_A_CARRIER.md).
