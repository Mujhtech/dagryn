CREATE TYPE subscription_status AS ENUM (
    'active', 'trialing', 'past_due', 'canceled', 'unpaid', 'incomplete'
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_account_id      UUID NOT NULL REFERENCES billing_accounts(id) ON DELETE CASCADE,
    plan_id                 UUID NOT NULL REFERENCES billing_plans(id),
    stripe_subscription_id  TEXT UNIQUE,
    status                  subscription_status NOT NULL DEFAULT 'active',
    seat_count              INT NOT NULL DEFAULT 1,
    current_period_start    TIMESTAMPTZ,
    current_period_end      TIMESTAMPTZ,
    cancel_at_period_end    BOOLEAN NOT NULL DEFAULT FALSE,
    canceled_at             TIMESTAMPTZ,
    trial_end               TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_billing_account ON subscriptions(billing_account_id);
CREATE INDEX idx_subscriptions_stripe ON subscriptions(stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;
CREATE INDEX idx_subscriptions_status ON subscriptions(status) WHERE status = 'active';
