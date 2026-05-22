-- DashFetchr initial schema
-- PostgreSQL 16+

BEGIN;

-- ============================================================================
-- EXTENSIONS
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- ============================================================================
-- ENUMS (as CHECK constraints, easier to extend)
-- ============================================================================

-- ============================================================================
-- RETAILERS
-- ============================================================================
CREATE TABLE retailers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    api_key_hash    TEXT NOT NULL,
    webhook_url     TEXT,
    webhook_secret  TEXT,
    settings        JSONB NOT NULL DEFAULT '{}'::jsonb,
    sla             JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retailers_active ON retailers(is_active) WHERE is_active;

-- ============================================================================
-- CUSTOMERS (PII encrypted at app layer)
-- ============================================================================
CREATE TABLE customers (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_e164          TEXT UNIQUE NOT NULL,
    email               TEXT,
    name_encrypted      BYTEA,
    address_default     JSONB,
    locale              TEXT NOT NULL DEFAULT 'ro',
    consent_marketing   BOOLEAN NOT NULL DEFAULT FALSE,
    consent_marketing_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX idx_customers_phone ON customers(phone_e164) WHERE deleted_at IS NULL;

-- ============================================================================
-- AWB (internal tracking number, normalized across carriers)
-- ============================================================================
CREATE TABLE awbs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    internal_awb        TEXT UNIQUE NOT NULL,  -- e.g. DF-2026-A1B2C3D4
    retailer_id         UUID NOT NULL REFERENCES retailers(id) ON DELETE RESTRICT,
    customer_id         UUID REFERENCES customers(id) ON DELETE SET NULL,
    external_awbs       JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- Example: [{"carrier":"sameday","awb":"123ABC","role":"mid_mile"},
    --          {"carrier":"bolt_food","awb":"BLT-789","role":"last_mile"}]
    package             JSONB NOT NULL,
    -- {"weight_kg":1.2,"length_cm":30,"width_cm":20,"height_cm":10,"declared_value_minor":15000,"currency":"RON"}
    declared_value_minor INT,
    declared_value_currency TEXT,
    state               TEXT NOT NULL,
    -- created | at_locker | scheduled | dispatched | in_transit | delivered | failed | cancelled | returned
    state_reason        TEXT,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_awb_state CHECK (state IN (
        'created','at_locker','scheduled','dispatched','in_transit',
        'delivered','failed','cancelled','returned'
    ))
);

CREATE INDEX idx_awbs_retailer ON awbs(retailer_id);
CREATE INDEX idx_awbs_customer ON awbs(customer_id);
CREATE INDEX idx_awbs_state_active ON awbs(state, updated_at) 
    WHERE state NOT IN ('delivered','cancelled');
CREATE INDEX idx_awbs_external_awbs_gin ON awbs USING gin (external_awbs jsonb_path_ops);

-- ============================================================================
-- DELIVERIES (concrete legs of an AWB; one AWB can have multiple)
-- ============================================================================
CREATE TABLE deliveries (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    awb_id                  UUID NOT NULL REFERENCES awbs(id) ON DELETE CASCADE,
    leg_number              INT NOT NULL CHECK (leg_number >= 1),
    -- 1 = first/mid mile, 2 = last mile (locker → home), N = next legs
    carrier_id              TEXT NOT NULL,        -- "bolt_food", "sameday", etc.
    carrier_version         TEXT NOT NULL,        -- "v1", "v2"
    carrier_external_id     TEXT,                  -- ID-ul cursei la carrier
    
    pickup                  JSONB NOT NULL,
    -- {"type":"locker|address|hub","lat":...,"lng":...,"address":"...","locker_id":"L123","access_code":"<encrypted>"}
    drop                    JSONB NOT NULL,
    -- {"type":"address","lat":...,"lng":...,"address":"...","floor":"...","instructions":"..."}
    
    scheduled_window        TSTZRANGE,             -- intervalul ales de client
    
    state                   TEXT NOT NULL,
    -- pending | assigned | en_route_pickup | picked_up | in_transit | en_route_drop | delivered | failed | cancelled | returned_to_origin
    state_reason            TEXT,
    
    rider                   JSONB,
    -- {"id":"...","name":"...","phone":"...","vehicle":"motorcycle|bike|foot|car"}
    
    price_quoted_minor      INT NOT NULL,
    price_charged_minor     INT,
    price_currency          TEXT NOT NULL DEFAULT 'RON',
    
    estimated_pickup_at     TIMESTAMPTZ,
    estimated_drop_at       TIMESTAMPTZ,
    actual_pickup_at        TIMESTAMPTZ,
    actual_drop_at          TIMESTAMPTZ,
    
    idempotency_key         TEXT,
    metadata                JSONB NOT NULL DEFAULT '{}'::jsonb,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (awb_id, leg_number),
    UNIQUE (carrier_id, carrier_external_id),
    UNIQUE (idempotency_key) DEFERRABLE INITIALLY DEFERRED,
    
    CONSTRAINT chk_delivery_state CHECK (state IN (
        'pending','assigned','en_route_pickup','picked_up',
        'in_transit','en_route_drop','delivered','failed',
        'cancelled','returned_to_origin'
    ))
);

