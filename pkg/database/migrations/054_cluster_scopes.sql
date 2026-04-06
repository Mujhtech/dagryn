-- +migrate Up

ALTER TABLE clusters
  ADD COLUMN IF NOT EXISTS slug TEXT,
  ADD COLUMN IF NOT EXISTS scope_type TEXT,
  ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS is_system_default BOOLEAN NOT NULL DEFAULT FALSE;

-- Drop legacy global unique constraint on name so scoped defaults can coexist.
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_name_key;

UPDATE clusters
SET slug = COALESCE(NULLIF(slug, ''), lower(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g'))),
    scope_type = COALESCE(scope_type, 'global')
WHERE slug IS NULL OR slug = '' OR scope_type IS NULL;

ALTER TABLE clusters
  ALTER COLUMN slug SET NOT NULL,
  ALTER COLUMN scope_type SET NOT NULL;

ALTER TABLE clusters
  ADD CONSTRAINT clusters_scope_type_valid
  CHECK (scope_type IN ('global', 'team', 'personal'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_global_slug
  ON clusters(slug)
  WHERE scope_type = 'global';

CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_team_slug
  ON clusters(team_id, slug)
  WHERE scope_type = 'team' AND team_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_personal_slug
  ON clusters(owner_user_id, slug)
  WHERE scope_type = 'personal' AND owner_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_team_name
  ON clusters(team_id, name)
  WHERE scope_type = 'team' AND team_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_personal_name
  ON clusters(owner_user_id, name)
  WHERE scope_type = 'personal' AND owner_user_id IS NOT NULL;

-- Create default team cluster for all existing teams.
INSERT INTO clusters (id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at)
SELECT gen_random_uuid(),
       'default',
       'default',
       'Default team cluster',
       '{"system_default":true}'::jsonb,
       'team',
       t.id,
       NULL,
       TRUE,
       NOW(),
       NOW()
FROM teams t
WHERE NOT EXISTS (
  SELECT 1 FROM clusters c
  WHERE c.scope_type = 'team' AND c.team_id = t.id AND c.slug = 'default'
);

-- Create default personal cluster for users with personal projects (team_id IS NULL).
INSERT INTO clusters (id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at)
SELECT gen_random_uuid(),
       'default',
       'default',
       'Default personal cluster',
       '{"system_default":true}'::jsonb,
       'personal',
       NULL,
       pm.user_id,
       TRUE,
       NOW(),
       NOW()
FROM project_members pm
JOIN projects p ON p.id = pm.project_id
WHERE p.team_id IS NULL
GROUP BY pm.user_id
HAVING NOT EXISTS (
  SELECT 1 FROM clusters c
  WHERE c.scope_type = 'personal' AND c.owner_user_id = pm.user_id AND c.slug = 'default'
);

-- +migrate Down

DROP INDEX IF EXISTS idx_clusters_personal_name;
DROP INDEX IF EXISTS idx_clusters_team_name;
DROP INDEX IF EXISTS idx_clusters_personal_slug;
DROP INDEX IF EXISTS idx_clusters_team_slug;
DROP INDEX IF EXISTS idx_clusters_global_slug;

ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_scope_type_valid;

-- When rolling back to legacy global naming, ensure names are unique before
-- re-adding clusters_name_key. Keep the first row for each name unchanged and
-- rewrite conflicting duplicates deterministically.
WITH ranked AS (
  SELECT id,
         name,
         ROW_NUMBER() OVER (
           PARTITION BY name
           ORDER BY CASE WHEN scope_type = 'global' THEN 0 ELSE 1 END, created_at, id
         ) AS rn
  FROM clusters
), to_rename AS (
  SELECT id,
         name || '__downgrade__' || replace(id::text, '-', '') AS new_name
  FROM ranked
  WHERE rn > 1
)
UPDATE clusters c
SET name = r.new_name
FROM to_rename r
WHERE c.id = r.id;

ALTER TABLE clusters
  DROP COLUMN IF EXISTS is_system_default,
  DROP COLUMN IF EXISTS owner_user_id,
  DROP COLUMN IF EXISTS team_id,
  DROP COLUMN IF EXISTS scope_type,
  DROP COLUMN IF EXISTS slug;

ALTER TABLE clusters
  ADD CONSTRAINT clusters_name_key UNIQUE (name);
