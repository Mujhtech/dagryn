-- Seed Pro, Team, and Enterprise billing plans.
-- The Free plan was already seeded in 030_billing_plans.sql.

-- Seed the free plan so every new account gets it automatically.
INSERT INTO billing_plans (
    stripe_price_id, name, slug, display_name, description, price_cents,
    max_projects, max_team_members, max_cache_bytes, max_bandwidth_bytes,
    max_concurrent_runs, cache_ttl_days, artifact_retention_days,
    log_retention_days, sort_order
) VALUES (
    'price_free', 'free', 'free', 'Free', 'Basic features for individuals', 0,
    3, 1, 1073741824, 5368709120,
    2, 7, 3, 7, 0
);

-- Pro plan: $15/month, solo developer with higher limits
INSERT INTO billing_plans (
    stripe_price_id, name, slug, display_name, description,
    price_cents, billing_period, is_per_seat,
    max_projects, max_team_members, max_cache_bytes, max_bandwidth_bytes,
    max_concurrent_runs, cache_ttl_days, artifact_retention_days,
    log_retention_days, container_execution, priority_queue,
    sso_enabled, audit_logs, sort_order
) VALUES (
    'price_pro', 'pro', 'pro', 'Pro',
    'For professional developers who need more power and longer retention.',
    1500, 'monthly', FALSE,
    25, 1, 10737418240, 53687091200,
    10, 30, 30, 30,
    TRUE, FALSE, FALSE, FALSE, 1
);

-- Team plan: $30/seat/month, collaborative teams
INSERT INTO billing_plans (
    stripe_price_id, name, slug, display_name, description,
    price_cents, billing_period, is_per_seat,
    max_projects, max_team_members, max_cache_bytes, max_bandwidth_bytes,
    max_concurrent_runs, cache_ttl_days, artifact_retention_days,
    log_retention_days, container_execution, priority_queue,
    sso_enabled, audit_logs, sort_order
) VALUES (
    'price_team', 'team', 'team', 'Team',
    'For teams that need shared caches, higher limits, and collaboration features.',
    3000, 'monthly', TRUE,
    NULL, 50, 53687091200, 268435456000,
    50, 90, 90, 90,
    TRUE, TRUE, FALSE, TRUE, 2
);

-- Enterprise plan: custom pricing, unlimited everything
INSERT INTO billing_plans (
    stripe_price_id, name, slug, display_name, description,
    price_cents, billing_period, is_per_seat,
    max_projects, max_team_members, max_cache_bytes, max_bandwidth_bytes,
    max_concurrent_runs, cache_ttl_days, artifact_retention_days,
    log_retention_days, container_execution, priority_queue,
    sso_enabled, audit_logs, sort_order
) VALUES (
    'price_enterprise', 'enterprise', 'enterprise', 'Enterprise',
    'Custom plans for large organizations with dedicated support and SLA.',
    0, 'monthly', TRUE,
    NULL, NULL, NULL, NULL,
    NULL, NULL, NULL, NULL,
    TRUE, TRUE, TRUE, TRUE, 3
);
