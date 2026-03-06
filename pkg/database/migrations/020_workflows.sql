-- Store workflow definitions synced from dagryn.toml
CREATE TABLE project_workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    is_default BOOLEAN DEFAULT false,
    config_hash VARCHAR(64),          -- SHA-256 of raw TOML for change detection
    raw_config TEXT,                  -- Original TOML content for diff viewing
    synced_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, name)
);

-- Store task definitions within a workflow
CREATE TABLE workflow_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES project_workflows(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    command TEXT NOT NULL,
    needs TEXT[],         -- Array of dependency task names
    inputs TEXT[],
    outputs TEXT[],
    plugins TEXT[],       -- Resolved plugin specs
    timeout_seconds INT,
    workdir VARCHAR(255),
    env JSONB,            -- Key-value env vars
    UNIQUE(workflow_id, name)
);

-- Link runs to workflow snapshots for versioning
ALTER TABLE runs ADD COLUMN workflow_id UUID REFERENCES project_workflows(id);
ALTER TABLE runs ADD COLUMN workflow_name VARCHAR(255);

CREATE INDEX idx_project_workflows_project ON project_workflows(project_id);
CREATE INDEX idx_project_workflows_hash ON project_workflows(config_hash);
CREATE INDEX idx_workflow_tasks_workflow ON workflow_tasks(workflow_id);
CREATE INDEX idx_runs_workflow ON runs(workflow_id);
