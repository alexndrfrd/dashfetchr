#!/usr/bin/env bash
# End-to-end demo: booking → custody pickup → custody delivery
# Prerequisites: docker compose up, migrations applied, API running (make run-api)
set -euo pipefail

API="${API:-http://localhost:8080}"
# Dev pilot retailer API key (seeded by migration 0003 / in-memory store).
API_KEY="${API_KEY:-df_dev_pilot_2026}"

SLOT_START=$(date -u -v+1H +"%Y-%m-%dT%H:00:00+00:00" 2>/dev/null || date -u -d "+1 hour" +"%Y-%m-%dT%H:00:00+00:00")
SLOT_END=$(date -u -v+2H +"%Y-%m-%dT%H:00:00+00:00" 2>/dev/null || date -u -d "+2 hours" +"%Y-%m-%dT%H:00:00+00:00")

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

echo "==> Get booking"
curl -sf "$API/v1/bookings/$DELIVERY_ID" -H "Authorization: Bearer $API_KEY" | jq .

echo "==> Custody: picked up"
curl -sf -X POST "$API/v1/deliveries/$DELIVERY_ID/custody" \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "package.picked_up",
    "actor": {"type": "rider", "carrier_id": "bolt_food", "id": "r1", "name": "Andrei"},
    "location": {"lat": 44.4546, "lng": 26.0987, "accuracy_m": 8},
    "photos": [{"s3_uri": "s3://dashfetchr-pod/demo/pickup.jpg", "sha256": "demo-pickup"}]
  }' | jq .

echo "==> Custody: delivered"
curl -sf -X POST "$API/v1/deliveries/$DELIVERY_ID/custody" \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "package.delivered",
    "actor": {"type": "rider", "carrier_id": "bolt_food", "id": "r1", "name": "Andrei"},
    "location": {"lat": 44.4632, "lng": 26.1062, "accuracy_m": 5},
    "photos": [{"s3_uri": "s3://dashfetchr-pod/demo/delivery.jpg", "sha256": "demo-delivery"}]
  }' | jq .

echo "==> Final status"
curl -sf "$API/v1/bookings/$DELIVERY_ID" -H "Authorization: Bearer $API_KEY" | jq .

echo "Done. delivery_id=$DELIVERY_ID"
