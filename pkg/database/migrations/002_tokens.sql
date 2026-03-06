-- Tokens table for JWT reference data (for revocation)
CREATE TABLE tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID,  -- NULL for user-scope tokens, references projects(id) when set
    token_type VARCHAR(50) NOT NULL,    -- 'access', 'refresh', 'device_code'
    jti VARCHAR(255) NOT NULL UNIQUE,   -- JWT ID for revocation lookup
    issued_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for JWT ID lookup (used on every authenticated request)
CREATE INDEX idx_tokens_jti ON tokens(jti);

-- Index for user's tokens
CREATE INDEX idx_tokens_user_id ON tokens(user_id);

-- Index for cleanup of expired tokens
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);

-- Index for project-scoped tokens
CREATE INDEX idx_tokens_project_id ON tokens(project_id) WHERE project_id IS NOT NULL;
