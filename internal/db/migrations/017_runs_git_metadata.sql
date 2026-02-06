-- Add fields for git platform metadata (PR title, commit message, commit author)
ALTER TABLE runs ADD COLUMN IF NOT EXISTS pr_title TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS pr_number INT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_message TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_author_name VARCHAR(255);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_author_email VARCHAR(255);

-- Index for PR number lookups
CREATE INDEX IF NOT EXISTS idx_runs_pr_number ON runs(project_id, pr_number) WHERE pr_number IS NOT NULL;
