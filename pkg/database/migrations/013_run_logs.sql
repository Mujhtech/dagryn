-- Run logs table for storing streaming log lines
CREATE TABLE run_logs (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    task_name VARCHAR(255) NOT NULL,
    stream VARCHAR(10) NOT NULL,              -- 'stdout' or 'stderr'
    line_num INT NOT NULL,                    -- Line number within task's stream
    content TEXT NOT NULL,                    -- The actual log line
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for fetching logs by run (ordered by creation)
CREATE INDEX idx_run_logs_run_id ON run_logs(run_id, id);

-- Index for fetching logs by task within a run
CREATE INDEX idx_run_logs_run_task ON run_logs(run_id, task_name, id);

-- Index for pagination queries
CREATE INDEX idx_run_logs_run_task_line ON run_logs(run_id, task_name, line_num);

-- Comment
COMMENT ON TABLE run_logs IS 'Stores streaming log lines for run tasks';
COMMENT ON COLUMN run_logs.stream IS 'Output stream: stdout or stderr';
COMMENT ON COLUMN run_logs.line_num IS 'Line number within the task stream, starting at 1';
