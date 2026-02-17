-- Add unified max_storage_bytes column to billing_plans.
-- This covers all storage (cache + artifacts) under one quota.
ALTER TABLE billing_plans ADD COLUMN IF NOT EXISTS max_storage_bytes BIGINT;

-- Backfill: set max_storage_bytes to match max_cache_bytes for existing plans
-- so the unified storage limit starts at the same level as the old cache-only limit.
UPDATE billing_plans SET max_storage_bytes = max_cache_bytes WHERE max_cache_bytes IS NOT NULL;
