CREATE TABLE cache_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_name VARCHAR(255) NOT NULL,
    cache_key VARCHAR(255) NOT NULL,
    digest_hash VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    hit_count INT NOT NULL DEFAULT 0,
    last_hit_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(project_id, task_name, cache_key)
);

CREATE TABLE cache_blobs (
    digest_hash VARCHAR(64) PRIMARY KEY,
    size_bytes BIGINT NOT NULL,
    ref_count INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE cache_quotas (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    max_size_bytes BIGINT NOT NULL DEFAULT 5368709120,
    current_size_bytes BIGINT NOT NULL DEFAULT 0,
    max_entries INT NOT NULL DEFAULT 10000,
    current_entries INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_cache_entries_project ON cache_entries(project_id);
CREATE INDEX idx_cache_entries_expires ON cache_entries(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_cache_entries_last_hit ON cache_entries(last_hit_at);
CREATE INDEX idx_cache_blobs_ref ON cache_blobs(ref_count) WHERE ref_count = 0;
