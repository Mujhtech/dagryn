-- Task results table for individual task execution results
CREATE TABLE task_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    task_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,         -- 'pending', 'running', 'success', 'failed', 'cached', 'skipped', 'cancelled'
    duration_ms BIGINT,
    exit_code INT,
    output TEXT,                          -- Combined stdout/stderr
    error_message TEXT,
    cache_hit BOOLEAN DEFAULT FALSE,
    cache_key VARCHAR(64),               -- Cache key if applicable
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for run's task results
CREATE INDEX idx_task_results_run_id ON task_results(run_id);

-- Index for finding task by name within a run
CREATE INDEX idx_task_results_run_task ON task_results(run_id, task_name);

-- Index for status-based queries
CREATE INDEX idx_task_results_status ON task_results(run_id, status);
