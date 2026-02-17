-- Add repo_url to projects for remote run execution (clone on trigger)
ALTER TABLE projects ADD COLUMN IF NOT EXISTS repo_url VARCHAR(512);

-- Index for webhook/project lookup by repo (optional, for future use)
-- CREATE INDEX idx_projects_repo_url ON projects(repo_url) WHERE repo_url IS NOT NULL;
