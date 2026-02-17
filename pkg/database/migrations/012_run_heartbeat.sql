-- Add heartbeat tracking columns for offline handling
ALTER TABLE runs ADD COLUMN last_heartbeat_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE runs ADD COLUMN client_disconnected BOOLEAN DEFAULT FALSE;

-- Index for finding stale runs (running status with old heartbeat)
CREATE INDEX idx_runs_stale ON runs(status, last_heartbeat_at) 
WHERE status = 'running' AND last_heartbeat_at IS NOT NULL;
