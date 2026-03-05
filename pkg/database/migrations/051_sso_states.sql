-- +migrate Up
CREATE TABLE IF NOT EXISTS sso_states (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id   UUID NOT NULL REFERENCES sso_connections(id) ON DELETE CASCADE,
    relay_state     TEXT NOT NULL UNIQUE,
    redirect_url    TEXT NOT NULL DEFAULT '',
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sso_states_relay_state ON sso_states(relay_state);
CREATE INDEX idx_sso_states_expires_at ON sso_states(expires_at);

-- +migrate Down
DROP TABLE IF EXISTS sso_states;
