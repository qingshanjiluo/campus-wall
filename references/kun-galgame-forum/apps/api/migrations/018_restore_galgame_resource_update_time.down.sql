-- 018 down: drop the restored column + its index. The /galgame list falls back
-- to ORDER BY g.updated (revert the Go sort change alongside this).
BEGIN;

DROP INDEX IF EXISTS idx_galgame_resource_update_time;

ALTER TABLE galgame DROP COLUMN IF EXISTS resource_update_time;

COMMIT;
