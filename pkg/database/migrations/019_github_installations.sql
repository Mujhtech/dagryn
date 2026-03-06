-- GitHub App installations and project linkage

-- Track GitHub App installations that Dagryn knows about.
CREATE TABLE IF NOT EXISTS github_installations (
    id UUID PRIMARY KEY,
    installation_id BIGINT NOT NULL UNIQUE,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL, -- e.g. "User" or "Organization"
    account_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Link projects to a specific GitHub App installation and repository.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS github_installation_id UUID,
    ADD COLUMN IF NOT EXISTS github_repo_id BIGINT;

-- Index for fast lookup by installation + repo (used for webhooks).
CREATE INDEX IF NOT EXISTS idx_projects_github_installation_repo
    ON projects(github_installation_id, github_repo_id)
    WHERE github_installation_id IS NOT NULL AND github_repo_id IS NOT NULL;

