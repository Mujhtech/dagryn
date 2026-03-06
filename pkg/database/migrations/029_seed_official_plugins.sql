-- Seed official plugins into the registry
-- All 16 plugins from plugins/ directory, linked to the 'dagryn' publisher

DO $$
DECLARE
    v_publisher_id UUID;
    v_plugin_id UUID;
BEGIN
    -- Look up the dagryn publisher (seeded by migration 027)
    SELECT id INTO v_publisher_id FROM plugin_publishers WHERE name = 'dagryn';
    IF v_publisher_id IS NULL THEN
        RAISE EXCEPTION 'Publisher "dagryn" not found. Ensure migration 027 has run.';
    END IF;

    -- cache-s3 (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'cache-s3', 'Cache files using Amazon S3', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- deploy-ssh
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'deploy-ssh', 'Deploy files to a remote server via SSH/SCP', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- docker-build (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'docker-build', 'Build and optionally push Docker images', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- eslint (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'eslint', 'Run ESLint for JavaScript/TypeScript linting', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- golangci-lint
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'golangci-lint', 'Run golangci-lint for Go code analysis', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- jest
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'jest', 'Run Jest for JavaScript/TypeScript testing', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- notify-discord
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'notify-discord', 'Send notifications to Discord via webhook', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- prettier
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'prettier', 'Run Prettier for code formatting', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- pytest
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'pytest', 'Run pytest for Python testing', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- setup-go (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'setup-go', 'Install and configure Go', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- setup-node (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'setup-node', 'Install and configure Node.js', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- setup-python
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'setup-python', 'Install and configure Python', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- setup-rust
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'setup-rust', 'Install and configure Rust toolchain', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- slack-notify
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'slack-notify', 'Send notifications to Slack via webhook', 'composite', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- slack-notify-integration (type: integration)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'slack-notify-integration', 'Send Slack notifications on run success or failure', 'integration', 'MIT', '1.0.0', FALSE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

    -- upload-artifact (featured)
    INSERT INTO registry_plugins (publisher_id, name, description, type, license, latest_version, featured, repository_url)
    VALUES (v_publisher_id, 'upload-artifact', 'Upload build artifacts to S3-compatible storage', 'composite', 'MIT', '1.0.0', TRUE, 'https://github.com/mujhtech/dagryn')
    ON CONFLICT (publisher_id, name) DO NOTHING
    RETURNING id INTO v_plugin_id;
    IF v_plugin_id IS NOT NULL THEN
        INSERT INTO plugin_versions (plugin_id, version, manifest_json)
        VALUES (v_plugin_id, '1.0.0', '{}')
        ON CONFLICT (plugin_id, version) DO NOTHING;
    END IF;

END $$;
