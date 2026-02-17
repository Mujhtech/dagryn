-- Device codes table for CLI device flow authentication
CREATE TABLE device_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code VARCHAR(255) NOT NULL UNIQUE,   -- Secret code for polling
    user_code VARCHAR(20) NOT NULL UNIQUE,      -- User-friendly code (e.g., ABCD-1234)
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,  -- Set when user authorizes
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    authorized_at TIMESTAMP WITH TIME ZONE,     -- Set when user authorizes
    poll_interval INTEGER NOT NULL DEFAULT 5,   -- Minimum polling interval in seconds
    verification_uri VARCHAR(255) NOT NULL,     -- URL for user to visit
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for device code lookup (used for polling)
CREATE INDEX idx_device_codes_device_code ON device_codes(device_code);

-- Index for user code lookup (used when user enters code)
CREATE INDEX idx_device_codes_user_code ON device_codes(user_code);

-- Index for cleanup of expired device codes
CREATE INDEX idx_device_codes_expires_at ON device_codes(expires_at);
