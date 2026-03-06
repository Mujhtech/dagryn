-- AI analysis results and their publication destinations.

CREATE TABLE IF NOT EXISTS ai_analyses (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id               UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status               TEXT NOT NULL DEFAULT 'pending',
    provider             TEXT,
    provider_mode        TEXT,
    model                TEXT,
    prompt_version       TEXT,
    prompt_hash          TEXT,
    response_hash        TEXT,
    summary              TEXT,
    root_cause           TEXT,
    confidence           FLOAT,
    evidence_json        JSONB NOT NULL DEFAULT '{}',
    raw_response_blob_key TEXT,
    error_message        TEXT,
    dedup_key            TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_analyses_project_id ON ai_analyses(project_id);
CREATE INDEX idx_ai_analyses_run_id ON ai_analyses(run_id);
CREATE INDEX idx_ai_analyses_dedup_key ON ai_analyses(dedup_key);
CREATE INDEX idx_ai_analyses_status ON ai_analyses(status);

CREATE TABLE IF NOT EXISTS ai_publications (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id   UUID NOT NULL REFERENCES ai_analyses(id) ON DELETE CASCADE,
    run_id        UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    destination   TEXT NOT NULL,
    external_id   TEXT,
    status        TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_publications_analysis_id ON ai_publications(analysis_id);
CREATE UNIQUE INDEX idx_ai_publications_run_destination ON ai_publications(run_id, destination);
