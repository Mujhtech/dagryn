-- Plugin registry tables

CREATE TABLE IF NOT EXISTS plugin_publishers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(200) NOT NULL DEFAULT '',
    avatar_url TEXT,
    website TEXT,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS registry_plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    publisher_id UUID NOT NULL REFERENCES plugin_publishers(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL DEFAULT 'composite',
    license VARCHAR(50),
    homepage TEXT,
    repository_url TEXT,
    total_downloads BIGINT NOT NULL DEFAULT 0,
    weekly_downloads BIGINT NOT NULL DEFAULT 0,
    stars INTEGER NOT NULL DEFAULT 0,
    featured BOOLEAN NOT NULL DEFAULT FALSE,
    deprecated BOOLEAN NOT NULL DEFAULT FALSE,
    latest_version VARCHAR(50) NOT NULL DEFAULT '0.0.0',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (publisher_id, name)
);

CREATE TABLE IF NOT EXISTS plugin_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES registry_plugins(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    manifest_json JSONB NOT NULL DEFAULT '{}',
    checksum_sha256 VARCHAR(64),
    downloads BIGINT NOT NULL DEFAULT 0,
    yanked BOOLEAN NOT NULL DEFAULT FALSE,
    release_notes TEXT,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (plugin_id, version)
);

CREATE TABLE IF NOT EXISTS plugin_downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES registry_plugins(id) ON DELETE CASCADE,
    version_id UUID NOT NULL REFERENCES plugin_versions(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_registry_plugins_publisher ON registry_plugins(publisher_id);
CREATE INDEX IF NOT EXISTS idx_registry_plugins_type ON registry_plugins(type);
CREATE INDEX IF NOT EXISTS idx_registry_plugins_downloads ON registry_plugins(total_downloads DESC);
CREATE INDEX IF NOT EXISTS idx_registry_plugins_featured ON registry_plugins(featured) WHERE featured = TRUE;
CREATE INDEX IF NOT EXISTS idx_plugin_versions_plugin ON plugin_versions(plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_downloads_plugin ON plugin_downloads(plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_downloads_date ON plugin_downloads(created_at);

-- Full-text search index
CREATE INDEX IF NOT EXISTS idx_registry_plugins_search ON registry_plugins USING gin (
    to_tsvector('english', name || ' ' || description)
);

-- Seed official publisher
INSERT INTO plugin_publishers (name, display_name, verified)
VALUES ('dagryn', 'Dagryn', TRUE)
ON CONFLICT (name) DO NOTHING;
