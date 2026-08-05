BEGIN;

DROP INDEX IF EXISTS uniq_galgame_activity_wiki_pr_id;

-- PR rows have a NULL wiki_revision_id; drop them before restoring NOT NULL.
DELETE FROM galgame_activity WHERE type = 'GALGAME_PR_CREATION';

ALTER TABLE galgame_activity DROP COLUMN IF EXISTS wiki_pr_id;
ALTER TABLE galgame_activity ALTER COLUMN wiki_revision_id SET NOT NULL;

COMMIT;
