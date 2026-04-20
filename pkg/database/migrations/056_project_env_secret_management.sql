-- +migrate Up

CREATE TABLE secret_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL,
  ciphertext BYTEA,
  key_ref TEXT,
  checksum TEXT,
  version TEXT,
  external_ref TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  rotated_at TIMESTAMPTZ,
  CONSTRAINT secret_records_provider_valid CHECK (provider IN ('db', 'aws_sm', 'gcp_sm', 'cloudflare')),
  CONSTRAINT secret_records_status_valid CHECK (status IN ('active', 'rotated', 'revoked')),
  CONSTRAINT secret_records_db_ciphertext_required CHECK (
    (provider = 'db' AND ciphertext IS NOT NULL) OR
    (provider <> 'db')
  )
);

CREATE TABLE project_env_vars (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  value_type TEXT NOT NULL,
  plain_value TEXT,
  environment TEXT,
  branch TEXT,
  required BOOLEAN NOT NULL DEFAULT false,
  enabled BOOLEAN NOT NULL DEFAULT true,
  provider TEXT NOT NULL,
  provider_ref TEXT,
  secret_record_id UUID REFERENCES secret_records(id) ON DELETE SET NULL,
  description TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT project_env_vars_value_type_valid CHECK (value_type IN ('plain', 'secret')),
  CONSTRAINT project_env_vars_provider_valid CHECK (provider IN ('db', 'aws_sm', 'gcp_sm', 'cloudflare')),
  CONSTRAINT project_env_vars_plain_value_rule CHECK (
    (value_type = 'plain' AND plain_value IS NOT NULL AND secret_record_id IS NULL) OR
    (value_type = 'secret')
  ),
  CONSTRAINT project_env_vars_secret_ref_rule CHECK (
    (value_type = 'plain') OR
    (provider = 'db' AND secret_record_id IS NOT NULL) OR
    (provider <> 'db' AND provider_ref IS NOT NULL)
  )
);

CREATE UNIQUE INDEX idx_project_env_vars_unique_scope
  ON project_env_vars (project_id, key, COALESCE(environment, ''), COALESCE(branch, ''))
  WHERE deleted_at IS NULL;

CREATE INDEX idx_project_env_vars_project_active
  ON project_env_vars (project_id, key)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_project_env_vars_scope
  ON project_env_vars (project_id, environment, branch)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_project_env_vars_secret_record
  ON project_env_vars (secret_record_id)
  WHERE secret_record_id IS NOT NULL;

CREATE TABLE env_audit_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  env_var_id UUID REFERENCES project_env_vars(id) ON DELETE SET NULL,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  provider TEXT,
  environment TEXT,
  branch TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE env_audit_events
  ADD CONSTRAINT env_audit_events_provider_valid CHECK (provider IS NULL OR provider IN ('db', 'aws_sm', 'gcp_sm', 'cloudflare'));

CREATE INDEX idx_env_audit_events_project_time
  ON env_audit_events (project_id, created_at DESC);

CREATE INDEX idx_env_audit_events_env_var
  ON env_audit_events (env_var_id)
  WHERE env_var_id IS NOT NULL;

-- +migrate Down

DROP TABLE IF EXISTS env_audit_events;
DROP TABLE IF EXISTS project_env_vars;
DROP TABLE IF EXISTS secret_records;
