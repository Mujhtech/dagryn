-- Projects table for linked workflow projects
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,  -- Optional team
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,         -- URL-friendly identifier
    path_hash VARCHAR(64),              -- SHA-256 of absolute path (for local linking)
    description TEXT,
    visibility VARCHAR(50) DEFAULT 'private',  -- 'private', 'team', 'public'
    config_path VARCHAR(255) DEFAULT 'dagryn.toml',  -- Path to config file
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_run_at TIMESTAMP WITH TIME ZONE,
    
    -- Unique slug within team (or globally if no team)
    UNIQUE(team_id, slug)
);

-- Index for team lookups
CREATE INDEX idx_projects_team_id ON projects(team_id);

-- Index for path hash lookups (linking local projects)
CREATE INDEX idx_projects_path_hash ON projects(path_hash) WHERE path_hash IS NOT NULL;

-- Index for visibility
CREATE INDEX idx_projects_visibility ON projects(visibility);

-- Add foreign key to tokens table now that projects exists
ALTER TABLE tokens ADD CONSTRAINT fk_tokens_project_id 
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
