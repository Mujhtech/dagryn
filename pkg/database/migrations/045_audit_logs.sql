-- Audit log table for tracking all actions across the system.
CREATE TABLE IF NOT EXISTS audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sequence_num  BIGSERIAL NOT NULL,
    team_id       UUID NOT NULL,
    project_id    UUID,
    actor_type    TEXT NOT NULL DEFAULT 'user',
    actor_id      UUID,
    actor_email   TEXT NOT NULL DEFAULT '',
    action        TEXT NOT NULL,
    category      TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    metadata      JSONB DEFAULT '{}',
    ip_address    TEXT NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL DEFAULT '',
    request_id    TEXT NOT NULL DEFAULT '',
    prev_hash     TEXT NOT NULL DEFAULT '',
    entry_hash    TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Primary query: list by team, ordered newest-first with cursor pagination.
CREATE INDEX IF NOT EXISTS idx_audit_logs_team_created
    ON audit_logs (team_id, created_at DESC, sequence_num DESC);

-- Filter by project within a team.
CREATE INDEX IF NOT EXISTS idx_audit_logs_project
    ON audit_logs (project_id, created_at DESC)
    WHERE project_id IS NOT NULL;

-- Filter by actor (user or api_key UUID).
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor
    ON audit_logs (actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

-- Filter by action category.
CREATE INDEX IF NOT EXISTS idx_audit_logs_category
    ON audit_logs (team_id, category, created_at DESC);

-- Filter by action name.
CREATE INDEX IF NOT EXISTS idx_audit_logs_action
    ON audit_logs (team_id, action, created_at DESC);

-- Search by actor email (survives user deletion).
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_email
    ON audit_logs (actor_email, created_at DESC)
    WHERE actor_email != '';

-- Deterministic ordering for hash chain verification.
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_logs_sequence
    ON audit_logs (team_id, sequence_num);
