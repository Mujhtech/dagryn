-- AI inline code suggestions (v2).
CREATE TABLE IF NOT EXISTS ai_suggestions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id       UUID NOT NULL REFERENCES ai_analyses(id) ON DELETE CASCADE,
    run_id            UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    file_path         TEXT NOT NULL,
    start_line        INT NOT NULL,
    end_line          INT NOT NULL,
    original_code     TEXT NOT NULL DEFAULT '',
    suggested_code    TEXT NOT NULL DEFAULT '',
    explanation       TEXT NOT NULL DEFAULT '',
    confidence        FLOAT NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'pending',
    github_comment_id TEXT,
    risk_score        FLOAT,
    failure_reason    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_suggestions_analysis_id ON ai_suggestions(analysis_id);
CREATE INDEX idx_ai_suggestions_run_id ON ai_suggestions(run_id);
CREATE INDEX idx_ai_suggestions_status ON ai_suggestions(status);