CREATE INDEX idx_deliveries_state ON deliveries(state, updated_at);
CREATE INDEX idx_deliveries_carrier ON deliveries(carrier_id, state);
CREATE INDEX idx_deliveries_awb ON deliveries(awb_id);
CREATE INDEX idx_deliveries_scheduled ON deliveries USING gist (scheduled_window);

-- ============================================================================
-- CUSTODY EVENTS (append-only, hash-chained)
-- ============================================================================
CREATE TABLE custody_events (
    event_id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    delivery_id     UUID NOT NULL REFERENCES deliveries(id) ON DELETE RESTRICT,
    sequence_num    BIGINT NOT NULL,
    
    type            TEXT NOT NULL,
    -- package.dropped_at_locker | pickup.requested_by_customer | rider.assigned
    -- | rider.arrived_at_locker | package.picked_up | rider.in_transit
    -- | rider.arrived_at_destination | package.delivered | delivery.failed
    -- | package.returned_to_locker
    
    occurred_at     TIMESTAMPTZ NOT NULL,
    carrier_time    TIMESTAMPTZ,
    
    actor           JSONB NOT NULL,
    -- {"type":"rider|system|customer|carrier","carrier_id":"bolt_food","id":"...","name":"..."}
    
    location        JSONB,
    -- {"lat":...,"lng":...,"accuracy_m":...,"altitude_m":...,"heading":...}
    
    photos          JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- [{"s3_uri":"...","sha256":"...","content_type":"image/jpeg","width":...,"height":...,"exif_gps":{...}}]
    
    signature       JSONB,
    
    reason          TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    
    prev_hash       TEXT NOT NULL,
    hash            TEXT NOT NULL,
    
    UNIQUE (delivery_id, sequence_num)
);

CREATE INDEX idx_custody_delivery ON custody_events(delivery_id, sequence_num);
CREATE INDEX idx_custody_type ON custody_events(type, occurred_at);

-- Enforce append-only: trigger prevents UPDATE/DELETE
CREATE OR REPLACE FUNCTION custody_events_prevent_modification() 
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'custody_events is append-only; UPDATE/DELETE not allowed';
END;
$$;

CREATE TRIGGER trg_custody_no_update 
    BEFORE UPDATE ON custody_events
    FOR EACH ROW 
    EXECUTE FUNCTION custody_events_prevent_modification();

CREATE TRIGGER trg_custody_no_delete 
    BEFORE DELETE ON custody_events
    FOR EACH ROW 
    EXECUTE FUNCTION custody_events_prevent_modification();

-- ============================================================================
-- ROUTING DECISIONS (audit trail)
-- ============================================================================
CREATE TABLE routing_decisions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    delivery_id         UUID NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
    strategy            TEXT NOT NULL,
    candidates          JSONB NOT NULL,
    -- [{"carrier":"bolt_food","cost_minor":900,"eta_min":20,"score":0.87,"available":true},...]
    chosen              JSONB NOT NULL,
    reasoning           TEXT,
    decided_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_delivery ON routing_decisions(delivery_id);
CREATE INDEX idx_routing_strategy ON routing_decisions(strategy, decided_at DESC);

