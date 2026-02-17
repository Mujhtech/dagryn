-- Append-only usage event log for metered billing.
CREATE TABLE IF NOT EXISTS usage_events (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_account_id UUID NOT NULL REFERENCES billing_accounts(id) ON DELETE CASCADE,
    project_id         UUID REFERENCES projects(id) ON DELETE SET NULL,
    event_type         TEXT NOT NULL,
    quantity           BIGINT NOT NULL,
    metadata           JSONB,
    recorded_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Stripe sync tracking
    reported_to_stripe BOOLEAN NOT NULL DEFAULT FALSE,
    stripe_usage_id    TEXT
);

CREATE INDEX idx_usage_events_account_time ON usage_events(billing_account_id, recorded_at DESC);
CREATE INDEX idx_usage_events_unreported ON usage_events(billing_account_id)
    WHERE reported_to_stripe = FALSE;
CREATE INDEX idx_usage_events_type ON usage_events(event_type, recorded_at DESC);
