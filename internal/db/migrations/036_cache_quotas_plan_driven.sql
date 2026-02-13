-- Extend cache_quotas to track bandwidth and link to billing account.
ALTER TABLE cache_quotas ADD COLUMN IF NOT EXISTS billing_account_id UUID
    REFERENCES billing_accounts(id);
ALTER TABLE cache_quotas ADD COLUMN IF NOT EXISTS max_bandwidth_bytes BIGINT
    NOT NULL DEFAULT 5368709120;
ALTER TABLE cache_quotas ADD COLUMN IF NOT EXISTS current_bandwidth_bytes BIGINT
    NOT NULL DEFAULT 0;
ALTER TABLE cache_quotas ADD COLUMN IF NOT EXISTS bandwidth_reset_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_cache_quotas_billing ON cache_quotas(billing_account_id)
    WHERE billing_account_id IS NOT NULL;
