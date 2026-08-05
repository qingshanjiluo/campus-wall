-- 016: Indexes for the home/activity hot path.
--
-- WHY: production CPU was pegged (load 15 on 4 cores, Postgres ~78%) by two
-- seq-scan storms that run on every home/detail render:
--
--   1. galgame_resource had NO index on galgame_id (only the id PK), so the
--      home galgame cards (`WHERE galgame_id IN (...~24 ids)`) and the detail
--      page (`WHERE galgame_id = $1`) seq-scanned all ~22.7k rows — ~1.76s
--      each (home_repo.go:83 SLOW SQL), billions of tuples cumulatively.
--
--   2. The "最新动态" feed (activity_repo.FetchTimeline) UNION-ALLs ~17 source
--      tables with NO per-subquery limit and NO usable index, so each call
--      fully scanned every source table — message (199k rows) alone read 5.7B
--      tuples cumulatively. The query is rewritten to push
--      `ORDER BY created DESC LIMIT N` into each subquery; these btree(created)
--      indexes make each branch an index-backed top-N instead of a full scan.
--
-- All indexes are IF NOT EXISTS so this is a no-op on the production DB, where
-- idx_galgame_resource_galgame_id and idx_message_type_created were already
-- created live via CREATE INDEX CONCURRENTLY (non-blocking emergency relief).

BEGIN;

-- (1) galgame_resource lookups by galgame_id (home cards + detail page).
CREATE INDEX IF NOT EXISTS idx_galgame_resource_galgame_id
  ON galgame_resource (galgame_id);

-- (2) Activity-feed per-subquery `ORDER BY created DESC LIMIT N` pushdown.
-- message is filtered by `type` AND ordered by created → composite index.
CREATE INDEX IF NOT EXISTS idx_message_type_created
  ON message (type, created DESC);

-- created(DESC) on every other activity source table so each UNION branch is
-- an index top-N. btree scans backward, so plain (created DESC) also serves
-- any ASC order; kept DESC to match the feed's ORDER BY created DESC.
CREATE INDEX IF NOT EXISTS idx_topic_created                   ON topic (created DESC);
CREATE INDEX IF NOT EXISTS idx_topic_reply_created             ON topic_reply (created DESC);
CREATE INDEX IF NOT EXISTS idx_topic_comment_created           ON topic_comment (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_created                 ON galgame (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_comment_created         ON galgame_comment (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_resource_created        ON galgame_resource (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_rating_created          ON galgame_rating (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_rating_comment_created  ON galgame_rating_comment (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_website_created         ON galgame_website (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_website_comment_created ON galgame_website_comment (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_toolset_created         ON galgame_toolset (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_toolset_resource_created ON galgame_toolset_resource (created DESC);
CREATE INDEX IF NOT EXISTS idx_galgame_toolset_comment_created ON galgame_toolset_comment (created DESC);
CREATE INDEX IF NOT EXISTS idx_todo_created                    ON todo (created DESC);
CREATE INDEX IF NOT EXISTS idx_update_log_created              ON update_log (created DESC);

COMMIT;
