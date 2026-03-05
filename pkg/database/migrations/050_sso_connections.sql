-- +migrate Up
CREATE TABLE IF NOT EXISTS sso_connections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL UNIQUE REFERENCES teams(id) ON DELETE CASCADE,

    -- IdP configuration
    idp_entity_id   TEXT NOT NULL DEFAULT '',
    idp_sso_url     TEXT NOT NULL DEFAULT '',
    idp_metadata_url TEXT,
    idp_metadata_xml TEXT,
    certificate      TEXT NOT NULL DEFAULT '',

    -- SP configuration (auto-derived from base URL + team slug)
    sp_entity_id    TEXT NOT NULL DEFAULT '',
    sp_acs_url      TEXT NOT NULL DEFAULT '',

    -- SCIM provisioning
    scim_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    scim_token_hash TEXT,

    -- Enforcement
    enforce_sso     BOOLEAN NOT NULL DEFAULT FALSE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sso_connections_team_id ON sso_connections(team_id);
CREATE UNIQUE INDEX idx_sso_connections_sp_entity_id ON sso_connections(sp_entity_id) WHERE sp_entity_id != '';

-- +migrate Down
DROP TABLE IF EXISTS sso_connections;
