-- Store which user linked the repo (for clone auth: use their stored GitHub token).
ALTER TABLE projects ADD COLUMN IF NOT EXISTS repo_linked_by_user_id UUID REFERENCES users(id);
