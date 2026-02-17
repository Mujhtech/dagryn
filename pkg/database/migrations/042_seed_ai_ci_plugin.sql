-- Seed the ai-ci integration plugin into the registry.
DO $$
DECLARE
    v_publisher_id UUID;
    v_plugin_id UUID;
BEGIN
    SELECT id INTO v_publisher_id FROM plugin_publishers WHERE name = 'dagryn';
    IF v_publisher_id IS NULL THEN
        RAISE EXCEPTION 'Publisher "dagryn" not found. Ensure migration 027 has run.';
    END IF;

    -- ai-ci (integration, featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'ai-ci', 'AI-powered failure analysis and inline suggestions for CI runs', 'integration', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;
END $$;
