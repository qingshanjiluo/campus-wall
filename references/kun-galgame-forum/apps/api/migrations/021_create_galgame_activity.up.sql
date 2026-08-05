-- 021: galgame_activity — local mirror of wiki "edit landed" events so the
-- forum activity timeline can show "user X 编辑了 galgame Y" WITHOUT querying the
-- remote wiki at render time.
--
-- Why a local mirror (not a live remote merge): FetchTimeline is a single
-- PostgreSQL UNION ALL that pushes `ORDER BY created DESC LIMIT N` into each
-- source (index-backed top-N). A remote source can't join that SQL and would
-- break pagination + COUNT(*) + tie the timeline's availability to the wiki.
-- So the wiki-revision sync cron pulls GET /galgame/revisions/recent (the wiki's
-- merged-revision feed) and upserts rows here; the timeline reads this table as
-- one more UNION source.
--
-- The wiki owns galgame edits (galgame_revision, action='merged'); user_id here
-- is the real editor (revision.UserID), so unlike GALGAME_CREATION (user_id=0,
-- actor filled from the wiki brief) this source carries a correct local actor.
--
-- wiki_revision_id (= galgame_revision.id) is the dedup key → the cron is
-- idempotent on re-runs / retries.
BEGIN;

CREATE TABLE IF NOT EXISTS galgame_activity (
  id               SERIAL PRIMARY KEY,
  wiki_revision_id BIGINT      NOT NULL UNIQUE,
  galgame_id       INTEGER     NOT NULL,
  user_id          INTEGER     NOT NULL,
  type             TEXT        NOT NULL DEFAULT 'GALGAME_EDIT',
  created          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The timeline pushes `ORDER BY created DESC LIMIT N` into each UNION branch;
-- this index makes that an index-backed top-N (matches idx_<table>_created on
-- the other sources, migration 016).
CREATE INDEX IF NOT EXISTS idx_galgame_activity_created ON galgame_activity (created DESC);

COMMIT;
