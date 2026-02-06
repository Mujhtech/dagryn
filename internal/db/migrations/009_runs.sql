-- Runs table for workflow execution history
CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    targets TEXT[],                      -- Tasks that were requested
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'success', 'failed', 'cancelled'
    total_tasks INT DEFAULT 0,
    completed_tasks INT DEFAULT 0,
    failed_tasks INT DEFAULT 0,
    cache_hits INT DEFAULT 0,
    duration_ms BIGINT,
    error_message TEXT,
    triggered_by VARCHAR(50),            -- 'cli', 'api', 'dashboard', 'ci'
    triggered_by_user_id UUID REFERENCES users(id),
    git_branch VARCHAR(255),
    git_commit VARCHAR(40),
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for project's runs (most recent first)
CREATE INDEX idx_runs_project_id ON runs(project_id, created_at DESC);

-- Index for finding running jobs
CREATE INDEX idx_runs_status ON runs(status) WHERE status IN ('pending', 'running');

-- Index for user's runs
CREATE INDEX idx_runs_user_id ON runs(triggered_by_user_id) WHERE triggered_by_user_id IS NOT NULL;
