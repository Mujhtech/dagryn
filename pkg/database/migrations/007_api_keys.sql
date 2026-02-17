-- API keys table for CLI/CI authentication
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,  -- NULL = user scope
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,     -- SHA-256 hash of key
    key_prefix VARCHAR(10) NOT NULL,    -- First 10 chars for identification (e.g., "dg_live_abc")
    scope VARCHAR(50) NOT NULL,         -- 'user' or 'project'
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,  -- NULL = never expires
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraint: project_id required if scope is 'project'
    CONSTRAINT check_project_scope CHECK (
        (scope = 'user' AND project_id IS NULL) OR
        (scope = 'project' AND project_id IS NOT NULL)
    )
);

-- Index for key prefix lookups (fast API key validation)
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Index for user's API keys
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

-- Index for project's API keys
CREATE INDEX idx_api_keys_project_id ON api_keys(project_id) WHERE project_id IS NOT NULL;

-- Index for non-revoked keys
CREATE INDEX idx_api_keys_active ON api_keys(key_prefix) WHERE revoked_at IS NULL;
