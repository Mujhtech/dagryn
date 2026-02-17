-- A billing account is the entity that owns a subscription.
-- Either a solo user or a team. Exactly one of user_id / team_id is set.
CREATE TABLE IF NOT EXISTS billing_accounts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    team_id             UUID UNIQUE REFERENCES teams(id) ON DELETE CASCADE,
    stripe_customer_id  TEXT UNIQUE,
    email               TEXT NOT NULL,
    name                TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT billing_accounts_owner_check CHECK (
        (user_id IS NOT NULL AND team_id IS NULL) OR
        (user_id IS NULL AND team_id IS NOT NULL)
    )
);

CREATE INDEX idx_billing_accounts_user ON billing_accounts(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_billing_accounts_team ON billing_accounts(team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_billing_accounts_stripe ON billing_accounts(stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;
