-- Add version field to track workflow versions
ALTER TABLE project_workflows ADD COLUMN version INT NOT NULL DEFAULT 1;

-- Create index for version lookups
CREATE INDEX idx_project_workflows_version ON project_workflows(project_id, version DESC);
