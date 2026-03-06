-- +migrate Up

-- Clusters table: logical groupings of workers
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Workers table: registered agent nodes
-- Merges worker_resources into workers (1:1 relationship, no benefit from separation)
CREATE TABLE workers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname TEXT NOT NULL,
    os TEXT NOT NULL,
    arch TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'bare-metal',
    labels JSONB NOT NULL DEFAULT '{}',
    capabilities TEXT[] NOT NULL DEFAULT '{}',
    max_concurrent_tasks INT NOT NULL DEFAULT 4,
    version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'online',  -- online, draining, offline
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    auth_token_hash TEXT NOT NULL,           -- Hashed registration token
    cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL,

    -- Inline resource snapshot (merged from worker_resources per review recommendation)
    cpu_millicores_available BIGINT NOT NULL DEFAULT 0,
    memory_bytes_available BIGINT NOT NULL DEFAULT 0,
    disk_bytes_available BIGINT NOT NULL DEFAULT 0,
    cpu_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    active_tasks INT NOT NULL DEFAULT 0,
    resources_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fix review issue #9: UNIQUE(hostname, cluster_id) with NULL cluster_id
-- PostgreSQL treats NULL != NULL, so use a partial unique index for NULL cluster_id
-- and a regular unique constraint for non-NULL cluster_id.
CREATE UNIQUE INDEX idx_workers_hostname_cluster ON workers(hostname, cluster_id)
    WHERE cluster_id IS NOT NULL;
CREATE UNIQUE INDEX idx_workers_hostname_no_cluster ON workers(hostname)
    WHERE cluster_id IS NULL;

-- Task assignments: tracks distributed task dispatch
CREATE TABLE task_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    task_name TEXT NOT NULL,
    worker_id UUID REFERENCES workers(id) ON DELETE SET NULL,
    cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, assigned, running, completed, failed, reassigned
    assigned_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    result JSONB,                           -- Serialized task result (error as string)
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_workers_status ON workers(status);
CREATE INDEX idx_workers_cluster ON workers(cluster_id);
CREATE INDEX idx_workers_labels ON workers USING GIN(labels);
CREATE INDEX idx_task_assignments_run ON task_assignments(run_id);
CREATE INDEX idx_task_assignments_worker ON task_assignments(worker_id);
CREATE INDEX idx_task_assignments_status ON task_assignments(status);
CREATE INDEX idx_task_assignments_run_task ON task_assignments(run_id, task_name);

-- +migrate Down

DROP TABLE IF EXISTS task_assignments;
DROP TABLE IF EXISTS workers;
DROP TABLE IF EXISTS clusters;
