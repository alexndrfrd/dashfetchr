-- Dev seed: pilot retailer for local API demos (curl / scripts/demo.sh)
BEGIN;

INSERT INTO retailers (id, name, slug, api_key_hash, settings, sla)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Dev Pilot Retailer',
    'dev-pilot',
    'dev-not-for-production',
    '{}'::jsonb,
    '{}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

COMMIT;
