-- Store GitHub PR comment ID for runs so we can update the same
-- comment instead of posting a new one for every run.
ALTER TABLE runs ADD COLUMN IF NOT EXISTS github_pr_comment_id BIGINT;

