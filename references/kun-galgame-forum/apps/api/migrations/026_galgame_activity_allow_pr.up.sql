-- 026: galgame_activity also records "user submitted an update request (PR)"
-- events, restoring the GALGAME_PR_CREATION activity-timeline entry that was
-- dropped when the galgame_pr table moved to the wiki service (it can no longer
-- be queried locally; see internal/activity/repository/activity_repo.go). The
-- forum re-mirrors PR submissions here from its SubmitPR proxy.
--
-- A row is EITHER a merged edit (dedup key wiki_revision_id) OR a submitted PR
-- (dedup key wiki_pr_id) — never both. Make wiki_revision_id nullable and add a
-- nullable wiki_pr_id; a UNIQUE index over a nullable column treats NULLs as
-- distinct (multiple NULLs allowed), so each key only dedups its own row type.
BEGIN;

ALTER TABLE galgame_activity ALTER COLUMN wiki_revision_id DROP NOT NULL;
ALTER TABLE galgame_activity ADD COLUMN IF NOT EXISTS wiki_pr_id BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_galgame_activity_wiki_pr_id
  ON galgame_activity (wiki_pr_id);

COMMIT;
