-- Team members table for team membership and roles
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',  -- 'owner', 'admin', 'member'
    invited_by UUID REFERENCES users(id),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

-- Index for user's team memberships
CREATE INDEX idx_team_members_user_id ON team_members(user_id);

-- Index for team's members
CREATE INDEX idx_team_members_team_id ON team_members(team_id);

-- Index for role-based queries
CREATE INDEX idx_team_members_role ON team_members(team_id, role);
