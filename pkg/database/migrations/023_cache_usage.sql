CREATE TABLE cache_usage (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    bytes_uploaded BIGINT NOT NULL DEFAULT 0,
    bytes_downloaded BIGINT NOT NULL DEFAULT 0,
    cache_hits INT NOT NULL DEFAULT 0,
    cache_misses INT NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, date)
);

CREATE INDEX idx_cache_usage_date ON cache_usage(date);
