-- Audit webhooks for SIEM integration (Splunk, Datadog, etc.)
CREATE TABLE IF NOT EXISTS audit_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret_encrypted TEXT NOT NULL,
    description TEXT DEFAULT '',
    event_filter TEXT[] DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_by UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_audit_webhooks_team_active ON audit_webhooks(team_id) WHERE is_active = true;
