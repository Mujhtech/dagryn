-- +migrate Up

CREATE TABLE cluster_worker_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  key_prefix VARCHAR(16) NOT NULL,
  scope_type TEXT NOT NULL,
  team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
  owner_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL,
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ,
  CONSTRAINT cluster_worker_tokens_scope_valid CHECK (scope_type IN ('team', 'personal')),
  CONSTRAINT cluster_worker_tokens_scope_binding CHECK (
    (scope_type = 'team' AND team_id IS NOT NULL AND owner_user_id IS NULL) OR
    (scope_type = 'personal' AND owner_user_id IS NOT NULL AND team_id IS NULL)
  )
);

CREATE UNIQUE INDEX idx_cluster_worker_tokens_hash ON cluster_worker_tokens(key_hash);
CREATE INDEX idx_cluster_worker_tokens_prefix ON cluster_worker_tokens(key_prefix);
CREATE INDEX idx_cluster_worker_tokens_team ON cluster_worker_tokens(team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_cluster_worker_tokens_owner ON cluster_worker_tokens(owner_user_id) WHERE owner_user_id IS NOT NULL;
CREATE INDEX idx_cluster_worker_tokens_active ON cluster_worker_tokens(key_prefix) WHERE revoked_at IS NULL;

-- +migrate Down

DROP TABLE IF EXISTS cluster_worker_tokens;
