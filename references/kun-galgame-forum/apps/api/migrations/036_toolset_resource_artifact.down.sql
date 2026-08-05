DROP INDEX IF EXISTS idx_toolset_resource_artifact_uuid;
ALTER TABLE galgame_toolset_resource DROP COLUMN IF EXISTS artifact_uuid;
