BEGIN;

UPDATE retailers
SET api_key_hash = 'dev-not-for-production'
WHERE id = '00000000-0000-0000-0000-000000000001';

COMMIT;
