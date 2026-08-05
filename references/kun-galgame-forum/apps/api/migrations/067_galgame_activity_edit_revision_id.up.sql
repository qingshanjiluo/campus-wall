-- 067: galgame_activity gains `edit_revision_id` — the dedup key for edit
-- events that arrive from the CATALOG editing engine's revision feed
-- (GET /api/v1/catalog/edit-revisions/feed) instead of the retiring wiki feed
-- (GET /internal/galgame/revisions/recent).
--
-- Why a NEW column rather than reusing wiki_revision_id: the two ids live in
-- different namespaces. `wiki_revision_id` is galgame_revision.id; the engine's
-- id counts edit_revision rows across every entity family. Writing an engine id
-- into the wiki column would collide with a wiki row that legitimately owns that
-- number — the same class of mistake the wiki_pr_id split avoided in 026.
--
-- Rows keep carrying `wiki_revision_number`: the engine's per-entity `seq` IS
-- the per-galgame revision number the diff endpoint takes as :rev (verified
-- against the old feed — every (galgame_id, revision) pair it ever delivered
-- appears in the engine feed under the same (entity_id, seq)), so the activity
-- card builds its diff URL exactly as before.
--
-- NULL-able + a plain unique index (Postgres treats NULLs as distinct), so the
-- three writers coexist: wiki-fed GALGAME_EDIT rows (historic), engine-fed
-- GALGAME_EDIT rows (from this wave on), and GALGAME_PR_CREATION rows.
BEGIN;

ALTER TABLE galgame_activity
  ADD COLUMN IF NOT EXISTS edit_revision_id BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_galgame_activity_edit_revision_id
  ON galgame_activity (edit_revision_id);

COMMIT;
