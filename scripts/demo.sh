#!/usr/bin/env bash
# End-to-end demo: booking → full custody chain → delivered
# Works in memory mode (USE_MEMORY_STORE=true) — no Bolt key needed.
# Prerequisites: API running (make run-api or USE_MEMORY_STORE=true go run ./cmd/api)
set -euo pipefail

API="${API:-http://localhost:8080}"
# Dev pilot retailer API key (seeded by migration 0003 / in-memory store).
API_KEY="${API_KEY:-df_dev_pilot_2026}"
# Rider token: {carrier_id}:{secret} — default matches BOLT_RIDER_SECRET dev default.
RIDER_TOKEN="${RIDER_TOKEN:-bolt_food:local-dev-rider-secret}"
RIDER='{"type":"rider","carrier_id":"bolt_food","id":"r1","name":"Andrei"}'

SLOT_START=$(date -u -v+1H +"%Y-%m-%dT%H:00:00+00:00" 2>/dev/null || date -u -d "+1 hour" +"%Y-%m-%dT%H:00:00+00:00")
SLOT_END=$(date -u -v+2H +"%Y-%m-%dT%H:00:00+00:00" 2>/dev/null || date -u -d "+2 hours" +"%Y-%m-%dT%H:00:00+00:00")

custody() {
  local type="$1" location="$2" photos="${3:-}"
  local payload="{\"type\":\"$type\",\"actor\":$RIDER,\"location\":$location"
  [[ -n "$photos" ]] && payload="$payload,\"photos\":[$photos]"
  payload="$payload}"
  curl -sf -X POST "$API/v1/deliveries/$DELIVERY_ID/custody" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $RIDER_TOKEN" \
    -d "$payload" | jq .
}

PICKUP_LOC='{"lat":44.4546,"lng":26.0987,"accuracy_m":8}'
DROP_LOC='{"lat":44.4632,"lng":26.1062,"accuracy_m":5}'
PICKUP_PHOTO='{"s3_uri":"s3://dashfetchr-pod/demo/pickup.jpg","sha256":"demo-pickup"}'
DELIVERY_PHOTO='{"s3_uri":"s3://dashfetchr-pod/demo/delivery.jpg","sha256":"demo-delivery"}'

echo "==> Health"
curl -sf "$API/healthz" | jq .

echo "==> Create booking"
BOOKING=$(curl -sf -X POST "$API/v1/bookings" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $API_KEY" \
  -d "{
    \"external_awb\": \"SD-DEMO-001\",
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
    \"slot_start\": \"$SLOT_START\",
    \"slot_end\": \"$SLOT_END\"
  }")
echo "$BOOKING" | jq .

DELIVERY_ID=$(echo "$BOOKING" | jq -r '.delivery_id')
if [[ -z "$DELIVERY_ID" || "$DELIVERY_ID" == "null" ]]; then
  echo "booking failed" >&2
  exit 1
fi

echo "==> Get booking (state: pending)"
curl -sf "$API/v1/bookings/$DELIVERY_ID" -H "Authorization: Bearer $API_KEY" | jq '{state:.delivery.State,awb_state:.awb.State}'

echo "==> Custody: rider assigned (pending → assigned)"
custody "rider.assigned" "$PICKUP_LOC"

echo "==> Custody: rider arrived at locker (assigned → en_route_pickup)"
custody "rider.arrived_at_locker" "$PICKUP_LOC" "$PICKUP_PHOTO"

echo "==> Custody: package picked up (en_route_pickup → picked_up)"
custody "package.picked_up" "$PICKUP_LOC" "$PICKUP_PHOTO"

echo "==> Custody: rider in transit (picked_up → in_transit)"
custody "rider.in_transit" "$DROP_LOC"

echo "==> Custody: package delivered (in_transit → delivered)"
custody "package.delivered" "$DROP_LOC" "$DELIVERY_PHOTO"

echo "==> Final status"
curl -sf "$API/v1/bookings/$DELIVERY_ID" -H "Authorization: Bearer $API_KEY" \
  | jq '{state:.delivery.State,custody_events:(.custody|length)}'

echo "Done. delivery_id=$DELIVERY_ID"