-- ============================================================================
-- CARRIER CONFIGURATION
-- ============================================================================
CREATE TABLE carrier_credentials (
    carrier_id          TEXT PRIMARY KEY,
    encrypted_creds     BYTEA NOT NULL,
    kms_key_id          TEXT NOT NULL,
    sandbox             BOOLEAN NOT NULL DEFAULT FALSE,
    rotated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE carrier_routing_config (
    carrier_id          TEXT NOT NULL,
    version             TEXT NOT NULL,
    rollout_percent     INT NOT NULL DEFAULT 0 CHECK (rollout_percent BETWEEN 0 AND 100),
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    notes               TEXT,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (carrier_id, version)
);

CREATE TABLE carrier_webhooks_inbox (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    carrier_id          TEXT NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    signature_valid     BOOLEAN NOT NULL,
    raw_body            BYTEA NOT NULL,
    headers             JSONB NOT NULL,
    processed_at        TIMESTAMPTZ,
    error               TEXT,
    delivery_id         UUID REFERENCES deliveries(id) ON DELETE SET NULL,
    event_ids           UUID[]
);

CREATE INDEX idx_webhooks_unprocessed 
    ON carrier_webhooks_inbox(carrier_id, received_at) 
    WHERE processed_at IS NULL;

CREATE INDEX idx_webhooks_delivery ON carrier_webhooks_inbox(delivery_id);

-- ============================================================================
-- PAYMENTS
-- ============================================================================
CREATE TABLE payments (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    delivery_id         UUID NOT NULL REFERENCES deliveries(id) ON DELETE RESTRICT,
    provider            TEXT NOT NULL,             -- 'stripe' | 'netopia'
    provider_payment_id TEXT,
    amount_minor        INT NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'RON',
    state               TEXT NOT NULL,             -- pending | authorized | captured | failed | refunded
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_payment_id)
);

CREATE INDEX idx_payments_delivery ON payments(delivery_id);
CREATE INDEX idx_payments_state ON payments(state) WHERE state IN ('pending','authorized');

-- ============================================================================
-- NOTIFICATIONS LOG
-- ============================================================================
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id     UUID REFERENCES customers(id) ON DELETE SET NULL,
    delivery_id     UUID REFERENCES deliveries(id) ON DELETE SET NULL,
    channel         TEXT NOT NULL,                 -- 'whatsapp' | 'sms' | 'email' | 'push'
    template        TEXT NOT NULL,
    payload         JSONB NOT NULL,
    state           TEXT NOT NULL,                 -- 'queued' | 'sent' | 'delivered' | 'failed'
    provider        TEXT,
    provider_id     TEXT,
    error           TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_customer ON notifications(customer_id);
CREATE INDEX idx_notifications_delivery ON notifications(delivery_id);
CREATE INDEX idx_notifications_state ON notifications(state, created_at) 
    WHERE state IN ('queued','failed');

-- ============================================================================
-- AUDIT LOG (general purpose, for admin actions etc)
-- ============================================================================
CREATE TABLE audit_log (
    id              BIGSERIAL PRIMARY KEY,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type      TEXT NOT NULL,  -- 'system' | 'admin' | 'customer' | 'rider' | 'retailer' | 'carrier'
    actor_id        TEXT,
    action          TEXT NOT NULL,
    target_type     TEXT,
    target_id       TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address      INET,
    user_agent      TEXT
);

CREATE INDEX idx_audit_occurred_at ON audit_log(occurred_at DESC);
CREATE INDEX idx_audit_actor ON audit_log(actor_type, actor_id);
CREATE INDEX idx_audit_target ON audit_log(target_type, target_id);

-- ============================================================================
-- IDEMPOTENCY KEYS (for inbound API requests)
-- ============================================================================
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    retailer_id     UUID REFERENCES retailers(id) ON DELETE CASCADE,
    method          TEXT NOT NULL,
    path            TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INT,
    response_body   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_idempotency_created ON idempotency_keys(created_at);

-- ============================================================================
-- UPDATED_AT TRIGGERS
-- ============================================================================
CREATE OR REPLACE FUNCTION set_updated_at() 
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_retailers_updated_at BEFORE UPDATE ON retailers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_awbs_updated_at BEFORE UPDATE ON awbs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_deliveries_updated_at BEFORE UPDATE ON deliveries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_payments_updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
