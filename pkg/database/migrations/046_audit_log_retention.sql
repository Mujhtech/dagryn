-- Retention policy per team (how long to keep audit logs).
CREATE TABLE IF NOT EXISTS audit_log_retention_policies (
    team_id        UUID PRIMARY KEY,
    retention_days INT NOT NULL DEFAULT 90,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     UUID
);
