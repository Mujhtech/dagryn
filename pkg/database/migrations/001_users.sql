-- Users table for OAuth authenticated users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    avatar_url TEXT,
    provider VARCHAR(50) NOT NULL,      -- 'github' or 'google'
    provider_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider, provider_id)
);

-- Index for faster lookups by email
CREATE INDEX idx_users_email ON users(email);

-- Index for OAuth provider lookups
CREATE INDEX idx_users_provider ON users(provider, provider_id);
