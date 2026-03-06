-- Add commit author avatar URL to runs
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_author_avatar_url TEXT;
