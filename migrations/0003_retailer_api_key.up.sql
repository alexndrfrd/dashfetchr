-- Set the dev pilot retailer's API key hash so the seeded retailer can
-- authenticate. Raw key (local dev only): df_dev_pilot_2026
-- Hash = sha256(raw), hex-encoded — see internal/auth.HashAPIKey.
BEGIN;

UPDATE retailers
SET api_key_hash = 'ae796e53c8e29e10b568210673453d72a28a13b4595efd7adb8bc445f32508e8'
WHERE id = '00000000-0000-0000-0000-000000000001';

COMMIT;
