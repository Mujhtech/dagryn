-- Project members table for direct project membership (overrides team role)
CREATE TABLE project_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',  -- 'owner', 'admin', 'member', 'viewer'
    invited_by UUID REFERENCES users(id),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, user_id)
);

-- Index for user's project memberships
CREATE INDEX idx_project_members_user_id ON project_members(user_id);

-- Index for project's members
CREATE INDEX idx_project_members_project_id ON project_members(project_id);

-- Index for role-based queries
CREATE INDEX idx_project_members_role ON project_members(project_id, role);
