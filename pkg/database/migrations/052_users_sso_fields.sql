-- +migrate Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS scim_external_id TEXT;

CREATE INDEX idx_users_scim_external_id ON users(scim_external_id) WHERE scim_external_id IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_users_scim_external_id;
ALTER TABLE users DROP COLUMN IF EXISTS scim_external_id;
ALTER TABLE users DROP COLUMN IF EXISTS deactivated_at;
