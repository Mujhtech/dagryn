-- Invitations table for pending team/project invitations
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    invited_by UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE,  -- Invite token
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Must be either team or project invite, not both
    CONSTRAINT check_invite_target CHECK (
        (team_id IS NOT NULL AND project_id IS NULL) OR
        (team_id IS NULL AND project_id IS NOT NULL)
    )
);

-- Index for email lookups
CREATE INDEX idx_invitations_email ON invitations(email);

-- Index for token lookups
CREATE INDEX idx_invitations_token ON invitations(token);

-- Index for pending invitations (not accepted, not expired)
CREATE INDEX idx_invitations_pending ON invitations(email, expires_at) 
    WHERE accepted_at IS NULL;
