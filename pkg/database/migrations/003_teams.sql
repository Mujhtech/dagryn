-- Teams table for grouping projects and users
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,  -- URL-friendly identifier
    owner_id UUID NOT NULL REFERENCES users(id),
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for owner lookups
CREATE INDEX idx_teams_owner_id ON teams(owner_id);

-- Index for slug lookups
CREATE INDEX idx_teams_slug ON teams(slug);
