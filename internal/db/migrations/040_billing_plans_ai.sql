-- Add AI quota columns to billing_plans.
ALTER TABLE billing_plans ADD COLUMN IF NOT EXISTS max_ai_analyses_per_month INT;
ALTER TABLE billing_plans ADD COLUMN IF NOT EXISTS ai_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE billing_plans ADD COLUMN IF NOT EXISTS ai_suggestions_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- Seed AI values for existing plans.
UPDATE billing_plans SET max_ai_analyses_per_month = 10,  ai_enabled = TRUE, ai_suggestions_enabled = FALSE WHERE slug = 'free';
UPDATE billing_plans SET max_ai_analyses_per_month = 100, ai_enabled = TRUE, ai_suggestions_enabled = TRUE  WHERE slug = 'pro';
UPDATE billing_plans SET max_ai_analyses_per_month = 500, ai_enabled = TRUE, ai_suggestions_enabled = TRUE  WHERE slug = 'team';
UPDATE billing_plans SET max_ai_analyses_per_month = NULL, ai_enabled = TRUE, ai_suggestions_enabled = TRUE  WHERE slug = 'enterprise';
