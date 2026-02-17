-- Stores the plan catalog (synced from Stripe Products/Prices).
CREATE TABLE IF NOT EXISTS billing_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_price_id TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    display_name    TEXT NOT NULL,
    description     TEXT,
    price_cents     INT NOT NULL DEFAULT 0,
    billing_period  TEXT NOT NULL DEFAULT 'monthly',
    is_per_seat     BOOLEAN NOT NULL DEFAULT FALSE,

    -- Quota limits (nullable = unlimited for enterprise)
    max_projects         INT,
    max_team_members     INT,
    max_cache_bytes      BIGINT,
    max_bandwidth_bytes  BIGINT,
    max_concurrent_runs  INT,
    cache_ttl_days       INT,
    artifact_retention_days INT,
    log_retention_days   INT,

    -- Feature flags
    container_execution  BOOLEAN NOT NULL DEFAULT FALSE,
    priority_queue       BOOLEAN NOT NULL DEFAULT FALSE,
    sso_enabled          BOOLEAN NOT NULL DEFAULT FALSE,
    audit_logs           BOOLEAN NOT NULL DEFAULT FALSE,

    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_plans_slug ON billing_plans(slug);
CREATE INDEX idx_billing_plans_active ON billing_plans(is_active) WHERE is_active = TRUE;


