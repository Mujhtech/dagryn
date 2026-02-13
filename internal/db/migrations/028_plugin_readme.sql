-- +migrate Up
ALTER TABLE registry_plugins ADD COLUMN readme TEXT;

-- +migrate Down
-- ALTER TABLE registry_plugins DROP COLUMN IF EXISTS readme;
