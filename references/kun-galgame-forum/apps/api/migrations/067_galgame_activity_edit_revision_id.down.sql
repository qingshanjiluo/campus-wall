BEGIN;

DROP INDEX IF EXISTS uniq_galgame_activity_edit_revision_id;

-- Engine-fed edit rows carry a NULL wiki_revision_id; without the column that
-- identifies them they would be undeletable duplicates if the up-migration ran
-- again, so drop them with the column.
DELETE FROM galgame_activity WHERE edit_revision_id IS NOT NULL;

ALTER TABLE galgame_activity DROP COLUMN IF EXISTS edit_revision_id;

COMMIT;
